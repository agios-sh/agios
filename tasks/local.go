package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// localSource implements Source using markdown files with YAML frontmatter.
type localSource struct {
	name        string
	dir         string // absolute path to tasks directory
	projectRoot string
}

func newLocalSource(name, dir, projectRoot string) *localSource {
	if dir == "" {
		dir = ".agios/tasks"
	}
	absDir := dir
	if !filepath.IsAbs(dir) {
		absDir = filepath.Join(projectRoot, dir)
	}
	return &localSource{name: name, dir: absDir, projectRoot: projectRoot}
}

func (s *localSource) Name() string { return s.name }
func (s *localSource) Type() string { return "local" }

func (s *localSource) List(opts ListOptions) ([]TaskSummary, error) {
	tasks, err := s.readAll()
	if err != nil {
		return nil, err
	}

	// For --status ready, build a set of open task IDs for blocking checks.
	filterReady := strings.EqualFold(opts.Status, "ready")
	var openIDs map[string]bool
	if filterReady {
		openIDs = make(map[string]bool)
		for _, t := range tasks {
			if strings.EqualFold(t.Status, "open") {
				openIDs[t.ID] = true
			}
		}
	}

	var result []TaskSummary
	for _, t := range tasks {
		if filterReady {
			// Ready = open and not blocked by any open task.
			if !strings.EqualFold(t.Status, "open") {
				continue
			}
			blocked := false
			for _, bid := range t.BlockedBy {
				if openIDs[bid] {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
		} else {
			if opts.Status != "" && !strings.EqualFold(t.Status, opts.Status) {
				continue
			}
		}
		if opts.Assignee != "" && !strings.EqualFold(t.Assignee, opts.Assignee) {
			continue
		}
		result = append(result, TaskSummary{
			ID:        t.ID,
			Title:     t.Title,
			Status:    t.Status,
			Assignee:  t.Assignee,
			BlockedBy: t.BlockedBy,
			Updated:   t.Updated.Format(time.RFC3339),
			Source:    s.name,
		})
	}
	return result, nil
}

func (s *localSource) Get(id string) (*Task, error) {
	path := s.taskPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %s not found", id)
		}
		return nil, err
	}
	return s.parseTask(data, id)
}

func (s *localSource) Create(opts CreateOptions) (*Task, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return nil, fmt.Errorf("creating tasks directory: %w", err)
	}

	id := s.nextID()
	now := time.Now()
	status := opts.Status
	if status == "" {
		status = "open"
	}

	t := &Task{
		ID:        id,
		Title:     opts.Title,
		Status:    status,
		Assignee:  opts.Assignee,
		BlockedBy: opts.BlockedBy,
		Body:      opts.Body,
		Source:    s.name,
		Created:   now,
		Updated:   now,
	}

	if err := s.writeTask(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *localSource) Update(id string, opts UpdateOptions) (*Task, error) {
	t, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	if opts.Title != nil {
		t.Title = *opts.Title
	}
	if opts.Body != nil {
		t.Body = *opts.Body
	}
	if opts.Status != nil {
		t.Status = *opts.Status
	}
	if opts.Assignee != nil {
		t.Assignee = *opts.Assignee
	}
	if opts.BlockedBy != nil {
		t.BlockedBy = *opts.BlockedBy
	}
	t.Updated = time.Now()

	if err := s.writeTask(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *localSource) Comment(id string, body string) (*Task, error) {
	t, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	t.Comments = append(t.Comments, Comment{
		Author:    "user",
		Body:      body,
		Timestamp: time.Now(),
	})
	t.Updated = time.Now()

	if err := s.writeTask(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *localSource) RecentActivity(since time.Time) ([]Task, error) {
	tasks, err := s.readAll()
	if err != nil {
		return nil, err
	}

	var recent []Task
	for _, t := range tasks {
		if t.Updated.After(since) {
			recent = append(recent, t)
		}
	}

	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Updated.After(recent[j].Updated)
	})

	if len(recent) > 5 {
		recent = recent[:5]
	}
	return recent, nil
}

func (s *localSource) Summary() (map[string]int, error) {
	tasks, err := s.readAll()
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, t := range tasks {
		counts[t.Status]++
	}
	return counts, nil
}

// readAll reads all task files from the directory.
func (s *localSource) readAll() ([]Task, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tasks []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		t, err := s.parseTask(data, id)
		if err != nil {
			continue
		}
		tasks = append(tasks, *t)
	}
	return tasks, nil
}

// taskPath returns the file path for a given task ID.
func (s *localSource) taskPath(id string) string {
	return filepath.Join(s.dir, id+".md")
}

// nextID scans existing files and returns the next sequential integer ID.
func (s *localSource) nextID() string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return "1"
	}

	maxID := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if n, err := strconv.Atoi(name); err == nil && n > maxID {
			maxID = n
		}
	}
	return strconv.Itoa(maxID + 1)
}

// parseTask parses a markdown file with YAML frontmatter into a Task.
func (s *localSource) parseTask(data []byte, id string) (*Task, error) {
	content := string(data)

	t := &Task{
		ID:     id,
		Source: s.name,
	}

	// Parse frontmatter (between --- lines)
	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[4:], "\n---\n")
		if end >= 0 {
			frontmatter := content[4 : 4+end]
			content = content[4+end+5:] // skip past closing ---\n

			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				key, val, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				key = strings.TrimSpace(key)
				val = strings.TrimSpace(val)

				switch key {
				case "title":
					t.Title = val
				case "status":
					t.Status = val
				case "assignee":
					t.Assignee = val
				case "blocked_by":
					if val != "" {
						for _, b := range strings.Split(val, ",") {
							b = strings.TrimSpace(b)
							if b != "" {
								t.BlockedBy = append(t.BlockedBy, b)
							}
						}
					}
				case "created":
					if ts, err := time.Parse(time.RFC3339, val); err == nil {
						t.Created = ts
					}
				case "updated":
					if ts, err := time.Parse(time.RFC3339, val); err == nil {
						t.Updated = ts
					}
				}
			}
		}
	}

	// Parse body and comments
	commentIdx := strings.Index(content, "\n## Comments\n")
	if commentIdx >= 0 {
		t.Body = strings.TrimSpace(content[:commentIdx])
		t.Comments = parseComments(content[commentIdx+len("\n## Comments\n"):])
	} else {
		t.Body = strings.TrimSpace(content)
	}

	return t, nil
}

// parseComments parses the comments section of a task file.
func parseComments(section string) []Comment {
	var comments []Comment
	parts := strings.Split(section, "\n### ")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// First line is the timestamp, rest is body
		lines := strings.SplitN(part, "\n", 2)
		if len(lines) == 0 {
			continue
		}

		c := Comment{Author: "user"}
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(lines[0])); err == nil {
			c.Timestamp = ts
		}
		if len(lines) > 1 {
			c.Body = strings.TrimSpace(lines[1])
		}
		comments = append(comments, c)
	}
	return comments
}

// writeTask writes a task to its markdown file.
func (s *localSource) writeTask(t *Task) error {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString("title: " + t.Title + "\n")
	b.WriteString("status: " + t.Status + "\n")
	if t.Assignee != "" {
		b.WriteString("assignee: " + t.Assignee + "\n")
	}
	if len(t.BlockedBy) > 0 {
		b.WriteString("blocked_by: " + strings.Join(t.BlockedBy, ", ") + "\n")
	}
	b.WriteString("created: " + t.Created.Format(time.RFC3339) + "\n")
	b.WriteString("updated: " + t.Updated.Format(time.RFC3339) + "\n")
	b.WriteString("---\n")

	if t.Body != "" {
		b.WriteString(t.Body + "\n")
	}

	if len(t.Comments) > 0 {
		b.WriteString("\n## Comments\n")
		for _, c := range t.Comments {
			b.WriteString("\n### " + c.Timestamp.Format(time.RFC3339) + "\n")
			b.WriteString(c.Body + "\n")
		}
	}

	return os.WriteFile(s.taskPath(t.ID), []byte(b.String()), 0644)
}
