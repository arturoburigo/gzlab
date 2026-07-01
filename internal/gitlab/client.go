// Package gitlab is the only layer allowed to talk to the GitLab API SDK.
// The rest of the app (CLI, TUI) depends on the Client interface below, not
// on gitlab.com/gitlab-org/api/client-go directly.
package gitlab

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a requested resource doesn't exist.
var ErrNotFound = errors.New("not found")

// ListMergeRequestsOptions filters MRs returned by ListMergeRequests.
type ListMergeRequestsOptions struct {
	// State defaults to MergeRequestStateOpened when empty.
	State MergeRequestState
	// SourceBranch, when set, restricts results to that branch.
	SourceBranch string
}

// Client is gitlab-tui's own view of the GitLab API, scoped to what the
// product needs. It exists so the UI never depends on the upstream SDK
// directly.
type Client interface {
	// CurrentUser returns the user the client is authenticated as. Used to
	// validate a token during `auth login`.
	CurrentUser(ctx context.Context) (*User, error)

	// GetProjectByPath resolves a GitLab project from its
	// "namespace/project" path, as parsed from a git remote.
	GetProjectByPath(ctx context.Context, path string) (*Project, error)

	// ListMergeRequests lists merge requests for a project.
	ListMergeRequests(ctx context.Context, projectID int, opts ListMergeRequestsOptions) ([]*MergeRequest, error)

	// GetMergeRequest fetches full detail (including pipeline status) for a
	// single merge request.
	GetMergeRequest(ctx context.Context, projectID, iid int) (*MergeRequest, error)

	// FindMergeRequestForBranch returns the open merge request whose source
	// branch matches branch, or ErrNotFound if none exists.
	FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*MergeRequest, error)
}
