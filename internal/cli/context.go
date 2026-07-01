package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/arturoburigo/gitlab-tui/internal/config"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

const apiTimeout = 15 * time.Second

// resolveProfileName picks the profile to use: the --profile flag, falling
// back to default_profile from config.
func resolveProfileName(cfg *config.Config) (string, error) {
	if profileFlag != "" {
		return profileFlag, nil
	}
	if cfg.DefaultProfile != "" {
		return cfg.DefaultProfile, nil
	}
	return "", fmt.Errorf("no profile specified: pass --profile or set a default_profile (run `gitlab-tui auth login`)")
}

// loadActiveProfile loads the config file and resolves the profile to use
// for this invocation.
func loadActiveProfile() (cfg *config.Config, name string, profile config.Profile, err error) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, "", config.Profile{}, err
	}
	cfg, err = config.Load(path)
	if err != nil {
		return nil, "", config.Profile{}, err
	}
	name, err = resolveProfileName(cfg)
	if err != nil {
		return nil, "", config.Profile{}, err
	}
	p, ok := cfg.Profiles[name]
	if !ok {
		return nil, "", config.Profile{}, fmt.Errorf("profile %q not found in config", name)
	}
	return cfg, name, p, nil
}

// newClientForProfile builds an authenticated GitLab client for a profile.
func newClientForProfile(p config.Profile) (gitlab.Client, error) {
	token, err := p.ResolveToken()
	if err != nil {
		return nil, fmt.Errorf("resolving token: %w", err)
	}
	return gitlab.NewClient(p.Host, token)
}

func withAPITimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, apiTimeout)
}

// removeProfile deletes a profile from the config file at path, clearing
// default_profile if it pointed at the removed profile.
func removeProfile(path string, cfg *config.Config, name string) error {
	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(cfg.Profiles, name)
	if cfg.DefaultProfile == name {
		cfg.DefaultProfile = ""
	}
	return config.Save(path, cfg)
}
