package history

import (
	"os"
	"path/filepath"
)

// DefaultPath returns ~/.config/gitlab-tui/history.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gitlab-tui", "history.json"), nil
}
