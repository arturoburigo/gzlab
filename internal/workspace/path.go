package workspace

import (
	"os"
	"path/filepath"
)

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gitlab-tui", "workspaces.json"), nil
}
