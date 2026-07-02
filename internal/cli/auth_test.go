package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func fakeGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 735, "username": "arturo.burigo", "name": "Arturo Burigo"})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAuthLogin_SavesProfileAndNeverPersistsToken(t *testing.T) {
	home := withIsolatedHome(t)
	srv := fakeGitLabServer(t)
	t.Setenv("GITLAB_EMPRESA_TOKEN", "glpat-should-not-be-saved")

	orig := profileFlag
	t.Cleanup(func() { profileFlag = orig })

	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(srv.URL + "\nempresa\nGITLAB_EMPRESA_TOKEN\n"))
	root.SetArgs([]string{"auth", "login"})

	if err := root.Execute(); err != nil {
		t.Fatalf("auth login error = %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "arturo.burigo") {
		t.Errorf("auth login output missing authenticated user:\n%s", out.String())
	}

	path := home + "/.config/gitlab-tui/config.yaml"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	if strings.Contains(string(raw), "glpat-should-not-be-saved") {
		t.Error("config file contains the raw token; only token_env should be persisted")
	}
	if !strings.Contains(string(raw), "GITLAB_EMPRESA_TOKEN") {
		t.Error("config file missing token_env reference")
	}
}

func TestAuthStatus_NoProfileConfigured(t *testing.T) {
	withIsolatedHome(t)
	if _, err := runCommand(t, "auth", "status"); err == nil {
		t.Error("expected an error when no profile is configured")
	}
}
