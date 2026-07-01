package gitdetect

import (
	"os/exec"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "feature-PD-26527")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-q", "-m", "init")
	run("remote", "add", "origin", "git@gitlab.example.com:team/service.git")
	return dir
}

func TestRepoRootAndOriginAndBranch(t *testing.T) {
	dir := initTestRepo(t)

	root, err := RepoRoot(dir)
	if err != nil {
		t.Fatalf("RepoRoot() error = %v", err)
	}
	if root == "" {
		t.Error("RepoRoot() returned empty string")
	}

	origin, err := OriginURL(dir)
	if err != nil {
		t.Fatalf("OriginURL() error = %v", err)
	}
	if origin != "git@gitlab.example.com:team/service.git" {
		t.Errorf("OriginURL() = %q, want %q", origin, "git@gitlab.example.com:team/service.git")
	}

	branch, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if branch != "feature-PD-26527" {
		t.Errorf("CurrentBranch() = %q, want %q", branch, "feature-PD-26527")
	}
}

func TestRepoRootNotARepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := RepoRoot(dir); err != ErrNotARepo {
		t.Errorf("RepoRoot() error = %v, want ErrNotARepo", err)
	}
}

func TestOriginURLNoRemote(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if _, err := OriginURL(dir); err != ErrNoRemote {
		t.Errorf("OriginURL() error = %v, want ErrNoRemote", err)
	}
}
