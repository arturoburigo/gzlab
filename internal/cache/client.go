package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// Client wraps a GitLab client with a local JSON cache. It never caches tokens,
// errors, or job logs.
type Client struct {
	next      gitlab.Client
	store     *Store
	namespace string
	ttls      TTLs
}

// NewClient wraps next with a local cache. ttls optionally overrides the
// per-type retention windows (DefaultTTLs() otherwise) — at most one value
// is used, so existing 3-arg call sites keep working unchanged.
func NewClient(next gitlab.Client, store *Store, namespace string, ttls ...TTLs) gitlab.Client {
	if next == nil || store == nil {
		return next
	}
	t := DefaultTTLs()
	if len(ttls) > 0 {
		t = ttls[0]
	}
	return &Client{next: next, store: store, namespace: namespace, ttls: t}
}

func (c *Client) CurrentUser(ctx context.Context) (*gitlab.User, error) {
	var cached gitlab.User
	if c.get("current-user", &cached) {
		return &cached, nil
	}
	user, err := c.next.CurrentUser(ctx)
	if err == nil && user != nil {
		_ = c.set("current-user", c.ttls.CurrentUser, user)
	}
	return user, err
}

func (c *Client) GetProjectByPath(ctx context.Context, path string) (*gitlab.Project, error) {
	var cached gitlab.Project
	if c.get("project", &cached, path) {
		return &cached, nil
	}
	project, err := c.next.GetProjectByPath(ctx, path)
	if err == nil && project != nil {
		_ = c.set("project", c.ttls.Project, project, path)
	}
	return project, err
}

func (c *Client) ListMergeRequests(ctx context.Context, projectID int, opts gitlab.ListMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	var cached []*gitlab.MergeRequest
	if c.get("mr-list", &cached, projectID, opts.State, opts.SourceBranch) {
		return cached, nil
	}
	mrs, err := c.next.ListMergeRequests(ctx, projectID, opts)
	if err == nil {
		_ = c.set("mr-list", c.ttls.MergeRequestList, mrs, projectID, opts.State, opts.SourceBranch)
	}
	return mrs, err
}

func (c *Client) ListMyMergeRequests(ctx context.Context, opts gitlab.ListMyMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	var cached []*gitlab.MergeRequest
	if c.get("mr-list-mine", &cached, opts.State, opts.Scope, opts.ReviewerUsername) {
		return cached, nil
	}
	mrs, err := c.next.ListMyMergeRequests(ctx, opts)
	if err == nil {
		_ = c.set("mr-list-mine", c.ttls.MergeRequestList, mrs, opts.State, opts.Scope, opts.ReviewerUsername)
	}
	return mrs, err
}

func (c *Client) Search(ctx context.Context, opts gitlab.GlobalSearchOptions) ([]gitlab.GlobalSearchResult, error) {
	return c.next.Search(ctx, opts)
}

func (c *Client) GetMergeRequest(ctx context.Context, projectID, iid int) (*gitlab.MergeRequest, error) {
	var cached gitlab.MergeRequest
	if c.get("mr-detail", &cached, projectID, iid) {
		return &cached, nil
	}
	mr, err := c.next.GetMergeRequest(ctx, projectID, iid)
	if err == nil && mr != nil {
		_ = c.set("mr-detail", c.ttls.MergeRequestDetail, mr, projectID, iid)
	}
	return mr, err
}

func (c *Client) ListMergeRequestDiffs(ctx context.Context, projectID, iid int) ([]*gitlab.MergeRequestDiff, error) {
	var cached []*gitlab.MergeRequestDiff
	if c.get("mr-diffs", &cached, projectID, iid) {
		return cached, nil
	}
	diffs, err := c.next.ListMergeRequestDiffs(ctx, projectID, iid)
	if err == nil {
		_ = c.set("mr-diffs", c.ttls.Diff, diffs, projectID, iid)
	}
	return diffs, err
}

func (c *Client) GetPipeline(ctx context.Context, projectID, pipelineID int) (*gitlab.Pipeline, error) {
	var cached gitlab.Pipeline
	if c.get("pipeline", &cached, projectID, pipelineID) {
		return &cached, nil
	}
	pipeline, err := c.next.GetPipeline(ctx, projectID, pipelineID)
	if err == nil && pipeline != nil {
		_ = c.set("pipeline", c.ttls.Pipeline, pipeline, projectID, pipelineID)
	}
	return pipeline, err
}

func (c *Client) ListPipelineJobs(ctx context.Context, projectID, pipelineID int) ([]*gitlab.Job, error) {
	var cached []*gitlab.Job
	if c.get("pipeline-jobs", &cached, projectID, pipelineID) {
		return cached, nil
	}
	jobs, err := c.next.ListPipelineJobs(ctx, projectID, pipelineID)
	if err == nil {
		_ = c.set("pipeline-jobs", c.ttls.PipelineJobs, jobs, projectID, pipelineID)
	}
	return jobs, err
}

func (c *Client) FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*gitlab.MergeRequest, error) {
	mrs, err := c.ListMergeRequests(ctx, projectID, gitlab.ListMergeRequestsOptions{SourceBranch: branch})
	if err != nil {
		return nil, err
	}
	if len(mrs) == 0 {
		return nil, fmt.Errorf("no open merge request for branch %q: %w", branch, gitlab.ErrNotFound)
	}
	full, err := c.GetMergeRequest(ctx, projectID, mrs[0].IID)
	if err != nil && errors.Is(err, gitlab.ErrNotFound) {
		return mrs[0], nil
	}
	return full, err
}

func (c *Client) ListCommits(ctx context.Context, projectID int, opts gitlab.ListCommitsOptions) ([]gitlab.Commit, error) {
	var cached []gitlab.Commit
	if c.get("commits", &cached, projectID, opts.Author, opts.Limit) {
		return cached, nil
	}
	commits, err := c.next.ListCommits(ctx, projectID, opts)
	if err == nil {
		_ = c.set("commits", c.ttls.Commits, commits, projectID, opts.Author, opts.Limit)
	}
	return commits, err
}

func (c *Client) ListMyContributionEvents(ctx context.Context, opts gitlab.ListContributionEventsOptions) ([]gitlab.ContributionEvent, error) {
	var cached []gitlab.ContributionEvent
	if c.get("contribution-events", &cached, opts.After, opts.Limit) {
		return cached, nil
	}
	events, err := c.next.ListMyContributionEvents(ctx, opts)
	if err == nil {
		_ = c.set("contribution-events", c.ttls.ContributionEvents, events, opts.After, opts.Limit)
	}
	return events, err
}

func (c *Client) get(kind string, dst any, parts ...any) bool {
	return c.store.Get(c.key(kind, parts...), dst)
}

func (c *Client) set(kind string, ttl time.Duration, value any, parts ...any) error {
	return c.store.Set(c.key(kind, parts...), ttl, value)
}

func (c *Client) key(kind string, parts ...any) string {
	values := []string{c.namespace, kind}
	for _, part := range parts {
		values = append(values, fmt.Sprint(part))
	}
	return strings.Join(values, "\x00")
}
