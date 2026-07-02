package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arturoburigo/gitlab-tui/internal/config"
	"github.com/arturoburigo/gitlab-tui/internal/dashboard"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

type mockClient struct {
	mrs    []*gitlab.MergeRequest
	detail *gitlab.MergeRequest
}

func (m *mockClient) CurrentUser(ctx context.Context) (*gitlab.User, error) { return nil, nil }
func (m *mockClient) GetProjectByPath(ctx context.Context, path string) (*gitlab.Project, error) {
	return nil, nil
}
func (m *mockClient) ListMergeRequests(ctx context.Context, projectID int, opts gitlab.ListMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	return m.mrs, nil
}
func (m *mockClient) GetMergeRequest(ctx context.Context, projectID, iid int) (*gitlab.MergeRequest, error) {
	return m.detail, nil
}
func (m *mockClient) FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*gitlab.MergeRequest, error) {
	return nil, gitlab.ErrNotFound
}

func testDashContext() *dashboard.Context {
	return &dashboard.Context{
		ProfileName: "empresa",
		Profile:     config.Profile{Host: "https://gitlab.services.betha.cloud"},
		Project:     &gitlab.Project{ID: 2087, PathWithNamespace: "atendimento/protocolo/cadastros/api-protocolo-cadastros", WebURL: "https://gitlab.services.betha.cloud/team/service"},
		Branch:      "bugfix-PD-26527",
		MergeRequest: &gitlab.MergeRequest{
			IID: 251, Title: "Alinha ao commons", State: gitlab.MergeRequestStateOpened,
			WebURL:   "https://gitlab.services.betha.cloud/team/service/-/merge_requests/251",
			Pipeline: &gitlab.Pipeline{ID: 1, Status: gitlab.PipelineStatusFailed},
		},
	}
}

func loadedModel(t *testing.T, client *mockClient) Model {
	t.Helper()
	m := New(Deps{NewClient: func(config.Profile) (gitlab.Client, error) { return client, nil }})
	updated, _ := m.Update(dashboardLoadedMsg{ctx: testDashContext()})
	got := updated.(Model)
	if got.loading {
		t.Fatal("expected loading=false after dashboardLoadedMsg")
	}
	if got.client == nil {
		t.Fatal("expected client to be set after dashboardLoadedMsg")
	}
	return got
}

func TestUpdate_DashboardLoaded(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	if m.dash.ProfileName != "empresa" {
		t.Errorf("ProfileName = %q, want %q", m.dash.ProfileName, "empresa")
	}
}

func TestUpdate_DashboardLoaded_ClientConstructionError(t *testing.T) {
	m := New(Deps{NewClient: func(config.Profile) (gitlab.Client, error) { return nil, errors.New("boom") }})
	updated, _ := m.Update(dashboardLoadedMsg{ctx: testDashContext()})
	got := updated.(Model)
	if got.err == nil {
		t.Error("expected err to be set when NewClient fails")
	}
}

func TestUpdate_ErrMsg(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(errMsg{err: errors.New("boom")})
	got := updated.(Model)
	if got.loading {
		t.Error("expected loading=false after errMsg")
	}
	if got.err == nil {
		t.Error("expected err to be set")
	}
}

func TestHandleKey_Quit(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHandleKey_SwitchToListAndBack(t *testing.T) {
	client := &mockClient{mrs: []*gitlab.MergeRequest{{IID: 1, Title: "a"}, {IID: 2, Title: "b"}}}
	m := loadedModel(t, client)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = updated.(Model)
	if m.screen != screenList {
		t.Fatalf("screen = %v, want screenList", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected loadMRListCmd")
	}

	msg := cmd()
	listMsg, ok := msg.(mrListLoadedMsg)
	if !ok {
		t.Fatalf("expected mrListLoadedMsg, got %T", msg)
	}
	updated, _ = m.Update(listMsg)
	m = updated.(Model)
	if len(m.mrs) != 2 {
		t.Fatalf("len(mrs) = %d, want 2", len(m.mrs))
	}

	// down moves the cursor
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	// down again stays at the last index (no overflow)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (clamped)", m.cursor)
	}

	// esc returns to the dashboard
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenDashboard {
		t.Errorf("screen = %v, want screenDashboard", m.screen)
	}
}

func TestHandleKey_EnterLoadsDetail(t *testing.T) {
	client := &mockClient{
		mrs:    []*gitlab.MergeRequest{{IID: 251, Title: "Alinha ao commons"}},
		detail: &gitlab.MergeRequest{IID: 251, Title: "Alinha ao commons", SourceBranch: "bugfix-PD-26527", TargetBranch: "master"},
	}
	m := loadedModel(t, client)
	m.screen = screenList
	m.mrs = client.mrs

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected loadMRDetailCmd")
	}
	msg := cmd()
	detailMsg, ok := msg.(mrDetailLoadedMsg)
	if !ok {
		t.Fatalf("expected mrDetailLoadedMsg, got %T", msg)
	}
	updated, _ := m.Update(detailMsg)
	m = updated.(Model)
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", m.screen)
	}
	if m.detail == nil || m.detail.IID != 251 {
		t.Errorf("detail = %+v, want IID 251", m.detail)
	}
}

func TestCurrentURL(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	if got := m.currentURL(); got != "https://gitlab.services.betha.cloud/team/service/-/merge_requests/251" {
		t.Errorf("currentURL() on dashboard = %q", got)
	}

	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{WebURL: "https://gitlab.services.betha.cloud/x/-/merge_requests/9"}
	if got := m.currentURL(); got != "https://gitlab.services.betha.cloud/x/-/merge_requests/9" {
		t.Errorf("currentURL() on detail = %q", got)
	}
}

func TestView_Dashboard(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	view := m.View()

	for _, want := range []string{"empresa", "atendimento/protocolo/cadastros/api-protocolo-cadastros", "bugfix-PD-26527", "!251", "failed"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q\n%s", want, view)
		}
	}
}

func TestView_List(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenList
	m.mrs = []*gitlab.MergeRequest{
		{IID: 1, Title: "First MR", State: gitlab.MergeRequestStateOpened},
		{IID: 2, Title: "Second MR, in draft", State: gitlab.MergeRequestStateOpened, Draft: true},
	}
	m.cursor = 1

	view := m.View()
	for _, want := range []string{"!1", "First MR", "!2", "Second MR, in draft", "(draft)", "> "} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (list) missing %q\n%s", want, view)
		}
	}
}

func TestView_List_Empty(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenList
	m.mrs = nil

	if view := m.View(); !strings.Contains(view, "No open merge requests") {
		t.Errorf("View() (empty list) = %q, want the empty-state message", view)
	}
}

func TestView_Detail(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{
		IID: 251, Title: "Alinha ao commons",
		SourceBranch: "bugfix-PD-26527", TargetBranch: "master",
		Author: "arturo.burigo", State: gitlab.MergeRequestStateOpened,
		HasConflicts:      true,
		ApprovalsRequired: 2, ApprovalsGiven: 1,
		Pipeline: &gitlab.Pipeline{Status: gitlab.PipelineStatusRunning},
	}

	view := m.View()
	for _, want := range []string{
		"!251", "Alinha ao commons",
		"bugfix-PD-26527", "master",
		"arturo.burigo", "opened",
		"running", "1/2", "Has conflicts",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (detail) missing %q\n%s", want, view)
		}
	}
}

func TestView_Loading(t *testing.T) {
	m := New(Deps{})
	if !strings.Contains(m.View(), "Loading") {
		t.Errorf("View() = %q, want it to mention loading", m.View())
	}
}

func TestView_Error(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(errMsg{err: errors.New("token expired")})
	view := updated.(Model).View()
	if !strings.Contains(view, "token expired") {
		t.Errorf("View() = %q, want it to mention the error", view)
	}
}
