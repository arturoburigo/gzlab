package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/arturoburigo/gzlab/internal/config"
	"github.com/arturoburigo/gzlab/internal/gitdetect"
	"github.com/arturoburigo/gzlab/internal/history"
	"github.com/arturoburigo/gzlab/internal/tui"
	"github.com/arturoburigo/gzlab/internal/workspace"
)

func runTUI(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	repoRoot, err := gitdetect.RepoRoot(dir)
	if err != nil {
		return fmt.Errorf("gzlab must be run inside a git repository: %w", err)
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

	historyPath, err := history.DefaultPath()
	if err != nil {
		return err
	}
	workspacePath, err := workspace.DefaultPath()
	if err != nil {
		return err
	}

	deps := tui.Deps{
		Config:          cfg,
		NewClient:       newClientForProfileWithCache(cfg),
		Remote:          remote,
		RepoRoot:        repoRoot,
		Branch:          branch,
		ProfileOverride: profileFlag,
		HistoryPath:     historyPath,
		WorkspacePath:   workspacePath,
	}

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.UI.Mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	_, err = tea.NewProgram(tui.New(deps), opts...).Run()
	return err
}
