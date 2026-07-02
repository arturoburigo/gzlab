package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned by Load when the config file doesn't exist yet.
var ErrNotFound = fmt.Errorf("config file not found; run `gitlab-tui auth login` to create one")

// Load reads and parses the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reading config at %s: %w", path, err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config at %s is not valid YAML: %w", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config at %s is invalid: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config to path, creating parent directories as needed.
// The file is written with 0600 permissions since it may contain a
// fallback token.
func Save(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("refusing to save invalid config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	// MkdirAll only applies the mode to directories it creates, so a
	// pre-existing directory with looser permissions (e.g. from a dotfiles
	// sync) wouldn't otherwise be tightened.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("securing config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config to %s: %w", path, err)
	}
	// Same reasoning as above: WriteFile only applies the mode when creating
	// the file, not when overwriting an existing one with looser permissions.
	// The config may contain a fallback token, so this isn't optional.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("securing config file: %w", err)
	}
	return nil
}
