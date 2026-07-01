package config

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath returns ~/.config/gitlab-tui/config.yaml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gitlab-tui", "config.yaml"), nil
}

// ExpandHome expands a leading "~" (or "~/...") to the user's home directory.
// Paths without a leading "~" are returned unchanged.
func ExpandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}
