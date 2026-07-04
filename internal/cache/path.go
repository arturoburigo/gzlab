package cache

import (
	"os"
	"path/filepath"
)

// DefaultPath returns the root directory for gzlab's local cache.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "gitlab-tui"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "gitlab-tui"), nil
}
