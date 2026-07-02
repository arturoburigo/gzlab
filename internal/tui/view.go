package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

func (m Model) View() string {
	if m.loading {
		return m.frameView("Loading", "Loading...\n\n"+footerStyle.Render("Fetching GitLab data"), []hint{{"q", "quit"}})
	}
	if m.err != nil {
		body := errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + footerStyle.Render("Press r to retry or q to quit.")
		return m.frameView("Error", body, []hint{{"r", "retry"}, {"q", "quit"}})
	}

	if m.showHelp {
		return m.frameView("Help", m.renderHelp(), []hint{{"?/esc", "close"}, {"q", "quit"}})
	}
	return m.frameView(m.screenTitle(), m.screenBody(), m.screenHints())
}

func (m Model) screenBody() string {
	switch m.screen {
	case screenList:
		return m.renderList()
	case screenDetail:
		return m.renderDetail()
	case screenDiff:
		return m.renderDiff()
	case screenPipeline:
		return m.renderPipeline()
	case screenJobLog:
		return m.renderJobLog()
	case screenDiscussions:
		return m.renderDiscussions()
	default:
		return m.renderDashboard()
	}
}

func (m Model) screenHints() []hint {
	switch m.screen {
	case screenList:
		return m.listHints()
	case screenDetail:
		return m.detailHints()
	case screenDiff:
		return m.diffHints()
	case screenPipeline:
		return m.pipelineHints()
	case screenJobLog:
		return m.jobLogHints()
	case screenDiscussions:
		return m.discussHints()
	default:
		return m.dashboardHints()
	}
}

// renderHelp lists the current screen's keybindings vertically and untruncated
// — the full set the footer can only show a width-limited slice of. It reuses
// the same *Hints() the footer draws from, so the two never drift.
func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Keybindings — "+m.screenTitle()) + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 48)) + "\n\n")
	for _, h := range m.screenHints() {
		if h.desc == "" {
			continue // skip count/position indicators like "1-5/10"
		}
		b.WriteString("  " + footerKey.Render(fmt.Sprintf("%-10s", h.key)) + footerStyle.Render(h.desc) + "\n")
	}
	return b.String()
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

type hint struct {
	key  string
	desc string
}

func renderHints(hints []hint, width int) string {
	if width <= 0 {
		width = 120
	}
	parts := make([]string, 0, len(hints))
	used := 0
	for _, h := range hints {
		part := footerKey.Render(h.key)
		if h.desc != "" {
			part += " " + footerStyle.Render(h.desc)
		}
		add := lipgloss.Width(part)
		if len(parts) > 0 {
			add += 3
		}
		if used+add > width {
			break
		}
		used += add
		parts = append(parts, part)
	}
	return strings.Join(parts, footerStyle.Render(" · "))
}

func (m Model) frameView(title, body string, hints []hint) string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 30
	}

	bodyH := max(3, height-2)
	innerW := max(1, width-4)
	body = clampBlock(body, innerW, bodyH-2)
	pane := paneStyle.Width(width - 2).MaxHeight(bodyH).Render(body)
	return strings.Join([]string{
		m.headerView(title, width),
		pane,
		m.footerView(hints, width),
	}, "\n")
}

func (m Model) headerView(title string, width int) string {
	logo := logoStyle.Render("GL")
	left := logo + "  " + titleStyle.Render(title)
	if m.dash != nil {
		left += "  " + labelStyle.Render("project ") + valueStyle.Render(shortPath(m.dash.Project.PathWithNamespace))
		left += "  " + labelStyle.Render("branch ") + valueStyle.Render(truncate(m.dash.Branch, 24))
	}
	right := ""
	if m.loading {
		right = footerStyle.Render("loading")
	}
	return spread(left, right, width)
}

func (m Model) footerView(hints []hint, width int) string {
	if m.confirmActive {
		prompt := confirmStyle.Render(m.confirmPrompt + "  [y]es / [n]o")
		return spread(prompt, "", width)
	}
	right := ""
	if m.status != "" {
		right = footerStyle.Render(truncate(m.status, max(8, width/2)))
	}
	return spread(renderHints(hints, width-lipgloss.Width(right)-2), right, width)
}

func (m Model) screenTitle() string {
	switch m.screen {
	case screenList:
		return "Merge Requests"
	case screenDetail:
		if m.detail != nil {
			return fmt.Sprintf("MR !%d", m.detail.IID)
		}
		return "Merge Request"
	case screenDiff:
		if m.detail != nil {
			return fmt.Sprintf("Diff !%d", m.detail.IID)
		}
		return "Diff"
	case screenPipeline:
		if m.pipeline != nil {
			return fmt.Sprintf("Pipeline #%d", m.pipeline.ID)
		}
		return "Pipeline"
	case screenJobLog:
		if m.jobLog.job != nil {
			return fmt.Sprintf("Log: %s", m.jobLog.job.Name)
		}
		return "Job Log"
	case screenDiscussions:
		if m.detail != nil {
			return fmt.Sprintf("Discussions !%d", m.detail.IID)
		}
		return "Discussions"
	default:
		return "Dashboard"
	}
}

func spread(left, right string, width int) string {
	if width <= 0 {
		return left
	}
	if right == "" {
		if lipgloss.Width(left) > width {
			return truncate(left, width)
		}
		return left
	}
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	if leftW+1+rightW > width {
		left = truncate(left, max(0, width-rightW-1))
		leftW = lipgloss.Width(left)
	}
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func clampBlock(s string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			lines[i] = truncate(line, width)
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func kv(key, value string) string {
	return labelStyle.Render(fmt.Sprintf("%-11s", key)) + value
}

func shortPath(path string) string {
	if path == "" {
		return "-"
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func (m Model) contentHeight() int {
	if m.height <= 0 {
		return 30
	}
	return max(5, m.height-5)
}

func (m Model) jobListHeight() int {
	if m.height <= 0 {
		return 18
	}
	return max(3, m.height-9)
}

func (m Model) logBodyHeight() int {
	return max(3, m.contentHeight()-2)
}
