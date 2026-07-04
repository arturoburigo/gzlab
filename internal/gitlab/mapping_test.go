package gitlab

import (
	"testing"
	"time"

	gl "gitlab.com/gitlab-org/api/client-go"
)

func TestToProject(t *testing.T) {
	got := toProject(&gl.Project{
		ID:                42,
		PathWithNamespace: "team/service",
		Name:              "service",
		WebURL:            "https://gitlab.example.com/team/service",
		DefaultBranch:     "main",
	})

	want := &Project{
		ID:                42,
		PathWithNamespace: "team/service",
		Name:              "service",
		WebURL:            "https://gitlab.example.com/team/service",
		DefaultBranch:     "main",
	}
	if *got != *want {
		t.Errorf("toProject() = %+v, want %+v", got, want)
	}
}

func TestToMergeRequestFromBasic(t *testing.T) {
	created := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	basic := &gl.BasicMergeRequest{
		IID:          250,
		ProjectID:    7,
		Title:        "Ajusta cadastro",
		State:        "opened",
		Draft:        true,
		SourceBranch: "feature-PD-26527",
		TargetBranch: "main",
		Author:       &gl.BasicUser{Username: "arturo.burigo"},
		WebURL:       "https://gitlab.example.com/team/service/-/merge_requests/250",
		HasConflicts: false,
		CreatedAt:    &created,
	}

	got := toMergeRequestFromBasic(basic)

	if got.IID != 250 || got.ProjectID != 7 || got.Title != "Ajusta cadastro" {
		t.Errorf("basic fields not mapped correctly: %+v", got)
	}
	if got.State != MergeRequestStateOpened {
		t.Errorf("State = %q, want %q", got.State, MergeRequestStateOpened)
	}
	if !got.Draft {
		t.Error("Draft = false, want true")
	}
	if got.Author != "arturo.burigo" {
		t.Errorf("Author = %q, want %q", got.Author, "arturo.burigo")
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created)
	}
}

func TestToMergeRequestFromBasic_NilAuthorAndTimes(t *testing.T) {
	got := toMergeRequestFromBasic(&gl.BasicMergeRequest{IID: 1})
	if got.Author != "" {
		t.Errorf("Author = %q, want empty string for nil author", got.Author)
	}
	if !got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt = %v, want zero value for nil timestamp", got.CreatedAt)
	}
	if got.ProjectPath != "" {
		t.Errorf("ProjectPath = %q, want empty string for nil references", got.ProjectPath)
	}
}

func TestToMergeRequestFromBasic_ProjectPathFromReferences(t *testing.T) {
	got := toMergeRequestFromBasic(&gl.BasicMergeRequest{
		IID:        99,
		References: &gl.IssueReferences{Full: "team/other-service!99"},
	})
	if got.ProjectPath != "team/other-service" {
		t.Errorf("ProjectPath = %q, want %q", got.ProjectPath, "team/other-service")
	}
}

func TestToMergeRequest_MapsPipeline(t *testing.T) {
	created := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	mr := &gl.MergeRequest{
		BasicMergeRequest: gl.BasicMergeRequest{IID: 250},
		Pipeline: &gl.PipelineInfo{
			ID:        18392,
			Status:    "failed",
			Ref:       "feature-PD-26527",
			WebURL:    "https://gitlab.example.com/team/service/-/pipelines/18392",
			CreatedAt: &created,
		},
	}

	got := toMergeRequest(mr)
	if got.Pipeline == nil {
		t.Fatal("Pipeline = nil, want non-nil")
	}
	if got.Pipeline.Status != PipelineStatusFailed {
		t.Errorf("Pipeline.Status = %q, want %q", got.Pipeline.Status, PipelineStatusFailed)
	}
	if got.Pipeline.ID != 18392 {
		t.Errorf("Pipeline.ID = %d, want %d", got.Pipeline.ID, 18392)
	}
}

func TestToMergeRequest_NilPipeline(t *testing.T) {
	got := toMergeRequest(&gl.MergeRequest{BasicMergeRequest: gl.BasicMergeRequest{IID: 1}})
	if got.Pipeline != nil {
		t.Errorf("Pipeline = %+v, want nil", got.Pipeline)
	}
}

func TestToContributionEvent_UsesTargetTitle(t *testing.T) {
	created := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	got := toContributionEvent(&gl.ContributionEvent{
		ActionName:  "opened",
		TargetTitle: "Adjust invoice validation",
		CreatedAt:   &created,
	})
	if got.Action != "opened" || got.Target != "Adjust invoice validation" {
		t.Errorf("got = %+v", got)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created)
	}
}

func TestToContributionEvent_PushFallsBackToRef(t *testing.T) {
	got := toContributionEvent(&gl.ContributionEvent{
		ActionName: "pushed to",
		PushData:   gl.ContributionEventPushData{Ref: "feature-PD-26112-desmock"},
	})
	if got.Target != "feature-PD-26112-desmock" {
		t.Errorf("Target = %q, want the pushed ref", got.Target)
	}
}
