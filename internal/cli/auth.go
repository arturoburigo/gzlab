package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/arturoburigo/gzlab/internal/config"
)

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage GitLab authentication profiles",
	}
	cmd.AddCommand(newAuthLoginCommand())
	cmd.AddCommand(newAuthStatusCommand())
	cmd.AddCommand(newAuthLogoutCommand())
	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate against a GitLab instance and save a profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createProfile(cmd, "")
		},
	}
}

// createProfile runs the interactive "authenticate + save a profile" flow
// shared by `auth login` and `profile add`. When presetName is non-empty
// (profile add <name>) the name prompt is skipped.
func createProfile(cmd *cobra.Command, presetName string) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())

	host, err := promptLine(out, reader, "GitLab host (e.g. https://gitlab.com): ")
	if err != nil {
		return err
	}
	if host == "" {
		return fmt.Errorf("host is required")
	}

	name := presetName
	if name == "" {
		name, err = promptLine(out, reader, "Profile name (e.g. empresa): ")
		if err != nil {
			return err
		}
	}
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	defaultTokenEnv := fmt.Sprintf("GITLAB_%s_TOKEN", strings.ToUpper(name))
	tokenEnv, err := promptLine(out, reader, fmt.Sprintf("Environment variable holding the token [%s]: ", defaultTokenEnv))
	if err != nil {
		return err
	}
	if tokenEnv == "" {
		tokenEnv = defaultTokenEnv
	}

	token := os.Getenv(tokenEnv)
	if token == "" {
		if _, err := fmt.Fprintf(out, "%s is not set. Paste the token now (input hidden): ", tokenEnv); err != nil {
			return err
		}
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(out) //nolint:errcheck // best-effort newline after hidden input
		if err != nil {
			return fmt.Errorf("reading token: %w", err)
		}
		token = strings.TrimSpace(string(tokenBytes))
	}
	if token == "" {
		return fmt.Errorf("a token is required: export %s or paste it when prompted", tokenEnv)
	}

	ctx, cancel := withAPITimeout(cmd.Context())
	defer cancel()

	client, err := newClientForProfile(config.Profile{Host: host, Token: token})
	if err != nil {
		return err
	}
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("could not authenticate against %s: %w", host, err)
	}

	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			return err
		}
		cfg = config.Default()
	}
	cfg.Profiles[name] = config.Profile{Host: host, TokenEnv: tokenEnv}
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = name
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "Logged in to %s as %s (@%s). Profile %q saved to %s.\n", host, user.Name, user.Username, name, path)
	return err
}

func newAuthStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the currently authenticated profile and user",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, name, profile, err := loadActiveProfile()
			if err != nil {
				return err
			}

			client, err := newClientForProfile(profile)
			if err != nil {
				return fmt.Errorf("profile %q: %w", name, err)
			}

			ctx, cancel := withAPITimeout(cmd.Context())
			defer cancel()
			user, err := client.CurrentUser(ctx)
			if err != nil {
				return fmt.Errorf("profile %q: token is not valid: %w", name, err)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\nHost: %s\nUser: %s (@%s)\n", name, profile.Host, user.Name, user.Username)
			return err
		},
	}
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout [profile]",
		Short: "Remove a saved profile",
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

			name := profileFlag
			if len(args) == 1 {
				name = args[0]
			}
			if name == "" {
				name = cfg.DefaultProfile
			}
			if name == "" {
				return fmt.Errorf("no profile specified: pass a name, --profile, or configure a default_profile")
			}

			if err := removeProfile(path, cfg, name); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q.\n", name)
			return err
		},
	}
}

func promptLine(out io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
