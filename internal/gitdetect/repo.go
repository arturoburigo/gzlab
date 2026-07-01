// Package gitdetect inspects the local git repository: its origin remote
// and current branch. It shells out to the git binary rather than
// reimplementing git internals.
package gitdetect

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotARepo indicates dir isn't inside a git working tree.
var ErrNotARepo = errors.New("not a git repository")

// ErrNoRemote indicates the repo has no "origin" remote configured.
var ErrNoRemote = errors.New("no \"origin\" remote configured")

// ErrDetachedHead indicates HEAD doesn't point at a branch.
var ErrDetachedHead = errors.New("not on a branch (detached HEAD)")

// RepoRoot returns the top-level directory of the git repo containing dir.
func RepoRoot(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ErrNotARepo
	}
	return out, nil
}

// OriginURL returns the URL of the repo's "origin" remote.
func OriginURL(dir string) (string, error) {
	out, err := runGit(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", ErrNoRemote
	}
	return out, nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(dir string) (string, error) {
	out, err := runGit(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", ErrDetachedHead
	}
	return out, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
