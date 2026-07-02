package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderCommits() string {
	height := m.discussBodyHeight()

	title := "Commits"
	if m.detail != nil {
		title = fmt.Sprintf("Commits on !%d", m.detail.IID)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 72)) + "\n")

	if len(m.commits) == 0 {
		b.WriteString(footerStyle.Render("No commits.") + "\n")
		return b.String()
	}

	start := min(m.commitOffset, max(0, len(m.commits)-1))
	end := min(start+height, len(m.commits))
	for i := start; i < end; i++ {
		c := m.commits[i]
		line := discussHeaderStyle.Render(c.ShortID) + "  " + c.Title
		meta := c.AuthorName
		if !c.CreatedAt.IsZero() {
			meta += " · " + c.CreatedAt.Format("2006-01-02")
		}
		if meta != " " && strings.TrimSpace(meta) != "" {
			line += footerStyle.Render("  (" + strings.TrimSpace(meta) + ")")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m Model) commitsHints() []hint {
	total := len(m.commits)
	height := m.discussBodyHeight()
	start := min(m.commitOffset, max(0, total-1))
	end := min(start+height, total)
	return []hint{
		{fmt.Sprintf("%d-%d/%d", min(start+1, total), end, total), ""},
		{"j/k", "scroll"},
		{"g/G", "top/end"},
		{"o", "open"},
		{"r", "refresh"},
		{"esc", "back"},
		{"q", "quit"},
	}
}

func (m *Model) scrollCommits(delta int) {
	height := m.discussBodyHeight()
	maxOffset := max(0, len(m.commits)-max(1, height))
	m.commitOffset = min(max(m.commitOffset+delta, 0), maxOffset)
}
