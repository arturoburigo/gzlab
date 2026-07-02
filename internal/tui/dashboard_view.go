package tui

import (
	"fmt"
	"strings"

	"github.com/arturoburigo/gitlab-tui/internal/history"
)

// dashRecentLimit caps how many recent entries each dashboard card shows.
const dashRecentLimit = 5

func (m Model) renderDashboard() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("GitLab TUI") + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 48)) + "\n\n")
	b.WriteString(kv("profile", valueStyle.Render(m.dash.ProfileName)) + "\n")
	b.WriteString(kv("project", m.dash.Project.PathWithNamespace) + "\n")
	b.WriteString(kv("branch", m.dash.Branch) + "\n\n")

	if m.dash.MergeRequest == nil {
		b.WriteString(footerStyle.Render("No open merge request for this branch.") + "\n")
	} else {
		mr := m.dash.MergeRequest
		fmt.Fprintf(&b, "%s%s !%d %s\n", m.dashGutter(m.dashCursor < 0), tableHead.Render("MR"), mr.IID, mr.Title)
		b.WriteString(kv("status", string(mr.State)+draftSuffix(mr)) + "\n")
		b.WriteString(kv("pipeline", renderPipeline(mr.Pipeline)) + "\n")
		if mr.ApprovalsRequired > 0 {
			b.WriteString(kv("approvals", fmt.Sprintf("%d/%d", mr.ApprovalsGiven, mr.ApprovalsRequired)) + "\n")
		}
	}

	m.renderRecentBranches(&b)
	m.renderRecentProjects(&b)
	return b.String()
}

func (m Model) renderRecentBranches(b *strings.Builder) {
	items := m.recentBranchItems()
	if len(items) == 0 {
		return
	}
	b.WriteString("\n" + tableHead.Render("Recent branches") + "\n")
	for i, br := range items {
		if i >= dashRecentLimit {
			break
		}
		line := m.dashGutter(m.dashCursor == i) + br.Name
		if br.MRIID != 0 {
			line += fmt.Sprintf("  !%d %s", br.MRIID, truncate(br.MRTitle, 40))
		}
		style := valueStyle
		if m.dashCursor == i {
			style = selectedStyle
		}
		b.WriteString(style.Render(line) + "\n")
	}
}

func (m Model) renderRecentProjects(b *strings.Builder) {
	items := m.recentProjectItems()
	if len(items) == 0 {
		return
	}
	b.WriteString("\n" + tableHead.Render("Recent projects") + "\n")
	for i, p := range items {
		if i >= dashRecentLimit {
			break
		}
		b.WriteString("  " + shortPath(p.Path) + "\n")
	}
}

func (m Model) dashboardHints() []hint {
	actions := []hint{{"m", "merge requests"}}
	if m.dash.MergeRequest != nil || len(m.recentBranchItems()) > 0 {
		actions = append(actions, hint{"enter", "detail"})
	}
	if len(m.recentBranchItems()) > 0 {
		actions = append(actions, hint{"j/k", "select"})
	}
	if m.currentURL() != "" {
		actions = append(actions, hint{"o", "open"}, hint{"y", "copy link"})
	}
	if m.dash.MergeRequest != nil {
		actions = append(actions, hint{"Y", "copy summary"})
	}
	return append(actions, hint{"r", "refresh"}, hint{"?", "help"}, hint{"q", "quit"})
}

// recentBranchItems is the recent-branches list the dashboard shows: recorded
// branches other than the current one, which the MR panel above already covers.
func (m Model) recentBranchItems() []history.Branch {
	current := ""
	if m.dash != nil {
		current = m.dash.Branch
	}
	items := make([]history.Branch, 0, len(m.recentBranches))
	for _, br := range m.recentBranches {
		if br.Name != current {
			items = append(items, br)
		}
	}
	return items
}

func (m Model) recentProjectItems() []history.Project {
	current := ""
	if m.dash != nil && m.dash.Project != nil {
		current = m.dash.Project.PathWithNamespace
	}
	items := make([]history.Project, 0, len(m.recentProjects))
	for _, p := range m.recentProjects {
		if p.Path != current {
			items = append(items, p)
		}
	}
	return items
}

// dashMinCursor is -1 when a current-branch MR occupies the top slot, else 0.
func (m Model) dashMinCursor() int {
	if m.dash != nil && m.dash.MergeRequest != nil {
		return -1
	}
	return 0
}

func (m Model) dashMaxCursor() int {
	if n := len(m.recentBranchItems()); n < dashRecentLimit {
		return n - 1
	}
	return dashRecentLimit - 1
}

func (m Model) dashGutter(selected bool) string {
	if selected {
		return cursorStyle.Render("> ")
	}
	return "  "
}
