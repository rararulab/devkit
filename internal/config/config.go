// Package config reads .devkit.toml from the repository root.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Config represents the .devkit.toml configuration file.
type Config struct {
	AgentMD AgentMDConfig `toml:"agent-md"`
	Deps    DepsConfig    `toml:"deps"`
}

// AgentMDConfig controls the check-agent-md command.
type AgentMDConfig struct {
	CratesDir string `toml:"crates_dir"`
}

// DepsConfig controls the check-deps command.
type DepsConfig struct {
	CratesDir string              `toml:"crates_dir"`
	Layers    map[string][]string `toml:"layers"`
}

// Load reads .devkit.toml from the current directory or any parent,
// walking up until it finds the file or reaches the filesystem root.
func Load() (*Config, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	return loadFrom(dir)
}

// loadFrom walks up from startDir looking for .devkit.toml.
func loadFrom(startDir string) (*Config, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".devkit.toml")
		data, err := os.ReadFile(candidate)
		if err == nil {
			var cfg Config
			if err := toml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", candidate, err)
			}
			return &cfg, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no .devkit.toml found (searched from %s to filesystem root)", startDir)
		}
		dir = parent
	}
}
