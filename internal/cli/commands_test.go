package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	localcache "github.com/arturoburigo/gzlab/internal/cache"
	"github.com/arturoburigo/gzlab/internal/config"
)

// withIsolatedHome points $HOME at a fresh temp dir for the duration of the
// test, so config.DefaultPath() never touches the real user config.
func withIsolatedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func writeTestConfig(t *testing.T, home string, cfg *config.Config) {
	t.Helper()
	path := filepath.Join(home, ".config", "gitlab-tui", "config.yaml")
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func runCommand(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	orig := profileFlag
	t.Cleanup(func() { profileFlag = orig })

	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), err
}

func TestConfigShow_MasksToken(t *testing.T) {
	home := withIsolatedHome(t)
	cfg := config.Default()
	cfg.DefaultProfile = "empresa"
	cfg.Profiles["empresa"] = config.Profile{Host: "https://gitlab.services.betha.cloud", Token: "glpat-super-secret"}
	writeTestConfig(t, home, cfg)

	out, err := runCommand(t, "config", "show")
	if err != nil {
		t.Fatalf("config show error = %v", err)
	}
	if bytes.Contains([]byte(out), []byte("glpat-super-secret")) {
		t.Errorf("config show leaked the raw token:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("********")) {
		t.Errorf("config show did not mask the token:\n%s", out)
	}
}

func TestConfigShow_MissingConfig(t *testing.T) {
	withIsolatedHome(t)
	if _, err := runCommand(t, "config", "show"); err == nil {
		t.Error("expected an error when no config file exists")
	}
}

func TestProfileList(t *testing.T) {
	home := withIsolatedHome(t)
	cfg := config.Default()
	cfg.DefaultProfile = "empresa"
	cfg.Profiles["empresa"] = config.Profile{Host: "https://gitlab.services.betha.cloud", TokenEnv: "GITLAB_EMPRESA_TOKEN"}
	cfg.Profiles["pessoal"] = config.Profile{Host: "https://gitlab.com", TokenEnv: "GITLAB_PESSOAL_TOKEN"}
	writeTestConfig(t, home, cfg)

	out, err := runCommand(t, "profile", "list")
	if err != nil {
		t.Fatalf("profile list error = %v", err)
	}
	for _, want := range []string{"empresa", "pessoal", "gitlab.services.betha.cloud", "gitlab.com"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("profile list output missing %q:\n%s", want, out)
		}
	}
}

func TestProfileRemove(t *testing.T) {
	home := withIsolatedHome(t)
	cfg := config.Default()
	cfg.DefaultProfile = "empresa"
	cfg.Profiles["empresa"] = config.Profile{Host: "https://gitlab.services.betha.cloud", TokenEnv: "GITLAB_EMPRESA_TOKEN"}
	writeTestConfig(t, home, cfg)

	if _, err := runCommand(t, "profile", "remove", "empresa"); err != nil {
		t.Fatalf("profile remove error = %v", err)
	}

	out, err := runCommand(t, "profile", "list")
	if err != nil {
		t.Fatalf("profile list error = %v", err)
	}
	if bytes.Contains([]byte(out), []byte("empresa")) {
		t.Errorf("profile still listed after removal:\n%s", out)
	}
}

func TestProfileRemove_UnknownProfile(t *testing.T) {
	home := withIsolatedHome(t)
	writeTestConfig(t, home, config.Default())

	if _, err := runCommand(t, "profile", "remove", "does-not-exist"); err == nil {
		t.Error("expected an error removing an unknown profile")
	}
}

func TestVersionCommand(t *testing.T) {
	out, err := runCommand(t, "version")
	if err != nil {
		t.Fatalf("version error = %v", err)
	}
	if out == "" {
		t.Error("version command produced no output")
	}
}

func TestCacheClear(t *testing.T) {
	home := withIsolatedHome(t)
	root := filepath.Join(home, ".cache", "gitlab-tui")
	store := localcache.NewStore(root)
	if err := store.Set("x", 1, "cached"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	out, err := runCommand(t, "cache", "clear")
	if err != nil {
		t.Fatalf("cache clear error = %v", err)
	}
	if !strings.Contains(out, root) {
		t.Fatalf("cache clear output %q does not mention %q", out, root)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("cache dir stat error = %v, want not exist", err)
	}
}
