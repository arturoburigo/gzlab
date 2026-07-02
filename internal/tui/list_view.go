package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Merge Requests - %s", m.dash.Project.PathWithNamespace)) + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 72)) + "\n")
	b.WriteString(tableHead.Render(fmt.Sprintf("  %-7s %-44s %s", "MR", "TITLE", "STATE")) + "\n")

	if len(m.mrs) == 0 {
		b.WriteString("\n" + footerStyle.Render("No open merge requests.") + "\n")
	}
	for i, mr := range m.mrs {
		cursor := "  "
		style := valueStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}
		line := fmt.Sprintf("%s!%-6d %-44s %s", cursor, mr.IID, truncate(mr.Title, 44), mr.State)
		if mr.Draft {
			line += " (draft)"
		}
		b.WriteString(style.Render(line) + "\n")
	}
	return b.String()
}

func (m Model) listHints() []hint {
	return []hint{{"↑↓", "select"}, {"enter", "view"}, {"esc", "back"}, {"r", "refresh"}, {"q", "quit"}}
}
