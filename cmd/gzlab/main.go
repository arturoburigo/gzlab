// Command gzlab is the entrypoint for the GitLab TUI.
package main

import (
	"fmt"
	"os"

	"github.com/arturoburigo/gzlab/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
