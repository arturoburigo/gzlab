package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/arturoburigo/gzlab/internal/config"
)

// initTestRepoForCLI creates a throwaway git repo (with a commit and origin
// remote) for testing commands that need to detect the current repository.
func initTestRepoForCLI(t *testing.T, originURL string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "feature-x")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-q", "-m", "init")
	run("remote", "add", "origin", originURL)
	return dir
}

// chdirTemp changes the process's working directory to dir for the duration
// of the test, restoring it afterward — needed since resolveProject and the
// pipeline/mr checkout commands detect the repo via os.Getwd().
func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func stubGLab(t *testing.T, fn func(ctx context.Context, dir string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := glabRunner
	glabRunner = fn
	t.Cleanup(func() { glabRunner = orig })
}

func TestMRCheckout(t *testing.T) {
	dir := initTestRepoForCLI(t, "git@gitlab.example.com:team/service.git")
	chdirTemp(t, dir)

	var gotArgs []string
	stubGLab(t, func(ctx context.Context, d string, args ...string) ([]byte, error) {
		gotArgs = args
		cmd := exec.Command("git", "checkout", "-b", "feature-mr-42")
		cmd.Dir = d
		out, err := cmd.CombinedOutput()
		return out, err
	})

	out, err := runCommand(t, "mr", "checkout", "42")
	if err != nil {
		t.Fatalf("mr checkout error = %v\noutput:\n%s", err, out)
	}
	wantArgs := []string{"mr", "checkout", "42"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
		}
	}
	if !strings.Contains(out, "feature-mr-42") {
		t.Errorf("mr checkout output = %q, want it to mention the checked-out branch", out)
	}
}

func TestMRCheckout_RequiresGitRepo(t *testing.T) {
	chdirTemp(t, t.TempDir())
	if _, err := runCommand(t, "mr", "checkout", "42"); err == nil {
		t.Error("expected an error outside a git repository")
	}
}

func TestPipelineLogs(t *testing.T) {
	dir := initTestRepoForCLI(t, "git@gitlab.example.com:team/service.git")
	chdirTemp(t, dir)

	stubGLab(t, func(ctx context.Context, d string, args ...string) ([]byte, error) {
		return []byte("build log line 1\nbuild log line 2\n"), nil
	})

	out, err := runCommand(t, "pipeline", "logs", "99")
	if err != nil {
		t.Fatalf("pipeline logs error = %v", err)
	}
	if !strings.Contains(out, "build log line 1") {
		t.Errorf("pipeline logs output = %q", out)
	}
}

func TestPipelineList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approvals"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"approvals_required": 0, "approvals_left": 0}`))
		case strings.Contains(r.URL.Path, "/merge_requests/251"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"iid": 251, "title": "Alinha ao commons", "state": "opened",
				"pipeline": {"id": 3237626, "status": "failed"}}`))
		case strings.Contains(r.URL.Path, "/merge_requests"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"iid": 251, "title": "Alinha ao commons", "state": "opened"}]`))
		case strings.Contains(r.URL.Path, "/projects/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 2087, "path_with_namespace": "team/service", "name": "service"}`))
		default:
			t.Errorf("unexpected request path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	home := withIsolatedHome(t)
	cfg := config.Default()
	cfg.DefaultProfile = "empresa"
	cfg.Profiles["empresa"] = config.Profile{Host: srv.URL, Token: "glpat-anything"}
	writeTestConfig(t, home, cfg)

	dir := initTestRepoForCLI(t, srv.URL+"/team/service.git")
	chdirTemp(t, dir)

	stubGLab(t, func(ctx context.Context, d string, args ...string) ([]byte, error) {
		return []byte(`{"id": 3237626, "status": "failed", "ref": "refs/merge-requests/251/head",
			"jobs": [{"id": 10, "name": "test", "stage": "test", "status": "failed"}]}`), nil
	})

	out, err := runCommand(t, "pipeline", "list")
	if err != nil {
		t.Fatalf("pipeline list error = %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"3237626", "failed", "test"} {
		if !strings.Contains(out, want) {
			t.Errorf("pipeline list output missing %q:\n%s", want, out)
		}
	}
}
