package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

func (m Model) renderCommits() string {
	if len(m.commits) == 0 {
		return footerStyle.Render("No commits.") + "\n"
	}

	height := m.discussBodyHeight()
	width := m.contentWidth()
	var b strings.Builder
	start := min(m.commitOffset, max(0, len(m.commits)-1))
	end := min(start+height, len(m.commits))
	for i := start; i < end; i++ {
		b.WriteString(renderCommitRow(m.commits[i], width, i == m.commitCursor) + "\n")
	}
	return b.String()
}

// renderCommitRow lays out a commit as fixed columns — an 8-char SHA, a
// width-aware truncated title, and a right-aligned "author · date" — instead
// of one run of text where a long title pushes the metadata off the pane
// entirely. The selected row reuses the discussions screen's cursor pattern:
// a ▶ gutter plus one uniform highlight style, built from plain text so
// nothing gets nested inside it (see D1).
func renderCommitRow(c gitlab.Commit, width int, selected bool) string {
	cursor := emptyMarker
	if selected {
		cursor = cursorMarker
	}
	cursorW := lipgloss.Width(cursor)

	meta := strings.TrimSpace(c.AuthorName)
	if !c.CreatedAt.IsZero() {
		if meta != "" {
			meta += " · "
		}
		meta += c.CreatedAt.Format("2006-01-02")
	}

	sha := fmt.Sprintf("%-8s", truncate(c.ShortID, 8))
	titleW := max(10, width-cursorW-8-1-lipgloss.Width(meta)-2)
	title := truncate(c.Title, titleW)

	if selected {
		return selectedStyle.Render(spread(cursor+sha+" "+title, meta, width))
	}
	return spread(cursor+discussHeaderStyle.Render(sha)+" "+title, footerStyle.Render(meta), width)
}

func (m Model) commitsHints() []hint {
	total := len(m.commits)
	height := m.discussBodyHeight()
	start := min(m.commitOffset, max(0, total-1))
	end := min(start+height, total)
	return []hint{
		{fmt.Sprintf("%d-%d/%d", min(start+1, total), end, total), ""},
		{"j/k", "select"},
		{"g/G", "top/end"},
		{"o", "open"},
		{"r", "refresh"},
		{"esc", "back"},
		{"q", "quit"},
	}
}

// scrollCommits moves the commit cursor and scrolls the offset to keep it
// visible — mirroring discussState.moveCursor rather than moving a bare
// scroll offset with no highlighted row at all.
func (m *Model) scrollCommits(delta int) {
	if len(m.commits) == 0 {
		return
	}
	height := m.discussBodyHeight()
	m.commitCursor = min(max(m.commitCursor+delta, 0), len(m.commits)-1)
	if m.commitCursor < m.commitOffset {
		m.commitOffset = m.commitCursor
	}
	if m.commitCursor >= m.commitOffset+height {
		m.commitOffset = m.commitCursor - height + 1
	}
	maxOffset := max(0, len(m.commits)-max(1, height))
	m.commitOffset = min(max(m.commitOffset, 0), maxOffset)
}
