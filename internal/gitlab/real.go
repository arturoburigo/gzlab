package gitlab

import (
	"context"
	"errors"
	"fmt"

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
