package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

type diffState struct {
	files      []diffFile
	fileCursor int
	hunkCursor int
	lineOffset int

	searchQuery   string
	searchMatches []diffMatch
	searchCursor  int
}

type diffFile struct {
	oldPath   string
	newPath   string
	path      string
	flags     []string
	lines     []string
	hunks     []diffHunk
	additions int
	deletions int
}

func (f diffFile) isDeleted() bool {
	if f.newPath == "/dev/null" {
		return true
	}
	for _, flag := range f.flags {
		if flag == "deleted" {
			return true
		}
	}
	return false
}

type diffHunk struct {
	lineIndex int
	header    string
}

type diffMatch struct {
	fileIndex int
	lineIndex int
}

func newDiffState(diffs []*gitlab.MergeRequestDiff) diffState {
	state := diffState{files: parseDiffFiles(diffs)}
	if len(state.files) == 0 {
		state.files = []diffFile{{path: "diff", lines: []string{"No diff returned for this merge request."}}}
	}
	return state
}

func parseDiffFiles(diffs []*gitlab.MergeRequestDiff) []diffFile {
	files := make([]diffFile, 0, len(diffs))
	for _, diff := range diffs {
		if diff == nil {
			continue
		}
		if isRawGitDiff(diff) {
			files = append(files, parseRawGitDiff(diff.Diff)...)
			continue
		}
		files = append(files, diffFileFromGitLabDiff(diff))
	}
	return files
}

func isRawGitDiff(diff *gitlab.MergeRequestDiff) bool {
	return diff.OldPath == "" && diff.NewPath == "" && strings.Contains(diff.Diff, "diff --git ")
}

func parseRawGitDiff(raw string) []diffFile {
	var files []diffFile
	var current *diffFile
	for _, line := range splitDiffLines(raw) {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				finalizeDiffFile(current)
				files = append(files, *current)
			}
			current = &diffFile{path: pathFromDiffGitLine(line), lines: []string{line}}
			continue
		}
		if current == nil {
			current = &diffFile{path: "diff"}
		}
		current.lines = append(current.lines, line)
		applyDiffMetadata(current, line)
	}
	if current != nil {
		finalizeDiffFile(current)
		files = append(files, *current)
	}
	return files
}

func diffFileFromGitLabDiff(diff *gitlab.MergeRequestDiff) diffFile {
	file := diffFile{
		oldPath: diff.OldPath,
		newPath: diff.NewPath,
		path:    diffPath(diff),
		flags:   diffFlags(diff),
		lines:   splitDiffLines(diff.Diff),
	}
	if file.path == "" {
		file.path = "diff"
	}
	if diff.TooLarge {
		file.lines = append([]string{"diff too large; open in browser for full content"}, file.lines...)
	}
	if diff.Collapsed {
		file.lines = append([]string{"diff collapsed by GitLab"}, file.lines...)
	}
	finalizeDiffFile(&file)
	return file
}

func finalizeDiffFile(file *diffFile) {
	if file.path == "" {
		file.path = firstNonEmpty(file.newPath, file.oldPath, "diff")
	}
	file.hunks = file.hunks[:0]
	file.additions = 0
	file.deletions = 0
	for i, line := range file.lines {
		applyDiffMetadata(file, line)
		if strings.HasPrefix(line, "@@") {
			file.hunks = append(file.hunks, diffHunk{lineIndex: i, header: line})
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			file.additions++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			file.deletions++
		}
	}
}

func applyDiffMetadata(file *diffFile, line string) {
	switch {
	case strings.HasPrefix(line, "--- "):
		file.oldPath = trimDiffPath(strings.TrimPrefix(line, "--- "))
	case strings.HasPrefix(line, "+++ "):
		file.newPath = trimDiffPath(strings.TrimPrefix(line, "+++ "))
	case strings.HasPrefix(line, "new file mode"):
		file.flags = appendUnique(file.flags, "new")
	case strings.HasPrefix(line, "deleted file mode"):
		file.flags = appendUnique(file.flags, "deleted")
	case strings.HasPrefix(line, "rename from "):
		file.oldPath = strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
		file.flags = appendUnique(file.flags, "renamed")
	case strings.HasPrefix(line, "rename to "):
		file.newPath = strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
		file.flags = appendUnique(file.flags, "renamed")
	}
	if file.newPath != "" && file.newPath != "/dev/null" {
		file.path = file.newPath
	} else if file.oldPath != "" && file.oldPath != "/dev/null" {
		file.path = file.oldPath
	}
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func splitDiffLines(raw string) []string {
	raw = strings.TrimRight(raw, "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

func pathFromDiffGitLine(line string) string {
	body := strings.TrimSpace(strings.TrimPrefix(line, "diff --git "))
	if idx := strings.Index(body, " b/"); idx >= 0 {
		return trimDiffPath(body[idx+1:])
	}
	if idx := strings.Index(body, " \"b/"); idx >= 0 {
		return trimDiffPath(body[idx+2:])
	}
	parts := strings.Fields(line)
	if len(parts) >= 4 {
		return trimDiffPath(parts[3])
	}
	return body
}

func trimDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "\"")
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

func diffPath(diff *gitlab.MergeRequestDiff) string {
	if diff.DeletedFile {
		return firstNonEmpty(diff.OldPath, diff.NewPath)
	}
	return firstNonEmpty(diff.NewPath, diff.OldPath)
}

func diffFlags(diff *gitlab.MergeRequestDiff) []string {
	var flags []string
	if diff.NewFile {
		flags = append(flags, "new")
	}
	if diff.RenamedFile {
		flags = append(flags, "renamed")
	}
	if diff.DeletedFile {
		flags = append(flags, "deleted")
	}
	if diff.GeneratedFile {
		flags = append(flags, "generated")
	}
	if diff.TooLarge {
		flags = append(flags, "too large")
	}
	return flags
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (m Model) renderDiff() string {
	bodyHeight := m.contentHeight() - 2
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	sidebarWidth := min(34, max(18, m.width/4))
	diffWidth := max(30, m.width-sidebarWidth-10)

	title := "Diff"
	if m.detail != nil {
		title = fmt.Sprintf("Diff !%d - %s", m.detail.IID, m.detail.Title)
	}
	if m.diffSideBySide {
		title += "  " + footerStyle.Render("[side-by-side]")
	}
	if m.diffHideWhitespace {
		title += "  " + footerStyle.Render("[whitespace hidden]")
	}
	header := titleStyle.Render(title) + "\n"
	header += ruleStyle.Render(strings.Repeat("─", 72)) + "\n"
	sidebar := m.renderDiffFiles(sidebarWidth, bodyHeight)
	var content string
	if m.diffSideBySide {
		content = m.renderCurrentDiffFileSideBySide(diffWidth, bodyHeight)
	} else {
		content = m.renderCurrentDiffFile(diffWidth, bodyHeight)
	}
	return header + lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", content)
}

func (m Model) renderDiffFiles(width, height int) string {
	lines := make([]string, 0, len(m.diff.files)+2)
	lines = append(lines, tableHead.Render("Files"))
	for i, file := range m.diff.files {
		prefix := "  "
		style := valueStyle
		if i == m.diff.fileCursor {
			prefix = "> "
			style = selectedStyle
		}
		stats := fmt.Sprintf("+%d -%d", file.additions, file.deletions)
		line := spread(prefix+truncate(file.path, max(8, width-12)), stats, width)
		lines = append(lines, style.Render(line))
	}
	if len(m.diff.files) == 0 {
		lines = append(lines, footerStyle.Render("No files changed."))
	}
	return clampBlock(strings.Join(lines, "\n"), width, height)
}

func (m Model) renderCurrentDiffFile(width, height int) string {
	file := m.diff.currentFile()
	if file == nil {
		return clampBlock("No diff returned for this merge request.", width, height)
	}

	lines := file.lines
	if len(lines) == 0 {
		lines = []string{"No textual changes in this file."}
	}

	start := min(m.diff.lineOffset, max(0, len(lines)-1))
	end := min(start+height-2, len(lines))

	var b strings.Builder
	b.WriteString(diffFileTitle(*file) + "\n")
	for lineIndex, line := range lines[start:end] {
		active := m.diff.isActiveMatch(m.diff.fileCursor, start+lineIndex)
		b.WriteString(renderDiffLine(line, m.diffHideWhitespace, active) + "\n")
	}
	return clampBlock(b.String(), width, height)
}

// sideBySideRow is one visual row of the side-by-side diff view.
type sideBySideRow struct {
	left  string
	right string
}

// buildSideBySideRows pairs up a unified diff's line-level removal/addition
// runs into left/right rows: a run of consecutive "-" lines is paired
// index-wise against the run of "+" lines that follows it (the shorter run
// pads with a blank cell), which is how most terminal diff viewers do
// side-by-side without full intraline (LCS) diffing. Context/header lines
// mirror unchanged on both sides.
func buildSideBySideRows(lines []string) []sideBySideRow {
	rows := make([]sideBySideRow, 0, len(lines))
	i := 0
	for i < len(lines) {
		switch {
		case isDiffRemoval(lines[i]):
			var removals, additions []string
			for i < len(lines) && isDiffRemoval(lines[i]) {
				removals = append(removals, lines[i])
				i++
			}
			for i < len(lines) && isDiffAddition(lines[i]) {
				additions = append(additions, lines[i])
				i++
			}
			n := max(len(removals), len(additions))
			for j := range n {
				var row sideBySideRow
				if j < len(removals) {
					row.left = removals[j]
				}
				if j < len(additions) {
					row.right = additions[j]
				}
				rows = append(rows, row)
			}
		case isDiffAddition(lines[i]):
			rows = append(rows, sideBySideRow{right: lines[i]})
			i++
		default:
			rows = append(rows, sideBySideRow{left: lines[i], right: lines[i]})
			i++
		}
	}
	return rows
}

func isDiffRemoval(line string) bool {
	return strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---")
}

func isDiffAddition(line string) bool {
	return strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++")
}

// renderCurrentDiffFileSideBySide reuses m.diff.lineOffset as an approximate
// row offset: it isn't a pixel-exact mapping (paired rows can outnumber or
// undernumber the original lines when a hunk's +/- counts differ), but
// context lines — most of a typical diff — map 1:1, so drift stays small and
// self-corrects at the next context line.
func (m Model) renderCurrentDiffFileSideBySide(width, height int) string {
	file := m.diff.currentFile()
	if file == nil {
		return clampBlock("No diff returned for this merge request.", width, height)
	}

	rows := buildSideBySideRows(file.lines)
	if len(rows) == 0 {
		rows = []sideBySideRow{{left: "No textual changes in this file."}}
	}

	start := min(m.diff.lineOffset, max(0, len(rows)-1))
	end := min(start+height-2, len(rows))
	colWidth := max(10, (width-3)/2)

	var b strings.Builder
	b.WriteString(diffFileTitle(*file) + "\n")
	for _, row := range rows[start:end] {
		left := m.sideBySideCell(row.left, colWidth)
		right := m.sideBySideCell(row.right, colWidth)
		b.WriteString(left + " │ " + right + "\n")
	}
	return clampBlock(b.String(), width, height)
}

func (m Model) sideBySideCell(raw string, colWidth int) string {
	truncated := raw
	if len(truncated) > colWidth {
		truncated = truncate(truncated, colWidth)
	}
	styled := renderSideBySideCell(truncated, m.diffHideWhitespace)
	if pad := colWidth - lipgloss.Width(styled); pad > 0 {
		styled += strings.Repeat(" ", pad)
	}
	return styled
}

func renderSideBySideCell(line string, hideWhitespace bool) string {
	if line == "" {
		return ""
	}
	if hideWhitespace && isWhitespaceOnlyDiffLine(line) {
		return footerStyle.Render(string(line[0]) + " (ws)")
	}
	return styleDiffLineContent(line)
}

func diffFileTitle(file diffFile) string {
	title := file.path
	if len(file.flags) > 0 {
		title += " (" + strings.Join(file.flags, ", ") + ")"
	}
	if len(file.hunks) > 0 {
		title += "  " + footerStyle.Render(strconv.Itoa(len(file.hunks))+" hunks")
	}
	return titleStyle.Render(title)
}

func (m Model) diffHints() []hint {
	file := m.diff.currentFile()
	lines := 0
	if file != nil {
		lines = len(file.lines)
	}
	start := min(m.diff.lineOffset, max(0, lines-1))
	end := min(start+m.contentHeight()-4, lines)
	currentFile := 0
	if len(m.diff.files) > 0 {
		currentFile = m.diff.fileCursor + 1
	}
	return []hint{
		{fmt.Sprintf("file %d/%d", currentFile, len(m.diff.files)), ""},
		{fmt.Sprintf("%d-%d/%d", start+1, end, lines), ""},
		{"j/k", "scroll"},
		{"h/l", "file"},
		{"[/]", "hunk"},
		{"/", "search"},
		{"n/N", "match"},
		{"e", "editor"},
		{"w", "whitespace"},
		{"s", "side-by-side"},
		{"g/G", "top/end"},
		{"r", "refresh"},
		{"esc", "back"},
		{"q", "quit"},
	}
}

func (m Model) diffLines() []string {
	file := m.diff.currentFile()
	if file == nil {
		return nil
	}
	return file.lines
}

func renderDiffLine(line string, hideWhitespace, activeMatch bool) string {
	if hideWhitespace && isWhitespaceOnlyDiffLine(line) {
		return footerStyle.Render(string(line[0]) + " (whitespace only)")
	}
	if activeMatch {
		return searchStyle.Render(line)
	}
	return styleDiffLineContent(line)
}

func styleDiffLineContent(line string) string {
	switch {
	case strings.HasPrefix(line, "diff --"):
		return selectedStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return hunkStyle.Render(line)
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return additionStyle.Render(line)
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return deletionStyle.Render(line)
	default:
		return line
	}
}

// isWhitespaceOnlyDiffLine reports whether a unified-diff +/- line's content
// (after the prefix) is entirely whitespace — a trailing-space fix or a
// blank line added/removed, the kind of noise "hide whitespace" suppresses.
func isWhitespaceOnlyDiffLine(line string) bool {
	if len(line) == 0 {
		return false
	}
	if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
		return false
	}
	prefix := line[0]
	if prefix != '+' && prefix != '-' {
		return false
	}
	return strings.TrimSpace(line[1:]) == ""
}

func (d *diffState) currentFile() *diffFile {
	if len(d.files) == 0 || d.fileCursor < 0 || d.fileCursor >= len(d.files) {
		return nil
	}
	return &d.files[d.fileCursor]
}

func (d *diffState) moveFile(delta int) {
	if len(d.files) == 0 {
		return
	}
	d.fileCursor = min(max(d.fileCursor+delta, 0), len(d.files)-1)
	d.hunkCursor = 0
	d.lineOffset = 0
}

func (d *diffState) moveHunk(delta, height int) {
	file := d.currentFile()
	if file == nil || len(file.hunks) == 0 {
		return
	}
	d.hunkCursor = min(max(d.hunkCursor+delta, 0), len(file.hunks)-1)
	d.lineOffset = min(file.hunks[d.hunkCursor].lineIndex, d.maxLineOffset(height))
}

func (d *diffState) scroll(delta, height int) {
	d.lineOffset = min(max(d.lineOffset+delta, 0), d.maxLineOffset(height))
	d.syncHunkCursor()
}

func (d *diffState) scrollToEnd(height int) {
	d.lineOffset = d.maxLineOffset(height)
	d.syncHunkCursor()
}

func (d *diffState) search(query string, height int) bool {
	d.searchQuery = strings.TrimSpace(query)
	d.searchMatches = nil
	d.searchCursor = 0
	if d.searchQuery == "" {
		return false
	}

	needle := strings.ToLower(d.searchQuery)
	for fileIndex, file := range d.files {
		for lineIndex, line := range file.lines {
			if strings.Contains(strings.ToLower(line), needle) {
				d.searchMatches = append(d.searchMatches, diffMatch{fileIndex: fileIndex, lineIndex: lineIndex})
			}
		}
	}
	if len(d.searchMatches) == 0 {
		return false
	}

	d.searchCursor = d.firstMatchAtOrAfterCurrent()
	d.applySearchMatch(height)
	return true
}

func (d *diffState) moveSearchMatch(delta, height int) bool {
	if len(d.searchMatches) == 0 {
		if d.searchQuery == "" {
			return false
		}
		return d.search(d.searchQuery, height)
	}
	d.searchCursor = (d.searchCursor + delta + len(d.searchMatches)) % len(d.searchMatches)
	d.applySearchMatch(height)
	return true
}

func (d *diffState) firstMatchAtOrAfterCurrent() int {
	for i, match := range d.searchMatches {
		if match.fileIndex > d.fileCursor {
			return i
		}
		if match.fileIndex == d.fileCursor && match.lineIndex >= d.lineOffset {
			return i
		}
	}
	return 0
}

func (d *diffState) applySearchMatch(height int) {
	if len(d.searchMatches) == 0 || d.searchCursor < 0 || d.searchCursor >= len(d.searchMatches) {
		return
	}
	match := d.searchMatches[d.searchCursor]
	d.fileCursor = match.fileIndex
	d.lineOffset = min(match.lineIndex, d.maxLineOffset(height))
	d.syncHunkCursor()
}

func (d diffState) isActiveMatch(fileIndex, lineIndex int) bool {
	if len(d.searchMatches) == 0 || d.searchCursor < 0 || d.searchCursor >= len(d.searchMatches) {
		return false
	}
	match := d.searchMatches[d.searchCursor]
	return match.fileIndex == fileIndex && match.lineIndex == lineIndex
}

func (d diffState) searchStatus() string {
	if d.searchQuery == "" {
		return ""
	}
	if len(d.searchMatches) == 0 {
		return fmt.Sprintf("No matches for %q", d.searchQuery)
	}
	return fmt.Sprintf("Match %d/%d for %q", d.searchCursor+1, len(d.searchMatches), d.searchQuery)
}

func (d *diffState) maxLineOffset(height int) int {
	file := d.currentFile()
	if file == nil {
		return 0
	}
	return max(0, len(file.lines)-max(1, height-4))
}

func (d *diffState) syncHunkCursor() {
	file := d.currentFile()
	if file == nil || len(file.hunks) == 0 {
		d.hunkCursor = 0
		return
	}
	for i, hunk := range file.hunks {
		if hunk.lineIndex <= d.lineOffset {
			d.hunkCursor = i
		}
	}
}
