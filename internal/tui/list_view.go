package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Merge Requests — %s", m.dash.Project.PathWithNamespace)) + "\n\n")

	if len(m.mrs) == 0 {
		b.WriteString("No open merge requests.\n\n")
	}
	for i, mr := range m.mrs {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}
		line := fmt.Sprintf("%s!%-5d %-40s %s", cursor, mr.IID, truncate(mr.Title, 40), mr.State)
		if mr.Draft {
			line += " (draft)"
		}
		b.WriteString(style.Render(line) + "\n")
	}

	if m.status != "" {
		b.WriteString("\n" + footerStyle.Render(m.status))
	}
	b.WriteString("\n" + joinFooter("enter view", "esc back", "r refresh", "q quit"))
	return b.String()
}
