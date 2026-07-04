package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// wideLayoutMinWidth is the terminal width at which the nav sidebar renders
// as a real column; below it, nav mode falls back to a horizontal strip.
// Note: the default fallback width used when no WindowSizeMsg has arrived
// yet is 100 (see frameView/withOverlay) — keep this above that so an
// unsized model renders in narrow (single-pane) layout, not a half-sized
// sidebar squeezed against an unmeasured terminal.
const wideLayoutMinWidth = 120

// navSidebarColW is the sidebar's content width in wide layout.
const navSidebarColW = 18

func (m Model) View() string {
	var view string
	switch {
	case m.err != nil && m.dash == nil:
		// Boot failure: nothing else to show yet, so this is the only
		// reasonable full-screen state.
		body := errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + footerStyle.Render("Press r to retry or q to quit.")
		view = m.frameView("Error", body, []hint{{"r", "retry"}, {"q", "quit"}})
	case m.dashLoading && m.screen == screenDashboard:
		view = m.frameView("Loading", m.renderDashboardLoading(), []hint{{"q", "quit"}})
	default:
		view = m.frameView(m.screenTitle(), m.screenBody(), m.screenHints())
	}

	switch {
	case m.confirmActive:
		return m.withOverlay(view, "Confirm", m.renderConfirmOverlay())
	case m.commandPaletteActive:
		return m.withOverlay(view, "Command Palette", m.renderCommandPalette())
	case m.showHelp:
		return m.withOverlay(view, "Help", m.renderHelp())
	default:
		return view
	}
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
	case screenCommits:
		return m.renderCommits()
	case screenSearch:
		return m.renderSearch()
	case screenWorkspace:
		return m.renderWorkspace()
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
	case screenCommits:
		return m.commitsHints()
	case screenSearch:
		return m.searchHints()
	case screenWorkspace:
		return m.workspaceHints()
	default:
		return m.dashboardHints()
	}
}

// globalHints are the keybindings available on every screen; footerView
// right-anchors them so they always survive width truncation, and
// renderHelp appends them to whatever screen-specific list it's showing.
func globalHints() []hint {
	return []hint{{"?", "help"}, {"ctrl+k", "commands"}}
}

// renderHelp lists the current screen's keybindings vertically and untruncated
// — the full set the footer can only show a width-limited slice of. It reuses
// the same *Hints() the footer draws from, so the two never drift.
func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Keybindings — "+m.screenTitle()) + "\n")
	b.WriteString(rule(48) + "\n\n")
	for _, h := range append(m.screenHints(), globalHints()...) {
		if h.desc == "" {
			continue // skip count/position indicators like "1-5/10"
		}
		b.WriteString("  " + footerKey.Render(fmt.Sprintf("%-10s", h.key)) + footerStyle.Render(h.desc) + "\n")
	}
	return b.String()
}

func (m Model) renderConfirmOverlay() string {
	var b strings.Builder
	b.WriteString(confirmStyle.Render(m.confirmPrompt) + "\n\n")
	b.WriteString(footerKey.Render("enter/y") + footerStyle.Render(" confirm"))
	b.WriteString(footerStyle.Render("  ·  "))
	b.WriteString(footerKey.Render("n/esc") + footerStyle.Render(" cancel"))
	return b.String()
}

func (m Model) renderCommandPalette() string {
	entries := m.filteredCommandPaletteEntries()
	var b strings.Builder
	b.WriteString(labelStyle.Render(promptMarker) + valueStyle.Render(m.commandPaletteInput) + "\n")
	b.WriteString(rule(56) + "\n")
	if len(entries) == 0 {
		b.WriteString(footerStyle.Render("No commands found.") + "\n")
		return b.String()
	}
	cursor := min(m.commandPaletteCursor, len(entries)-1)
	limit := min(8, len(entries))
	offset := 0
	if cursor >= limit {
		offset = cursor - limit + 1
	}
	for i := 0; i < limit; i++ {
		entryIndex := offset + i
		entry := entries[entryIndex]
		line := fmt.Sprintf("%s  %s", entry.title, footerStyle.Render(entry.description))
		if entryIndex == cursor {
			b.WriteString(overlaySelectedStyle.Render(cursorMarker+line) + "\n")
			continue
		}
		b.WriteString(emptyMarker + line + "\n")
	}
	if hidden := len(entries) - offset - limit; hidden > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  %d more...", hidden)) + "\n")
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

// truncate cuts s to at most n display columns, ANSI- and rune-aware: it
// never splits a multibyte rune or an SGR escape sequence mid-way (the
// byte-slicing this replaced could bleed color/background past the cut and
// mangle multibyte titles into mojibake).
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(s, n, "…")
}

type hint struct {
	key  string
	desc string
}

// renderHints renders hints left to right until width runs out, appending a
// trailing "…" when something had to be dropped so truncation is visible
// rather than silent.
func renderHints(hints []hint, width int) string {
	if width <= 0 {
		return ""
	}
	parts := make([]string, 0, len(hints))
	used := 0
	truncated := false
	for _, h := range hints {
		part := footerKey.Render(h.key)
		if h.desc != "" {
			part += " " + footerStyle.Render(h.desc)
		}
		add := lipgloss.Width(part)
		sep := 0
		if len(parts) > 0 {
			sep = 3
		}
		if used+sep+add > width {
			truncated = true
			break
		}
		used += sep + add
		parts = append(parts, part)
	}
	out := strings.Join(parts, footerStyle.Render(" · "))
	if truncated && out != "" {
		out += footerStyle.Render(" …")
	}
	return out
}

// renderFooterHints composes the screen-specific hints (left, truncatable)
// with globalHints (right-anchored, always visible) so help/command-palette
// keys never get silently dropped when a screen has many hints.
func renderFooterHints(hints []hint, width int) string {
	global := renderHints(globalHints(), width)
	globalW := lipgloss.Width(global)
	primaryWidth := width - globalW - 3
	primary := renderHints(hints, primaryWidth)
	if primary == "" {
		return global
	}
	return primary + footerStyle.Render(" · ") + global
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

	lines := []string{m.headerView(title, width)}
	bodyH := max(3, height-2)
	if width < wideLayoutMinWidth && m.navActive {
		lines = append(lines, m.navStripView(width))
		bodyH = max(3, bodyH-1)
	}
	footer := m.footerView(hints, width)
	if m.commentActive {
		footer = m.renderComposeBar(width)
	}
	lines = append(lines, m.mainPaneView(body, width, bodyH), footer)
	return strings.Join(lines, "\n")
}

// renderComposeBar replaces the footer's hints row while composing a
// comment: typing used to insert raw "\n" into the right-aligned status
// string, growing the frame past the terminal height, and past ~half a
// screen of text footerView's head-anchored truncation left the user typing
// blind. This is a dedicated full-width line, tail-anchored so the visible
// slice always includes what was just typed, with newlines shown as "⏎"
// instead of actually breaking the line.
func (m Model) renderComposeBar(width int) string {
	label := labelStyle.Render(m.commentPrompt())
	text := strings.ReplaceAll(m.commentInput, "\n", "⏎") + "▌"
	avail := max(4, width-lipgloss.Width(label))
	return label + valueStyle.Render(truncatePathLeft(text, avail))
}

func (m Model) withOverlay(base, title, body string) string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 30
	}

	maxOverlayW := min(72, max(10, width-4))
	maxOverlayH := min(18, max(5, height-4))
	contentW := max(10, maxOverlayW-6)
	contentH := max(3, maxOverlayH-4)
	content := overlayTitleStyle.Render(title) + "\n\n" + clampWidthCapHeight(body, contentW, contentH)
	box := overlayStyle.Width(maxOverlayW - 2).Render(content)
	boxW := lipgloss.Width(box)
	boxLines := strings.Split(box, "\n")
	startRow := max(0, (height-len(boxLines))/2)
	startCol := max(0, (width-boxW)/2)

	lines := strings.Split(base, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, boxLine := range boxLines {
		row := startRow + i
		if row >= len(lines) {
			break
		}
		line := strings.Repeat(" ", startCol) + boxLine
		if lineW := lipgloss.Width(line); lineW < width {
			line += strings.Repeat(" ", width-lineW)
		}
		lines[row] = line
	}
	return strings.Join(lines, "\n")
}

func (m Model) mainPaneView(body string, width, height int) string {
	if width >= wideLayoutMinWidth {
		sideW := navSidebarColW
		mainW := max(20, width-sideW-4)
		side := clampBlock(m.sidebarView(), sideW-2, height-2)
		main := clampBlock(body, mainW-2, height-2)
		return lipgloss.JoinHorizontal(lipgloss.Top,
			sidebarStyle.Width(sideW).Height(height-2).Render(side),
			paneStyle.Width(mainW).Height(height-2).Render(main),
		)
	}
	innerW := max(1, width-4)
	body = clampBlock(body, innerW, height-2)
	return paneStyle.Width(width - 2).MaxHeight(height).Render(body)
}

func (m Model) sidebarView() string {
	items := navItems()
	var b strings.Builder
	b.WriteString(tableHead.Render("Navigation") + "\n\n")
	for i, item := range items {
		line := emptyMarker + item.name
		if m.navActive && i == m.navCursor {
			line = cursorMarker + item.name
			b.WriteString(sidebarSelectedStyle.Render(line) + "\n")
			continue
		}
		if item.matches(m.screen) {
			line = activeMarker + item.name
			b.WriteString(sidebarActiveStyle.Render(line) + "\n")
			continue
		}
		b.WriteString(sidebarMutedStyle.Render(line) + "\n")
	}
	return b.String()
}

// navStripView renders a one-line horizontal nav strip for when the sidebar
// column is collapsed (narrow terminals) but nav mode is still active, so the
// user isn't navigating an invisible menu with only footer text as feedback.
func (m Model) navStripView(width int) string {
	items := navItems()
	parts := make([]string, 0, len(items))
	for i, item := range items {
		switch {
		case m.navActive && i == m.navCursor:
			parts = append(parts, sidebarSelectedStyle.Render(cursorMarker+item.name))
		case item.matches(m.screen):
			parts = append(parts, sidebarActiveStyle.Render(activeMarker+item.name))
		default:
			parts = append(parts, sidebarMutedStyle.Render(emptyMarker+item.name))
		}
	}
	line := strings.Join(parts, footerStyle.Render(" · "))
	return clampStyledLine(line, width)
}

type navItem struct {
	name   string
	target screen
	group  []screen
}

func (i navItem) matches(s screen) bool {
	if s == i.target {
		return true
	}
	for _, grouped := range i.group {
		if s == grouped {
			return true
		}
	}
	return false
}

func navItems() []navItem {
	return []navItem{
		{name: "Dashboard", target: screenDashboard},
		{name: "MRs", target: screenList, group: []screen{screenDetail, screenDiff, screenDiscussions, screenCommits}},
		{name: "Pipeline", target: screenPipeline, group: []screen{screenJobLog}},
		{name: "Search", target: screenSearch},
		{name: "Workspace", target: screenWorkspace},
	}
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
	right := ""
	switch {
	case m.err != nil:
		right = errorStyle.Render(truncate(fmt.Sprintf("Error: %v", m.err), max(8, width/2)))
	case m.status != "":
		right = footerStyle.Render(truncate(m.status, max(8, width/2)))
	}
	return spread(renderFooterHints(hints, width-lipgloss.Width(right)-2), right, width)
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
	case screenCommits:
		if m.detail != nil {
			return fmt.Sprintf("Commits !%d", m.detail.IID)
		}
		return "Commits"
	case screenSearch:
		return "Search"
	case screenWorkspace:
		return "Workspace"
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

// clampBlock fits s into exactly width x height: lines are truncated to width
// and the block is padded with blank lines to height. Use this for content
// that must fill its container (main panes); for content that should shrink
// to fit (modals), use clampWidthCapHeight instead.
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

// clampWidthCapHeight truncates lines to width and caps the line count at
// maxHeight, but never pads — unlike clampBlock, a 3-line body stays 3 lines.
func clampWidthCapHeight(s string, width, maxHeight int) string {
	if width <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if maxHeight > 0 && len(lines) > maxHeight {
		lines = lines[:maxHeight]
	}
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			lines[i] = truncate(line, width)
		}
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

// visibleWindow returns the [start, end) bounds for showing up to limit rows
// around cursor out of count total — the same windowing renderCommandPalette
// already did for its own list, extracted so list/search/workspace screens
// can stop dumping every row unconditionally into clampBlock's hard clip
// (which just makes the selection vanish below the fold on a long list).
func visibleWindow(count, cursor, limit int) (start, end int) {
	if limit <= 0 || count == 0 {
		return 0, 0
	}
	limit = min(limit, count)
	cursor = min(max(cursor, 0), count-1)
	if cursor >= limit {
		start = cursor - limit + 1
	}
	end = min(start+limit, count)
	return start, end
}

// rule draws a horizontal divider width columns wide, using the same glyph
// everywhere (screens used to mix "─" and ASCII "-" at inconsistent widths).
func rule(width int) string {
	if width <= 0 {
		return ""
	}
	return ruleStyle.Render(strings.Repeat("─", width))
}

// contentHeight is the number of body rows available inside the main pane,
// derived from the same expression frameView/mainPaneView use to size it —
// height-2 for the frame's own top/bottom lines, then -2 more for the pane's
// border+padding.
func (m Model) contentHeight() int {
	if m.height <= 0 {
		return 30
	}
	return max(5, m.height-4)
}

// contentWidth mirrors mainPaneView's real budget: sidebar (18) + its
// left/right margin (4) + the main pane's border+padding (2) = 24 columns of
// chrome in wide layout; just border+padding (4) in narrow layout.
func (m Model) contentWidth() int {
	width := m.width
	if width <= 0 {
		width = 100
	}
	if width >= wideLayoutMinWidth {
		return max(10, width-24)
	}
	return max(10, width-4)
}

func (m Model) logBodyHeight() int {
	return max(3, m.contentHeight()-2)
}
