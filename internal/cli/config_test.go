package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/arturoburigo/gzlab/internal/config"
)

func TestResolveEditorCommand(t *testing.T) {
	tests := []struct {
		name      string
		uiEditor  string
		envEditor string
		envUnset  bool
		want      []string
	}{
		{
			name:     "ui.editor takes precedence",
			uiEditor: "code --wait",
			want:     []string{"code", "--wait"},
		},
		{
			name:      "falls back to $EDITOR when ui.editor is unset",
			uiEditor:  "",
			envEditor: "nano",
			want:      []string{"nano"},
		},
		{
			name:      "whitespace-only ui.editor falls back to $EDITOR instead of panicking",
			uiEditor:  "   ",
			envEditor: "nano",
			want:      []string{"nano"},
		},
		{
			name:      "whitespace-only $EDITOR falls back to vi instead of panicking",
			uiEditor:  "",
			envEditor: "   ",
			want:      []string{"vi"},
		},
		{
			name:     "nothing configured falls back to vi",
			uiEditor: "",
			envUnset: true,
			want:     []string{"vi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envUnset {
				t.Setenv("EDITOR", "")
			} else {
				t.Setenv("EDITOR", tt.envEditor)
			}

			cfg := &config.Config{UI: config.UIConfig{Editor: tt.uiEditor}}
			got := resolveEditorCommand(cfg)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveEditorCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveEditorCommand_ExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}

	cfg := &config.Config{UI: config.UIConfig{Editor: "~/bin/myeditor --wait"}}
	want := []string{filepath.Join(home, "bin", "myeditor"), "--wait"}
	if got := resolveEditorCommand(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("resolveEditorCommand() = %v, want %v", got, want)
	}
}
