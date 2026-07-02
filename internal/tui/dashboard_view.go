package tui

import (
	"fmt"
	"strings"
)

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
		fmt.Fprintf(&b, "%s !%d %s\n", tableHead.Render("MR"), mr.IID, mr.Title)
		b.WriteString(kv("status", string(mr.State)+draftSuffix(mr)) + "\n")
		b.WriteString(kv("pipeline", renderPipeline(mr.Pipeline)) + "\n")
		if mr.ApprovalsRequired > 0 {
			b.WriteString(kv("approvals", fmt.Sprintf("%d/%d", mr.ApprovalsGiven, mr.ApprovalsRequired)) + "\n")
		}
	}

	return b.String()
}

func (m Model) dashboardHints() []hint {
	actions := []hint{{"m", "merge requests"}}
	if m.dash.MergeRequest != nil {
		actions = append(actions, hint{"enter", "detail"})
	}
	if m.currentURL() != "" {
		actions = append(actions, hint{"o", "open"}, hint{"y", "copy link"})
	}
	return append(actions, hint{"r", "refresh"}, hint{"q", "quit"})
}
