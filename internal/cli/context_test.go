package cli

import (
	"path/filepath"
	"testing"

	"github.com/arturoburigo/gitlab-tui/internal/config"
)

func TestResolveProfileName(t *testing.T) {
	orig := profileFlag
	t.Cleanup(func() { profileFlag = orig })

	cfg := &config.Config{DefaultProfile: "empresa", Profiles: map[string]config.Profile{
		"empresa": {Host: "https://gitlab.example.com", TokenEnv: "X"},
	}}

	t.Run("flag takes precedence", func(t *testing.T) {
		profileFlag = "pessoal"
		defer func() { profileFlag = "" }()
		got, err := resolveProfileName(cfg)
		if err != nil {
			t.Fatalf("resolveProfileName() error = %v", err)
		}
		if got != "pessoal" {
			t.Errorf("resolveProfileName() = %q, want %q", got, "pessoal")
		}
	})

	t.Run("falls back to default_profile", func(t *testing.T) {
		profileFlag = ""
		got, err := resolveProfileName(cfg)
		if err != nil {
			t.Fatalf("resolveProfileName() error = %v", err)
		}
		if got != "empresa" {
			t.Errorf("resolveProfileName() = %q, want %q", got, "empresa")
		}
	})

	t.Run("errors with nothing configured", func(t *testing.T) {
		profileFlag = ""
		empty := &config.Config{Profiles: map[string]config.Profile{}}
		if _, err := resolveProfileName(empty); err == nil {
			t.Error("resolveProfileName() expected error, got nil")
		}
	})
}

func TestRemoveProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Default()
	cfg.DefaultProfile = "empresa"
	cfg.Profiles["empresa"] = config.Profile{Host: "https://gitlab.example.com", TokenEnv: "X"}
	cfg.Profiles["pessoal"] = config.Profile{Host: "https://gitlab.com", TokenEnv: "Y"}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := removeProfile(path, cfg, "empresa"); err != nil {
		t.Fatalf("removeProfile() error = %v", err)
	}

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := reloaded.Profiles["empresa"]; ok {
		t.Error("profile \"empresa\" still present after removal")
	}
	if reloaded.DefaultProfile != "" {
		t.Errorf("DefaultProfile = %q, want cleared", reloaded.DefaultProfile)
	}
	if _, ok := reloaded.Profiles["pessoal"]; !ok {
		t.Error("profile \"pessoal\" should be unaffected")
	}

	if err := removeProfile(path, cfg, "does-not-exist"); err == nil {
		t.Error("removeProfile() expected error for missing profile, got nil")
	}
}
