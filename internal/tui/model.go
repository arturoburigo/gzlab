// Package tui implements gzlab's Bubble Tea interface: a dashboard
// showing the current branch's merge request, a project MR list, and MR
// detail — the "Primeira Slice Recomendada" from the product plan.
package tui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arturoburigo/gzlab/internal/config"
	"github.com/arturoburigo/gzlab/internal/dashboard"
	"github.com/arturoburigo/gzlab/internal/gitdetect"
	"github.com/arturoburigo/gzlab/internal/gitlab"
	"github.com/arturoburigo/gzlab/internal/history"
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
	screenCommits
	screenSearch
	screenWorkspace
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
	WorkspacePath   string
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
	navCursor      int
	navActive      bool

	// dashLoading is the "load as one" gate: true from New/refresh until BOTH
	// the dashboard context and its best-effort stats have arrived, so the
	// whole dashboard paints at once behind a spinner instead of popping in
	// piecemeal. spinnerFrame/spinnerGen drive that spinner; spinnerGen is
	// bumped on each (re)load so a lingering tick from a finished load is
	// recognised as stale and stops itself.
	dashLoading  bool
	spinnerFrame int
	spinnerGen   int

	// dashCommits/dashMRs/dashAssignedMRs/dashActivity are the dashboard's
	// best-effort personal-stats enrichment (recent commits, MR state
	// breakdown, MRs assigned to the current user, recent cross-project
	// activity) — nil until dashboardStatsLoadedMsg arrives, and possibly
	// still empty after (a failed fetch is swallowed, not surfaced as an
	// error).
	dashCommits     []gitlab.Commit
	dashMRs         []*gitlab.MergeRequest
	dashAssignedMRs []*gitlab.MergeRequest
	dashActivity    []gitlab.ContributionEvent

	mrs      []*gitlab.MergeRequest
	cursor   int
	mrScope  mrScope
	mrFilter mrQuickFilter

	detail *gitlab.MergeRequest

	diffs []*gitlab.MergeRequestDiff
	diff  diffState

	diffSearchActive   bool
	diffSearchInput    string
	diffHideWhitespace bool
	diffSideBySide     bool

	pipeline          *gitlab.Pipeline
	jobs              []*gitlab.Job
	jobCursor         int
	jobOffset         int
	pollGen           int
	lastPipelineFetch time.Time

	jobLog             logState
	jobLogSearchActive bool
	jobLogSearchInput  string
	jobLogFollowing    bool
	jobLogPollGen      int
	lastJobLogFetch    time.Time

	discuss        discussState
	commentActive  bool
	commentInput   string
	commentReplyID string // empty: new top-level comment; set: replying to that discussion

	commits      []gitlab.Commit
	commitCursor int
	commitOffset int

	searchActive  bool
	searchInput   string
	searchResults []gitlab.GlobalSearchResult
	searchCursor  int
	searchLoading bool

	workspaces      []workspaceView
	workspaceCursor int

	commandPaletteActive bool
	commandPaletteInput  string
	commandPaletteCursor int

	summaryFormat summaryFormat

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
	if deps.Config != nil {
		applyTheme(deps.Config.UI.Theme)
	}
	return Model{deps: deps, loading: true, dashLoading: true}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadDashboardCmd(m.deps), spinnerTickCmd(m.spinnerGen))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// A recoverable error (a failed poll, "saving log", ...) shows in the
	// footer until something succeeds — clearing it only on
	// dashboardLoadedMsg left it stuck on screen even after a successful
	// non-dashboard refresh. Any other *Loaded/*Done/tick message means some
	// async operation just completed, which is the signal to drop it.
	switch msg.(type) {
	case errMsg, tea.WindowSizeMsg, tea.MouseMsg, tea.KeyMsg:
	default:
		m.err = nil
	}

	switch msg := msg.(type) {
	case dashboardLoadedMsg:
		m.loading = false
		m.dash = msg.ctx
		client, err := m.deps.NewClient(msg.ctx.Profile)
		if err != nil {
			m.err = err
			m.dashLoading = false
			return m, nil
		}
		m.client = client
		// Keep the spinner up until the stats land too ("load as one"); if
		// there are none to fetch, reveal the dashboard now instead of hanging
		// on a message that will never arrive.
		statsCmd := loadDashboardStatsCmd(m.client, msg.ctx)
		if statsCmd == nil {
			m.dashLoading = false
		}
		return m, tea.Batch(recordHistoryCmd(m.deps, msg.ctx), statsCmd)

	case historyLoadedMsg:
		m.recentProjects = msg.projects
		m.recentBranches = msg.branches
		m.dashCursor = m.dashMinCursor()
		return m, nil

	case dashboardStatsLoadedMsg:
		m.dashLoading = false
		m.dashCommits = msg.commits
		m.dashMRs = msg.mrs
		m.dashAssignedMRs = msg.assignedMRs
		m.dashActivity = msg.activity
		return m, nil

	case spinnerTickMsg:
		if msg.gen != m.spinnerGen || !m.dashLoading {
			return m, nil
		}
		m.spinnerFrame++
		return m, spinnerTickCmd(m.spinnerGen)

	case mrListLoadedMsg:
		m.loading = false
		m.mrs = msg.mrs
		m.cursor = 0
		return m, nil

	case mrDetailLoadedMsg:
		m.loading = false
		m.detail = msg.mr
		m.screen = screenDetail
		m.navActive = false
		return m, nil

	case mrDiffLoadedMsg:
		m.loading = false
		m.diffs = msg.diffs
		m.diff = newDiffState(msg.diffs)
		m.diffSearchActive = false
		m.diffSearchInput = ""
		m.screen = screenDiff
		m.navActive = false
		return m, nil

	case pipelineLoadedMsg:
		m.loading = false
		// A poll-triggered refresh of the SAME pipeline shouldn't reset the
		// user's place in the job list — only a genuine navigation (a
		// different pipeline, or arriving at this screen fresh) should.
		samePipeline := m.screen == screenPipeline && m.pipeline != nil && msg.pipeline != nil && m.pipeline.ID == msg.pipeline.ID
		m.pipeline = msg.pipeline
		m.jobs = msg.jobs
		m.lastPipelineFetch = time.Now()
		if samePipeline {
			m.jobCursor = min(m.jobCursor, max(0, len(m.jobs)-1))
			m.jobOffset = min(m.jobOffset, max(0, len(m.jobs)-1))
		} else {
			m.jobCursor = 0
			m.jobOffset = 0
			m.navActive = false
		}
		m.screen = screenPipeline
		m.pollGen++
		if isPipelineActive(m.pipeline) {
			return m, pipelinePollTickCmd(m.pollGen)
		}
		return m, nil

	case pipelinePollTickMsg:
		if msg.gen != m.pollGen || m.screen != screenPipeline || m.detail == nil || !isPipelineActive(m.pipeline) {
			return m, nil
		}
		return m, loadPipelineCmd(m.deps, m.detail)

	case jobLogLoadedMsg:
		m.loading = false
		sameJob := m.jobLog.job != nil && msg.job != nil && m.jobLog.job.ID == msg.job.ID
		if !sameJob {
			m.jobLogFollowing = false
		}
		prevOffset, prevQuery := m.jobLog.lineOffset, m.jobLog.searchQuery
		m.jobLog = newLogState(msg.job, msg.log)
		m.jobLogSearchActive = false
		m.jobLogSearchInput = ""
		m.screen = screenJobLog
		m.lastJobLogFetch = time.Now()
		if sameJob {
			// A follow-tick refresh used to silently reset lineOffset and
			// wipe the active search/match counter on every tick; carry them
			// across (follow mode's scrollToEnd below still takes over if
			// it's still active — that jump is the point of "follow").
			m.jobLog.lineOffset = min(prevOffset, m.jobLog.maxLineOffset(m.logBodyHeight()))
			if prevQuery != "" {
				m.jobLog.search(prevQuery, m.logBodyHeight())
			}
		} else {
			m.navActive = false
		}

		if !m.jobLogFollowing {
			return m, nil
		}
		if msg.job == nil || !isJobActive(msg.job.Status) {
			m.jobLogFollowing = false
			m.status = "Job finished; follow mode stopped."
			return m, nil
		}
		m.jobLog.scrollToEnd(m.logBodyHeight())
		m.jobLogPollGen++
		return m, jobLogFollowTickCmd(m.jobLogPollGen)

	case jobLogFollowTickMsg:
		if !m.jobLogFollowing || msg.gen != m.jobLogPollGen || m.screen != screenJobLog || m.jobLog.job == nil {
			return m, nil
		}
		return m, followJobLogCmd(m.deps, m.jobLog.job.ID)

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
		if msg.projects != nil || msg.branches != nil {
			m.recentProjects = msg.projects
			m.recentBranches = msg.branches
		}
		m.status = "Checked out " + msg.branch
		return m, nil

	case discussionsLoadedMsg:
		m.loading = false
		m.discuss = newDiscussState(msg.discussions, m.contentWidth())
		m.commentActive = false
		m.commentInput = ""
		m.screen = screenDiscussions
		m.navActive = false
		return m, nil

	case commitsLoadedMsg:
		m.loading = false
		m.commits = msg.commits
		m.commitCursor = 0
		m.commitOffset = 0
		m.screen = screenCommits
		m.navActive = false
		return m, nil

	case searchLoadedMsg:
		m.searchLoading = false
		if msg.query == m.searchInput {
			m.searchResults = msg.results
			m.searchCursor = 0
		}
		return m, nil

	case workspacesLoadedMsg:
		m.loading = false
		m.workspaces = msg.workspaces
		if m.workspaceCursor >= len(m.workspaces) {
			m.workspaceCursor = max(0, len(m.workspaces)-1)
		}
		m.screen = screenWorkspace
		m.navActive = false
		return m, nil

	case workspaceSavedMsg:
		m.loading = false
		m.workspaces = msg.workspaces
		m.status = msg.status
		m.screen = screenWorkspace
		m.navActive = false
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
			m.confirmCmd = checkoutMRCmd(m.deps, m.dash, m.detail)
			return m, nil
		}
		return m, checkoutMRCmd(m.deps, m.dash, m.detail)

	case statusMsg:
		m.status = msg.text
		return m, nil

	case errMsg:
		m.loading = false
		m.dashLoading = false
		m.err = msg.err
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if len(m.discuss.discussions) > 0 {
			// Discussion bodies are word-wrapped at the width they were
			// loaded at; without this a resize left them wrapped for
			// whatever width the MR was first opened at instead of reflowing.
			cursor, offset := m.discuss.cursor, m.discuss.lineOffset
			m.discuss = newDiscussState(m.discuss.discussions, m.contentWidth())
			m.discuss.cursor = min(cursor, max(0, len(m.discuss.threads)-1))
			m.discuss.lineOffset = min(offset, m.discuss.maxLineOffset(m.discussBodyHeight()))
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	case tea.MouseButtonWheelDown:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress {
			return m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmActive {
		return m.handleConfirmKey(msg)
	}
	if m.commandPaletteActive {
		return m.handleCommandPaletteKey(msg)
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
	if m.searchActive {
		return m.handleGlobalSearchKey(msg)
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

	case "ctrl+k":
		m.commandPaletteActive = true
		m.commandPaletteInput = ""
		m.commandPaletteCursor = 0
		return m, nil

	case "?":
		m.showHelp = true

	case "esc":
		m.status = ""
		switch m.screen {
		case screenJobLog:
			m.screen = screenPipeline
		case screenDiff, screenPipeline, screenDiscussions, screenCommits:
			m.screen = screenDetail
		case screenSearch, screenWorkspace:
			m.screen = screenDashboard
		case screenDetail:
			m.screen = screenList
		case screenList:
			m.screen = screenDashboard
		}
		m.navActive = false
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
		if m.screen == screenCommits && m.detail != nil {
			return m, loadCommitsCmd(m.deps, m.detail.IID)
		}
		if m.screen == screenSearch && strings.TrimSpace(m.searchInput) != "" {
			m.searchLoading = true
			return m, loadSearchCmd(m.client, m.searchInput, m.currentProject())
		}
		if m.screen == screenWorkspace {
			m.loading = true
			return m, loadWorkspacesCmd(m.deps, m.client)
		}
		// Dashboard refresh reloads "as one" too: spinner up, new generation so
		// any lingering tick from the previous load stops itself.
		m.dashLoading = true
		m.spinnerGen++
		return m, tea.Batch(loadDashboardCmd(m.deps), spinnerTickCmd(m.spinnerGen))

	case "/":
		if m.client == nil {
			return m, nil
		}
		if m.screen == screenDiff {
			m.diffSearchActive = true
			m.diffSearchInput = m.diff.searchQuery
			m.status = "/" + m.diffSearchInput
			return m, nil
		}
		if m.screen == screenJobLog {
			m.jobLogSearchActive = true
			m.jobLogSearchInput = m.jobLog.searchQuery
			m.status = "/" + m.jobLogSearchInput
			return m, nil
		}
		{
			m.screen = screenSearch
			m.navActive = false
			m.searchActive = true
			m.searchInput = ""
			m.searchResults = nil
			m.searchCursor = 0
			m.status = "search: "
			return m, nil
		}

	case "m":
		if m.dash == nil || m.client == nil {
			return m, nil
		}
		m.screen = screenList
		m.navActive = false
		m.loading = true
		m.status = ""
		m.mrScope = mrScopeProject
		m.mrFilter = mrFilterAll
		m.cursor = 0
		return m, loadMRListCmd(m.client, m.dash.Project.ID)

	case "f":
		if m.screen == screenList && m.client != nil {
			m.mrScope = m.mrScope.next()
			m.cursor = 0
			m.loading = true
			m.status = ""
			return m, loadMRScopeCmd(m.client, m.mrScope, m.projectID())
		}
		if m.screen == screenJobLog && m.jobLog.job != nil {
			if m.jobLogFollowing {
				m.jobLogFollowing = false
				m.status = "Follow mode off."
				return m, nil
			}
			if !isJobActive(m.jobLog.job.Status) {
				m.status = "Job isn't running; nothing to follow."
				return m, nil
			}
			m.jobLogFollowing = true
			m.jobLogPollGen++
			m.status = "Follow mode on."
			return m, jobLogFollowTickCmd(m.jobLogPollGen)
		}

	case "F":
		if m.screen == screenList {
			m.mrFilter = m.mrFilter.next()
			m.cursor = 0
			m.status = "Filter: " + m.mrFilter.label()
		}

	case "o":
		if url := m.currentURL(); url != "" {
			return m, openBrowserCmd(url)
		}

	case "y":
		if m.screen == screenJobLog {
			if line := m.jobLog.currentLineText(); line != "" {
				return m, copyToClipboardCmd(line, "Line copied to clipboard.")
			}
			return m, nil
		}
		if m.screen == screenDiff {
			if line := m.diff.currentLineText(); line != "" {
				return m, copyToClipboardCmd(line, "Line copied to clipboard.")
			}
			return m, nil
		}
		if url := m.currentURL(); url != "" {
			return m, copyLinkCmd(url)
		}

	case "Y":
		if m.screen == screenWorkspace && len(m.workspaces) > 0 {
			return m, copyToClipboardCmd(workspaceSummary(m.workspaces[m.workspaceCursor]), "Workspace summary copied to clipboard.")
		}
		if m.screen == screenDiff {
			if hunk := m.diff.currentHunkText(); hunk != "" {
				return m, copyToClipboardCmd(hunk, "Hunk copied to clipboard.")
			}
			return m, nil
		}
		if mr := m.summaryMR(); mr != nil {
			text := mrSummary(mr, m.projectPath(), m.summaryFormat, m.summaryExtras(mr))
			return m, copyToClipboardCmd(text, fmt.Sprintf("Summary copied to clipboard (%s).", m.summaryFormat.label()))
		}

	case "T":
		m.summaryFormat = m.summaryFormat.next()
		m.status = "Summary format: " + m.summaryFormat.label()

	case "up", "k":
		moved := true
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
		case m.screen == screenCommits:
			m.scrollCommits(-1)
		case m.screen == screenDashboard && m.dashCursor > m.dashMinCursor():
			m.dashCursor--
		case m.screen == screenSearch && m.searchCursor > 0:
			m.searchCursor--
		case m.screen == screenWorkspace && m.workspaceCursor > 0:
			m.workspaceCursor--
		default:
			moved = false
		}
		if !moved {
			m.moveNav(-1)
		}

	case "down", "j":
		moved := true
		switch {
		case m.screen == screenList && m.cursor < len(m.filteredMRs())-1:
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
		case m.screen == screenCommits:
			m.scrollCommits(1)
		case m.screen == screenDashboard && m.dashCursor < m.dashMaxCursor():
			m.dashCursor++
		case m.screen == screenSearch && m.searchCursor < len(m.searchResults)-1:
			m.searchCursor++
		case m.screen == screenWorkspace && m.workspaceCursor < len(m.workspaces)-1:
			m.workspaceCursor++
		default:
			moved = false
		}
		if !moved {
			m.moveNav(1)
		}

	case "left", "h":
		if m.screen == screenDiff {
			m.diff.moveFile(-1)
		} else {
			m.navActive = true
			m.navCursor = m.activeNavIndex()
		}

	case "right", "l":
		if m.screen == screenDiff {
			m.diff.moveFile(1)
		} else if m.navActive {
			return m.activateNav()
		}

	case "[":
		if m.screen == screenDiff {
			m.diff.moveHunk(-1, m.contentHeight())
		}

	case "]":
		if m.screen == screenDiff {
			m.diff.moveHunk(1, m.contentHeight())
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

	case "W":
		if m.detail != nil && m.dash != nil {
			m.loading = true
			m.status = ""
			return m, addCurrentMRToWorkspaceCmd(m.deps, m.dash, m.detail)
		}

	case "shift+tab":
		fallthrough
	case "tab":
		m.screen = screenWorkspace
		m.navActive = false
		m.loading = true
		return m, loadWorkspacesCmd(m.deps, m.client)

	case "s":
		if m.screen == screenDiff {
			m.diffSideBySide = !m.diffSideBySide
		}
		if m.screen == screenJobLog && m.jobLog.job != nil {
			return m, saveJobLogCmd(m.deps, m.jobLog.job, m.jobLog.raw)
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
			m.commentReplyID = ""
			m.status = "comment: "
		}

	case "C":
		if m.screen == screenDetail && m.detail != nil {
			m.loading = true
			m.status = ""
			return m, loadCommitsCmd(m.deps, m.detail.IID)
		}

	case "enter":
		if m.navActive {
			return m.activateNav()
		}
		if m.screen == screenList {
			mrs := m.filteredMRs()
			if len(mrs) > 0 && m.cursor >= 0 && m.cursor < len(mrs) {
				selected := mrs[m.cursor]
				m.loading = true
				return m, loadMRDetailCmd(m.client, selected.ProjectID, selected.IID)
			}
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
		if m.screen == screenSearch && len(m.searchResults) > 0 {
			return m.openSearchResult()
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
		if m.screen == screenCommits {
			m.commitCursor = 0
			m.commitOffset = 0
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
		if m.screen == screenCommits && len(m.commits) > 0 {
			m.commitCursor = len(m.commits) - 1
			m.commitOffset = max(0, len(m.commits)-max(1, m.discussBodyHeight()))
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
		case m.screen == screenDiscussions && m.detail != nil:
			if thread, ok := m.discuss.currentThread(); ok {
				m.commentActive = true
				m.commentInput = ""
				m.commentReplyID = thread.id
				m.status = "reply: "
			}
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
		if m.screen == screenWorkspace && m.detail != nil && m.dash != nil {
			m.loading = true
			m.status = ""
			return m, removeCurrentMRFromWorkspaceCmd(m.deps, m.dash, m.detail)
		}
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

func (m *Model) moveNav(delta int) {
	count := len(navItems())
	if count == 0 {
		return
	}
	if !m.navActive {
		m.navCursor = m.activeNavIndex()
		m.navActive = true
	}
	m.navCursor = (m.navCursor + delta + count) % count
	m.status = "navigation: " + navItems()[m.navCursor].name
}

func (m Model) activeNavIndex() int {
	for i, item := range navItems() {
		if item.matches(m.screen) {
			return i
		}
	}
	return 0
}

func (m Model) activateNav() (tea.Model, tea.Cmd) {
	items := navItems()
	if len(items) == 0 {
		return m, nil
	}
	if m.navCursor < 0 || m.navCursor >= len(items) {
		m.navCursor = m.activeNavIndex()
	}
	m.navActive = false
	m.status = ""
	switch items[m.navCursor].target {
	case screenDashboard:
		m.screen = screenDashboard
		return m, nil
	case screenList:
		if m.dash == nil || m.client == nil {
			m.status = "Merge requests are not available yet."
			return m, nil
		}
		m.screen = screenList
		m.loading = true
		m.mrScope = mrScopeProject
		m.mrFilter = mrFilterAll
		m.cursor = 0
		return m, loadMRListCmd(m.client, m.dash.Project.ID)
	case screenPipeline:
		if m.detail != nil && m.detail.Pipeline != nil {
			m.loading = true
			return m, loadPipelineCmd(m.deps, m.detail)
		}
		if m.dash != nil && m.dash.MergeRequest != nil && m.dash.MergeRequest.Pipeline != nil {
			m.detail = m.dash.MergeRequest
			m.loading = true
			return m, loadPipelineCmd(m.deps, m.dash.MergeRequest)
		}
		m.status = "No pipeline is available for the current MR."
		return m, nil
	case screenSearch:
		if m.client == nil {
			m.status = "Search is not available yet."
			return m, nil
		}
		m.screen = screenSearch
		m.searchActive = true
		m.searchInput = ""
		m.searchResults = nil
		m.searchCursor = 0
		m.status = "search: "
		return m, nil
	case screenWorkspace:
		if m.client == nil {
			m.status = "Workspace is not available yet."
			return m, nil
		}
		m.screen = screenWorkspace
		m.loading = true
		return m, loadWorkspacesCmd(m.deps, m.client)
	default:
		m.screen = items[m.navCursor].target
		return m, nil
	}
}

type commandPaletteEntry struct {
	title       string
	description string
	terms       string
	enabled     bool
	run         func(Model) (tea.Model, tea.Cmd)
}

func (m Model) handleCommandPaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.commandPaletteActive = false
		m.commandPaletteInput = ""
		m.commandPaletteCursor = 0
		return m, nil
	case "up", "k":
		if m.commandPaletteCursor > 0 {
			m.commandPaletteCursor--
		}
		return m, nil
	case "down", "j":
		if maxCursor := len(m.filteredCommandPaletteEntries()) - 1; m.commandPaletteCursor < maxCursor {
			m.commandPaletteCursor++
		}
		return m, nil
	case "enter":
		entries := m.filteredCommandPaletteEntries()
		if len(entries) == 0 {
			return m, nil
		}
		if m.commandPaletteCursor >= len(entries) {
			m.commandPaletteCursor = len(entries) - 1
		}
		entry := entries[m.commandPaletteCursor]
		m.commandPaletteActive = false
		m.commandPaletteInput = ""
		m.commandPaletteCursor = 0
		return entry.run(m)
	case "backspace", "ctrl+h":
		if len(m.commandPaletteInput) > 0 {
			runes := []rune(m.commandPaletteInput)
			m.commandPaletteInput = string(runes[:len(runes)-1])
			m.commandPaletteCursor = 0
		}
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.commandPaletteInput += string(msg.Runes)
		m.commandPaletteCursor = 0
	}
	return m, nil
}

func (m Model) filteredCommandPaletteEntries() []commandPaletteEntry {
	query := strings.ToLower(strings.TrimSpace(m.commandPaletteInput))
	entries := make([]commandPaletteEntry, 0)
	for _, entry := range m.commandPaletteEntries() {
		if !entry.enabled {
			continue
		}
		haystack := strings.ToLower(entry.title + " " + entry.description + " " + entry.terms)
		if query == "" || strings.Contains(haystack, query) {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (m Model) commandPaletteEntries() []commandPaletteEntry {
	keyAction := func(key string) func(Model) (tea.Model, tea.Cmd) {
		return func(m Model) (tea.Model, tea.Cmd) {
			return m.handleKey(keyMsg(key))
		}
	}
	return []commandPaletteEntry{
		{title: "Go to dashboard", description: "Return to the dashboard", terms: "home", enabled: m.screen != screenDashboard, run: func(m Model) (tea.Model, tea.Cmd) {
			m.screen = screenDashboard
			m.status = ""
			return m, nil
		}},
		{title: "Open merge requests", description: "List project merge requests", terms: "mrs list", enabled: m.dash != nil && m.client != nil, run: keyAction("m")},
		{title: "Open global search", description: "Search projects, MRs, and branches", terms: "find", enabled: m.client != nil, run: keyAction("/")},
		{title: "Open workspace", description: "Load the multi-repo workspace", terms: "multi repo tab", enabled: m.client != nil, run: keyAction("tab")},
		{title: "Refresh current screen", description: "Reload visible data", terms: "reload", enabled: m.dash != nil, run: keyAction("r")},
		{title: "Open in browser", description: "Open the current GitLab URL", terms: "web url", enabled: m.currentURL() != "", run: keyAction("o")},
		{title: "Copy current link", description: "Copy the current GitLab URL", terms: "clipboard url", enabled: m.currentURL() != "", run: keyAction("y")},
		{title: "Show current MR", description: "Open the current branch merge request", terms: "detail", enabled: m.screen == screenDashboard && m.dash != nil && m.dash.MergeRequest != nil, run: keyAction("enter")},
		{title: "Open diff", description: "Load the selected MR diff", terms: "changes files", enabled: m.detail != nil, run: keyAction("d")},
		{title: "Open pipeline", description: "Load the selected MR pipeline", terms: "ci jobs", enabled: m.detail != nil && m.detail.Pipeline != nil, run: keyAction("p")},
		{title: "Open discussions", description: "Load MR threads and comments", terms: "comments threads", enabled: m.detail != nil, run: keyAction("c")},
		{title: "Open commits", description: "Load MR commits", terms: "changes history", enabled: m.detail != nil, run: keyAction("C")},
		{title: "Checkout MR branch", description: "Check out the selected MR locally", terms: "branch git", enabled: m.detail != nil, run: keyAction("b")},
		{title: "Approve MR", description: "Approve the selected merge request", terms: "review", enabled: m.screen == screenDetail && m.detail != nil, run: keyAction("a")},
		{title: "Remove approval", description: "Revoke your approval from the MR", terms: "review revoke", enabled: m.screen == screenDetail && m.detail != nil, run: keyAction("A")},
		{title: "Merge MR", description: "Merge the selected merge request", terms: "accept", enabled: m.screen == screenDetail && m.detail != nil, run: keyAction("M")},
		{title: "Cancel pipeline", description: "Cancel the current pipeline", terms: "stop ci", enabled: (m.screen == screenPipeline && m.pipeline != nil) || (m.screen == screenDetail && m.detail != nil && m.detail.Pipeline != nil), run: keyAction("x")},
	}
}

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
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

func (m Model) handleGlobalSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.searchActive = false
		m.status = ""
		return m, nil
	case "enter":
		m.searchActive = false
		if strings.TrimSpace(m.searchInput) == "" {
			m.status = ""
			return m, nil
		}
		m.searchLoading = true
		m.status = ""
		return m, loadSearchCmd(m.client, m.searchInput, m.currentProject())
	case "backspace", "ctrl+h":
		if len(m.searchInput) > 0 {
			runes := []rune(m.searchInput)
			m.searchInput = string(runes[:len(runes)-1])
		}
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.searchInput += string(msg.Runes)
	}
	return m, nil
}

// commentPrompt is the status-bar prefix while composing — distinguishes a
// reply to a specific thread from a new top-level comment.
func (m Model) commentPrompt() string {
	if m.commentReplyID != "" {
		return "reply: "
	}
	return "comment: "
}

func (m Model) handleCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.commentActive = false
		m.commentInput = ""
		m.commentReplyID = ""
		m.status = "Comment cancelled."
		return m, nil
	case "alt+enter":
		m.commentInput += "\n"
		return m, nil
	case "enter":
		body := strings.TrimSpace(m.commentInput)
		replyID := m.commentReplyID
		m.commentActive = false
		m.commentInput = ""
		m.commentReplyID = ""
		if body == "" {
			m.status = "Empty comment, nothing posted."
			return m, nil
		}
		if m.detail == nil {
			return m, nil
		}
		m.loading = true
		m.status = ""
		if replyID != "" {
			return m, replyDiscussionCmd(m.deps, m.detail.IID, replyID, body)
		}
		return m, postCommentCmd(m.deps, m.detail.IID, body)
	case "backspace", "ctrl+h":
		if len(m.commentInput) > 0 {
			runes := []rune(m.commentInput)
			m.commentInput = string(runes[:len(runes)-1])
		}
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.commentInput += string(msg.Runes)
	}
	return m, nil
}

func (m Model) openSearchResult() (tea.Model, tea.Cmd) {
	if m.searchCursor < 0 || m.searchCursor >= len(m.searchResults) {
		return m, nil
	}
	result := m.searchResults[m.searchCursor]
	switch result.Type {
	case "merge_request":
		if result.MR != nil {
			m.loading = true
			projectID := result.MR.ProjectID
			if projectID == 0 && m.dash != nil && m.dash.Project != nil {
				projectID = m.dash.Project.ID
			}
			if projectID == 0 {
				m.status = "Search result has no project id."
				return m, nil
			}
			return m, loadMRDetailCmd(m.client, projectID, result.MR.IID)
		}
	case "project":
		if result.Project != nil && result.Project.WebURL != "" {
			return m, openBrowserCmd(result.Project.WebURL)
		}
	case "branch":
		if result.Branch != nil && result.Branch.WebURL != "" {
			return m, openBrowserCmd(result.Branch.WebURL)
		}
	}
	return m, nil
}

func (m Model) currentProject() *gitlab.Project {
	if m.dash != nil {
		return m.dash.Project
	}
	return nil
}

func (m Model) projectID() int {
	if m.dash != nil && m.dash.Project != nil {
		return m.dash.Project.ID
	}
	return 0
}

func (m *Model) scrollDiff(delta int) {
	m.diff.scroll(delta, m.contentHeight())
}

func (m *Model) scrollJobLog(delta int) {
	m.jobLog.scroll(delta, m.logBodyHeight())
}

// ensureJobVisible scrolls jobOffset so jobCursor falls within the window
// jobWindowFrom would actually render — a flat "cursor >= offset+height"
// check assumes one row per job, which undercounts once stage headers are in
// play, letting `j` walk the highlight into rows the frame had clipped.
func (m *Model) ensureJobVisible() {
	if m.jobListHeight() <= 0 {
		return
	}
	if m.jobCursor < m.jobOffset {
		m.jobOffset = m.jobCursor
		return
	}
	for m.jobOffset < m.jobCursor {
		_, end := m.jobWindowFrom(m.jobOffset)
		if m.jobCursor < end {
			return
		}
		m.jobOffset++
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
	case screenDiscussions, screenCommits:
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
