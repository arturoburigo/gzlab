// Package tui implements gitlab-tui's Bubble Tea interface: a dashboard
// showing the current branch's merge request, a project MR list, and MR
// detail — the "Primeira Slice Recomendada" from the product plan.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/arturoburigo/gitlab-tui/internal/config"
	"github.com/arturoburigo/gitlab-tui/internal/dashboard"
	"github.com/arturoburigo/gitlab-tui/internal/gitdetect"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

type screen int

const (
	screenDashboard screen = iota
	screenList
	screenDetail
)

// Deps are the pre-resolved local-repo facts and constructors the TUI
// needs to bootstrap itself. Built by the CLI layer, which already ran
// git detection and config loading before starting the program.
type Deps struct {
	Config          *config.Config
	NewClient       dashboard.NewClientFunc
	Remote          *gitdetect.RemoteInfo
	Branch          string
	ProfileOverride string
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

	mrs    []*gitlab.MergeRequest
	cursor int

	detail *gitlab.MergeRequest
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

	case statusMsg:
		m.status = msg.text
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.status = ""
		switch m.screen {
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

	case "up", "k":
		if m.screen == screenList && m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.screen == screenList && m.cursor < len(m.mrs)-1 {
			m.cursor++
		}

	case "enter":
		if m.screen == screenList && len(m.mrs) > 0 {
			selected := m.mrs[m.cursor]
			m.loading = true
			return m, loadMRDetailCmd(m.client, m.dash.Project.ID, selected.IID)
		}
	}
	return m, nil
}

func (m Model) currentURL() string {
	switch m.screen {
	case screenDetail:
		if m.detail != nil {
			return m.detail.WebURL
		}
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
