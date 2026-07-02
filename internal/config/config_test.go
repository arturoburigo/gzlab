package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.DefaultProfile = "empresa"
	cfg.Profiles["empresa"] = Profile{
		Host:     "https://gitlab.services.betha.cloud",
		TokenEnv: "GITLAB_EMPRESA_TOKEN",
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.DefaultProfile != "empresa" {
		t.Errorf("DefaultProfile = %q, want %q", loaded.DefaultProfile, "empresa")
	}
	p, ok := loaded.Profiles["empresa"]
	if !ok {
		t.Fatalf("profile %q not found after reload", "empresa")
	}
	if p.Host != "https://gitlab.services.betha.cloud" {
		t.Errorf("Host = %q, want %q", p.Host, "https://gitlab.services.betha.cloud")
	}
}

func TestSave_TightensPreExistingLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permissions don't apply on windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Simulate a config file/dir that already existed with looser
	// permissions (e.g. restored from a dotfiles sync) before gitlab-tui
	// ever wrote to it.
	if err := os.WriteFile(path, []byte("default_profile: \"\"\nprofiles: {}\n"), 0o644); err != nil {
		t.Fatalf("seeding pre-existing file: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("seeding pre-existing dir perms: %v", err)
	}

	cfg := Default()
	cfg.Profiles["empresa"] = Profile{Host: "https://gitlab.example.com", Token: "glpat-secret"}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(file) error = %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("file permissions = %o, want 0600 (config may contain a token)", got)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("dir permissions = %o, want 0700", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(filepath.Join(dir, "does-not-exist.yaml"))
	if err != ErrNotFound {
		t.Errorf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "empty config is valid",
			cfg:     Default(),
			wantErr: false,
		},
		{
			name: "profile missing host",
			cfg: &Config{Profiles: map[string]Profile{
				"empresa": {TokenEnv: "X"},
			}},
			wantErr: true,
		},
		{
			name: "profile missing token source",
			cfg: &Config{Profiles: map[string]Profile{
				"empresa": {Host: "https://gitlab.example.com"},
			}},
			wantErr: true,
		},
		{
			name: "default_profile not configured",
			cfg: &Config{
				DefaultProfile: "missing",
				Profiles:       map[string]Profile{},
			},
			wantErr: true,
		},
		{
			name: "invalid diff mode",
			cfg: &Config{
				Profiles: map[string]Profile{},
				Diff:     DiffConfig{Mode: "sideways"},
			},
			wantErr: true,
		},
		{
			name: "two profiles with the same host is ambiguous",
			cfg: &Config{Profiles: map[string]Profile{
				"work_a": {Host: "https://gitlab.example.com", TokenEnv: "X"},
				"work_b": {Host: "https://gitlab.example.com/", TokenEnv: "Y"}, // trailing slash, still the same host
			}},
			wantErr: true,
		},
		{
			name: "same hostname on different profiles is fine",
			cfg: &Config{Profiles: map[string]Profile{
				"empresa": {Host: "https://gitlab.services.betha.cloud", TokenEnv: "X"},
				"pessoal": {Host: "https://gitlab.com", TokenEnv: "Y"},
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	t.Run("from env", func(t *testing.T) {
		t.Setenv("GITLAB_TEST_TOKEN", "glpat-from-env")
		p := Profile{Host: "https://gitlab.example.com", TokenEnv: "GITLAB_TEST_TOKEN"}
		got, err := p.ResolveToken()
		if err != nil {
			t.Fatalf("ResolveToken() error = %v", err)
		}
		if got != "glpat-from-env" {
			t.Errorf("ResolveToken() = %q, want %q", got, "glpat-from-env")
		}
	})

	t.Run("env unset falls back to inline token", func(t *testing.T) {
		p := Profile{Host: "https://gitlab.example.com", TokenEnv: "GITLAB_TOTALLY_UNSET_VAR", Token: "fallback"}
		got, err := p.ResolveToken()
		if err != nil {
			t.Fatalf("ResolveToken() error = %v", err)
		}
		if got != "fallback" {
			t.Errorf("ResolveToken() = %q, want %q", got, "fallback")
		}
	})

	t.Run("nothing configured", func(t *testing.T) {
		p := Profile{Host: "https://gitlab.example.com"}
		if _, err := p.ResolveToken(); err == nil {
			t.Error("ResolveToken() expected error, got nil")
		}
	})
}

func TestExpandHome(t *testing.T) {
	if got := ExpandHome("/absolute/path"); got != "/absolute/path" {
		t.Errorf("ExpandHome() = %q, want unchanged path", got)
	}
	if got := ExpandHome("~/x"); got == "~/x" {
		t.Errorf("ExpandHome() did not expand ~: %q", got)
	}
}
