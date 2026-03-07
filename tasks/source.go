package tasks

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/agios-sh/agios/config"
)

// Source is the interface that all task sources must implement.
type Source interface {
	Name() string
	Type() string
	List(opts ListOptions) ([]TaskSummary, error)
	Get(id string) (*Task, error)
	Create(opts CreateOptions) (*Task, error)
	Update(id string, opts UpdateOptions) (*Task, error)
	Comment(id string, body string) (*Task, error)
	RecentActivity(since time.Time) ([]Task, error)
	Summary() (map[string]int, error)
}

// Task represents a full task with all details.
type Task struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Status   string    `json:"status"`
	Assignee string    `json:"assignee,omitempty"`
	Body     string    `json:"body,omitempty"`
	Source   string    `json:"source"`
	URL      string    `json:"url,omitempty"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
	Comments []Comment `json:"comments,omitempty"`
}

// TaskSummary is a compact representation for list views.
type TaskSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Assignee string `json:"assignee,omitempty"`
	Updated  string `json:"updated"`
	Source   string `json:"source"`
}

// Comment represents a comment on a task.
type Comment struct {
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Timestamp time.Time `json:"timestamp"`
}

// ListOptions for filtering task lists.
type ListOptions struct {
	Status   string
	Assignee string
}

// CreateOptions for creating a new task.
type CreateOptions struct {
	Title    string
	Body     string
	Status   string
	Assignee string
}

// UpdateOptions for updating a task. Nil fields are not changed.
type UpdateOptions struct {
	Title    *string
	Body     *string
	Status   *string
	Assignee *string
}

// resolveSources returns all available sources based on config or auto-detection.
func resolveSources(cfg *config.Config, projectRoot string) ([]Source, error) {
	if cfg.Tasks != nil && len(cfg.Tasks.Sources) > 0 {
		var sources []Source
		for _, s := range cfg.Tasks.Sources {
			src, err := instantiateSource(s, projectRoot)
			if err != nil {
				return nil, fmt.Errorf("source %q: %w", s.Name, err)
			}
			sources = append(sources, src)
		}
		return sources, nil
	}

	// Auto-detect
	return autoDetectSources(projectRoot), nil
}

// instantiateSource creates a Source from a config entry.
func instantiateSource(s config.TaskSource, projectRoot string) (Source, error) {
	switch s.Type {
	case "local":
		return newLocalSource(s.Name, s.Dir, projectRoot), nil
	case "github":
		return newGitHubSource(s.Name, s.Repo, projectRoot)
	default:
		return nil, fmt.Errorf("unknown source type: %q", s.Type)
	}
}

// autoDetectSources returns sources based on what's available in the environment.
func autoDetectSources(projectRoot string) []Source {
	sources := []Source{newLocalSource("local", "", projectRoot)}

	if ghAvailable(projectRoot) {
		if src, err := newGitHubSource("github", "", projectRoot); err == nil {
			sources = append(sources, src)
		}
	}

	return sources
}

// ghAvailable checks if the GitHub CLI is installed, a GitHub remote exists,
// and the user is authenticated.
func ghAvailable(projectRoot string) bool {
	// Check gh CLI exists
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}

	// Check for GitHub remote
	repo := detectGitHubRepo(projectRoot)
	if repo == "" {
		return false
	}

	// Check auth
	cmd := exec.Command("gh", "auth", "status")
	cmd.Dir = projectRoot
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// detectGitHubRepo parses `git remote -v` output for a github.com URL.
// Returns "owner/repo" or empty string.
func detectGitHubRepo(projectRoot string) string {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return parseGitHubRemote(string(out))
}

// parseGitHubRemote extracts "owner/repo" from git remote -v output.
// Prefers the "origin" remote.
func parseGitHubRemote(output string) string {
	var fallback string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		remoteName := parts[0]
		url := parts[1]

		repo := extractGitHubRepo(url)
		if repo == "" {
			continue
		}

		if remoteName == "origin" {
			return repo
		}
		if fallback == "" {
			fallback = repo
		}
	}
	return fallback
}

// extractGitHubRepo extracts "owner/repo" from a GitHub URL.
// Supports SSH aliases like git@github.com-alias:owner/repo.git
func extractGitHubRepo(url string) string {
	// SSH: git@github.com:owner/repo.git or git@github.com-alias:owner/repo.git
	if strings.HasPrefix(url, "git@github.com") {
		// Find the colon that separates host from path
		idx := strings.Index(url, ":")
		if idx >= 0 {
			repo := url[idx+1:]
			repo = strings.TrimSuffix(repo, ".git")
			if strings.Contains(repo, "/") {
				return repo
			}
		}
	}

	// HTTPS: https://github.com/owner/repo.git
	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if strings.HasPrefix(url, prefix) {
			repo := strings.TrimPrefix(url, prefix)
			repo = strings.TrimSuffix(repo, ".git")
			// Remove trailing slashes or extra path segments
			parts := strings.SplitN(repo, "/", 3)
			if len(parts) >= 2 {
				return parts[0] + "/" + parts[1]
			}
		}
	}

	return ""
}

// resolveDefault picks the default source from the list.
func resolveDefault(sources []Source, configDefault string) Source {
	if len(sources) == 0 {
		return nil
	}

	// Explicit source name
	if configDefault != "" && configDefault != "auto" {
		for _, s := range sources {
			if s.Name() == configDefault {
				return s
			}
		}
	}

	// Auto: pick first non-local source
	for _, s := range sources {
		if s.Type() != "local" {
			return s
		}
	}

	// Fall back to first source (local)
	return sources[0]
}

// resolveSource parses --source from args, returns the selected source and remaining args.
func resolveSource(args []string, sources []Source, defaultSource Source) (Source, []string, error) {
	var sourceName string
	var remaining []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--source" && i+1 < len(args) {
			sourceName = args[i+1]
			i++ // skip value
		} else {
			remaining = append(remaining, args[i])
		}
	}

	if sourceName == "" {
		return defaultSource, remaining, nil
	}

	for _, s := range sources {
		if s.Name() == sourceName {
			return s, remaining, nil
		}
	}

	var names []string
	for _, s := range sources {
		names = append(names, s.Name())
	}
	return nil, remaining, fmt.Errorf("source %q not found (available: %s)", sourceName, strings.Join(names, ", "))
}
