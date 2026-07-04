package tui

import (
	"fmt"
	"strings"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

func (m Model) renderSearch() string {
	width := m.contentWidth()
	var b strings.Builder
	b.WriteString(titleStyle.Render("Search") + "\n")
	b.WriteString(rule(width) + "\n")
	prompt := "/"
	if m.searchActive {
		prompt = promptMarker
	}
	input := valueStyle.Render(m.searchInput)
	if m.searchActive {
		input += cursorStyle.Render("▌")
	}
	b.WriteString(labelStyle.Render(prompt) + input + "\n\n")
	if m.searchLoading {
		b.WriteString(footerStyle.Render("Searching...") + "\n")
		return b.String()
	}
	if strings.TrimSpace(m.searchInput) == "" {
		b.WriteString(footerStyle.Render("Type a project, merge request, or branch query.") + "\n")
		return b.String()
	}
	if len(m.searchResults) == 0 {
		b.WriteString(footerStyle.Render("No results.") + "\n")
		return b.String()
	}

	limit := max(3, m.contentHeight()-4)
	start, end := visibleWindow(len(m.searchResults), m.searchCursor, limit)
	if start > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  ↑ %d more above", start)) + "\n")
	}
	for i := start; i < end; i++ {
		result := m.searchResults[i]
		selected := i == m.searchCursor
		cursor := emptyMarker
		if selected {
			cursor = cursorMarker
		}
		line := cursor + renderSearchResult(result)
		if selected {
			b.WriteString(selectedStyle.Width(width).Render(line) + "\n")
			continue
		}
		b.WriteString(valueStyle.Render(line) + "\n")
	}
	if hidden := len(m.searchResults) - end; hidden > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  ↓ %d more below", hidden)) + "\n")
	}
	return b.String()
}

func renderSearchResult(result gitlab.GlobalSearchResult) string {
	switch result.Type {
	case "project":
		if result.Project != nil {
			return fmt.Sprintf("project       %s", result.Project.PathWithNamespace)
		}
	case "merge_request":
		if result.MR != nil {
			return fmt.Sprintf("merge_request !%-6d %s", result.MR.IID, truncate(result.MR.Title, 54))
		}
	case "branch":
		if result.Branch != nil {
			project := ""
			if result.Project != nil {
				project = shortPath(result.Project.PathWithNamespace) + " "
			}
			return fmt.Sprintf("branch        %s%s", project, result.Branch.Name)
		}
	}
	return result.Type
}

func (m Model) searchHints() []hint {
	return []hint{{"/", "edit"}, {"enter", "open"}, {"j/k", "select"}, {"esc", "back"}, {"r", "refresh"}, {"q", "quit"}}
}
