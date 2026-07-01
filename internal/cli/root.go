// Package cli wires the gitlab-tui command tree.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arturoburigo/gitlab-tui/internal/version"
)

var profileFlag string

// NewRootCommand builds the root `gitlab-tui` command and its subcommands.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "gitlab-tui",
		Short: "A terminal UI for GitLab merge requests, diffs and pipelines",
		Long: `gitlab-tui is a terminal UI for GitLab focused on the daily developer
workflow: merge requests, diffs, pipelines and job logs, across multiple
GitLab profiles (e.g. work and personal).`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&profileFlag, "profile", "", "profile to use (overrides default_profile in config)")
	root.AddCommand(newVersionCommand())
	root.AddCommand(newConfigCommand())
	root.AddCommand(newAuthCommand())
	root.AddCommand(newProfileCommand())

	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the gitlab-tui version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return err
		},
	}
}
