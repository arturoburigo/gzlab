package tui

import (
	"fmt"
	"strings"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

func (m Model) View() string {
	if m.loading {
		return "\n  Loading...\n"
	}
	if m.err != nil {
		return "\n" + errorStyle.Render(fmt.Sprintf("  Error: %v", m.err)) + "\n\n  Press q to quit, r to retry.\n"
	}

	switch m.screen {
	case screenList:
		return m.renderList()
	case screenDetail:
		return m.renderDetail()
	default:
		return m.renderDashboard()
	}
}

func draftSuffix(mr *gitlab.MergeRequest) string {
	if mr.Draft {
		return " (draft)"
	}
	return ""
}

func renderPipeline(p *gitlab.Pipeline) string {
	if p == nil {
		return "none"
	}
	return pipelineStyle(p.Status).Render(string(p.Status))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func joinFooter(actions ...string) string {
	return footerStyle.Render(strings.Join(actions, " • "))
}
