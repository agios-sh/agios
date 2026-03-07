// Package config handles loading and saving agios.yaml configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const FileName = "agios.yaml"

// Config represents the agios.yaml configuration file.
type Config struct {
	Apps  []string     `yaml:"apps"`
	Tasks *TasksConfig `yaml:"tasks,omitempty"`

	// Path is the absolute path to the config file. Not serialized.
	Path string `yaml:"-"`
}

// TasksConfig defines the tasks configuration section in agios.yaml.
type TasksConfig struct {
	Default string       `yaml:"default,omitempty"`
	Sources []TaskSource `yaml:"sources,omitempty"`
}

// TaskSource defines a single task source (e.g., local files, GitHub Issues).
type TaskSource struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`           // "local" or "github"
	Repo string `yaml:"repo,omitempty"` // github only: "owner/repo", auto-detected if empty
	Dir  string `yaml:"dir,omitempty"`  // local only: path relative to project root
}

// Load walks up from startDir to find the nearest agios.yaml,
// similar to how git finds .git/. Returns the parsed config or an error.
func Load(startDir string) (*Config, error) {
	path, err := Find(startDir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg.Path = path
	return &cfg, nil
}

// Find walks up from startDir to locate the nearest agios.yaml.
// Returns the absolute path to the config file or an error if not found.
func Find(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	for {
		candidate := filepath.Join(dir, FileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding config
			return "", fmt.Errorf("%s not found (walked up from %s)", FileName, startDir)
		}
		dir = parent
	}
}

// Save writes the config back to its Path.
func (c *Config) Save() error {
	if c.Path == "" {
		return fmt.Errorf("config has no path set")
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(c.Path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", c.Path, err)
	}
	return nil
}

// HasApp checks whether the given app name is in the Apps list.
func (c *Config) HasApp(name string) bool {
	for _, a := range c.Apps {
		if a == name {
			return true
		}
	}
	return false
}
