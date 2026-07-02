package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/arturoburigo/gitlab-tui/internal/config"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and edit the gitlab-tui config file",
	}
	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigEditCommand())
	return cmd
}

func newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the current config, with tokens masked",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			masked := *cfg
			masked.Profiles = make(map[string]config.Profile, len(cfg.Profiles))
			for name, p := range cfg.Profiles {
				if p.Token != "" {
					p.Token = "********"
				}
				masked.Profiles[name] = p
			}

			out, err := yaml.Marshal(&masked)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "# %s\n%s", path, out)
			return err
		},
	}
}

func newConfigEditCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the config file in ui.editor (config.yaml), $EDITOR, or vi",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if err := config.Save(path, config.Default()); err != nil {
					return fmt.Errorf("creating default config: %w", err)
				}
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			parts := resolveEditorCommand(cfg)
			c := exec.Command(parts[0], append(parts[1:], path)...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		},
	}
}

// resolveEditorCommand picks the editor to launch: config.yaml's ui.editor,
// then $EDITOR, then vi. A candidate that's empty or whitespace-only (e.g.
// EDITOR="  ") is skipped rather than producing an empty argv[0].
func resolveEditorCommand(cfg *config.Config) []string {
	for _, candidate := range []string{cfg.UI.Editor, os.Getenv("EDITOR")} {
		if parts := strings.Fields(candidate); len(parts) > 0 {
			// exec.Command does no shell expansion, so a "~/bin/foo"-style
			// editor path needs it done here.
			parts[0] = config.ExpandHome(parts[0])
			return parts
		}
	}
	return []string{"vi"}
}
