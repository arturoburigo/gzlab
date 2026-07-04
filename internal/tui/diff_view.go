package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// diffSideBySideMinWidth is the diff-content budget (not terminal width)
// below which side-by-side columns get too narrow to be useful.
const diffSideBySideMinWidth = 100

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
	lineNums  []diffLineNum
	hunks     []diffHunk
	additions int
	deletions int
	// lineIndents is, per rendered line, the common leading-whitespace width of
	// that line's hunk — stripped at render time so a deeply-nested hunk isn't
	// pushed off-screen by indentation every line in it shares. It's per-hunk
	// (not per-file) so a shallow hunk elsewhere in the same file can't hold a
	// deep hunk hostage to its smaller indent.
	lineIndents []int
}

// diffLineNum is the old/new source line number a rendered diff row maps to;
// zero means "no number for this row" (a header or metadata line).
type diffLineNum struct {
	old, new int
}

var hunkHeaderPattern = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// computeLineNumbers walks a file's rendered lines, tracking the old/new
// counters a unified diff implies, so each row can carry a line-number
// gutter — the only positional cue today is the raw "@@" header text.
func computeLineNumbers(lines []string) []diffLineNum {
	nums := make([]diffLineNum, len(lines))
	var old, new int
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			if m := hunkHeaderPattern.FindStringSubmatch(line); m != nil {
				old, _ = strconv.Atoi(m[1])
				new, _ = strconv.Atoi(m[2])
			}
		case isDiffMetadataLine(line):
		case isDiffAddition(line):
			nums[i] = diffLineNum{new: new}
			new++
		case isDiffRemoval(line):
			nums[i] = diffLineNum{old: old}
			old++
		default:
			nums[i] = diffLineNum{old: old, new: new}
			old++
			new++
		}
	}
	return nums
}

// diffGutter renders a fixed-width dim "old new " line-number prefix, blank
// for rows with no number (headers/metadata).
func diffGutter(n diffLineNum) string {
	oldCol := "    "
	if n.old > 0 {
		oldCol = fmt.Sprintf("%4d", n.old)
	}
	newCol := "    "
	if n.new > 0 {
		newCol = fmt.Sprintf("%4d", n.new)
	}
	return footerStyle.Render(oldCol + " " + newCol + " ")
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

// maxDiffFileLines caps how many lines of a single file's diff are kept for
// display — the large-diff fallback (Épico 13). Without it, a generated
// lockfile or minified bundle changing thousands of lines makes the viewer
// unusably slow to scroll and search.
const maxDiffFileLines = 4000

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
	file.lineNums = computeLineNumbers(file.lines)
	file.lineIndents = hunkIndents(file.lines)
	truncateDiffFile(file)
}

// hunkIndents returns, per line, the common leading-whitespace of the hunk that
// line belongs to (0 for lines before the first hunk). Each "@@" starts a new
// hunk whose indent is computed independently, so a deeply-nested hunk gets its
// full indent stripped even when a shallower hunk shares the file.
func hunkIndents(lines []string) []int {
	indents := make([]int, len(lines))
	i := 0
	for i < len(lines) {
		if !strings.HasPrefix(lines[i], "@@") {
			i++
			continue
		}
		start := i
		i++
		for i < len(lines) && !strings.HasPrefix(lines[i], "@@") {
			i++
		}
		indent := commonDiffIndent(lines[start:i])
		for j := start; j < i; j++ {
			indents[j] = indent
		}
	}
	return indents
}

// diffLineMarker reports the unified-diff marker char of a content line — '+',
// '-', or ' ' (context) — and whether line is a content line at all (false for
// "@@", "+++"/"---", "diff --git", etc.). Body is what follows the marker.
func diffLineMarker(line string) (marker byte, isContent bool) {
	if line == "" {
		return 0, false
	}
	switch line[0] {
	case '+':
		return '+', !strings.HasPrefix(line, "+++")
	case '-':
		return '-', !strings.HasPrefix(line, "---")
	case ' ':
		return ' ', true
	default:
		return 0, false
	}
}

// commonDiffIndent is the largest leading-space count shared by every non-blank
// code line's body (textwrap.dedent-style). Stripping it preserves the diff's
// relative indentation while reclaiming the horizontal room a deeply-nested
// block wastes on indentation identical on every line.
func commonDiffIndent(lines []string) int {
	min := -1
	for _, line := range lines {
		if _, ok := diffLineMarker(line); !ok {
			continue
		}
		body := line[1:]
		if strings.TrimSpace(body) == "" {
			continue // blank lines don't constrain the common indent
		}
		indent := len(body) - len(strings.TrimLeft(body, " "))
		if min < 0 || indent < min {
			min = indent
		}
		if min == 0 {
			break
		}
	}
	if min < 0 {
		return 0
	}
	return min
}

// dedentDiffLine strips up to n leading spaces from a content line's body,
// keeping its +/-/space marker; non-content lines (headers/metadata) pass
// through untouched.
func dedentDiffLine(line string, n int) string {
	if n <= 0 {
		return line
	}
	marker, ok := diffLineMarker(line)
	if !ok {
		return line
	}
	body := line[1:]
	stripped := 0
	for stripped < n && stripped < len(body) && body[stripped] == ' ' {
		stripped++
	}
	return string(marker) + body[stripped:]
}

// truncateDiffFile trims file.lines/hunks to maxDiffFileLines for display.
// additions/deletions above are already computed from the full diff, so the
// file-list stats stay accurate; only rendering and hunk navigation shrink.
func truncateDiffFile(file *diffFile) {
	if len(file.lines) <= maxDiffFileLines {
		return
	}
	total := len(file.lines)
	file.lines = append(file.lines[:maxDiffFileLines:maxDiffFileLines],
		fmt.Sprintf("… diff truncated: showing %d of %d lines; press 'o' to open the full file in the browser …", maxDiffFileLines, total))
	file.lineNums = append(file.lineNums[:maxDiffFileLines:maxDiffFileLines], diffLineNum{})
	file.lineIndents = append(file.lineIndents[:maxDiffFileLines:maxDiffFileLines], 0)
	file.flags = appendUnique(file.flags, "truncated")

	kept := file.hunks[:0]
	for _, h := range file.hunks {
		if h.lineIndex < maxDiffFileLines {
			kept = append(kept, h)
		}
	}
	file.hunks = kept
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
	// Expand tabs before any width math runs: lipgloss.Width counts a tab as
	// 0 columns while every terminal renders it 4+ wide, so clamping/padding
	// against unexpanded tabs undercounts every indented line (Go diffs are
	// full of them) and the pane grows a stair-stepped extra row per line.
	raw = strings.ReplaceAll(raw, "\t", "    ")
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
	sidebarWidth := m.diffSidebarWidth()
	// Budget from the frame's real content width, not m.width directly —
	// the nav sidebar + pane borders/padding already eat ~24-25 columns that
	// the old "m.width-10" guess didn't account for, so long lines used to
	// overshoot the pane and get byte-truncated with ANSI-bleed artifacts.
	gutter := 0
	if sidebarWidth > 0 {
		gutter = 2
	}
	diffWidth := max(30, m.contentWidth()-sidebarWidth-gutter)
	sideBySideEngaged := m.diffSideBySide && diffWidth >= diffSideBySideMinWidth

	title := "Diff"
	if m.detail != nil {
		title = fmt.Sprintf("Diff !%d - %s", m.detail.IID, m.detail.Title)
	}
	switch {
	case sideBySideEngaged:
		title += "  " + footerStyle.Render("[side-by-side]")
	case m.diffSideBySide:
		title += "  " + footerStyle.Render("[side-by-side: widen terminal]")
	}
	if m.diffHideWhitespace {
		title += "  " + footerStyle.Render("[whitespace hidden]")
	}
	header := titleStyle.Render(title) + "\n"
	header += rule(m.contentWidth()) + "\n"
	var content string
	if sideBySideEngaged {
		content = m.renderCurrentDiffFileSideBySide(diffWidth, bodyHeight)
	} else {
		content = m.renderCurrentDiffFile(diffWidth, bodyHeight)
	}
	if sidebarWidth == 0 {
		return header + content
	}
	sidebar := m.renderDiffFiles(sidebarWidth, bodyHeight)
	return header + lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", content)
}

func (m Model) diffSidebarWidth() int {
	if len(m.diff.files) <= 1 {
		return 0
	}
	return min(34, max(18, m.width/5))
}

func (m Model) renderDiffFiles(width, height int) string {
	lines := make([]string, 0, len(m.diff.files)+2)
	lines = append(lines, tableHead.Render("Files"))
	for i, file := range m.diff.files {
		prefix := emptyMarker
		selected := i == m.diff.fileCursor
		if selected {
			prefix = cursorMarker
		}
		path := prefix + truncatePathLeft(file.path, max(8, width-12))
		if selected {
			// A selected row is a single inverse-video style applied once;
			// nesting the addition/deletion colors inside it would let their
			// own reset codes cut the selection background short mid-line.
			stats := fmt.Sprintf("+%d -%d", file.additions, file.deletions)
			lines = append(lines, selectedStyle.Render(spread(path, stats, width)))
			continue
		}
		stats := additionStyle.Render(fmt.Sprintf("+%d", file.additions)) + " " + deletionStyle.Render(fmt.Sprintf("-%d", file.deletions))
		lines = append(lines, spread(valueStyle.Render(path), stats, width))
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
		idx := start + lineIndex
		active := m.diff.isActiveMatch(m.diff.fileCursor, idx)
		gutter := ""
		if idx < len(file.lineNums) {
			gutter = diffGutter(file.lineNums[idx])
		}
		indent := 0
		if idx < len(file.lineIndents) {
			indent = file.lineIndents[idx]
		}
		b.WriteString(gutter + renderDiffLine(line, file.path, indent, m.diffHideWhitespace, active, m.diff.searchQuery) + "\n")
	}
	return clampBlock(b.String(), width, height)
}

// sideBySideRow is one visual row of the side-by-side diff view. indent is the
// per-hunk leading-whitespace stripped from both cells (see diffFile.lineIndents).
type sideBySideRow struct {
	left   string
	right  string
	span   string
	indent int
}

// buildSideBySideRows pairs up a unified diff's line-level removal/addition
// runs into left/right rows: a run of consecutive "-" lines is paired
// index-wise against the run of "+" lines that follows it (the shorter run
// pads with a blank cell), which is how most terminal diff viewers do
// side-by-side without full intraline (LCS) diffing. Context/header lines
// mirror unchanged on both sides. lineIndents carries each source line's
// per-hunk dedent width onto the rows it produces.
func buildSideBySideRows(lines []string, lineIndents []int) []sideBySideRow {
	indentAt := func(i int) int {
		if i >= 0 && i < len(lineIndents) {
			return lineIndents[i]
		}
		return 0
	}
	rows := make([]sideBySideRow, 0, len(lines))
	i := 0
	for i < len(lines) {
		switch {
		case isDiffMetadataLine(lines[i]):
			i++
		case strings.HasPrefix(lines[i], "@@"):
			rows = append(rows, sideBySideRow{span: lines[i]})
			i++
		case isDiffRemoval(lines[i]):
			indent := indentAt(i)
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
				row := sideBySideRow{indent: indent}
				if j < len(removals) {
					row.left = removals[j]
				}
				if j < len(additions) {
					row.right = additions[j]
				}
				rows = append(rows, row)
			}
		case isDiffAddition(lines[i]):
			rows = append(rows, sideBySideRow{right: lines[i], indent: indentAt(i)})
			i++
		default:
			rows = append(rows, sideBySideRow{left: lines[i], right: lines[i], indent: indentAt(i)})
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

func isDiffMetadataLine(line string) bool {
	for _, prefix := range []string{
		"diff --git ",
		"index ",
		"--- ",
		"+++ ",
		"new file mode ",
		"deleted file mode ",
		"similarity index ",
		"rename from ",
		"rename to ",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
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

	rows := buildSideBySideRows(file.lines, file.lineIndents)
	if len(rows) == 0 {
		rows = []sideBySideRow{{left: "No textual changes in this file."}}
	}

	start := min(m.diff.lineOffset, max(0, len(rows)-1))
	end := min(start+height-2, len(rows))
	colWidth := max(10, (width-5)/2)

	var b strings.Builder
	b.WriteString(diffFileTitle(*file) + "\n")
	b.WriteString(m.sideBySideHeader(*file, colWidth) + "\n")
	for _, row := range rows[start:end] {
		if row.span != "" {
			b.WriteString(clampStyledLine(hunkStyle.Render(row.span), width) + "\n")
			continue
		}
		left := m.sideBySideCell(row.left, colWidth, file.path, row.indent)
		right := m.sideBySideCell(row.right, colWidth, file.path, row.indent)
		b.WriteString(left + ruleStyle.Render(" │ ") + right + "\n")
	}
	return clampBlock(b.String(), width, height)
}

func (m Model) sideBySideHeader(file diffFile, colWidth int) string {
	oldName := firstNonEmpty(file.oldPath, file.path, "old")
	newName := firstNonEmpty(file.newPath, file.path, "new")
	left := tableHead.Render(fitDiffCell("OLD  "+oldName, colWidth))
	right := tableHead.Render(fitDiffCell("NEW  "+newName, colWidth))
	return left + ruleStyle.Render(" │ ") + right
}

func (m Model) sideBySideCell(raw string, colWidth int, filename string, indent int) string {
	// Style first (syntax highlighting emits per-token ANSI), then truncate
	// with the ANSI-aware truncate so colour spans aren't cut mid-escape.
	styled := renderSideBySideCell(dedentDiffLine(raw, indent), filename, m.diffHideWhitespace)
	if lipgloss.Width(styled) > colWidth {
		styled = truncate(styled, colWidth)
	}
	if pad := colWidth - lipgloss.Width(styled); pad > 0 {
		styled += strings.Repeat(" ", pad)
	}
	return styled
}

// truncatePathLeft truncates from the left, keeping the tail — the basename,
// the part that actually distinguishes files in the same package — intact
// instead of cutting it off in favor of a shared parent directory prefix.
func truncatePathLeft(path string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(path)
	if w <= width {
		return path
	}
	return ansi.TruncateLeft(path, w-width+1, "…")
}

func clampStyledLine(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return truncate(line, max(1, width))
}

func fitDiffCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = truncate(s, width)
	if pad := width - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func renderSideBySideCell(line, filename string, hideWhitespace bool) string {
	if line == "" {
		return ""
	}
	if hideWhitespace && isWhitespaceOnlyDiffLine(line) {
		return footerStyle.Render(string(line[0]) + " (ws)")
	}
	return styleDiffLineContent(line, filename)
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
		{"y", "copy line"},
		{"Y", "copy hunk"},
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

// renderDiffLine styles line for the unified view. When query is non-empty
// and matches, only the matched span is highlighted (active gets the full
// search style, other matches a dimmer one) instead of the whole line being
// painted over — with N visible matches, "Match 3/17" used to give no
// on-screen way to tell which of the other N-1 was which.
func renderDiffLine(line, filename string, indent int, hideWhitespace, activeMatch bool, query string) string {
	line = dedentDiffLine(line, indent)
	if hideWhitespace && isWhitespaceOnlyDiffLine(line) {
		return footerStyle.Render(string(line[0]) + " (whitespace only)")
	}
	if query == "" {
		return styleDiffLineContent(line, filename)
	}
	idx := strings.Index(strings.ToLower(line), strings.ToLower(query))
	if idx < 0 {
		return styleDiffLineContent(line, filename)
	}
	base := lineContentStyle(line)
	matchStyle := searchDimStyle
	if activeMatch {
		matchStyle = searchStyle
	}
	before, match, after := line[:idx], line[idx:idx+len(query)], line[idx+len(query):]
	return base.Render(before) + matchStyle.Render(match) + base.Render(after)
}

// styleDiffLineContent colours one diff line: +/- lines keep a solid green/red
// foreground (the diff signal), while context and other lines are
// syntax-highlighted by language so the surrounding code reads in colour.
func styleDiffLineContent(line, filename string) string {
	switch {
	case strings.HasPrefix(line, "diff --"):
		return selectedStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return hunkStyle.Render(line)
	case isDiffAddition(line):
		return additionStyle.Render(line)
	case isDiffRemoval(line):
		return deletionStyle.Render(line)
	case isDiffMetadataLine(line):
		return line
	default:
		// Context line: keep its leading space marker plain, syntax-highlight
		// the code after it.
		if len(line) > 0 && line[0] == ' ' {
			return " " + highlightCode(filename, line[1:])
		}
		return highlightCode(filename, line)
	}
}

func lineContentStyle(line string) lipgloss.Style {
	switch {
	case strings.HasPrefix(line, "diff --"):
		return selectedStyle
	case strings.HasPrefix(line, "@@"):
		return hunkStyle
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return additionStyle
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return deletionStyle
	default:
		return lipgloss.NewStyle()
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

// currentLineText is what 'y' copies: the active search match in the
// current file if one is selected, else the line at the top of the viewport.
func (d diffState) currentLineText() string {
	file := d.currentFile()
	if file == nil {
		return ""
	}
	if len(d.searchMatches) > 0 && d.searchCursor >= 0 && d.searchCursor < len(d.searchMatches) {
		match := d.searchMatches[d.searchCursor]
		if match.fileIndex == d.fileCursor && match.lineIndex >= 0 && match.lineIndex < len(file.lines) {
			return file.lines[match.lineIndex]
		}
	}
	if d.lineOffset >= 0 && d.lineOffset < len(file.lines) {
		return file.lines[d.lineOffset]
	}
	return ""
}

// currentHunkText is what 'Y' copies: the full text of the hunk currently in
// view (from its "@@ ... @@" header to the next hunk or end of file).
func (d diffState) currentHunkText() string {
	file := d.currentFile()
	if file == nil || len(file.hunks) == 0 {
		return ""
	}
	idx := min(max(d.hunkCursor, 0), len(file.hunks)-1)
	start := file.hunks[idx].lineIndex
	end := len(file.lines)
	if idx+1 < len(file.hunks) {
		end = file.hunks[idx+1].lineIndex
	}
	return strings.Join(file.lines[start:end], "\n")
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
