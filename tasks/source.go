package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/agios-sh/agios/config"
)

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

type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Assignee  string    `json:"assignee,omitempty"`
	BlockedBy []string  `json:"blocked_by,omitempty"`
	Body      string    `json:"body,omitempty"`
	Source    string    `json:"source"`
	URL       string    `json:"url,omitempty"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Comments  []Comment `json:"comments,omitempty"`
}

type TaskSummary struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Assignee  string   `json:"assignee,omitempty"`
	BlockedBy []string `json:"blocked_by,omitempty"`
	Updated   string   `json:"updated"`
	Source    string   `json:"source"`
}

type Comment struct {
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Timestamp time.Time `json:"timestamp"`
}

type ListOptions struct {
	Status   string
	Assignee string
}

type CreateOptions struct {
	Title     string
	Body      string
	Status    string
	Assignee  string
	BlockedBy []string
}

// Nil fields are not changed.
type UpdateOptions struct {
	Title     *string
	Body      *string
	Status    *string
	Assignee  *string
	BlockedBy *[]string
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
	default:
		return nil, fmt.Errorf("unknown source type: %q", s.Type)
	}
}

// autoDetectSources returns sources based on what's available in the environment.
func autoDetectSources(projectRoot string) []Source {
	return []Source{newLocalSource("local", "", projectRoot)}
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

	// Fall back to first source
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
