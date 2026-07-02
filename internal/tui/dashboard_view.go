package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderDashboard() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("GitLab TUI") + "\n\n")
	b.WriteString(labelStyle.Render("Profile: ") + m.dash.ProfileName + "\n")
	b.WriteString(labelStyle.Render("Project: ") + m.dash.Project.PathWithNamespace + "\n")
	b.WriteString(labelStyle.Render("Branch:  ") + m.dash.Branch + "\n\n")

	if m.dash.MergeRequest == nil {
		b.WriteString("No open merge request for this branch.\n\n")
	} else {
		mr := m.dash.MergeRequest
		fmt.Fprintf(&b, "MR: !%d — %s\n", mr.IID, mr.Title)
		b.WriteString(labelStyle.Render("Status:    ") + string(mr.State) + draftSuffix(mr) + "\n")
		b.WriteString(labelStyle.Render("Pipeline:  ") + renderPipeline(mr.Pipeline) + "\n")
		if mr.ApprovalsRequired > 0 {
			fmt.Fprintf(&b, "%s%d/%d\n", labelStyle.Render("Approvals: "), mr.ApprovalsGiven, mr.ApprovalsRequired)
		}
		b.WriteString("\n")
	}

	if m.status != "" {
		b.WriteString(footerStyle.Render(m.status) + "\n\n")
	}

	b.WriteString(m.dashboardFooter())
	return b.String()
}

func (m Model) dashboardFooter() string {
	actions := []string{"m merge requests"}
	if m.currentURL() != "" {
		actions = append(actions, "o open browser", "y copy link")
	}
	actions = append(actions, "r refresh", "q quit")
	return joinFooter(actions...)
}
