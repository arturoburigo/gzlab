package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gzlab/internal/config"
	"github.com/arturoburigo/gzlab/internal/dashboard"
	"github.com/arturoburigo/gzlab/internal/gitlab"
	"github.com/arturoburigo/gzlab/internal/history"
)

type mockClient struct {
	mrs              []*gitlab.MergeRequest
	assignedMRs      []*gitlab.MergeRequest
	detail           *gitlab.MergeRequest
	diffs            []*gitlab.MergeRequestDiff
	pipeline         *gitlab.Pipeline
	jobs             []*gitlab.Job
	commits          []gitlab.Commit
	activity         []gitlab.ContributionEvent
	user             *gitlab.User
	lastActivityOpts gitlab.ListContributionEventsOptions
}

func (m *mockClient) CurrentUser(ctx context.Context) (*gitlab.User, error) { return m.user, nil }
func (m *mockClient) GetProjectByPath(ctx context.Context, path string) (*gitlab.Project, error) {
	return nil, nil
}
func (m *mockClient) ListMergeRequests(ctx context.Context, projectID int, opts gitlab.ListMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	return m.mrs, nil
}
func (m *mockClient) ListMyMergeRequests(ctx context.Context, opts gitlab.ListMyMergeRequestsOptions) ([]*gitlab.MergeRequest, error) {
	if opts.Scope == gitlab.MergeRequestsScopeAssignedToMe {
		return m.assignedMRs, nil
	}
	return m.mrs, nil
}
func (m *mockClient) Search(ctx context.Context, opts gitlab.GlobalSearchOptions) ([]gitlab.GlobalSearchResult, error) {
	return nil, nil
}
func (m *mockClient) GetMergeRequest(ctx context.Context, projectID, iid int) (*gitlab.MergeRequest, error) {
	return m.detail, nil
}
func (m *mockClient) ListMergeRequestDiffs(ctx context.Context, projectID, iid int) ([]*gitlab.MergeRequestDiff, error) {
	return m.diffs, nil
}
func (m *mockClient) GetPipeline(ctx context.Context, projectID, pipelineID int) (*gitlab.Pipeline, error) {
	return m.pipeline, nil
}
func (m *mockClient) ListPipelineJobs(ctx context.Context, projectID, pipelineID int) ([]*gitlab.Job, error) {
	return m.jobs, nil
}
func (m *mockClient) FindMergeRequestForBranch(ctx context.Context, projectID int, branch string) (*gitlab.MergeRequest, error) {
	return nil, gitlab.ErrNotFound
}
func (m *mockClient) ListCommits(ctx context.Context, projectID int, opts gitlab.ListCommitsOptions) ([]gitlab.Commit, error) {
	return m.commits, nil
}
func (m *mockClient) ListMyContributionEvents(ctx context.Context, opts gitlab.ListContributionEventsOptions) ([]gitlab.ContributionEvent, error) {
	m.lastActivityOpts = opts
	return m.activity, nil
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
	m := New(Deps{
		RepoRoot: "/repo",
		NewClient: func(config.Profile) (gitlab.Client, error) {
			return client, nil
		},
	})
	updated, _ := m.Update(dashboardLoadedMsg{ctx: testDashContext()})
	got := updated.(Model)
	if got.loading {
		t.Fatal("expected loading=false after dashboardLoadedMsg")
	}
	if got.client == nil {
		t.Fatal("expected client to be set after dashboardLoadedMsg")
	}
	// The dashboard now loads "as one": the stats phase still pending keeps the
	// spinner up (dashLoading). Deliver an (empty) stats message so the fixture
	// represents a fully-loaded dashboard, which is what most tests render.
	settled, _ := got.Update(dashboardStatsLoadedMsg{})
	return settled.(Model)
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

func TestHandleKey_DashboardEnterLoadsDetail(t *testing.T) {
	client := &mockClient{
		detail: &gitlab.MergeRequest{IID: 251, Title: "Alinha ao commons", SourceBranch: "bugfix-PD-26527", TargetBranch: "master"},
	}
	m := loadedModel(t, client)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected loadMRDetailCmd from dashboard")
	}
	msg := cmd()
	if _, ok := msg.(mrDetailLoadedMsg); !ok {
		t.Fatalf("expected mrDetailLoadedMsg, got %T", msg)
	}
}

func TestHandleKey_DiffViewer(t *testing.T) {
	client := &mockClient{
		detail: &gitlab.MergeRequest{IID: 251, Title: "Alinha ao commons"},
	}
	m := loadedModel(t, client)
	m.screen = screenDetail
	m.detail = client.detail
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if dir != "/repo" {
			t.Errorf("glab dir = %q, want /repo", dir)
		}
		wantArgs := []string{"mr", "diff", "251", "--color=never"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Errorf("glab args = %#v, want %#v", args, wantArgs)
		}
		return []byte("diff --git a/old.go b/new.go\n@@ -1 +1 @@\n-old\n+new\n"), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected loadMRDiffCmd")
	}
	msg := cmd()
	diffMsg, ok := msg.(mrDiffLoadedMsg)
	if !ok {
		t.Fatalf("expected mrDiffLoadedMsg, got %T", msg)
	}
	updated, _ = m.Update(diffMsg)
	m = updated.(Model)
	if m.screen != screenDiff {
		t.Errorf("screen = %v, want screenDiff", m.screen)
	}
	view := m.View()
	for _, want := range []string{"Diff !251", "diff --git a/old.go b/new.go", "-old", "+new"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (diff) missing %q\n%s", want, view)
		}
	}
}

func TestDiffViewer_NavigatesFilesAndHunks(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiff
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "Review diff", SourceBranch: "feature/diff-view", TargetBranch: "main"}
	m.height = 12
	m.width = 100
	m.diff = newDiffState([]*gitlab.MergeRequestDiff{{
		Diff: strings.Join([]string{
			"diff --git a/one.go b/one.go",
			"--- a/one.go",
			"+++ b/one.go",
			"@@ -1 +1 @@",
			"-old",
			"+new",
			"@@ -20 +20 @@",
			"-old two",
			"+new two",
			"diff --git a/two.go b/two.go",
			"--- a/two.go",
			"+++ b/two.go",
			"@@ -1 +1 @@",
			"-before",
			"+after",
		}, "\n"),
	}})

	if len(m.diff.files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(m.diff.files))
	}
	if got := m.diff.files[0].path; got != "one.go" {
		t.Fatalf("first file path = %q, want one.go", got)
	}
	if got := len(m.diff.files[0].hunks); got != 2 {
		t.Fatalf("first file hunks = %d, want 2", got)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = updated.(Model)
	if m.diff.fileCursor != 1 {
		t.Fatalf("fileCursor = %d, want 1", m.diff.fileCursor)
	}
	if !strings.Contains(m.View(), "two.go") {
		t.Fatalf("View() missing selected second file\n%s", m.View())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m = updated.(Model)
	if m.diff.hunkCursor != 1 {
		t.Fatalf("hunkCursor = %d, want 1", m.diff.hunkCursor)
	}
	if m.diff.lineOffset == 0 {
		t.Fatal("expected lineOffset to move when jumping to next hunk")
	}
}

func TestHandleKey_DiffCopyLineAndHunk(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiff
	m.detail = &gitlab.MergeRequest{IID: 251}
	m.diff = newDiffState([]*gitlab.MergeRequestDiff{{
		Diff: strings.Join([]string{
			"diff --git a/one.go b/one.go",
			"--- a/one.go",
			"+++ b/one.go",
			"@@ -1 +1 @@",
			"-old",
			"+new",
		}, "\n"),
	}})
	m.diff.lineOffset = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a copy command for 'y' on the diff screen")
	}
	msg := cmd()
	status, ok := msg.(statusMsg)
	if !ok || !strings.Contains(status.text, "Line copied") {
		t.Fatalf("expected line-copied status, got %#v", msg)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a copy command for 'Y' on the diff screen")
	}
	msg = cmd()
	status, ok = msg.(statusMsg)
	if !ok || !strings.Contains(status.text, "Hunk copied") {
		t.Fatalf("expected hunk-copied status, got %#v", msg)
	}
}

func TestFinalizeDiffFile_TruncatesVeryLargeFiles(t *testing.T) {
	lines := make([]string, maxDiffFileLines+100)
	lines[0] = "diff --git a/big.txt b/big.txt"
	for i := 1; i < len(lines); i++ {
		lines[i] = fmt.Sprintf("+line %d", i)
	}

	files := parseRawGitDiff(strings.Join(lines, "\n"))
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	file := files[0]
	if len(file.lines) != maxDiffFileLines+1 { // +1 for the truncation note
		t.Errorf("len(file.lines) = %d, want %d", len(file.lines), maxDiffFileLines+1)
	}
	if !strings.Contains(file.lines[len(file.lines)-1], "truncated") {
		t.Errorf("expected a truncation note as the last line, got %q", file.lines[len(file.lines)-1])
	}
	if file.additions != len(lines)-1 {
		t.Errorf("additions = %d, want %d (stats should reflect the full diff, not the truncated view)", file.additions, len(lines)-1)
	}
	found := false
	for _, f := range file.flags {
		if f == "truncated" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected flags to include \"truncated\", got %v", file.flags)
	}
}

func TestDiffViewer_SearchesAcrossFiles(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiff
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "Review diff"}
	m.height = 12
	m.width = 100
	m.diff = newDiffState([]*gitlab.MergeRequestDiff{{
		Diff: strings.Join([]string{
			"diff --git a/one.go b/one.go",
			"@@ -1 +1 @@",
			"-old",
			"+new",
			"diff --git a/two.go b/two.go",
			"@@ -1 +1 @@",
			"-before",
			"+target line",
		}, "\n"),
	}})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(Model)
	if !m.diffSearchActive {
		t.Fatal("expected search input to be active")
	}
	for _, r := range "target" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.diffSearchActive {
		t.Fatal("expected search input to close after enter")
	}
	if m.diff.fileCursor != 1 {
		t.Fatalf("fileCursor = %d, want 1", m.diff.fileCursor)
	}
	if m.diff.searchQuery != "target" {
		t.Fatalf("searchQuery = %q, want target", m.diff.searchQuery)
	}
	if len(m.diff.searchMatches) != 1 {
		t.Fatalf("matches = %d, want 1", len(m.diff.searchMatches))
	}
	if !strings.Contains(m.status, "Match 1/1") {
		t.Fatalf("status = %q, want match status", m.status)
	}
}

func TestDiffState_ParsesGitLabFileDiffs(t *testing.T) {
	state := newDiffState([]*gitlab.MergeRequestDiff{{
		OldPath:     "old.go",
		NewPath:     "new.go",
		Diff:        "@@ -1 +1 @@\n-old\n+new\n",
		RenamedFile: true,
	}})

	if len(state.files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(state.files))
	}
	file := state.files[0]
	if file.path != "new.go" {
		t.Errorf("path = %q, want new.go", file.path)
	}
	if file.additions != 1 || file.deletions != 1 {
		t.Errorf("stats = +%d -%d, want +1 -1", file.additions, file.deletions)
	}
	if len(file.hunks) != 1 {
		t.Errorf("hunks = %d, want 1", len(file.hunks))
	}
	if !reflect.DeepEqual(file.flags, []string{"renamed"}) {
		t.Errorf("flags = %#v, want renamed", file.flags)
	}
}

func TestCurrentURL_DiffFile(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiff
	m.detail = &gitlab.MergeRequest{
		IID:          251,
		WebURL:       "https://gitlab.services.betha.cloud/team/service/-/merge_requests/251",
		SourceBranch: "feature/diff/view",
		TargetBranch: "main",
	}
	m.diff = newDiffState([]*gitlab.MergeRequestDiff{{
		Diff: "diff --git a/internal/tui/diff view.go b/internal/tui/diff view.go\n@@ -1 +1 @@\n-old\n+new\n",
	}})

	want := "https://gitlab.services.betha.cloud/team/service/-/blob/feature%2Fdiff%2Fview/internal/tui/diff%20view.go"
	if got := m.currentURL(); got != want {
		t.Fatalf("currentURL() = %q, want %q", got, want)
	}
}

func TestEditorCommand_UsesEditorEnvAndRepoRoot(t *testing.T) {
	t.Setenv("EDITOR", "myeditor")
	deps := Deps{RepoRoot: "/repo"}
	cmd := editorCommand(deps, "internal/tui/model.go")
	wantArgs := []string{"myeditor", filepath.Join("/repo", "internal/tui/model.go")}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Errorf("args = %#v, want %#v", cmd.Args, wantArgs)
	}
}

func TestEditorCommand_DefaultsToVi(t *testing.T) {
	t.Setenv("EDITOR", "")
	cmd := editorCommand(Deps{RepoRoot: "/repo"}, "main.go")
	if cmd.Args[0] != "vi" {
		t.Errorf("args[0] = %q, want vi", cmd.Args[0])
	}
}

func TestHandleKey_DiffOpenEditor(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiff
	m.detail = &gitlab.MergeRequest{IID: 251}
	m.diff = newDiffState([]*gitlab.MergeRequestDiff{{
		Diff: "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
	}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected openEditorCmd for the diff screen")
	}
	// cmd() just returns bubbletea's internal exec-wrapper message (tested by
	// bubbletea itself); confirm our wiring doesn't panic when invoked.
	_ = cmd()
}

func TestIsWhitespaceOnlyDiffLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"+   ", true},
		{"+", true},
		{"-\t", true},
		{"+real content", false},
		{"-real content", false},
		{" context line", false},
		{"+++ b/file.go", false},
		{"--- a/file.go", false},
		{"@@ -1 +1 @@", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isWhitespaceOnlyDiffLine(c.line); got != c.want {
			t.Errorf("isWhitespaceOnlyDiffLine(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestHandleKey_DiffWhitespaceToggle(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiff
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "Trim trailing space"}
	m.height = 20
	m.width = 100
	m.diff = newDiffState([]*gitlab.MergeRequestDiff{{
		Diff: strings.Join([]string{
			"diff --git a/main.go b/main.go",
			"@@ -1,2 +1,2 @@",
			"-real change",
			"+real change fixed",
			"-   ",
			"+",
		}, "\n"),
	}})

	if strings.Contains(m.View(), "whitespace only") {
		t.Fatal("did not expect whitespace placeholder before toggling")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = updated.(Model)
	if !m.diffHideWhitespace {
		t.Fatal("expected diffHideWhitespace to be enabled after pressing w")
	}

	view := m.View()
	if !strings.Contains(view, "whitespace hidden") {
		t.Errorf("View() missing the whitespace-hidden title marker\n%s", view)
	}
	if !strings.Contains(view, "whitespace only") {
		t.Errorf("View() missing the collapsed whitespace-only line\n%s", view)
	}
	if !strings.Contains(view, "real change fixed") {
		t.Errorf("View() should still show real content changes\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = updated.(Model)
	if m.diffHideWhitespace {
		t.Fatal("expected diffHideWhitespace to toggle back off")
	}
}

func TestBuildSideBySideRows(t *testing.T) {
	lines := []string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,5 +1,5 @@",
		" context1",
		"-removed1",
		"-removed2",
		"-removed3",
		"+added1",
		" context2",
		"+added-only",
	}
	rows := buildSideBySideRows(lines)
	want := []sideBySideRow{
		{span: "@@ -1,5 +1,5 @@"},
		{left: " context1", right: " context1"},
		{left: "-removed1", right: "+added1"},
		{left: "-removed2", right: ""},
		{left: "-removed3", right: ""},
		{left: " context2", right: " context2"},
		{left: "", right: "+added-only"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("buildSideBySideRows() = %#v, want %#v", rows, want)
	}
}

func TestHandleKey_DiffSideBySideToggle(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiff
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "Refactor helper"}
	m.height = 20
	m.width = 180
	m.diff = newDiffState([]*gitlab.MergeRequestDiff{{
		Diff: strings.Join([]string{
			"diff --git a/main.go b/main.go",
			"@@ -1 +1 @@",
			"-return oldValue",
			"+return newValue",
		}, "\n"),
	}})

	if strings.Contains(m.View(), "[side-by-side]") {
		t.Fatal("did not expect side-by-side marker before toggling")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if !m.diffSideBySide {
		t.Fatal("expected diffSideBySide to be enabled after pressing s")
	}

	view := m.View()
	if !strings.Contains(view, "[side-by-side]") {
		t.Errorf("View() missing the side-by-side title marker\n%s", view)
	}
	if !strings.Contains(view, "return oldValue") || !strings.Contains(view, "return newValue") {
		t.Errorf("View() should show both sides of the change\n%s", view)
	}
	if !strings.Contains(view, "OLD  main.go") || !strings.Contains(view, "NEW  main.go") {
		t.Errorf("View() should show compact side-by-side headers\n%s", view)
	}
	if strings.Contains(view, "diff --git") {
		t.Errorf("View() should not render git metadata inside side-by-side columns\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if m.diffSideBySide {
		t.Fatal("expected diffSideBySide to toggle back off")
	}
}

func TestHandleKey_PipelineViewer(t *testing.T) {
	client := &mockClient{
		detail: &gitlab.MergeRequest{IID: 251, Title: "Alinha ao commons", Pipeline: &gitlab.Pipeline{ID: 3237626}},
	}
	m := loadedModel(t, client)
	m.screen = screenDetail
	m.detail = client.detail
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		wantArgs := []string{"ci", "get", "--pipeline-id", "3237626", "--with-job-details", "--output", "json"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Errorf("glab args = %#v, want %#v", args, wantArgs)
		}
		return []byte(`{
			"id": 3237626,
			"status": "failed",
			"source": "merge_request_event",
			"ref": "refs/merge-requests/251/head",
			"duration": 125,
			"jobs": [
				{"id": 10, "stage": "test", "name": "unit tests", "status": "failed", "duration": 61, "failure_reason": "script_failure"},
				{"id": 11, "stage": "build", "name": "docker", "status": "success", "allow_failure": true}
			]
		}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected loadPipelineCmd")
	}
	msg := cmd()
	pipelineMsg, ok := msg.(pipelineLoadedMsg)
	if !ok {
		t.Fatalf("expected pipelineLoadedMsg, got %T", msg)
	}
	updated, _ = m.Update(pipelineMsg)
	m = updated.(Model)
	if m.screen != screenPipeline {
		t.Errorf("screen = %v, want screenPipeline", m.screen)
	}
	view := m.View()
	for _, want := range []string{"Pipeline #3237626", "failed", "merge_request_event", "Stage: test", "Stage: build", "unit tests", "script_failure", "allow_failure"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (pipeline) missing %q\n%s", want, view)
		}
	}
}

func TestHandleKey_JobLogViewer(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenPipeline
	m.jobs = []*gitlab.Job{
		{ID: 10, Name: "unit tests", Stage: "test", Status: gitlab.JobStatusFailed, WebURL: "https://gitlab.services.betha.cloud/team/service/-/jobs/10"},
		{ID: 11, Name: "docker", Stage: "build", Status: gitlab.JobStatusSuccess},
	}
	m.jobCursor = 0
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		wantArgs := []string{"api", "projects/:id/jobs/10/trace"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Errorf("glab args = %#v, want %#v", args, wantArgs)
		}
		return []byte("Running tests...\nassert 1 == 2\nError: test failed\nDone\n"), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected loadJobLogCmd")
	}
	msg := cmd()
	logMsg, ok := msg.(jobLogLoadedMsg)
	if !ok {
		t.Fatalf("expected jobLogLoadedMsg, got %T", msg)
	}
	updated, _ = m.Update(logMsg)
	m = updated.(Model)
	if m.screen != screenJobLog {
		t.Errorf("screen = %v, want screenJobLog", m.screen)
	}
	if m.jobLog.job.ID != 10 {
		t.Errorf("jobLog.job.ID = %d, want 10", m.jobLog.job.ID)
	}

	view := m.View()
	for _, want := range []string{"Log: unit tests", "Running tests...", "Error: test failed"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (job log) missing %q\n%s", want, view)
		}
	}

	if got := m.currentURL(); got != "https://gitlab.services.betha.cloud/team/service/-/jobs/10" {
		t.Errorf("currentURL() (job log) = %q", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenPipeline {
		t.Errorf("screen after esc = %v, want screenPipeline", m.screen)
	}
}

func TestHandleKey_PipelineJobActions(t *testing.T) {
	client := &mockClient{
		detail: &gitlab.MergeRequest{IID: 251, Pipeline: &gitlab.Pipeline{ID: 3237626}},
	}
	m := loadedModel(t, client)
	m.screen = screenPipeline
	m.detail = client.detail
	m.pipeline = &gitlab.Pipeline{ID: 3237626}
	m.jobs = []*gitlab.Job{{ID: 10, Name: "unit tests"}}
	m.jobCursor = 0

	var calls [][]string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	m = updated.(Model)
	retryJobMsg, ok := cmd().(pipelineActionDoneMsg)
	if !ok || !strings.Contains(retryJobMsg.status, "Retried job #10") {
		t.Fatalf("retry job message = %#v", retryJobMsg)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = updated.(Model)
	triggerMsg, ok := cmd().(pipelineActionDoneMsg)
	if !ok || !strings.Contains(triggerMsg.status, "Triggered job #10") {
		t.Fatalf("trigger job message = %#v", triggerMsg)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = updated.(Model)
	retryPipelineMsg, ok := cmd().(pipelineActionDoneMsg)
	if !ok || !strings.Contains(retryPipelineMsg.status, "Retried pipeline #3237626") {
		t.Fatalf("retry pipeline message = %#v", retryPipelineMsg)
	}

	wantCalls := [][]string{
		{"ci", "retry", "10"},
		{"ci", "trigger", "10"},
		{"api", "--method", "POST", "projects/:id/pipelines/3237626/retry"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("glab calls = %#v, want %#v", calls, wantCalls)
	}

	updated, cmd = m.Update(retryPipelineMsg)
	m = updated.(Model)
	if m.screen != screenPipeline || !m.loading {
		t.Fatalf("after action-done: screen=%v loading=%v, want screenPipeline+loading", m.screen, m.loading)
	}
	if cmd == nil {
		t.Fatal("expected a reload command after action done")
	}
}

func TestHandleKey_CancelPipelineConfirm(t *testing.T) {
	client := &mockClient{
		detail: &gitlab.MergeRequest{IID: 251, Pipeline: &gitlab.Pipeline{ID: 3237626}},
	}
	m := loadedModel(t, client)
	m.screen = screenPipeline
	m.detail = client.detail
	m.pipeline = &gitlab.Pipeline{ID: 3237626}

	var calls [][]string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("expected no immediate command before confirming")
	}
	if !m.confirmActive || !strings.Contains(m.confirmPrompt, "3237626") {
		t.Fatalf("confirmActive=%v confirmPrompt=%q, want an active prompt mentioning the pipeline id", m.confirmActive, m.confirmPrompt)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(Model)
	if m.confirmActive || cmd != nil || len(calls) != 0 {
		t.Fatalf("reject: confirmActive=%v cmd=%v calls=%#v, want closed/no-op", m.confirmActive, cmd, calls)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if m.confirmActive || cmd == nil {
		t.Fatalf("accept: confirmActive=%v cmd=%v, want closed with a command", m.confirmActive, cmd)
	}
	doneMsg, ok := cmd().(pipelineActionDoneMsg)
	if !ok || !strings.Contains(doneMsg.status, "Cancelled pipeline #3237626") {
		t.Fatalf("cancel message = %#v", doneMsg)
	}
	wantCalls := [][]string{{"ci", "cancel", "pipeline", "3237626"}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("glab calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestHandleKey_RetryJob_FromJobLogScreen(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenJobLog
	m.jobLog = newLogState(&gitlab.Job{ID: 42, Name: "flaky test"}, "some log output")

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected retryJobCmd")
	}
	doneMsg, ok := cmd().(pipelineActionDoneMsg)
	if !ok || !strings.Contains(doneMsg.status, "Retried job #42") {
		t.Fatalf("retry job message = %#v", doneMsg)
	}
	wantArgs := []string{"ci", "retry", "42"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestHandleKey_ApproveMR(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251}

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected approveMRCmd")
	}
	doneMsg, ok := cmd().(mrActionDoneMsg)
	if !ok || !strings.Contains(doneMsg.status, "Approved !251") {
		t.Fatalf("approve message = %#v", doneMsg)
	}
	wantArgs := []string{"mr", "approve", "251"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestHandleKey_RevokeApprovalConfirm(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251}

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("expected no immediate command before confirming")
	}
	if !m.confirmActive || !strings.Contains(m.confirmPrompt, "!251") {
		t.Fatalf("confirmActive=%v confirmPrompt=%q", m.confirmActive, m.confirmPrompt)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected the confirmed revoke command")
	}
	doneMsg, ok := cmd().(mrActionDoneMsg)
	if !ok || !strings.Contains(doneMsg.status, "Approval removed from !251") {
		t.Fatalf("revoke message = %#v", doneMsg)
	}
	wantArgs := []string{"mr", "revoke", "251"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestHandleKey_ToggleDraftReady(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, Draft: false}

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = updated.(Model)
	doneMsg, ok := cmd().(mrActionDoneMsg)
	if !ok || !strings.Contains(doneMsg.status, "as draft") {
		t.Fatalf("toggle-to-draft message = %#v", doneMsg)
	}
	wantArgs := []string{"mr", "update", "251", "--draft"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}

	m.detail = &gitlab.MergeRequest{IID: 251, Draft: true}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = updated.(Model)
	doneMsg, ok = cmd().(mrActionDoneMsg)
	if !ok || !strings.Contains(doneMsg.status, "as ready") {
		t.Fatalf("toggle-to-ready message = %#v", doneMsg)
	}
	wantArgs = []string{"mr", "update", "251", "--ready"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestHandleKey_MergeConfirm(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, TargetBranch: "main"}

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("M")})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("expected no immediate command before confirming merge")
	}
	if !m.confirmActive || !strings.Contains(m.confirmPrompt, "main") {
		t.Fatalf("confirmActive=%v confirmPrompt=%q, want it to mention the target branch", m.confirmActive, m.confirmPrompt)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	doneMsg, ok := cmd().(mrActionDoneMsg)
	if !ok || !strings.Contains(doneMsg.status, "Merged !251") {
		t.Fatalf("merge message = %#v", doneMsg)
	}
	wantArgs := []string{"mr", "merge", "251", "--yes"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func initGitRepoForCheckout(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", branch)
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-q", "-m", "init")
	return dir
}

func TestHandleKey_CheckoutBranch(t *testing.T) {
	repoDir := initGitRepoForCheckout(t, "checked-out-branch")
	m := loadedModel(t, &mockClient{})
	m.deps.RepoRoot = repoDir
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, SourceBranch: "checked-out-branch"}

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	// b first inspects the working tree; a clean tree reports not-dirty.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected preCheckoutCmd")
	}
	prepared, ok := cmd().(checkoutPreparedMsg)
	if !ok {
		t.Fatalf("expected checkoutPreparedMsg, got %T", prepared)
	}
	if prepared.dirty {
		t.Fatal("a clean repo should not report dirty")
	}

	// a clean tree proceeds straight to the checkout, no confirmation
	updated, cmd = m.Update(prepared)
	m = updated.(Model)
	if m.confirmActive {
		t.Fatal("clean tree should not prompt for confirmation")
	}
	if cmd == nil {
		t.Fatal("expected checkoutMRCmd after a clean pre-check")
	}
	checkedOut, ok := cmd().(mrCheckedOutMsg)
	if !ok {
		t.Fatalf("expected mrCheckedOutMsg, got %T", checkedOut)
	}
	if checkedOut.branch != "checked-out-branch" {
		t.Errorf("checkedOut.branch = %q, want checked-out-branch", checkedOut.branch)
	}
	wantArgs := []string{"mr", "checkout", "251"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}

	updated, _ = m.Update(checkedOut)
	m = updated.(Model)
	if m.deps.Branch != "checked-out-branch" {
		t.Errorf("deps.Branch = %q, want checked-out-branch", m.deps.Branch)
	}
	if m.dash.Branch != "checked-out-branch" {
		t.Errorf("dash.Branch = %q, want checked-out-branch", m.dash.Branch)
	}
}

func TestHandleKey_CheckoutBranch_DirtyTreePrompts(t *testing.T) {
	repoDir := initGitRepoForCheckout(t, "feature")
	if err := os.WriteFile(filepath.Join(repoDir, "wip.txt"), []byte("uncommitted"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := loadedModel(t, &mockClient{})
	m.deps.RepoRoot = repoDir
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, SourceBranch: "feature"}

	called := false
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		called = true
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = updated.(Model)
	prepared, ok := cmd().(checkoutPreparedMsg)
	if !ok || !prepared.dirty {
		t.Fatalf("expected a dirty checkoutPreparedMsg, got %#v", prepared)
	}

	updated, _ = m.Update(prepared)
	m = updated.(Model)
	if !m.confirmActive || !strings.Contains(m.confirmPrompt, "251") {
		t.Fatalf("dirty tree should prompt; confirmActive=%v prompt=%q", m.confirmActive, m.confirmPrompt)
	}
	if called {
		t.Fatal("glab checkout must not run before confirmation")
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected the confirmed checkout command")
	}
	if _, ok := cmd().(mrCheckedOutMsg); !ok {
		t.Fatalf("expected mrCheckedOutMsg after confirm, got %T", cmd())
	}
	if !called {
		t.Fatal("expected glab checkout to run after confirmation")
	}
}

func TestJobLog_SearchAndErrorJump(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenJobLog
	m.height = 20
	m.width = 100
	m.jobLog = newLogState(&gitlab.Job{ID: 10, Name: "unit tests"}, strings.Join([]string{
		"Running tests...",
		"test one ok",
		"Error: assertion failed",
		"test two ok",
		"Error: timeout",
		"Done",
	}, "\n"))

	if !reflect.DeepEqual(m.jobLog.errorMatches, []int{2, 4}) {
		t.Fatalf("errorMatches = %v, want [2 4]", m.jobLog.errorMatches)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(Model)
	if !m.jobLogSearchActive {
		t.Fatal("expected job log search input to be active")
	}
	for _, r := range "timeout" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.jobLogSearchActive {
		t.Fatal("expected job log search input to close after enter")
	}
	if !reflect.DeepEqual(m.jobLog.searchMatches, []int{4}) {
		t.Fatalf("searchMatches = %v, want [4]", m.jobLog.searchMatches)
	}
	if !strings.Contains(m.status, "Match 1/1") {
		t.Fatalf("status = %q, want match status", m.status)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(Model)
	if got := m.jobLog.errorMatches[m.jobLog.errorCursor]; got != 2 {
		t.Fatalf("first error jump landed on line %d, want 2", got)
	}
	if !strings.Contains(m.status, "Error 1/2") {
		t.Fatalf("status = %q, want error status", m.status)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(Model)
	if got := m.jobLog.errorMatches[m.jobLog.errorCursor]; got != 4 {
		t.Fatalf("second error jump landed on line %d, want 4", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = updated.(Model)
	if got := m.jobLog.errorMatches[m.jobLog.errorCursor]; got != 2 {
		t.Fatalf("backward error jump landed on line %d, want 2", got)
	}
}

func TestLogState_ParsesTrace(t *testing.T) {
	raw := "section_start:1690000000:step_script\r\x1b[0K\x1b[36;1mRunning script\x1b[0;m\n" +
		"\x1b[32mok\x1b[0m: build passed\n" +
		"ERROR: something broke\n" +
		"section_end:1690000000:step_script\r\x1b[0K"

	state := newLogState(&gitlab.Job{ID: 1}, raw)

	want := []string{"Running script", "ok: build passed", "ERROR: something broke"}
	if !reflect.DeepEqual(state.lines, want) {
		t.Fatalf("lines = %#v, want %#v", state.lines, want)
	}
	if !reflect.DeepEqual(state.errorMatches, []int{2}) {
		t.Fatalf("errorMatches = %v, want [2]", state.errorMatches)
	}
	if state.errorCursor != -1 {
		t.Fatalf("errorCursor = %d, want -1 before first jump", state.errorCursor)
	}
}

func TestLogState_TruncatesVeryLargeLogsButKeepsRawForSaving(t *testing.T) {
	lines := make([]string, maxLogDisplayLines+50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	raw := strings.Join(lines, "\n")

	state := newLogState(&gitlab.Job{ID: 1}, raw)

	if state.truncated != 50 {
		t.Errorf("truncated = %d, want 50", state.truncated)
	}
	if len(state.lines) != maxLogDisplayLines {
		t.Errorf("len(lines) = %d, want %d", len(state.lines), maxLogDisplayLines)
	}
	if state.lines[0] != "line 50" {
		t.Errorf("first displayed line = %q, want the tail to be kept", state.lines[0])
	}
	if state.raw != raw {
		t.Error("raw should keep the full untruncated trace for saving")
	}
}

func TestLogState_CurrentLineText(t *testing.T) {
	state := newLogState(&gitlab.Job{ID: 1}, "one\ntwo\nthree")
	state.lineOffset = 1
	if got := state.currentLineText(); got != "two" {
		t.Errorf("currentLineText() = %q, want %q", got, "two")
	}

	state.search("three", 10)
	if got := state.currentLineText(); got != "three" {
		t.Errorf("currentLineText() with active search = %q, want %q", got, "three")
	}
}

func TestJobLogFollow_TogglesOnAndOff(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenJobLog
	m.jobLog = newLogState(&gitlab.Job{ID: 9, Status: gitlab.JobStatusRunning}, "log")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	got := updated.(Model)
	if !got.jobLogFollowing {
		t.Fatal("expected follow mode to turn on for a running job")
	}
	if cmd == nil {
		t.Fatal("expected a follow tick command to be scheduled")
	}

	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	got = updated.(Model)
	if got.jobLogFollowing {
		t.Error("expected follow mode to turn back off")
	}
	if cmd != nil {
		t.Error("expected no command when turning follow mode off")
	}
}

func TestJobLogFollow_RefusesForFinishedJob(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenJobLog
	m.jobLog = newLogState(&gitlab.Job{ID: 9, Status: gitlab.JobStatusSuccess}, "log")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if updated.(Model).jobLogFollowing {
		t.Error("expected follow mode to refuse a job that already finished")
	}
	if cmd != nil {
		t.Error("expected no command for a finished job")
	}
}

func TestJobLogFollowTick_StaleGenerationIsNoop(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenJobLog
	m.jobLogFollowing = true
	m.jobLogPollGen = 2
	m.jobLog = newLogState(&gitlab.Job{ID: 9, Status: gitlab.JobStatusRunning}, "log")

	_, cmd := m.Update(jobLogFollowTickMsg{gen: 1})
	if cmd != nil {
		t.Error("expected a stale-generation follow tick to be dropped")
	}
}

func TestJobLogLoaded_StopsFollowingWhenJobFinishes(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenJobLog
	m.jobLogFollowing = true
	m.jobLogPollGen = 1
	m.jobLog = newLogState(&gitlab.Job{ID: 9, Status: gitlab.JobStatusRunning}, "log")

	updated, cmd := m.Update(jobLogLoadedMsg{job: &gitlab.Job{ID: 9, Status: gitlab.JobStatusSuccess}, log: "log"})
	got := updated.(Model)
	if got.jobLogFollowing {
		t.Error("expected follow mode to stop once the job reaches a terminal status")
	}
	if cmd != nil {
		t.Error("expected no further poll command once follow mode stops")
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

	// The full project path only needs to appear once — the header already
	// shows it (shortened to its last two segments); the body no longer
	// repeats it, reclaiming those rows for the enriched MR card (D6).
	for _, want := range []string{"empresa", "cadastros/api-protocolo-cadastros", "bugfix-PD-26527", "!251", "failed"} {
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
	for _, want := range []string{"!1", "First MR", "!2", "Second MR, in draft", "DRAFT", cursorMarker} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (list) missing %q\n%s", want, view)
		}
	}
}

func TestApplyTheme_OpenCodeThemes(t *testing.T) {
	dark := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	t.Cleanup(func() {
		lipgloss.SetHasDarkBackground(dark)
		applyTheme("dark")
	})

	applyTheme("tokyo-night")
	if palette.accent != lipgloss.Color("#7aa2f7") {
		t.Fatalf("tokyo-night accent = %v, want #7aa2f7", palette.accent)
	}

	applyTheme("catppuccin")
	if palette.secondary != lipgloss.Color("#f38ba8") {
		t.Fatalf("catppuccin secondary = %v, want #f38ba8", palette.secondary)
	}
}

func TestApplyTheme_DarkUsesAdaptiveOCTheme(t *testing.T) {
	dark := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(dark) })

	applyTheme("dark")
	oc := ocByID[defaultAdaptiveTheme]
	if palette.accent != lipgloss.Color(oc.dark.primary) {
		t.Fatalf("dark accent = %v, want %v (adaptive %s theme)", palette.accent, oc.dark.primary, defaultAdaptiveTheme)
	}
}

func TestApplyTheme_TerminalIsExplicitCRTPalette(t *testing.T) {
	applyTheme("terminal")
	if palette.accent != lipgloss.Color("#00ff66") {
		t.Fatalf("terminal accent = %v, want #00ff66", palette.accent)
	}

	retroPalette := palette
	applyTheme("crt")
	if palette != retroPalette {
		t.Fatalf("crt palette = %+v, want %+v", palette, retroPalette)
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

func TestHandleKey_SidebarNavigationFallbackOnEmptyList(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenList
	m.mrs = nil

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command while selecting sidebar item, got %T", cmd)
	}
	if !m.navActive {
		t.Fatal("expected sidebar navigation to become active")
	}
	if m.navCursor != 0 {
		t.Fatalf("navCursor = %d, want Dashboard index 0", m.navCursor)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command when opening dashboard, got %T", cmd)
	}
	if m.screen != screenDashboard {
		t.Fatalf("screen = %v, want screenDashboard", m.screen)
	}
	if m.navActive {
		t.Fatal("expected sidebar navigation to close after activation")
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

func TestParseGLabDiscussions(t *testing.T) {
	raw := []map[string]any{
		{
			"id": "abc123",
			"notes": []any{
				map[string]any{
					"id":         1,
					"body":       "Please rename this",
					"system":     false,
					"resolvable": true,
					"resolved":   false,
					"created_at": "2026-07-01T10:00:00Z",
					"author":     map[string]any{"username": "reviewer", "name": "The Reviewer"},
				},
			},
		},
		{
			"id": "def456",
			"notes": []any{
				map[string]any{
					"id":     2,
					"body":   "marked this merge request as ready",
					"system": true,
					"author": map[string]any{"username": "author"},
				},
			},
		},
	}

	discussions := parseGLabDiscussions(raw)
	if len(discussions) != 2 {
		t.Fatalf("len(discussions) = %d, want 2", len(discussions))
	}
	first := discussions[0]
	if first.ID != "abc123" || len(first.Notes) != 1 {
		t.Fatalf("first discussion = %+v", first)
	}
	note := first.Notes[0]
	if note.Author != "reviewer" {
		t.Errorf("author = %q, want reviewer (username preferred over name)", note.Author)
	}
	if note.Body != "Please rename this" {
		t.Errorf("body = %q", note.Body)
	}
	if note.System || !note.Resolvable || note.Resolved {
		t.Errorf("flags: system=%v resolvable=%v resolved=%v, want false/true/false", note.System, note.Resolvable, note.Resolved)
	}
	if note.CreatedAt.IsZero() {
		t.Error("expected created_at to parse")
	}
	if !discussions[1].Notes[0].System {
		t.Error("expected the second discussion's note to be flagged as a system note")
	}
}

func TestHandleKey_DiscussionsViewer(t *testing.T) {
	client := &mockClient{
		detail: &gitlab.MergeRequest{IID: 251, Title: "Alinha ao commons", WebURL: "https://gitlab.services.betha.cloud/team/service/-/merge_requests/251"},
	}
	m := loadedModel(t, client)
	m.screen = screenDetail
	m.detail = client.detail
	m.height = 20
	m.width = 100

	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		wantArgs := []string{"api", "projects/:id/merge_requests/251/discussions?per_page=100", "--paginate"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Errorf("glab args = %#v, want %#v", args, wantArgs)
		}
		return []byte(`[
			{"id": "abc", "notes": [
				{"id": 1, "body": "please fix the typo", "system": false, "author": {"username": "reviewer"}, "created_at": "2026-07-01T10:00:00Z"}
			]}
		]`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected loadDiscussionsCmd")
	}
	msg := cmd()
	loaded, ok := msg.(discussionsLoadedMsg)
	if !ok {
		t.Fatalf("expected discussionsLoadedMsg, got %T", msg)
	}
	updated, _ = m.Update(loaded)
	m = updated.(Model)
	if m.screen != screenDiscussions {
		t.Errorf("screen = %v, want screenDiscussions", m.screen)
	}

	view := m.View()
	for _, want := range []string{"Discussions", "251", "reviewer", "please fix the typo"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (discussions) missing %q\n%s", want, view)
		}
	}

	if got := m.currentURL(); got != "https://gitlab.services.betha.cloud/team/service/-/merge_requests/251" {
		t.Errorf("currentURL() (discussions) = %q, want the MR URL", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenDetail {
		t.Errorf("screen after esc = %v, want screenDetail", m.screen)
	}
}

func TestHandleKey_PostComment(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiscussions
	m.detail = &gitlab.MergeRequest{IID: 251}
	m.height = 20
	m.width = 100

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(``), nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	if !m.commentActive {
		t.Fatal("expected the comment composer to be active after pressing c")
	}
	for _, r := range "looks good" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.commentActive {
		t.Fatal("expected the composer to close after enter")
	}
	if cmd == nil {
		t.Fatal("expected postCommentCmd")
	}
	posted, ok := cmd().(commentPostedMsg)
	if !ok || posted.iid != 251 {
		t.Fatalf("expected commentPostedMsg{iid:251}, got %#v", posted)
	}
	wantArgs := []string{"mr", "note", "251", "--message", "looks good"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestHandleKey_PostComment_EmptyIsNoop(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiscussions
	m.detail = &gitlab.MergeRequest{IID: 251}

	called := false
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command for an empty comment, got one returning %T", cmd())
	}
	if called {
		t.Fatal("glab should not run for an empty comment")
	}
	if m.commentActive {
		t.Fatal("composer should close even on empty input")
	}
}

func TestDiscussionView_FiltersSystemNotesAndMarksResolved(t *testing.T) {
	discussions := []gitlab.Discussion{
		{ID: "sys", Notes: []gitlab.Note{{Author: "gitlab", Body: "changed the description", System: true}}},
		{ID: "open", Notes: []gitlab.Note{{Author: "reviewer", Body: "rename this", Resolvable: true, Resolved: false}}},
		{ID: "done", Notes: []gitlab.Note{{Author: "reviewer", Body: "nit", Resolvable: true, Resolved: true}}},
		{ID: "plain", Notes: []gitlab.Note{{Author: "author", Body: "thanks"}}},
	}

	lines, threads := discussionView(discussions, 100)

	if len(threads) != 3 {
		t.Fatalf("len(threads) = %d, want 3 (the system-only thread should be filtered)", len(threads))
	}
	if threads[0].id != "open" || threads[1].id != "done" || threads[2].id != "plain" {
		t.Fatalf("thread ids = [%s %s %s], want [open done plain]", threads[0].id, threads[1].id, threads[2].id)
	}
	if threads[0].resolved {
		t.Error("open thread should report unresolved")
	}
	if !threads[1].resolved {
		t.Error("done thread should report resolved")
	}
	if threads[2].resolvable {
		t.Error("plain comment should not be resolvable")
	}

	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "changed the description") {
		t.Error("system note should be filtered out of the display")
	}
	if !strings.Contains(joined, "rename this") || !strings.Contains(joined, "thanks") {
		t.Error("human comments should be present in the display")
	}
	if lines[threads[0].headerLine] == "" {
		t.Error("a thread's header line should not be blank")
	}
}

func TestHandleKey_ResolveThread(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiscussions
	m.detail = &gitlab.MergeRequest{IID: 251}
	m.height = 20
	m.width = 100
	m.discuss = newDiscussState([]gitlab.Discussion{
		{ID: "abc", Notes: []gitlab.Note{{Author: "reviewer", Body: "please fix", Resolvable: true, Resolved: false}}},
	}, 100)

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected resolveDiscussionCmd")
	}
	done, ok := cmd().(discussionActionDoneMsg)
	if !ok || !strings.Contains(done.status, "Resolved") {
		t.Fatalf("resolve message = %#v", done)
	}
	wantArgs := []string{"api", "--method", "PUT", "projects/:id/merge_requests/251/discussions/abc?resolved=true"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestHandleKey_ReplyToThread(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiscussions
	m.detail = &gitlab.MergeRequest{IID: 251}
	m.discuss = newDiscussState([]gitlab.Discussion{
		{ID: "abc", Notes: []gitlab.Note{{Author: "reviewer", Body: "please fix"}}},
	}, 100)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("expected no immediate command, reply just opens the composer")
	}
	if !m.commentActive || m.commentReplyID != "abc" {
		t.Fatalf("expected composer open for reply to thread %q, got active=%v replyID=%q", "abc", m.commentActive, m.commentReplyID)
	}

	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{}`), nil
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ok")})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected replyDiscussionCmd")
	}
	if _, ok := cmd().(commentPostedMsg); !ok {
		t.Fatalf("expected commentPostedMsg, got %#v", cmd())
	}
	wantArgs := []string{"api", "--method", "POST", "projects/:id/merge_requests/251/discussions/abc/notes", "--field", "body=ok"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
	if m.commentReplyID != "" {
		t.Error("expected commentReplyID to reset after submitting")
	}
}

func TestHandleCommentKey_AltEnterInsertsNewline(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.commentActive = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = updated.(Model)

	if m.commentInput != "a\nb" {
		t.Fatalf("commentInput = %q, want %q", m.commentInput, "a\\nb")
	}
	if !m.commentActive {
		t.Error("expected composer to stay open after alt+enter")
	}
}

func TestHandleKey_ResolveThread_PlainCommentNoop(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiscussions
	m.detail = &gitlab.MergeRequest{IID: 251}
	m.discuss = newDiscussState([]gitlab.Discussion{
		{ID: "abc", Notes: []gitlab.Note{{Author: "author", Body: "just a comment"}}},
	}, 100)

	called := false
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command for a non-resolvable thread, got %T", cmd())
	}
	if called {
		t.Fatal("glab should not run when the selected thread is a plain comment")
	}
}

func TestMRSummary(t *testing.T) {
	mr := &gitlab.MergeRequest{
		IID: 251, Title: "Alinha ao commons",
		SourceBranch: "bugfix-PD-26527", TargetBranch: "master",
		Author: "arturo.burigo", State: gitlab.MergeRequestStateOpened,
		ApprovalsRequired: 2, ApprovalsGiven: 1,
		HasConflicts: true,
		Pipeline:     &gitlab.Pipeline{Status: gitlab.PipelineStatusFailed},
		WebURL:       "https://gitlab.services.betha.cloud/team/service/-/merge_requests/251",
	}

	got := mrSummary(mr, "namespace/project", summaryFormatPlain, summaryExtras{})
	for _, want := range []string{
		"!251 Alinha ao commons",
		"namespace/project",
		"bugfix-PD-26527 → master",
		"arturo.burigo",
		"opened",
		"failed",
		"1/2",
		"Conflicts: yes",
		"https://gitlab.services.betha.cloud/team/service/-/merge_requests/251",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("mrSummary missing %q\n---\n%s", want, got)
		}
	}
}

func TestMRSummary_OmitsEmptyFields(t *testing.T) {
	mr := &gitlab.MergeRequest{IID: 7, Title: "Minimal", SourceBranch: "a", TargetBranch: "b"}
	got := mrSummary(mr, "", summaryFormatPlain, summaryExtras{})
	for _, absent := range []string{"Project:", "Author:", "Pipeline:", "Approvals:", "Conflicts:"} {
		if strings.Contains(got, absent) {
			t.Errorf("mrSummary should omit %q for an empty field\n%s", absent, got)
		}
	}
}

func TestHandleKey_CopySummary(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "x", SourceBranch: "a", TargetBranch: "b"}

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")}); cmd == nil {
		t.Fatal("expected a copy-summary command when an MR is in context")
	}

	noMR := loadedModel(t, &mockClient{})
	noMR.screen = screenList
	noMR.detail = nil
	noMR.dash.MergeRequest = nil
	if _, cmd := noMR.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")}); cmd != nil {
		t.Fatalf("expected no command when no MR is in context, got %T", cmd())
	}
}

func TestRecordHistory_WritesProjectAndBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	recordHistory(path, testDashContext())

	store, err := history.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	projects := store.Projects("empresa")
	if len(projects) != 1 || projects[0].Path != "atendimento/protocolo/cadastros/api-protocolo-cadastros" {
		t.Fatalf("recorded projects = %+v", projects)
	}
	branches := store.Branches("empresa")
	if len(branches) != 1 || branches[0].Name != "bugfix-PD-26527" || branches[0].MRIID != 251 {
		t.Fatalf("recorded branches = %+v", branches)
	}
}

// firstBatchedMsg runs cmd and, if it's a tea.BatchMsg (dashboardLoadedMsg now
// fans out to both a history-recording command and a best-effort dashboard
// stats command via tea.Batch), returns the first sub-command's result whose
// type matches T.
func firstBatchedMsg[T any](t *testing.T, cmd tea.Cmd) (T, bool) {
	t.Helper()
	var zero T
	if cmd == nil {
		return zero, false
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			if v, ok := firstBatchedMsg[T](t, sub); ok {
				return v, true
			}
		}
		return zero, false
	}
	v, ok := msg.(T)
	return v, ok
}

func TestUpdate_DashboardLoaded_RecordsHistoryWhenConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	m := New(Deps{
		HistoryPath: path,
		NewClient:   func(config.Profile) (gitlab.Client, error) { return &mockClient{}, nil },
	})

	updated, cmd := m.Update(dashboardLoadedMsg{ctx: testDashContext()})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a record-history command when HistoryPath is set")
	}
	loaded, ok := firstBatchedMsg[historyLoadedMsg](t, cmd)
	if !ok {
		t.Fatal("expected a historyLoadedMsg among the batched dashboard-load commands")
	}
	if len(loaded.branches) != 1 || loaded.branches[0].MRIID != 251 {
		t.Errorf("recorded branches = %+v", loaded.branches)
	}

	// applying it lands the recents on the model for rendering
	updated, _ = m.Update(loaded)
	m = updated.(Model)
	if len(m.recentBranches) != 1 {
		t.Errorf("m.recentBranches = %+v, want one entry", m.recentBranches)
	}

	store, err := history.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(store.Branches("empresa")) != 1 {
		t.Errorf("expected one recorded branch on disk, got %+v", store.Branches("empresa"))
	}
}

func TestUpdate_DashboardLoaded_NoHistoryPathIsNoop(t *testing.T) {
	m := New(Deps{NewClient: func(config.Profile) (gitlab.Client, error) { return &mockClient{}, nil }})
	_, cmd := m.Update(dashboardLoadedMsg{ctx: testDashContext()})
	// The dashboard-stats command still fires regardless of HistoryPath (it's
	// independent, best-effort enrichment) — what must be absent is any
	// history-recording command.
	if _, ok := firstBatchedMsg[historyLoadedMsg](t, cmd); ok {
		t.Fatal("expected no history-recording command without a HistoryPath")
	}
}

func TestUpdate_DashboardStatsLoaded_PopulatesModelAndRenders(t *testing.T) {
	client := &mockClient{
		user:    &gitlab.User{Username: "arturo", Name: "Arturo Burigo"},
		commits: []gitlab.Commit{{ShortID: "abc1234", Title: "Fix retry logic", AuthorName: "Arturo Burigo", CreatedAt: time.Now()}},
		mrs: []*gitlab.MergeRequest{
			{IID: 1, State: gitlab.MergeRequestStateOpened},
			{IID: 2, State: gitlab.MergeRequestStateMerged},
			{IID: 3, State: gitlab.MergeRequestStateMerged},
		},
		assignedMRs: []*gitlab.MergeRequest{
			{IID: 42, Title: "Bump dependency", ProjectPath: "team/other-service", UpdatedAt: time.Now()},
		},
		activity: []gitlab.ContributionEvent{
			{Action: "opened", Target: "!101 Adjust invoice validation", CreatedAt: time.Now()},
		},
	}
	m := loadedModel(t, client)

	cmd := loadDashboardStatsCmd(m.client, m.dash)
	if cmd == nil {
		t.Fatal("expected a dashboard-stats command")
	}
	msg, ok := cmd().(dashboardStatsLoadedMsg)
	if !ok {
		t.Fatalf("expected dashboardStatsLoadedMsg, got %T", cmd())
	}

	updated, _ := m.Update(msg)
	m = updated.(Model)
	if len(m.dashCommits) != 1 || m.dashCommits[0].ShortID != "abc1234" {
		t.Errorf("dashCommits = %+v", m.dashCommits)
	}
	if len(m.dashMRs) != 3 {
		t.Errorf("dashMRs = %+v, want 3", m.dashMRs)
	}
	if len(m.dashAssignedMRs) != 1 || m.dashAssignedMRs[0].IID != 42 {
		t.Errorf("dashAssignedMRs = %+v", m.dashAssignedMRs)
	}
	if len(m.dashActivity) != 1 {
		t.Errorf("dashActivity = %+v, want 1", m.dashActivity)
	}

	view := m.View()
	for _, want := range []string{
		"Your recent commits", "abc1234", "Your merge requests", "opened", "merged",
		"Assigned to you", "team/other-service", "Bump dependency",
		"Contribution activity", time.Now().Format("January"), "1 total",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q\n%s", want, view)
		}
	}
}

func TestActivityHeatLevel_ScalesRelativeToBusiestDay(t *testing.T) {
	// A heavily-commenting day (60) shouldn't flatten a lighter one (5) to
	// the same top level the way a fixed "6+" cutoff would.
	const max = 60
	cases := map[int]int{0: 0, 5: 1, 15: 1, 30: 2, 45: 3, 60: 4}
	for count, want := range cases {
		if got := activityHeatLevel(count, max); got != want {
			t.Errorf("activityHeatLevel(%d, %d) = %d, want %d", count, max, got, want)
		}
	}
}

func TestActivityMonthStats(t *testing.T) {
	counts := map[int]int{1: 4, 2: 18, 3: 6} // today = 3: active streak of 3 days
	total, peakDay, peakCount, streak := activityMonthStats(counts, 3)
	if total != 28 || peakDay != 2 || peakCount != 18 || streak != 3 {
		t.Errorf("stats = (total %d, peak %d on day %d, streak %d), want (28, 18 on 2, 3)",
			total, peakCount, peakDay, streak)
	}
}

func TestActivityMonthStats_QuietTodayFallsBackToYesterday(t *testing.T) {
	// Nothing yet today (day 4) — the streak through yesterday still counts;
	// a quiet morning isn't a broken streak.
	counts := map[int]int{2: 3, 3: 5}
	_, _, _, streak := activityMonthStats(counts, 4)
	if streak != 2 {
		t.Errorf("streak = %d, want 2 (days 2-3, today still in progress)", streak)
	}
}

func TestActivityMonthStats_Empty(t *testing.T) {
	total, peakDay, peakCount, streak := activityMonthStats(map[int]int{}, 15)
	if total != 0 || peakDay != 0 || peakCount != 0 || streak != 0 {
		t.Errorf("stats on empty month = (%d, %d, %d, %d), want all zero",
			total, peakDay, peakCount, streak)
	}
}

func TestActivityHeatLevel_ZeroMaxIsLevelZero(t *testing.T) {
	if got := activityHeatLevel(0, 0); got != 0 {
		t.Errorf("activityHeatLevel(0, 0) = %d, want 0", got)
	}
	if got := activityHeatLevel(5, 0); got != 0 {
		t.Errorf("activityHeatLevel(5, 0) = %d, want 0 (no busiest day to scale against)", got)
	}
}

func TestActivityLevelColor_SpansThemeSubtleToGood(t *testing.T) {
	if got := activityLevelColor(0); got != palette.subtle {
		t.Errorf("activityLevelColor(0) = %v, want palette.subtle %v", got, palette.subtle)
	}
	if got := activityLevelColor(4); got != palette.good {
		t.Errorf("activityLevelColor(4) = %v, want palette.good %v", got, palette.good)
	}
}

func TestRenderContributionCalendar_CountsCurrentMonthOnly(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	m.dashActivity = []gitlab.ContributionEvent{
		{Action: "opened", CreatedAt: now},
		{Action: "commented on", CreatedAt: monthStart},             // the 1st, in-window
		{Action: "closed", CreatedAt: monthStart.AddDate(0, 0, -1)}, // last day of previous month, dropped
		{Action: "merged", CreatedAt: monthStart.AddDate(0, -1, 0)}, // a month earlier, dropped
	}

	view := m.View()
	if !strings.Contains(view, "Contribution activity") {
		t.Fatalf("View() missing contribution calendar header\n%s", view)
	}
	if !strings.Contains(view, now.Format("January")) {
		t.Errorf("View() should name the current month %q\n%s", now.Format("January"), view)
	}
	if !strings.Contains(view, "2 total") {
		t.Errorf("View() should count only the 2 current-month events\n%s", view)
	}
}

func TestDashboardLoadsAsOne(t *testing.T) {
	// "Load as one": the dashboard stays behind the spinner until BOTH phases
	// arrive — the context alone (phase 1) must not reveal it, only the stats
	// (phase 2) do — so the screen paints at once instead of popping in.
	m := New(Deps{NewClient: func(config.Profile) (gitlab.Client, error) { return &mockClient{}, nil }})
	if !m.dashLoading {
		t.Fatal("expected dashLoading=true from New")
	}
	afterCtx, _ := m.Update(dashboardLoadedMsg{ctx: testDashContext()})
	m = afterCtx.(Model)
	if !m.dashLoading {
		t.Error("expected dashLoading to stay true after dashboardLoadedMsg (stats still pending)")
	}
	if !strings.Contains(m.View(), "Loading") {
		t.Errorf("expected the spinner view while stats load, got %q", m.View())
	}
	afterStats, _ := m.Update(dashboardStatsLoadedMsg{})
	if afterStats.(Model).dashLoading {
		t.Error("expected dashLoading=false after dashboardStatsLoadedMsg")
	}
}

func TestSpinnerTickAdvancesOnlyWhileLoading(t *testing.T) {
	m := New(Deps{})
	m.dashLoading = true
	m.spinnerGen = 3

	advanced, cmd := m.Update(spinnerTickMsg{gen: 3})
	if got := advanced.(Model).spinnerFrame; got != 1 {
		t.Errorf("spinnerFrame = %d, want 1 after a matching tick", got)
	}
	if cmd == nil {
		t.Error("expected a follow-up tick while still loading")
	}

	// A stale-generation tick (from a superseded refresh) is a no-op.
	if stale, staleCmd := m.Update(spinnerTickMsg{gen: 2}); stale.(Model).spinnerFrame != 0 || staleCmd != nil {
		t.Error("stale-gen tick should neither advance the frame nor reschedule")
	}

	// Once loading is done, the tick stops rescheduling itself.
	m.dashLoading = false
	if done, doneCmd := m.Update(spinnerTickMsg{gen: 3}); done.(Model).spinnerFrame != 0 || doneCmd != nil {
		t.Error("tick after loading finished should stop")
	}
}

func TestRenderActivityFeed_ShowsRecentActionsBesideStrip(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	now := time.Now()
	m.dashActivity = []gitlab.ContributionEvent{
		{Action: "merged", Target: "PD-26112 group tasks", CreatedAt: now},
		{Action: "commented on", Target: "EX-9231 layout", CreatedAt: now.AddDate(0, 0, -1)},
		{Action: "pushed to", Target: "feature-PD-26112", CreatedAt: now.AddDate(0, 0, -2)},
	}

	view := m.View()
	for _, want := range []string{"Recent activity", "merged", "comment", "pushed", "PD-26112 group tasks"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q from the activity feed\n%s", want, view)
		}
	}
}

func TestActivityVerb(t *testing.T) {
	cases := map[string]string{
		"opened":       "opened",
		"closed":       "closed",
		"merged":       "merged",
		"accepted":     "merged",
		"commented on": "comment",
		"pushed to":    "pushed",
		"pushed new":   "pushed",
		"approved":     "approved",
	}
	for action, want := range cases {
		if got := activityVerb(action); got != want {
			t.Errorf("activityVerb(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestLoadDashboardStatsCmd_FetchesOpenAssignedMRsAcrossProjects(t *testing.T) {
	client := &mockClient{
		assignedMRs: []*gitlab.MergeRequest{{IID: 7, State: gitlab.MergeRequestStateOpened}},
	}
	cmd := loadDashboardStatsCmd(client, testDashContext())
	msg, ok := cmd().(dashboardStatsLoadedMsg)
	if !ok {
		t.Fatalf("expected dashboardStatsLoadedMsg, got %T", cmd())
	}
	if len(msg.assignedMRs) != 1 || msg.assignedMRs[0].IID != 7 {
		t.Errorf("assignedMRs = %+v, want the mock's single assigned MR", msg.assignedMRs)
	}
}

func TestLoadDashboardStatsCmd_ScopesActivityToCurrentMonth(t *testing.T) {
	client := &mockClient{
		activity: []gitlab.ContributionEvent{{Action: "opened"}},
	}
	cmd := loadDashboardStatsCmd(client, testDashContext())
	msg, ok := cmd().(dashboardStatsLoadedMsg)
	if !ok {
		t.Fatalf("expected dashboardStatsLoadedMsg, got %T", cmd())
	}
	if len(msg.activity) != 1 {
		t.Errorf("activity = %+v, want the mock's single event", msg.activity)
	}

	// After is the day before the 1st, so the 1st's own contributions (After
	// is date-exclusive) are still included.
	after := client.lastActivityOpts.After
	wantAfter := currentMonthStart(time.Now()).AddDate(0, 0, -1)
	if after.IsZero() || !after.Equal(wantAfter) {
		t.Errorf("After = %v, want %v (day before the current month's 1st)", after, wantAfter)
	}
}

func TestLoadDashboardStatsCmd_NoCurrentUserSkipsCommitsButStillFetchesMRs(t *testing.T) {
	client := &mockClient{
		mrs: []*gitlab.MergeRequest{{IID: 1, State: gitlab.MergeRequestStateOpened}},
	}
	cmd := loadDashboardStatsCmd(client, testDashContext())
	msg, ok := cmd().(dashboardStatsLoadedMsg)
	if !ok {
		t.Fatalf("expected dashboardStatsLoadedMsg, got %T", cmd())
	}
	if msg.commits != nil {
		t.Errorf("commits = %+v, want nil when CurrentUser is unavailable", msg.commits)
	}
	if len(msg.mrs) != 1 {
		t.Errorf("mrs = %+v, want 1 (MR fetch shouldn't depend on CurrentUser)", msg.mrs)
	}
}

func TestRecentItems_ExcludeCurrent(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.recentBranches = []history.Branch{{Name: "bugfix-PD-26527"}, {Name: "feature-x"}}
	m.recentProjects = []history.Project{
		{Path: "atendimento/protocolo/cadastros/api-protocolo-cadastros"},
		{Path: "team/other"},
	}
	if items := m.recentBranchItems(); len(items) != 1 || items[0].Name != "feature-x" {
		t.Errorf("recentBranchItems = %+v, want just feature-x (current branch excluded)", items)
	}
	if items := m.recentProjectItems(); len(items) != 1 || items[0].Path != "team/other" {
		t.Errorf("recentProjectItems = %+v, want just team/other (current project excluded)", items)
	}
}

func TestDashboard_RecentBranchNavigateAndOpen(t *testing.T) {
	client := &mockClient{detail: &gitlab.MergeRequest{IID: 99, Title: "Other MR"}}
	m := loadedModel(t, client)
	m.screen = screenDashboard
	m.recentBranches = []history.Branch{
		{Name: "bugfix-PD-26527", ProjectPath: "atendimento/protocolo/cadastros/api-protocolo-cadastros", MRIID: 251},
		{Name: "feature-x", ProjectPath: "atendimento/protocolo/cadastros/api-protocolo-cadastros", MRIID: 99, MRTitle: "Other MR"},
	}
	m.dashCursor = m.dashMinCursor() // -1: current-branch MR selected

	// j steps from the current MR into the recent list (feature-x; the current
	// branch is excluded from the list)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.dashCursor != 0 {
		t.Fatalf("dashCursor = %d, want 0 after j", m.dashCursor)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected loadMRDetailCmd for the selected recent branch")
	}
	if _, ok := cmd().(mrDetailLoadedMsg); !ok {
		t.Fatalf("expected mrDetailLoadedMsg, got %T", cmd())
	}

	// k steps back to the current-MR slot
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(Model)
	if m.dashCursor != -1 {
		t.Fatalf("dashCursor = %d, want -1 back at the current MR", m.dashCursor)
	}
}

func TestDashboard_RecentBranchOtherProjectNotOpenable(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDashboard
	m.recentBranches = []history.Branch{
		{Name: "feature-y", ProjectPath: "other/project", MRIID: 5, MRTitle: "elsewhere"},
	}
	m.dashCursor = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command for a branch in another project, got %T", cmd())
	}
	if !strings.Contains(m.status, "no open MR") {
		t.Fatalf("status = %q, want an explanation", m.status)
	}
}

func TestView_Dashboard_RecentCards(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDashboard
	m.height = 40
	m.width = 100
	m.recentBranches = []history.Branch{
		{Name: "feature-x", ProjectPath: "atendimento/protocolo/cadastros/api-protocolo-cadastros", MRIID: 99, MRTitle: "Other MR"},
	}
	m.recentProjects = []history.Project{{Path: "team/another-service"}}
	m.dashCursor = m.dashMinCursor()

	view := m.View()
	for _, want := range []string{"Recent branches", "feature-x", "!99", "Recent projects", "another-service"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (dashboard cards) missing %q\n%s", want, view)
		}
	}
}

func TestHandleKey_HelpOverlay(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "x"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp {
		t.Fatal("expected ? to open the help overlay")
	}
	view := m.View()
	if !strings.Contains(view, "Keybindings") {
		t.Errorf("help view missing 'Keybindings'\n%s", view)
	}
	if !strings.Contains(view, "MR !251") {
		t.Errorf("help overlay should preserve the current screen context\n%s", view)
	}
	for _, want := range []string{"approve", "merge", "comments"} {
		if !strings.Contains(view, want) {
			t.Errorf("help view missing the detail binding %q\n%s", want, view)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if m.showHelp {
		t.Fatal("expected ? to toggle help closed")
	}

	m.showHelp = true
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showHelp {
		t.Fatal("expected esc to close help")
	}
}

func TestHelpOverlay_DoesNotInterceptCommentInput(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDiscussions
	m.detail = &gitlab.MergeRequest{IID: 251}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)

	if m.showHelp {
		t.Fatal("? should not open help while composing a comment")
	}
	if m.commentInput != "?" {
		t.Errorf("commentInput = %q, want ?", m.commentInput)
	}
}

func TestView_ConfirmOverlay(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "x"}
	m.confirmActive = true
	m.confirmPrompt = "Merge !251 into main?"

	view := m.View()
	for _, want := range []string{"MR !251", "Confirm", "Merge !251 into main?", "enter/y"} {
		if !strings.Contains(view, want) {
			t.Errorf("confirm overlay missing %q\n%s", want, view)
		}
	}
	if strings.Contains(view, "[y]es / [n]o") {
		t.Errorf("confirm prompt should not be rendered in the footer anymore\n%s", view)
	}
}

func TestHandleKey_CommandPaletteExecutesFilteredCommand(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.detail = &gitlab.MergeRequest{IID: 251, Title: "x"}
	var gotArgs []string
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte("diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n"), nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(Model)
	if !m.commandPaletteActive {
		t.Fatal("expected ctrl+k to open the command palette")
	}
	view := m.View()
	for _, want := range []string{"Command Palette", "Open diff", "MR !251"} {
		if !strings.Contains(view, want) {
			t.Errorf("command palette view missing %q\n%s", want, view)
		}
	}

	for _, r := range "diff" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.commandPaletteActive {
		t.Fatal("expected enter to close the command palette")
	}
	if cmd == nil {
		t.Fatal("expected the filtered diff command to run")
	}
	if _, ok := cmd().(mrDiffLoadedMsg); !ok {
		t.Fatalf("expected mrDiffLoadedMsg, got %T", cmd())
	}
	wantArgs := []string{"mr", "diff", "251", "--color=never"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("glab args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestParseGLabCommits(t *testing.T) {
	raw := []map[string]any{
		{"short_id": "abc1234", "title": "Fix the bug", "author_name": "Arturo", "created_at": "2026-07-01T10:00:00Z"},
		{"short_id": "def5678", "title": "Add a test", "author_name": "Arturo"},
	}
	commits := parseGLabCommits(raw)
	if len(commits) != 2 {
		t.Fatalf("len = %d, want 2", len(commits))
	}
	if commits[0].ShortID != "abc1234" || commits[0].Title != "Fix the bug" || commits[0].AuthorName != "Arturo" {
		t.Errorf("commit[0] = %+v", commits[0])
	}
	if commits[0].CreatedAt.IsZero() {
		t.Error("expected created_at to parse")
	}
}

func TestHandleKey_CommitsViewer(t *testing.T) {
	client := &mockClient{detail: &gitlab.MergeRequest{IID: 251, Title: "x"}}
	m := loadedModel(t, client)
	m.screen = screenDetail
	m.detail = client.detail
	m.height = 20
	m.width = 100
	m.deps.RunGLab = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		wantArgs := []string{"api", "projects/:id/merge_requests/251/commits?per_page=100"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Errorf("glab args = %#v, want %#v", args, wantArgs)
		}
		return []byte(`[{"short_id":"abc1234","title":"Fix the bug","author_name":"Arturo","created_at":"2026-07-01T10:00:00Z"}]`), nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected loadCommitsCmd")
	}
	loaded, ok := cmd().(commitsLoadedMsg)
	if !ok {
		t.Fatalf("expected commitsLoadedMsg, got %T", cmd())
	}
	updated, _ = m.Update(loaded)
	m = updated.(Model)
	if m.screen != screenCommits {
		t.Errorf("screen = %v, want screenCommits", m.screen)
	}
	view := m.View()
	for _, want := range []string{"Commits", "abc1234", "Fix the bug"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() (commits) missing %q\n%s", want, view)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenDetail {
		t.Errorf("screen after esc = %v, want screenDetail", m.screen)
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

func TestPipelineLoaded_ActivePipelineSchedulesPoll(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.detail = &gitlab.MergeRequest{IID: 251, Pipeline: &gitlab.Pipeline{ID: 1}}

	// cmd is a tea.Tick command — invoking it would really sleep for
	// pipelinePollInterval, so this only checks that one was scheduled;
	// the gen-comparison logic itself is covered by the tests below.
	updated, cmd := m.Update(pipelineLoadedMsg{pipeline: &gitlab.Pipeline{ID: 1, Status: gitlab.PipelineStatusRunning}})
	got := updated.(Model)
	if got.pollGen != 1 {
		t.Errorf("pollGen = %d, want 1", got.pollGen)
	}
	if cmd == nil {
		t.Fatal("expected a poll tick command for a running pipeline")
	}
}

func TestPipelineLoaded_TerminalPipelineDoesNotPoll(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	_, cmd := m.Update(pipelineLoadedMsg{pipeline: &gitlab.Pipeline{ID: 1, Status: gitlab.PipelineStatusSuccess}})
	if cmd != nil {
		t.Error("expected no poll command for a terminal pipeline status")
	}
}

func TestPipelinePollTick_StaleGenerationIsNoop(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenPipeline
	m.pollGen = 2
	m.pipeline = &gitlab.Pipeline{ID: 1, Status: gitlab.PipelineStatusRunning}
	m.detail = &gitlab.MergeRequest{IID: 251, Pipeline: &gitlab.Pipeline{ID: 1}}

	_, cmd := m.Update(pipelinePollTickMsg{gen: 1})
	if cmd != nil {
		t.Error("expected a stale-generation tick to be dropped")
	}
}

func TestPipelinePollTick_WrongScreenIsNoop(t *testing.T) {
	m := loadedModel(t, &mockClient{})
	m.screen = screenDetail
	m.pollGen = 1
	m.pipeline = &gitlab.Pipeline{ID: 1, Status: gitlab.PipelineStatusRunning}
	m.detail = &gitlab.MergeRequest{IID: 251, Pipeline: &gitlab.Pipeline{ID: 1}}

	_, cmd := m.Update(pipelinePollTickMsg{gen: 1})
	if cmd != nil {
		t.Error("expected a tick to be dropped once the user has left the pipeline screen")
	}
}
