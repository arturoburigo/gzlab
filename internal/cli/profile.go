package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/arturoburigo/gzlab/internal/config"
)

func newProfileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "List and manage configured GitLab profiles",
	}
	cmd.AddCommand(newProfileListCommand())
	cmd.AddCommand(newProfileAddCommand())
	cmd.AddCommand(newProfileRemoveCommand())
	cmd.AddCommand(newProfileRenameCommand())
	cmd.AddCommand(newProfileTestCommand())
	return cmd
}

func newProfileAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add [name]",
		Short: "Authenticate against a GitLab instance and save a new profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return createProfile(cmd, name)
		},
	}
}

func newProfileRenameCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a configured profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]

			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			p, ok := cfg.Profiles[oldName]
			if !ok {
				return fmt.Errorf("profile %q not found", oldName)
			}
			if _, exists := cfg.Profiles[newName]; exists {
				return fmt.Errorf("profile %q already exists", newName)
			}

			delete(cfg.Profiles, oldName)
			cfg.Profiles[newName] = p
			if cfg.DefaultProfile == oldName {
				cfg.DefaultProfile = newName
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Renamed profile %q to %q.\n", oldName, newName)
			return err
		},
	}
}

func newProfileTestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "test [name]",
		Short: "Validate a profile's token against GitLab",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			if name == "" {
				name, err = resolveProfileName(cfg)
				if err != nil {
					return err
				}
			}
			p, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %q not found in config", name)
			}

			client, err := newClientForProfile(p)
			if err != nil {
				return fmt.Errorf("profile %q: %w", name, err)
			}

			ctx, cancel := withAPITimeout(cmd.Context())
			defer cancel()
			user, err := client.CurrentUser(ctx)
			if err != nil {
				return fmt.Errorf("profile %q: token is not valid: %w", name, err)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Profile %q OK: authenticated to %s as %s (@%s).\n", name, p.Host, user.Name, user.Username)
			return err
		},
	}
}

func newProfileListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(cfg.Profiles))
			for name := range cfg.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)

			out := cmd.OutOrStdout()
			if len(names) == 0 {
				_, err := fmt.Fprintln(out, "No profiles configured. Run `gzlab auth login`.")
				return err
			}
			for _, name := range names {
				p := cfg.Profiles[name]
				marker := "  "
				if name == cfg.DefaultProfile {
					marker = "* "
				}
				if _, err := fmt.Fprintf(out, "%s%s\t%s\t(token_env: %s)\n", marker, name, p.Host, p.TokenEnv); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newProfileRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a configured profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			if err := removeProfile(path, cfg, name); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q.\n", name)
			return err
		},
	}
}
