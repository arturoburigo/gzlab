// Package tui implements gitlab-tui's Bubble Tea interface: a dashboard
// showing the current branch's merge request, a project MR list, and MR
// detail — the "Primeira Slice Recomendada" from the product plan.
package tui

import (
	"fmt"
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arturoburigo/gitlab-tui/internal/config"
	"github.com/arturoburigo/gitlab-tui/internal/dashboard"
	"github.com/arturoburigo/gitlab-tui/internal/gitdetect"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
	"github.com/arturoburigo/gitlab-tui/internal/history"
)

type screen int

const (
	screenDashboard screen = iota
	screenList
	screenDetail
	screenDiff
	screenPipeline
	screenJobLog
	screenDiscussions
)

// Deps are the pre-resolved local-repo facts and constructors the TUI
// needs to bootstrap itself. Built by the CLI layer, which already ran
// git detection and config loading before starting the program.
type Deps struct {
	Config          *config.Config
	NewClient       dashboard.NewClientFunc
	Remote          *gitdetect.RemoteInfo
	RepoRoot        string
	Branch          string
	ProfileOverride string
	HistoryPath     string
	RunGLab         glabRunner
}

// Model is the root Bubble Tea model.
type Model struct {
	deps   Deps
	client gitlab.Client

	screen  screen
	loading bool
	err     error
	status  string

	dash *dashboard.Context

	recentProjects []history.Project
	recentBranches []history.Branch
	dashCursor     int

	mrs    []*gitlab.MergeRequest
	cursor int

	detail *gitlab.MergeRequest

	diffs []*gitlab.MergeRequestDiff
	diff  diffState

	diffSearchActive   bool
	diffSearchInput    string
	diffHideWhitespace bool
	diffSideBySide     bool

	pipeline  *gitlab.Pipeline
	jobs      []*gitlab.Job
	jobCursor int
	jobOffset int

	jobLog             logState
	jobLogSearchActive bool
	jobLogSearchInput  string

	discuss       discussState
	commentActive bool
	commentInput  string

	showHelp bool

	confirmActive bool
	confirmPrompt string
	confirmCmd    tea.Cmd

	width  int
	height int
}

// New builds the initial Model for deps. Loading starts once the model is
// handed to tea.NewProgram, which calls Init() for you.
func New(deps Deps) Model {
	return Model{deps: deps, loading: true}
}

func (m Model) Init() tea.Cmd {
	return loadDashboardCmd(m.deps)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardLoadedMsg:
		m.loading = false
		m.err = nil
		m.dash = msg.ctx
		client, err := m.deps.NewClient(msg.ctx.Profile)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.client = client
		return m, recordHistoryCmd(m.deps, msg.ctx)

	case historyLoadedMsg:
		m.recentProjects = msg.projects
		m.recentBranches = msg.branches
		m.dashCursor = m.dashMinCursor()
		return m, nil

	case mrListLoadedMsg:
		m.loading = false
		m.mrs = msg.mrs
		m.cursor = 0
		return m, nil

	case mrDetailLoadedMsg:
		m.loading = false
		m.detail = msg.mr
		m.screen = screenDetail
		return m, nil

	case mrDiffLoadedMsg:
		m.loading = false
		m.diffs = msg.diffs
		m.diff = newDiffState(msg.diffs)
		m.diffSearchActive = false
		m.diffSearchInput = ""
		m.screen = screenDiff
		return m, nil

	case pipelineLoadedMsg:
		m.loading = false
		m.pipeline = msg.pipeline
		m.jobs = msg.jobs
		m.jobCursor = 0
		m.jobOffset = 0
		m.screen = screenPipeline
		return m, nil

	case jobLogLoadedMsg:
		m.loading = false
		m.jobLog = newLogState(msg.job, msg.log)
		m.jobLogSearchActive = false
		m.jobLogSearchInput = ""
		m.screen = screenJobLog
		return m, nil

	case pipelineActionDoneMsg:
		m.status = msg.status
		m.loading = true
		m.screen = screenPipeline
		return m, loadPipelineCmd(m.deps, m.detail)

	case mrActionDoneMsg:
		m.status = msg.status
		m.loading = true
		return m, loadMRDetailCmd(m.client, m.dash.Project.ID, m.detail.IID)

	case mrCheckedOutMsg:
		m.loading = false
		m.deps.Branch = msg.branch
		if m.dash != nil {
			m.dash.Branch = msg.branch
		}
		m.status = "Checked out " + msg.branch
		return m, nil

	case discussionsLoadedMsg:
		m.loading = false
		m.discuss = newDiscussState(msg.discussions)
		m.commentActive = false
		m.commentInput = ""
		m.screen = screenDiscussions
		return m, nil

	case commentPostedMsg:
		m.status = "Comment posted."
		m.loading = true
		return m, loadDiscussionsCmd(m.deps, msg.iid)

	case discussionActionDoneMsg:
		m.status = msg.status
		m.loading = true
		return m, loadDiscussionsCmd(m.deps, msg.iid)

	case checkoutPreparedMsg:
		if msg.dirty {
			m.loading = false
			m.confirmActive = true
			m.confirmPrompt = fmt.Sprintf("Uncommitted changes present. Check out !%d anyway?", msg.iid)
			m.confirmCmd = checkoutMRCmd(m.deps, msg.iid)
			return m, nil
		}
		return m, checkoutMRCmd(m.deps, msg.iid)

	case statusMsg:
		m.status = msg.text
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmActive {
		return m.handleConfirmKey(msg)
	}
	if m.diffSearchActive {
		return m.handleDiffSearchKey(msg)
	}
	if m.jobLogSearchActive {
		return m.handleJobLogSearchKey(msg)
	}
	if m.commentActive {
		return m.handleCommentKey(msg)
	}
	if m.showHelp {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?", "esc":
			m.showHelp = false
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.showHelp = true

	case "esc":
		m.status = ""
		switch m.screen {
		case screenJobLog:
			m.screen = screenPipeline
		case screenDiff, screenPipeline, screenDiscussions:
			m.screen = screenDetail
		case screenDetail:
			m.screen = screenList
		case screenList:
			m.screen = screenDashboard
		}
		return m, nil

	case "r":
		if m.dash == nil {
			return m, nil
		}
		m.loading = true
		m.status = ""
		if m.screen == screenList {
			return m, loadMRListCmd(m.client, m.dash.Project.ID)
		}
		if m.screen == screenDetail && m.detail != nil {
			return m, loadMRDetailCmd(m.client, m.dash.Project.ID, m.detail.IID)
		}
		if m.screen == screenDiff && m.detail != nil {
			return m, loadMRDiffCmd(m.deps, m.detail.IID)
		}
		if m.screen == screenPipeline && m.detail != nil {
			return m, loadPipelineCmd(m.deps, m.detail)
		}
		if m.screen == screenJobLog && m.jobLog.job != nil {
			return m, loadJobLogCmd(m.deps, m.jobLog.job)
		}
		if m.screen == screenDiscussions && m.detail != nil {
			return m, loadDiscussionsCmd(m.deps, m.detail.IID)
		}
		return m, loadDashboardCmd(m.deps)

	case "m":
		if m.dash == nil || m.client == nil {
			return m, nil
		}
		m.screen = screenList
		m.loading = true
		m.status = ""
		return m, loadMRListCmd(m.client, m.dash.Project.ID)

	case "o":
		if url := m.currentURL(); url != "" {
			return m, openBrowserCmd(url)
		}

	case "y":
		if url := m.currentURL(); url != "" {
			return m, copyLinkCmd(url)
		}

	case "Y":
		if mr := m.summaryMR(); mr != nil {
			return m, copyToClipboardCmd(mrSummary(mr, m.projectPath()), "Summary copied to clipboard.")
		}

	case "up", "k":
		switch {
		case m.screen == screenList && m.cursor > 0:
			m.cursor--
		case m.screen == screenDiff:
			m.scrollDiff(-1)
		case m.screen == screenPipeline && m.jobCursor > 0:
			m.jobCursor--
			m.ensureJobVisible()
		case m.screen == screenJobLog:
			m.scrollJobLog(-1)
		case m.screen == screenDiscussions:
			m.discuss.moveCursor(-1, m.discussBodyHeight())
		case m.screen == screenDashboard && m.dashCursor > m.dashMinCursor():
			m.dashCursor--
		}

	case "down", "j":
		switch {
		case m.screen == screenList && m.cursor < len(m.mrs)-1:
			m.cursor++
		case m.screen == screenDiff:
			m.scrollDiff(1)
		case m.screen == screenPipeline && m.jobCursor < len(m.jobs)-1:
			m.jobCursor++
			m.ensureJobVisible()
		case m.screen == screenJobLog:
			m.scrollJobLog(1)
		case m.screen == screenDiscussions:
			m.discuss.moveCursor(1, m.discussBodyHeight())
		case m.screen == screenDashboard && m.dashCursor < m.dashMaxCursor():
			m.dashCursor++
		}

	case "left", "h":
		if m.screen == screenDiff {
			m.diff.moveFile(-1)
		}

	case "right", "l":
		if m.screen == screenDiff {
			m.diff.moveFile(1)
		}

	case "[":
		if m.screen == screenDiff {
			m.diff.moveHunk(-1, m.contentHeight())
		}

	case "]":
		if m.screen == screenDiff {
			m.diff.moveHunk(1, m.contentHeight())
		}

	case "/":
		if m.screen == screenDiff {
			m.diffSearchActive = true
			m.diffSearchInput = m.diff.searchQuery
			m.status = "/" + m.diffSearchInput
		}
		if m.screen == screenJobLog {
			m.jobLogSearchActive = true
			m.jobLogSearchInput = m.jobLog.searchQuery
			m.status = "/" + m.jobLogSearchInput
		}

	case "n":
		if m.screen == screenDiff {
			if m.diff.moveSearchMatch(1, m.contentHeight()) {
				m.status = m.diff.searchStatus()
			}
		}
		if m.screen == screenJobLog {
			if m.jobLog.moveSearchMatch(1, m.logBodyHeight()) {
				m.status = m.jobLog.searchStatus()
			}
		}

	case "N":
		if m.screen == screenDiff {
			if m.diff.moveSearchMatch(-1, m.contentHeight()) {
				m.status = m.diff.searchStatus()
			}
		}
		if m.screen == screenJobLog {
			if m.jobLog.moveSearchMatch(-1, m.logBodyHeight()) {
				m.status = m.jobLog.searchStatus()
			}
		}

	case "e":
		if m.screen == screenJobLog {
			if m.jobLog.moveErrorMatch(1, m.logBodyHeight()) {
				m.status = m.jobLog.errorStatus()
			}
		}
		if m.screen == screenDiff {
			if file := m.diff.currentFile(); file != nil && file.path != "" {
				return m, openEditorCmd(m.deps, file.path)
			}
		}

	case "E":
		if m.screen == screenJobLog {
			if m.jobLog.moveErrorMatch(-1, m.logBodyHeight()) {
				m.status = m.jobLog.errorStatus()
			}
		}

	case "w":
		if m.screen == screenDiff {
			m.diffHideWhitespace = !m.diffHideWhitespace
		}
		if m.screen == screenDetail && m.detail != nil {
			m.loading = true
			m.status = ""
			return m, toggleMRDraftCmd(m.deps, m.detail.IID, !m.detail.Draft)
		}

	case "s":
		if m.screen == screenDiff {
			m.diffSideBySide = !m.diffSideBySide
		}

	case "a":
		if m.screen == screenDetail && m.detail != nil {
			m.loading = true
			m.status = ""
			return m, approveMRCmd(m.deps, m.detail.IID)
		}

	case "A":
		if m.screen == screenDetail && m.detail != nil {
			m.confirmActive = true
			m.confirmPrompt = fmt.Sprintf("Remove your approval from !%d?", m.detail.IID)
			m.confirmCmd = revokeMRApprovalCmd(m.deps, m.detail.IID)
		}

	case "M":
		if m.screen == screenDetail && m.detail != nil {
			m.confirmActive = true
			m.confirmPrompt = fmt.Sprintf("Merge !%d into %s?", m.detail.IID, m.detail.TargetBranch)
			m.confirmCmd = mergeMRCmd(m.deps, m.detail.IID)
		}

	case "b":
		if m.screen == screenDetail && m.detail != nil {
			m.loading = true
			m.status = ""
			return m, preCheckoutCmd(m.deps, m.detail.IID)
		}

	case "c":
		if m.screen == screenDetail && m.detail != nil {
			m.loading = true
			m.status = ""
			return m, loadDiscussionsCmd(m.deps, m.detail.IID)
		}
		if m.screen == screenDiscussions && m.detail != nil {
			m.commentActive = true
			m.commentInput = ""
			m.status = "comment: "
		}

	case "enter":
		if m.screen == screenList && len(m.mrs) > 0 {
			selected := m.mrs[m.cursor]
			m.loading = true
			return m, loadMRDetailCmd(m.client, m.dash.Project.ID, selected.IID)
		}
		if m.screen == screenDashboard {
			items := m.recentBranchItems()
			if m.dashCursor >= 0 && m.dashCursor < len(items) {
				sel := items[m.dashCursor]
				if sel.MRIID != 0 && m.dash != nil && m.dash.Project != nil && sel.ProjectPath == m.dash.Project.PathWithNamespace {
					m.loading = true
					return m, loadMRDetailCmd(m.client, m.dash.Project.ID, sel.MRIID)
				}
				m.status = "That recent branch has no open MR in this project."
				return m, nil
			}
			if m.dash != nil && m.dash.MergeRequest != nil {
				m.loading = true
				return m, loadMRDetailCmd(m.client, m.dash.Project.ID, m.dash.MergeRequest.IID)
			}
		}
		if m.screen == screenPipeline && m.jobCursor >= 0 && m.jobCursor < len(m.jobs) {
			m.loading = true
			m.status = ""
			return m, loadJobLogCmd(m.deps, m.jobs[m.jobCursor])
		}

	case "d":
		if m.dash == nil || m.detail == nil {
			return m, nil
		}
		m.loading = true
		m.status = ""
		return m, loadMRDiffCmd(m.deps, m.detail.IID)

	case "p":
		if m.dash == nil || m.detail == nil || m.detail.Pipeline == nil {
			return m, nil
		}
		m.loading = true
		m.status = ""
		return m, loadPipelineCmd(m.deps, m.detail)

	case "g":
		if m.screen == screenDiff {
			m.diff.lineOffset = 0
		}
		if m.screen == screenPipeline {
			m.jobCursor = 0
			m.jobOffset = 0
		}
		if m.screen == screenJobLog {
			m.jobLog.lineOffset = 0
		}
		if m.screen == screenDiscussions {
			m.discuss.cursor = 0
			m.discuss.lineOffset = 0
		}

	case "G":
		if m.screen == screenDiff {
			m.diff.scrollToEnd(m.contentHeight())
		}
		if m.screen == screenPipeline && len(m.jobs) > 0 {
			m.jobCursor = len(m.jobs) - 1
			m.ensureJobVisible()
		}
		if m.screen == screenJobLog {
			m.jobLog.scrollToEnd(m.logBodyHeight())
		}
		if m.screen == screenDiscussions {
			if n := len(m.discuss.threads); n > 0 {
				m.discuss.cursor = n - 1
			}
			m.discuss.scrollToEnd(m.discussBodyHeight())
		}

	case "R":
		switch {
		case m.screen == screenPipeline && m.jobCursor >= 0 && m.jobCursor < len(m.jobs):
			m.loading = true
			m.status = ""
			return m, retryJobCmd(m.deps, m.jobs[m.jobCursor].ID)
		case m.screen == screenJobLog && m.jobLog.job != nil:
			m.loading = true
			m.status = ""
			return m, retryJobCmd(m.deps, m.jobLog.job.ID)
		}

	case "P":
		if m.screen == screenPipeline && m.pipeline != nil {
			m.loading = true
			m.status = ""
			return m, retryPipelineCmd(m.deps, m.pipeline.ID)
		}

	case "t":
		if m.screen == screenPipeline && m.jobCursor >= 0 && m.jobCursor < len(m.jobs) {
			m.loading = true
			m.status = ""
			return m, triggerJobCmd(m.deps, m.jobs[m.jobCursor].ID)
		}
		if m.screen == screenDiscussions && m.detail != nil {
			if thread, ok := m.discuss.currentThread(); ok && thread.resolvable {
				m.loading = true
				m.status = ""
				return m, resolveDiscussionCmd(m.deps, m.detail.IID, thread.id, !thread.resolved)
			}
		}

	case "x":
		var pipelineID int
		switch {
		case m.screen == screenPipeline && m.pipeline != nil:
			pipelineID = m.pipeline.ID
		case m.screen == screenDetail && m.detail != nil && m.detail.Pipeline != nil:
			pipelineID = m.detail.Pipeline.ID
		}
		if pipelineID != 0 {
			m.confirmActive = true
			m.confirmPrompt = fmt.Sprintf("Cancel pipeline #%d?", pipelineID)
			m.confirmCmd = cancelPipelineCmd(m.deps, pipelineID)
		}
	}
	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "y", "Y", "enter":
		cmd := m.confirmCmd
		m.confirmActive = false
		m.confirmPrompt = ""
		m.confirmCmd = nil
		m.loading = true
		m.status = ""
		return m, cmd
	default:
		m.confirmActive = false
		m.confirmPrompt = ""
		m.confirmCmd = nil
		m.status = "Cancelled."
		return m, nil
	}
}

func (m Model) handleDiffSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.diffSearchActive = false
		m.diffSearchInput = ""
		m.status = m.diff.searchStatus()
		return m, nil
	case "enter":
		m.diffSearchActive = false
		if m.diff.search(m.diffSearchInput, m.contentHeight()) {
			m.status = m.diff.searchStatus()
		} else {
			m.status = m.diff.searchStatus()
		}
		return m, nil
	case "backspace", "ctrl+h":
		if len(m.diffSearchInput) > 0 {
			runes := []rune(m.diffSearchInput)
			m.diffSearchInput = string(runes[:len(runes)-1])
		}
		m.status = "/" + m.diffSearchInput
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.diffSearchInput += string(msg.Runes)
		m.status = "/" + m.diffSearchInput
	}
	return m, nil
}

func (m Model) handleJobLogSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.jobLogSearchActive = false
		m.jobLogSearchInput = ""
		m.status = m.jobLog.searchStatus()
		return m, nil
	case "enter":
		m.jobLogSearchActive = false
		m.jobLog.search(m.jobLogSearchInput, m.logBodyHeight())
		m.status = m.jobLog.searchStatus()
		return m, nil
	case "backspace", "ctrl+h":
		if len(m.jobLogSearchInput) > 0 {
			runes := []rune(m.jobLogSearchInput)
			m.jobLogSearchInput = string(runes[:len(runes)-1])
		}
		m.status = "/" + m.jobLogSearchInput
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.jobLogSearchInput += string(msg.Runes)
		m.status = "/" + m.jobLogSearchInput
	}
	return m, nil
}

func (m Model) handleCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.commentActive = false
		m.commentInput = ""
		m.status = "Comment cancelled."
		return m, nil
	case "enter":
		body := strings.TrimSpace(m.commentInput)
		m.commentActive = false
		m.commentInput = ""
		if body == "" {
			m.status = "Empty comment, nothing posted."
			return m, nil
		}
		if m.detail == nil {
			return m, nil
		}
		m.loading = true
		m.status = ""
		return m, postCommentCmd(m.deps, m.detail.IID, body)
	case "backspace", "ctrl+h":
		if len(m.commentInput) > 0 {
			runes := []rune(m.commentInput)
			m.commentInput = string(runes[:len(runes)-1])
		}
		m.status = "comment: " + m.commentInput
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.commentInput += string(msg.Runes)
		m.status = "comment: " + m.commentInput
	}
	return m, nil
}

func (m *Model) scrollDiff(delta int) {
	m.diff.scroll(delta, m.contentHeight())
}

func (m *Model) scrollJobLog(delta int) {
	m.jobLog.scroll(delta, m.logBodyHeight())
}

func (m *Model) ensureJobVisible() {
	height := m.jobListHeight()
	if height <= 0 {
		return
	}
	if m.jobCursor < m.jobOffset {
		m.jobOffset = m.jobCursor
	}
	if m.jobCursor >= m.jobOffset+height {
		m.jobOffset = m.jobCursor - height + 1
	}
}

func (m Model) currentURL() string {
	switch m.screen {
	case screenDetail:
		if m.detail != nil {
			return m.detail.WebURL
		}
	case screenPipeline:
		if len(m.jobs) > 0 && m.jobCursor >= 0 && m.jobCursor < len(m.jobs) {
			return m.jobs[m.jobCursor].WebURL
		}
		if m.pipeline != nil {
			return m.pipeline.WebURL
		}
	case screenJobLog:
		if m.jobLog.job != nil {
			return m.jobLog.job.WebURL
		}
	case screenDiscussions:
		if m.detail != nil {
			return m.detail.WebURL
		}
	case screenDiff:
		return m.currentDiffFileURL()
	case screenDashboard:
		if m.dash != nil {
			if m.dash.MergeRequest != nil {
				return m.dash.MergeRequest.WebURL
			}
			if m.dash.Project != nil {
				return m.dash.Project.WebURL
			}
		}
	}
	return ""
}

func (m Model) currentDiffFileURL() string {
	if m.dash == nil || m.dash.Project == nil || m.detail == nil {
		return ""
	}
	file := m.diff.currentFile()
	if file == nil || file.path == "" {
		return m.detail.WebURL
	}
	ref := firstNonEmpty(m.detail.SourceBranch, m.dash.Branch)
	if file.isDeleted() {
		ref = firstNonEmpty(m.detail.TargetBranch, ref)
	}
	if ref == "" || m.dash.Project.WebURL == "" {
		return m.detail.WebURL
	}
	return strings.TrimRight(m.dash.Project.WebURL, "/") + "/-/blob/" + url.PathEscape(ref) + "/" + escapePathSegments(file.path)
}

func escapePathSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
