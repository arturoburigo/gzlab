// Command gitlab-tui is the entrypoint for the GitLab TUI.
package main

import (
	"fmt"
	"os"

	"github.com/arturoburigo/gitlab-tui/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
