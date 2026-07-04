package cli

import (
	"strings"
	"testing"

	"github.com/arturoburigo/gzlab/internal/config"
)

func TestProfileAdd(t *testing.T) {
	home := withIsolatedHome(t)
	srv := fakeGitLabServer(t)
	t.Setenv("GITLAB_PESSOAL_TOKEN", "glpat-should-not-be-saved")

	root := NewRootCommand()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(srv.URL + "\nGITLAB_PESSOAL_TOKEN\n"))
	root.SetArgs([]string{"profile", "add", "pessoal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("profile add error = %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "arturo.burigo") {
		t.Errorf("profile add output missing authenticated user:\n%s", out.String())
	}

	path := home + "/.config/gitlab-tui/config.yaml"
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("loading saved config: %v", err)
	}
	p, ok := cfg.Profiles["pessoal"]
	if !ok {
		t.Fatal("expected profile \"pessoal\" to be saved")
	}
	if p.Host != srv.URL || p.TokenEnv != "GITLAB_PESSOAL_TOKEN" {
		t.Errorf("saved profile = %+v", p)
	}
}

func TestProfileRename(t *testing.T) {
	home := withIsolatedHome(t)
	cfg := config.Default()
	cfg.DefaultProfile = "empresa"
	cfg.Profiles["empresa"] = config.Profile{Host: "https://gitlab.services.betha.cloud", TokenEnv: "GITLAB_EMPRESA_TOKEN"}
	writeTestConfig(t, home, cfg)

	out, err := runCommand(t, "profile", "rename", "empresa", "trabalho")
	if err != nil {
		t.Fatalf("profile rename error = %v", err)
	}
	if !strings.Contains(out, "empresa") || !strings.Contains(out, "trabalho") {
		t.Errorf("profile rename output = %q", out)
	}

	path := home + "/.config/gitlab-tui/config.yaml"
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reloading config: %v", err)
	}
	if _, ok := reloaded.Profiles["empresa"]; ok {
		t.Error("old profile name should no longer be present")
	}
	if _, ok := reloaded.Profiles["trabalho"]; !ok {
		t.Error("renamed profile should be present under the new name")
	}
	if reloaded.DefaultProfile != "trabalho" {
		t.Errorf("DefaultProfile = %q, want the rename to follow it", reloaded.DefaultProfile)
	}
}

func TestProfileRename_UnknownProfile(t *testing.T) {
	home := withIsolatedHome(t)
	writeTestConfig(t, home, config.Default())

	if _, err := runCommand(t, "profile", "rename", "does-not-exist", "new-name"); err == nil {
		t.Error("expected an error renaming an unknown profile")
	}
}

func TestProfileRename_TargetAlreadyExists(t *testing.T) {
	home := withIsolatedHome(t)
	cfg := config.Default()
	cfg.Profiles["a"] = config.Profile{Host: "https://gitlab.example.com/a", TokenEnv: "A"}
	cfg.Profiles["b"] = config.Profile{Host: "https://gitlab.example.com/b", TokenEnv: "B"}
	writeTestConfig(t, home, cfg)

	if _, err := runCommand(t, "profile", "rename", "a", "b"); err == nil {
		t.Error("expected an error renaming onto an already-existing profile name")
	}
}

func TestProfileTest_Success(t *testing.T) {
	home := withIsolatedHome(t)
	srv := fakeGitLabServer(t)
	cfg := config.Default()
	cfg.Profiles["empresa"] = config.Profile{Host: srv.URL, Token: "glpat-anything"}
	writeTestConfig(t, home, cfg)

	out, err := runCommand(t, "profile", "test", "empresa")
	if err != nil {
		t.Fatalf("profile test error = %v", err)
	}
	if !strings.Contains(out, "OK") || !strings.Contains(out, "arturo.burigo") {
		t.Errorf("profile test output = %q", out)
	}
}

func TestProfileTest_InvalidToken(t *testing.T) {
	home := withIsolatedHome(t)
	srv := fakeUnauthorizedGitLabServer(t)
	cfg := config.Default()
	cfg.Profiles["empresa"] = config.Profile{Host: srv.URL, Token: "glpat-invalid"}
	writeTestConfig(t, home, cfg)

	if _, err := runCommand(t, "profile", "test", "empresa"); err == nil {
		t.Error("expected an error for a profile with an invalid token")
	}
}
