// Package dashboard resolves the local repo (host + branch) against
// configured profiles and GitLab, producing the data the TUI's home
// screen renders. It depends only on gitlab.Client and config.Config, so
// it's testable without a real git checkout or network access.
package dashboard

import (
	"context"
	"errors"
	"fmt"

	"github.com/arturoburigo/gzlab/internal/config"
	"github.com/arturoburigo/gzlab/internal/gitdetect"
	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// Context is everything the dashboard screen needs to render.
type Context struct {
	ProfileName  string
	Profile      config.Profile
	Project      *gitlab.Project
	Branch       string
	MergeRequest *gitlab.MergeRequest // nil if the branch has no open MR
}

// NewClientFunc constructs a GitLab client for a profile. It's a func
// param (rather than calling gitlab.NewClient directly) so tests can
// supply a mock client without a real token or network access.
type NewClientFunc func(config.Profile) (gitlab.Client, error)

// Resolve matches remote against a configured profile, fetches the
// project, and looks up the merge request (if any) for branch.
//
// profileOverride, when non-empty (e.g. from --profile), is used as-is
// instead of matching remote's host against configured profiles.
func Resolve(ctx context.Context, cfg *config.Config, newClient NewClientFunc, remote *gitdetect.RemoteInfo, branch, profileOverride string) (*Context, error) {
	profileName := profileOverride
	if profileName == "" {
		hosts := make(map[string]string, len(cfg.Profiles))
		for name, p := range cfg.Profiles {
			hosts[name] = p.Host
		}
		matched, ok := gitdetect.MatchProfileByHost(hosts, remote.Host)
		if !ok {
			return nil, fmt.Errorf("no profile configured for host %s; run `gzlab auth login`", remote.Host)
		}
		profileName = matched
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("profile %q not found in config", profileName)
	}

	client, err := newClient(profile)
	if err != nil {
		return nil, err
	}

	project, err := client.GetProjectByPath(ctx, remote.Path)
	if err != nil {
		return nil, fmt.Errorf("fetching project %q: %w", remote.Path, err)
	}

	mr, err := client.FindMergeRequestForBranch(ctx, project.ID, branch)
	if err != nil {
		if !errors.Is(err, gitlab.ErrNotFound) {
			return nil, fmt.Errorf("finding merge request for branch %q: %w", branch, err)
		}
		mr = nil
	}

	return &Context{
		ProfileName:  profileName,
		Profile:      profile,
		Project:      project,
		Branch:       branch,
		MergeRequest: mr,
	}, nil
}
