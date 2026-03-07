package tasks

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// githubSource implements Source using GitHub Issues via the gh CLI.
type githubSource struct {
	name        string
	repo        string // "owner/repo"
	projectRoot string
}

func newGitHubSource(name, repo, projectRoot string) (*githubSource, error) {
	if repo == "" {
		repo = detectGitHubRepo(projectRoot)
		if repo == "" {
			return nil, fmt.Errorf("could not detect GitHub repo from git remote")
		}
	}
	return &githubSource{name: name, repo: repo, projectRoot: projectRoot}, nil
}

func (s *githubSource) Name() string { return s.name }
func (s *githubSource) Type() string { return "github" }

func (s *githubSource) List(opts ListOptions) ([]TaskSummary, error) {
	args := []string{"issue", "list", "-R", s.repo, "--json", "number,title,state,assignees,updatedAt", "--limit", "100"}
	if opts.Status != "" {
		state := mapStatusToGH(opts.Status)
		if state != "" {
			args = append(args, "--state", state)
		}
	}
	if opts.Assignee != "" {
		args = append(args, "--assignee", opts.Assignee)
	}

	out, err := s.gh(args...)
	if err != nil {
		return nil, err
	}

	var issues []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		UpdatedAt string `json:"updatedAt"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	var result []TaskSummary
	for _, issue := range issues {
		assignee := ""
		if len(issue.Assignees) > 0 {
			assignee = issue.Assignees[0].Login
		}
		result = append(result, TaskSummary{
			ID:       strconv.Itoa(issue.Number),
			Title:    issue.Title,
			Status:   mapGHToStatus(issue.State),
			Assignee: assignee,
			Updated:  issue.UpdatedAt,
			Source:   s.name,
		})
	}
	return result, nil
}

func (s *githubSource) Get(id string) (*Task, error) {
	out, err := s.gh("issue", "view", id, "-R", s.repo, "--json", "number,title,state,body,assignees,createdAt,updatedAt,comments,url")
	if err != nil {
		return nil, err
	}

	var issue struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		Body      string `json:"body"`
		URL       string `json:"url"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		Comments []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body      string `json:"body"`
			CreatedAt string `json:"createdAt"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	assignee := ""
	if len(issue.Assignees) > 0 {
		assignee = issue.Assignees[0].Login
	}

	created, _ := time.Parse(time.RFC3339, issue.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, issue.UpdatedAt)

	t := &Task{
		ID:       strconv.Itoa(issue.Number),
		Title:    issue.Title,
		Status:   mapGHToStatus(issue.State),
		Assignee: assignee,
		Body:     issue.Body,
		Source:   s.name,
		URL:      issue.URL,
		Created:  created,
		Updated:  updated,
	}

	for _, c := range issue.Comments {
		ts, _ := time.Parse(time.RFC3339, c.CreatedAt)
		t.Comments = append(t.Comments, Comment{
			Author:    c.Author.Login,
			Body:      c.Body,
			Timestamp: ts,
		})
	}

	return t, nil
}

func (s *githubSource) Create(opts CreateOptions) (*Task, error) {
	args := []string{"issue", "create", "-R", s.repo, "--title", opts.Title, "--body", opts.Body}
	if opts.Assignee != "" {
		args = append(args, "--assignee", opts.Assignee)
	}

	out, err := s.gh(args...)
	if err != nil {
		return nil, err
	}

	// gh issue create outputs the URL of the created issue
	url := strings.TrimSpace(string(out))

	// Extract issue number from URL (last path segment)
	parts := strings.Split(url, "/")
	id := parts[len(parts)-1]

	// Fetch the full issue to return
	return s.Get(id)
}

func (s *githubSource) Update(id string, opts UpdateOptions) (*Task, error) {
	// Edit fields
	editArgs := []string{"issue", "edit", id, "-R", s.repo}
	needEdit := false
	if opts.Title != nil {
		editArgs = append(editArgs, "--title", *opts.Title)
		needEdit = true
	}
	if opts.Body != nil {
		editArgs = append(editArgs, "--body", *opts.Body)
		needEdit = true
	}
	if opts.Assignee != nil {
		editArgs = append(editArgs, "--add-assignee", *opts.Assignee)
		needEdit = true
	}
	if needEdit {
		if _, err := s.gh(editArgs...); err != nil {
			return nil, err
		}
	}

	// Handle status changes via close/reopen
	if opts.Status != nil {
		switch *opts.Status {
		case "closed":
			if _, err := s.gh("issue", "close", id, "-R", s.repo); err != nil {
				return nil, err
			}
		case "open":
			if _, err := s.gh("issue", "reopen", id, "-R", s.repo); err != nil {
				return nil, err
			}
		}
	}

	return s.Get(id)
}

func (s *githubSource) Comment(id string, body string) (*Task, error) {
	if _, err := s.gh("issue", "comment", id, "-R", s.repo, "--body", body); err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *githubSource) RecentActivity(since time.Time) ([]Task, error) {
	// Use --state all so recently-closed issues are included in the dock view.
	out, err := s.gh("issue", "list", "-R", s.repo, "--state", "all",
		"--json", "number,title,state,assignees,updatedAt", "--limit", "10")
	if err != nil {
		return nil, err
	}

	var issues []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		UpdatedAt string `json:"updatedAt"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	var recent []Task
	for _, issue := range issues {
		t, _ := time.Parse(time.RFC3339, issue.UpdatedAt)
		if !t.After(since) {
			continue
		}
		assignee := ""
		if len(issue.Assignees) > 0 {
			assignee = issue.Assignees[0].Login
		}
		recent = append(recent, Task{
			ID:       strconv.Itoa(issue.Number),
			Title:    issue.Title,
			Status:   mapGHToStatus(issue.State),
			Assignee: assignee,
			Source:   s.name,
			Updated:  t,
		})
		if len(recent) >= 5 {
			break
		}
	}
	return recent, nil
}

func (s *githubSource) Summary() (map[string]int, error) {
	counts := map[string]int{}
	for _, state := range []string{"open", "closed"} {
		out, err := s.gh("issue", "list", "-R", s.repo, "--state", state, "--json", "number", "--limit", "1000")
		if err != nil {
			return nil, err
		}
		var issues []struct{ Number int }
		if err := json.Unmarshal(out, &issues); err != nil {
			return nil, err
		}
		counts[state] = len(issues)
	}
	return counts, nil
}

// gh runs a gh CLI command and returns stdout.
// Errors are sanitized to only expose actionable information, never raw CLI output.
func (s *githubSource) gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = s.projectRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, ghError(args, err)
	}
	return out, nil
}

// ghError translates a gh CLI error into a user-facing message.
// Raw stderr (which contains gh's full usage/flags output) is never exposed.
func ghError(args []string, err error) error {
	op := "operation"
	if len(args) >= 2 {
		op = args[0] + " " + args[1]
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		// Extract just the first line — the actual error message.
		// Subsequent lines are typically usage text and flag listings.
		if first, _, ok := strings.Cut(stderr, "\n"); ok {
			stderr = first
		}
		return fmt.Errorf("GitHub %s failed: %s", op, stderr)
	}
	return fmt.Errorf("GitHub %s failed: could not run gh CLI", op)
}

// mapStatusToGH maps our status to GitHub issue state.
func mapStatusToGH(status string) string {
	switch strings.ToLower(status) {
	case "open", "in_progress":
		return "open"
	case "closed":
		return "closed"
	default:
		return ""
	}
}

// mapGHToStatus maps GitHub issue state to our status.
func mapGHToStatus(state string) string {
	switch strings.ToUpper(state) {
	case "OPEN":
		return "open"
	case "CLOSED":
		return "closed"
	default:
		return strings.ToLower(state)
	}
}
