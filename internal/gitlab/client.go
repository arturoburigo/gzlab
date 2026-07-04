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

// ListCommitsOptions filters commits returned by ListCommits.
type ListCommitsOptions struct {
	// Author filters by commit author name/email substring match.
	Author string
	// Limit caps how many commits are returned (most-recent-first); 0 means
	// GitLab's default page size.
	Limit int
}

// Client is gzlab's own view of the GitLab API, scoped to what the
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

	// ListMyMergeRequests lists merge requests across projects via GitLab's
	// global endpoint, scoped by opts (created by me / assigned to me / to
	// review) — the cross-project MR list filters.
	ListMyMergeRequests(ctx context.Context, opts ListMyMergeRequestsOptions) ([]*MergeRequest, error)

	// Search returns a small mixed set of project, merge request, and branch
	// matches suitable for interactive navigation.
	Search(ctx context.Context, opts GlobalSearchOptions) ([]GlobalSearchResult, error)

	// GetMergeRequest fetches full detail (including pipeline status) for a
	// single merge request.
	GetMergeRequest(ctx context.Context, projectID, iid int) (*MergeRequest, error)

	// ListMergeRequestDiffs lists changed files for a merge request.
	ListMergeRequestDiffs(ctx context.Context, projectID, iid int) ([]*MergeRequestDiff, error)

	// GetPipeline fetches full detail for a pipeline.
	GetPipeline(ctx context.Context, projectID, pipelineID int) (*Pipeline, error)

	// ListPipelineJobs lists jobs for a pipeline.
	ListPipelineJobs(ctx context.Context, projectID, pipelineID int) ([]*Job, error)

	// FindMergeRequestForBranch returns the open merge request whose source
	// branch matches branch, or ErrNotFound if none exists.
	FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*MergeRequest, error)

	// ListCommits lists a project's commits across all branches, most-recent
	// first, optionally filtered to one author.
	ListCommits(ctx context.Context, projectID int, opts ListCommitsOptions) ([]Commit, error)

	// ListMyContributionEvents lists the current user's contribution events
	// (pushes, comments, MR/issue actions) across all projects, most-recent
	// first — powers the dashboard's activity feed and contribution strip.
	ListMyContributionEvents(ctx context.Context, opts ListContributionEventsOptions) ([]ContributionEvent, error)
}
