package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gl "gitlab.com/gitlab-org/api/client-go"
)

type realClient struct {
	api *gl.Client
}

// NewClient creates a Client authenticated against host using token.
func NewClient(host, token string) (Client, error) {
	api, err := gl.NewClient(token, gl.WithBaseURL(host))
	if err != nil {
		return nil, fmt.Errorf("creating GitLab client for %s: %w", host, err)
	}
	return &realClient{api: api}, nil
}

func (c *realClient) CurrentUser(ctx context.Context) (*User, error) {
	u, _, err := c.api.Users.CurrentUser(gl.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("fetching current user: %w", err)
	}
	return &User{ID: int(u.ID), Username: u.Username, Name: u.Name}, nil
}

func (c *realClient) GetProjectByPath(ctx context.Context, path string) (*Project, error) {
	p, resp, err := c.api.Projects.GetProject(path, nil, gl.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, fmt.Errorf("project %q: %w", path, ErrNotFound)
		}
		return nil, fmt.Errorf("fetching project %q: %w", path, err)
	}
	return toProject(p), nil
}

// maxMergeRequestPages bounds ListMergeRequests' pagination loop. GitLab
// caps PerPage at 100, so this allows up to 10,000 merge requests per
// project — a defensive ceiling, not a real-world limit.
const maxMergeRequestPages = 100
const maxPipelineJobPages = 100

func (c *realClient) ListMergeRequests(ctx context.Context, projectID int, opts ListMergeRequestsOptions) ([]*MergeRequest, error) {
	state := string(opts.State)
	if state == "" {
		state = string(MergeRequestStateOpened)
	}

	listOpts := &gl.ListProjectMergeRequestsOptions{
		State:       &state,
		ListOptions: gl.ListOptions{PerPage: 100, Page: 1},
	}
	if opts.SourceBranch != "" {
		listOpts.SourceBranch = &opts.SourceBranch
	}

	var result []*MergeRequest
	for page := 1; page <= maxMergeRequestPages; page++ {
		listOpts.Page = int64(page)
		mrs, resp, err := c.api.MergeRequests.ListProjectMergeRequests(projectID, listOpts, gl.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("listing merge requests for project %d: %w", projectID, err)
		}
		for _, mr := range mrs {
			result = append(result, toMergeRequestFromBasic(mr))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return result, nil
}

func (c *realClient) ListMyMergeRequests(ctx context.Context, opts ListMyMergeRequestsOptions) ([]*MergeRequest, error) {
	state := string(opts.State)
	if state == "" {
		state = string(MergeRequestStateOpened)
	}

	perPage := 100
	if opts.Limit > 0 && opts.Limit < perPage {
		perPage = opts.Limit
	}
	listOpts := &gl.ListMergeRequestsOptions{
		ListOptions: gl.ListOptions{PerPage: int64(perPage), Page: 1},
	}
	if state != string(MergeRequestStateAll) {
		listOpts.State = &state
	}
	if opts.Scope != "" {
		scope := string(opts.Scope)
		listOpts.Scope = &scope
	}
	if opts.ReviewerUsername != "" {
		listOpts.ReviewerUsername = &opts.ReviewerUsername
	}

	var result []*MergeRequest
	for page := 1; page <= maxMergeRequestPages; page++ {
		listOpts.Page = int64(page)
		mrs, resp, err := c.api.MergeRequests.ListMergeRequests(listOpts, gl.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("listing merge requests: %w", err)
		}
		for _, mr := range mrs {
			result = append(result, toMergeRequestFromBasic(mr))
		}
		if opts.Limit > 0 && len(result) >= opts.Limit {
			return result[:opts.Limit], nil
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return result, nil
}

func (c *realClient) ListCommits(ctx context.Context, projectID int, opts ListCommitsOptions) ([]Commit, error) {
	perPage := opts.Limit
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	listOpts := &gl.ListCommitsOptions{
		All:         gl.Ptr(true),
		ListOptions: gl.ListOptions{PerPage: int64(perPage), Page: 1},
	}
	if opts.Author != "" {
		listOpts.Author = &opts.Author
	}

	commits, _, err := c.api.Commits.ListCommits(projectID, listOpts, gl.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("listing commits for project %d: %w", projectID, err)
	}
	result := make([]Commit, 0, len(commits))
	for _, commit := range commits {
		result = append(result, toCommit(commit))
	}
	return result, nil
}

// maxContributionEventPages bounds ListMyContributionEvents' pagination —
// callers scope this to a short window via opts.After, so this is a
// defensive ceiling, not a real-world limit.
const maxContributionEventPages = 20

func (c *realClient) ListMyContributionEvents(ctx context.Context, opts ListContributionEventsOptions) ([]ContributionEvent, error) {
	perPage := 100
	if opts.Limit > 0 && opts.Limit < perPage {
		perPage = opts.Limit
	}
	listOpts := &gl.ListContributionEventsOptions{
		ListOptions: gl.ListOptions{PerPage: int64(perPage), Page: 1},
	}
	if !opts.After.IsZero() {
		after := gl.ISOTime(opts.After)
		listOpts.After = &after
	}

	var result []ContributionEvent
	for page := 1; page <= maxContributionEventPages; page++ {
		listOpts.Page = int64(page)
		events, resp, err := c.api.Events.ListCurrentUserContributionEvents(listOpts, gl.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("listing contribution events: %w", err)
		}
		for _, e := range events {
			result = append(result, toContributionEvent(e))
		}
		if opts.Limit > 0 && len(result) >= opts.Limit {
			return result[:opts.Limit], nil
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return result, nil
}

func (c *realClient) Search(ctx context.Context, opts GlobalSearchOptions) ([]GlobalSearchResult, error) {
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	results := make([]GlobalSearchResult, 0, limit)
	projects, err := c.searchProjects(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		results = append(results, GlobalSearchResult{Type: "project", Project: project})
		if len(results) >= limit {
			return results, nil
		}
	}

	mrs, err := c.searchMergeRequests(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	for _, mr := range mrs {
		results = append(results, GlobalSearchResult{Type: "merge_request", MR: mr})
		if len(results) >= limit {
			return results, nil
		}
	}

	branchProjects := projects
	if opts.Project != nil {
		branchProjects = []*Project{opts.Project}
	}
	for _, project := range branchProjects {
		branches, err := c.searchBranches(ctx, project.ID, query, limit)
		if err != nil {
			continue
		}
		for _, branch := range branches {
			results = append(results, GlobalSearchResult{Type: "branch", Project: project, Branch: branch})
			if len(results) >= limit {
				return results, nil
			}
		}
	}
	return results, nil
}

func (c *realClient) searchProjects(ctx context.Context, query string, limit int) ([]*Project, error) {
	opt := &gl.SearchOptions{ListOptions: gl.ListOptions{PerPage: int64(limit), Page: 1}}
	projects, _, err := c.api.Search.Projects(query, opt, gl.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("searching projects: %w", err)
	}
	result := make([]*Project, 0, len(projects))
	for _, project := range projects {
		result = append(result, toProject(project))
	}
	return result, nil
}

func (c *realClient) searchMergeRequests(ctx context.Context, query string, limit int) ([]*MergeRequest, error) {
	opt := &gl.SearchOptions{ListOptions: gl.ListOptions{PerPage: int64(limit), Page: 1}}
	mrs, _, err := c.api.Search.MergeRequests(query, opt, gl.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("searching merge requests: %w", err)
	}
	result := make([]*MergeRequest, 0, len(mrs))
	for _, mr := range mrs {
		result = append(result, toMergeRequest(mr))
	}
	return result, nil
}

func (c *realClient) searchBranches(ctx context.Context, projectID int, query string, limit int) ([]*Branch, error) {
	opt := &gl.ListBranchesOptions{
		Search:      gl.Ptr(query),
		ListOptions: gl.ListOptions{PerPage: int64(limit), Page: 1},
	}
	branches, _, err := c.api.Branches.ListBranches(projectID, opt, gl.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("searching branches for project %d: %w", projectID, err)
	}
	result := make([]*Branch, 0, len(branches))
	for _, branch := range branches {
		result = append(result, toBranch(branch))
	}
	return result, nil
}

func (c *realClient) GetMergeRequest(ctx context.Context, projectID, iid int) (*MergeRequest, error) {
	mr, resp, err := c.api.MergeRequests.GetMergeRequest(projectID, int64(iid), nil, gl.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, fmt.Errorf("merge request !%d: %w", iid, ErrNotFound)
		}
		return nil, fmt.Errorf("fetching merge request !%d: %w", iid, err)
	}

	result := toMergeRequest(mr)

	approvals, _, err := c.api.MergeRequests.GetMergeRequestApprovals(projectID, int64(iid), gl.WithContext(ctx))
	if err == nil && approvals != nil {
		result.ApprovalsRequired = int(approvals.ApprovalsRequired)
		result.ApprovalsGiven = int(approvals.ApprovalsRequired - approvals.ApprovalsLeft)
	}
	// Approvals are a GitLab Premium feature; a failure here (e.g. 403 on
	// Free tier) shouldn't block showing the rest of the MR.

	return result, nil
}

func (c *realClient) ListMergeRequestDiffs(ctx context.Context, projectID, iid int) ([]*MergeRequestDiff, error) {
	listOpts := &gl.ListMergeRequestDiffsOptions{
		ListOptions: gl.ListOptions{PerPage: 100, Page: 1},
	}

	var result []*MergeRequestDiff
	for page := 1; page <= maxMergeRequestPages; page++ {
		listOpts.Page = int64(page)
		diffs, resp, err := c.api.MergeRequests.ListMergeRequestDiffs(projectID, int64(iid), listOpts, gl.WithContext(ctx))
		if err != nil {
			if resp != nil && resp.StatusCode == 404 {
				return nil, fmt.Errorf("merge request !%d diffs: %w", iid, ErrNotFound)
			}
			return c.showMergeRequestRawDiffs(ctx, projectID, iid, err)
		}
		for _, diff := range diffs {
			result = append(result, toMergeRequestDiff(diff))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return result, nil
}

func (c *realClient) showMergeRequestRawDiffs(ctx context.Context, projectID, iid int, structuredErr error) ([]*MergeRequestDiff, error) {
	raw, resp, err := c.api.MergeRequests.ShowMergeRequestRawDiffs(projectID, int64(iid), &gl.ShowMergeRequestRawDiffsOptions{}, gl.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, fmt.Errorf("merge request !%d raw diffs: %w", iid, ErrNotFound)
		}
		return nil, fmt.Errorf("listing merge request !%d diffs: %w; raw diff fallback also failed: %w", iid, structuredErr, err)
	}
	return []*MergeRequestDiff{{Diff: string(raw)}}, nil
}

func (c *realClient) GetPipeline(ctx context.Context, projectID, pipelineID int) (*Pipeline, error) {
	pipeline, resp, err := c.api.Pipelines.GetPipeline(projectID, int64(pipelineID), gl.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, fmt.Errorf("pipeline %d: %w", pipelineID, ErrNotFound)
		}
		return nil, fmt.Errorf("fetching pipeline %d: %w", pipelineID, err)
	}
	return toPipeline(pipeline), nil
}

func (c *realClient) ListPipelineJobs(ctx context.Context, projectID, pipelineID int) ([]*Job, error) {
	listOpts := &gl.ListJobsOptions{
		ListOptions: gl.ListOptions{PerPage: 100, Page: 1},
	}

	var result []*Job
	for page := 1; page <= maxPipelineJobPages; page++ {
		listOpts.Page = int64(page)
		jobs, resp, err := c.api.Jobs.ListPipelineJobs(projectID, int64(pipelineID), listOpts, gl.WithContext(ctx))
		if err != nil {
			if resp != nil && resp.StatusCode == 404 {
				return nil, fmt.Errorf("pipeline %d jobs: %w", pipelineID, ErrNotFound)
			}
			return nil, fmt.Errorf("listing pipeline %d jobs: %w", pipelineID, err)
		}
		for _, job := range jobs {
			result = append(result, toJob(job))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return result, nil
}

func (c *realClient) FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*MergeRequest, error) {
	mrs, err := c.ListMergeRequests(ctx, projectID, ListMergeRequestsOptions{SourceBranch: branch})
	if err != nil {
		return nil, err
	}
	if len(mrs) == 0 {
		return nil, fmt.Errorf("no open merge request for branch %q: %w", branch, ErrNotFound)
	}

	full, err := c.GetMergeRequest(ctx, projectID, mrs[0].IID)
	if err != nil && errors.Is(err, ErrNotFound) {
		return mrs[0], nil
	}
	return full, err
}
