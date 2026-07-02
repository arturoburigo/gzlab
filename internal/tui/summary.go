package tui

import (
	"fmt"
	"strings"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

// mrSummary renders a plain-text, paste-ready summary of a merge request — for
// dropping into Slack, a standup note, or a review request. It carries no TUI
// styling, only the facts.
func mrSummary(mr *gitlab.MergeRequest, projectPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "!%d %s\n", mr.IID, mr.Title)
	if projectPath != "" {
		fmt.Fprintf(&b, "Project:   %s\n", projectPath)
	}
	fmt.Fprintf(&b, "Branch:    %s → %s\n", mr.SourceBranch, mr.TargetBranch)
	if mr.Author != "" {
		fmt.Fprintf(&b, "Author:    %s\n", mr.Author)
	}
	fmt.Fprintf(&b, "Status:    %s%s\n", mr.State, draftSuffix(mr))
	if mr.Pipeline != nil {
		fmt.Fprintf(&b, "Pipeline:  %s\n", mr.Pipeline.Status)
	}
	if mr.ApprovalsRequired > 0 {
		fmt.Fprintf(&b, "Approvals: %d/%d\n", mr.ApprovalsGiven, mr.ApprovalsRequired)
	}
	if mr.HasConflicts {
		b.WriteString("Conflicts: yes\n")
	}
	if mr.WebURL != "" {
		b.WriteString(mr.WebURL)
	}
	return strings.TrimRight(b.String(), "\n")
}

// summaryMR returns the merge request currently in context, if any: the open
// detail, else the dashboard's current-branch MR.
func (m Model) summaryMR() *gitlab.MergeRequest {
	if m.detail != nil {
		return m.detail
	}
	if m.dash != nil {
		return m.dash.MergeRequest
	}
	return nil
}

func (m Model) projectPath() string {
	if m.dash != nil && m.dash.Project != nil {
		return m.dash.Project.PathWithNamespace
	}
	return ""
}
