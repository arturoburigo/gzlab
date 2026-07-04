package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

type fakeClient struct {
	projectCalls  int
	listCalls     int
	detailCalls   int
	detailErr     error
	commitCalls   int
	activityCalls int
}

func (f *fakeClient) CurrentUser(ctx context.Context) (*gitlab.User, error) {
	return &gitlab.User{ID: 1, Username: "u"}, nil
}

func (f *fakeClient) GetProjectByPath(ctx context.Context, path string) (*gitlab.Project, error) {
	f.projectCalls++
	return &gitlab.Project{ID: 10, PathWithNamespace: path, Name: "repo"}, nil
}

func (f *fakeClient) ListMergeRequests(ctx context.Context, projectID int, opts gitlab.ListMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	f.listCalls++
	return []*gitlab.MergeRequest{{IID: 7, ProjectID: projectID, Title: "MR"}}, nil
}

func (f *fakeClient) ListMyMergeRequests(ctx context.Context, opts gitlab.ListMyMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	return []*gitlab.MergeRequest{{IID: 7, ProjectID: 10, Title: "MR"}}, nil
}

func (f *fakeClient) Search(ctx context.Context, opts gitlab.GlobalSearchOptions) ([]gitlab.GlobalSearchResult, error) {
	return nil, nil
}

func (f *fakeClient) GetMergeRequest(ctx context.Context, projectID, iid int) (*gitlab.MergeRequest, error) {
	f.detailCalls++
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	return &gitlab.MergeRequest{IID: iid, ProjectID: projectID, Title: "MR detail"}, nil
}

func (f *fakeClient) ListMergeRequestDiffs(ctx context.Context, projectID, iid int) ([]*gitlab.MergeRequestDiff, error) {
	return []*gitlab.MergeRequestDiff{{NewPath: "main.go"}}, nil
}

func (f *fakeClient) GetPipeline(ctx context.Context, projectID, pipelineID int) (*gitlab.Pipeline, error) {
	return &gitlab.Pipeline{ID: pipelineID}, nil
}

func (f *fakeClient) ListPipelineJobs(ctx context.Context, projectID, pipelineID int) ([]*gitlab.Job, error) {
	return []*gitlab.Job{{ID: 1, Name: "test"}}, nil
}

func (f *fakeClient) FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*gitlab.MergeRequest, error) {
	return &gitlab.MergeRequest{IID: 7, ProjectID: projectID, SourceBranch: branch}, nil
}

func (f *fakeClient) ListCommits(ctx context.Context, projectID int, opts gitlab.ListCommitsOptions) ([]gitlab.Commit, error) {
	f.commitCalls++
	return []gitlab.Commit{{ShortID: "abc1234", Title: "commit"}}, nil
}

func (f *fakeClient) ListMyContributionEvents(ctx context.Context, opts gitlab.ListContributionEventsOptions) ([]gitlab.ContributionEvent, error) {
	f.activityCalls++
	return []gitlab.ContributionEvent{{Action: "opened", Target: "!1 Test MR"}}, nil
}

func TestClientCachesProjectAndMRReads(t *testing.T) {
	base := &fakeClient{}
	client := NewClient(base, NewStore(t.TempDir()), "test")

	for i := 0; i < 2; i++ {
		if _, err := client.GetProjectByPath(context.Background(), "group/repo"); err != nil {
			t.Fatalf("GetProjectByPath() error = %v", err)
		}
		if _, err := client.ListMergeRequests(context.Background(), 10, gitlab.ListMergeRequestsOptions{}); err != nil {
			t.Fatalf("ListMergeRequests() error = %v", err)
		}
		if _, err := client.GetMergeRequest(context.Background(), 10, 7); err != nil {
			t.Fatalf("GetMergeRequest() error = %v", err)
		}
	}

	if base.projectCalls != 1 {
		t.Errorf("project calls = %d, want 1", base.projectCalls)
	}
	if base.listCalls != 1 {
		t.Errorf("list calls = %d, want 1", base.listCalls)
	}
	if base.detailCalls != 1 {
		t.Errorf("detail calls = %d, want 1", base.detailCalls)
	}
}

func TestClientCachesListCommits(t *testing.T) {
	base := &fakeClient{}
	client := NewClient(base, NewStore(t.TempDir()), "test")

	for range 2 {
		if _, err := client.ListCommits(context.Background(), 10, gitlab.ListCommitsOptions{Author: "arturo"}); err != nil {
			t.Fatalf("ListCommits() error = %v", err)
		}
	}
	if base.commitCalls != 1 {
		t.Errorf("commit calls = %d, want 1", base.commitCalls)
	}

	if _, err := client.ListCommits(context.Background(), 10, gitlab.ListCommitsOptions{Author: "someone-else"}); err != nil {
		t.Fatalf("ListCommits() error = %v", err)
	}
	if base.commitCalls != 2 {
		t.Errorf("commit calls after a different author = %d, want 2 (different opts shouldn't share a cache entry)", base.commitCalls)
	}
}

func TestClientCachesListMyContributionEvents(t *testing.T) {
	base := &fakeClient{}
	client := NewClient(base, NewStore(t.TempDir()), "test")

	for range 2 {
		if _, err := client.ListMyContributionEvents(context.Background(), gitlab.ListContributionEventsOptions{Limit: 100}); err != nil {
			t.Fatalf("ListMyContributionEvents() error = %v", err)
		}
	}
	if base.activityCalls != 1 {
		t.Errorf("activity calls = %d, want 1", base.activityCalls)
	}
}

func TestClientFindMergeRequestForBranchUsesCachedListAndDetail(t *testing.T) {
	base := &fakeClient{}
	client := NewClient(base, NewStore(t.TempDir()), "test")

	for i := 0; i < 2; i++ {
		mr, err := client.FindMergeRequestForBranch(context.Background(), 10, "feature")
		if err != nil {
			t.Fatalf("FindMergeRequestForBranch() error = %v", err)
		}
		if mr.Title != "MR detail" {
			t.Fatalf("FindMergeRequestForBranch() title = %q, want detail", mr.Title)
		}
	}

	if base.listCalls != 1 {
		t.Errorf("list calls = %d, want 1", base.listCalls)
	}
	if base.detailCalls != 1 {
		t.Errorf("detail calls = %d, want 1", base.detailCalls)
	}
}

func TestClientFindMergeRequestForBranchFallsBackToListResultWhenDetailIsNotFound(t *testing.T) {
	base := &fakeClient{detailErr: fmt.Errorf("detail: %w", gitlab.ErrNotFound)}
	client := NewClient(base, NewStore(t.TempDir()), "test")

	mr, err := client.FindMergeRequestForBranch(context.Background(), 10, "feature")
	if err != nil {
		t.Fatalf("FindMergeRequestForBranch() error = %v", err)
	}
	if mr.Title != "MR" {
		t.Fatalf("FindMergeRequestForBranch() title = %q, want list result", mr.Title)
	}
}
