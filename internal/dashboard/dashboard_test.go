package dashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/arturoburigo/gzlab/internal/config"
	"github.com/arturoburigo/gzlab/internal/gitdetect"
	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// mockClient is a minimal test double for gitlab.Client.
type mockClient struct {
	project *gitlab.Project
	mr      *gitlab.MergeRequest

	getProjectErr error
	findMRErr     error
}

func (m *mockClient) CurrentUser(ctx context.Context) (*gitlab.User, error) { return nil, nil }

func (m *mockClient) GetProjectByPath(ctx context.Context, path string) (*gitlab.Project, error) {
	if m.getProjectErr != nil {
		return nil, m.getProjectErr
	}
	return m.project, nil
}

func (m *mockClient) ListMergeRequests(ctx context.Context, projectID int, opts gitlab.ListMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	return nil, nil
}

func (m *mockClient) ListMyMergeRequests(ctx context.Context, opts gitlab.ListMyMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	return nil, nil
}

func (m *mockClient) Search(ctx context.Context, opts gitlab.GlobalSearchOptions) ([]gitlab.GlobalSearchResult, error) {
	return nil, nil
}

func (m *mockClient) GetMergeRequest(ctx context.Context, projectID, iid int) (*gitlab.MergeRequest, error) {
	return m.mr, nil
}

func (m *mockClient) ListMergeRequestDiffs(ctx context.Context, projectID, iid int) ([]*gitlab.MergeRequestDiff, error) {
	return nil, nil
}

func (m *mockClient) GetPipeline(ctx context.Context, projectID, pipelineID int) (*gitlab.Pipeline, error) {
	return nil, nil
}

func (m *mockClient) ListPipelineJobs(ctx context.Context, projectID, pipelineID int) ([]*gitlab.Job, error) {
	return nil, nil
}

func (m *mockClient) FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*gitlab.MergeRequest, error) {
	if m.findMRErr != nil {
		return nil, m.findMRErr
	}
	return m.mr, nil
}

func (m *mockClient) ListCommits(ctx context.Context, projectID int, opts gitlab.ListCommitsOptions) ([]gitlab.Commit, error) {
	return nil, nil
}

func (m *mockClient) ListMyContributionEvents(ctx context.Context, opts gitlab.ListContributionEventsOptions) ([]gitlab.ContributionEvent, error) {
	return nil, nil
}

func testConfig() *config.Config {
	return &config.Config{
		DefaultProfile: "empresa",
		Profiles: map[string]config.Profile{
			"empresa": {Host: "https://gitlab.services.betha.cloud", TokenEnv: "GITLAB_EMPRESA_TOKEN"},
		},
	}
}

func TestResolve_WithMergeRequest(t *testing.T) {
	client := &mockClient{
		project: &gitlab.Project{ID: 7, PathWithNamespace: "team/service"},
		mr:      &gitlab.MergeRequest{IID: 250, Title: "Ajusta cadastro"},
	}

	got, err := Resolve(context.Background(), testConfig(), func(config.Profile) (gitlab.Client, error) {
		return client, nil
	}, &gitdetect.RemoteInfo{Host: "https://gitlab.services.betha.cloud", Path: "team/service"}, "feature-PD-26527", "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.ProfileName != "empresa" {
		t.Errorf("ProfileName = %q, want %q", got.ProfileName, "empresa")
	}
	if got.Project.ID != 7 {
		t.Errorf("Project.ID = %d, want 7", got.Project.ID)
	}
	if got.MergeRequest == nil || got.MergeRequest.IID != 250 {
		t.Errorf("MergeRequest = %+v, want IID 250", got.MergeRequest)
	}
}

func TestResolve_NoMergeRequestForBranch(t *testing.T) {
	client := &mockClient{
		project:   &gitlab.Project{ID: 7, PathWithNamespace: "team/service"},
		findMRErr: gitlab.ErrNotFound,
	}

	got, err := Resolve(context.Background(), testConfig(), func(config.Profile) (gitlab.Client, error) {
		return client, nil
	}, &gitdetect.RemoteInfo{Host: "https://gitlab.services.betha.cloud", Path: "team/service"}, "main", "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.MergeRequest != nil {
		t.Errorf("MergeRequest = %+v, want nil", got.MergeRequest)
	}
}

func TestResolve_ProfileOverrideBypassesHostMatch(t *testing.T) {
	client := &mockClient{project: &gitlab.Project{ID: 7, PathWithNamespace: "team/service"}}

	got, err := Resolve(context.Background(), testConfig(), func(config.Profile) (gitlab.Client, error) {
		return client, nil
	}, &gitdetect.RemoteInfo{Host: "https://gitlab.com", Path: "team/service"}, "main", "empresa")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.ProfileName != "empresa" {
		t.Errorf("ProfileName = %q, want %q", got.ProfileName, "empresa")
	}
}

func TestResolve_UnknownProfileOverride(t *testing.T) {
	_, err := Resolve(context.Background(), testConfig(), func(config.Profile) (gitlab.Client, error) {
		return &mockClient{}, nil
	}, &gitdetect.RemoteInfo{Host: "https://gitlab.com", Path: "team/service"}, "main", "does-not-exist")
	if err == nil {
		t.Error("Resolve() expected error for unknown profile override, got nil")
	}
}

func TestResolve_NoProfileForHost(t *testing.T) {
	_, err := Resolve(context.Background(), testConfig(), func(config.Profile) (gitlab.Client, error) {
		return &mockClient{}, nil
	}, &gitdetect.RemoteInfo{Host: "https://gitlab.com", Path: "someone/project"}, "main", "")
	if err == nil {
		t.Error("Resolve() expected error for unmatched host, got nil")
	}
}

func TestResolve_ProjectFetchError(t *testing.T) {
	client := &mockClient{getProjectErr: errors.New("boom")}
	_, err := Resolve(context.Background(), testConfig(), func(config.Profile) (gitlab.Client, error) {
		return client, nil
	}, &gitdetect.RemoteInfo{Host: "https://gitlab.services.betha.cloud", Path: "team/service"}, "main", "")
	if err == nil {
		t.Error("Resolve() expected error when GetProjectByPath fails, got nil")
	}
}
