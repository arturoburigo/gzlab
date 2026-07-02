package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/arturoburigo/gitlab-tui/internal/config"
	"github.com/arturoburigo/gitlab-tui/internal/gitdetect"
	"github.com/arturoburigo/gitlab-tui/internal/tui"
)

func runTUI(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	repoRoot, err := gitdetect.RepoRoot(dir)
	if err != nil {
		return fmt.Errorf("gitlab-tui must be run inside a git repository: %w", err)
	}

	originURL, err := gitdetect.OriginURL(repoRoot)
	if err != nil {
		return fmt.Errorf("this repo has no \"origin\" remote configured: %w", err)
	}
	remote, err := gitdetect.ParseRemoteURL(originURL)
	if err != nil {
		return err
	}

	branch, err := gitdetect.CurrentBranch(repoRoot)
	if err != nil {
		return fmt.Errorf("could not determine the current branch: %w", err)
	}

	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	deps := tui.Deps{
		Config:          cfg,
		NewClient:       newClientForProfile,
		Remote:          remote,
		RepoRoot:        repoRoot,
		Branch:          branch,
		ProfileOverride: profileFlag,
	}

	_, err = tea.NewProgram(tui.New(deps), tea.WithAltScreen()).Run()
	return err
}
