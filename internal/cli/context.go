package cli

import (
	"context"
	"fmt"
	"time"

	localcache "github.com/arturoburigo/gzlab/internal/cache"
	"github.com/arturoburigo/gzlab/internal/config"
	"github.com/arturoburigo/gzlab/internal/gitlab"
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
	return "", fmt.Errorf("no profile specified: pass --profile or set a default_profile (run `gzlab auth login`)")
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

func newClientForProfileWithCache(cfg *config.Config) func(config.Profile) (gitlab.Client, error) {
	return func(p config.Profile) (gitlab.Client, error) {
		client, err := newClientForProfile(p)
		if err != nil {
			return nil, err
		}
		if cfg == nil || !cfg.Cache.Enabled {
			return client, nil
		}
		path, err := localcache.DefaultPath()
		if err != nil {
			return nil, err
		}
		namespace := p.Host + "|" + p.TokenEnv
		return localcache.NewClient(client, localcache.NewStore(path), namespace, cacheTTLs(cfg.Cache.TTL)), nil
	}
}

// cacheTTLs converts the config's per-type TTL settings into localcache.TTLs.
func cacheTTLs(t config.CacheTTLConfig) localcache.TTLs {
	return localcache.TTLs{
		CurrentUser:        time.Duration(t.CurrentUser),
		Project:            time.Duration(t.Project),
		MergeRequestList:   time.Duration(t.MergeRequestList),
		MergeRequestDetail: time.Duration(t.MergeRequestDetail),
		Diff:               time.Duration(t.Diff),
		Pipeline:           time.Duration(t.Pipeline),
		PipelineJobs:       time.Duration(t.PipelineJobs),
		Commits:            time.Duration(t.Commits),
		ContributionEvents: time.Duration(t.ContributionEvents),
	}
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
