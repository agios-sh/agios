// Package config handles loading and saving agios.yaml configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const FileName = "agios.yaml"

type Config struct {
	Apps  []string     `yaml:"apps"`
	Tasks *TasksConfig `yaml:"tasks,omitempty"`
	Path  string       `yaml:"-"` // absolute path to config file
}

type TasksConfig struct {
	Default string       `yaml:"default,omitempty"`
	Sources []TaskSource `yaml:"sources,omitempty"`
}

type TaskSource struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`          // "local"
	Dir  string `yaml:"dir,omitempty"` // path relative to project root
}

// Load finds and parses the nearest agios.yaml, walking up from startDir.
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

// Find returns the absolute path to the nearest agios.yaml above startDir.
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
			return "", fmt.Errorf("%s not found (walked up from %s)", FileName, startDir)
		}
		dir = parent
	}
}

func (c *Config) Save() error {
	if c.Path == "" {
		return fmt.Errorf("config has no path set")
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(c.Path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", c.Path, err)
	}
	return nil
}

func (c *Config) HasApp(name string) bool {
	for _, a := range c.Apps {
		if a == name {
			return true
		}
	}
	return false
}
