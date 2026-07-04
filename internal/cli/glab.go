package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// glabRunner shells out to glab. It's a package var (rather than a direct
// exec.Command call) so tests can substitute a fake without a real glab
// binary — the same seam internal/tui uses for its own glab calls.
var glabRunner = runGLabDefault

func runGLabDefault(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "glab", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GLAB_PAGER=cat", "PAGER=cat", "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("glab %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
