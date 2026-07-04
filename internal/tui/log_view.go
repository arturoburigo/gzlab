package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

var errorLinePattern = regexp.MustCompile(`(?i)\b(error|fail(ed|ure)?|fatal|exception|panic|traceback)\b`)

// maxLogDisplayLines caps how many lines the log viewer keeps in memory for
// rendering/search — very large logs (Épico 15) keep only the tail, where
// failures usually are. Saving to a file still writes the full raw trace.
const maxLogDisplayLines = 20000

type logState struct {
	job     *gitlab.Job
	raw     string // full untruncated trace, for "save to file"
	lines   []string
	isError []bool
	// truncated is how many leading lines were dropped from lines to stay
	// under maxLogDisplayLines; 0 means nothing was dropped.
	truncated int

	lineOffset int

	errorMatches []int
	errorCursor  int

	searchQuery   string
	searchMatches []int
	searchCursor  int
}

func newLogState(job *gitlab.Job, raw string) logState {
	lines := splitLogLines(raw)
	if len(lines) == 0 {
		lines = []string{"No log output for this job."}
	}

	state := logState{job: job, raw: raw, errorCursor: -1}
	if len(lines) > maxLogDisplayLines {
		state.truncated = len(lines) - maxLogDisplayLines
		lines = lines[state.truncated:]
	}
	state.lines = lines
	state.isError = make([]bool, len(lines))
	for i, line := range lines {
		if errorLinePattern.MatchString(line) {
			state.isError[i] = true
			state.errorMatches = append(state.errorMatches, i)
		}
	}
	return state
}

// splitLogLines cleans a raw GitLab CI trace into displayable lines: ANSI
// escape codes — including private CSI markers and OSC title/hyperlink
// sequences a plain "\x1b\[...[a-zA-Z]" regex misses — are stripped (the TUI
// applies its own styling), tabs are expanded (Go panics and Makefile output
// are full of them and lipgloss.Width counts a tab as 0 columns), and both
// "\r" and "\n" are treated as line breaks, since GitLab uses "\r" to
// overwrite progress-style output (npm/docker pulls) like a real terminal.
func splitLogLines(raw string) []string {
	raw = ansi.Strip(raw)
	raw = strings.ReplaceAll(raw, "\t", "    ")
	raw = strings.TrimRight(raw, "\r\n")
	if raw == "" {
		return nil
	}

	rawLines := strings.FieldsFunc(raw, func(r rune) bool { return r == '\r' || r == '\n' })
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if sectionMarkerPattern.MatchString(line) {
			continue
		}
		lines = append(lines, stripC0(line))
	}
	return lines
}

// stripC0 drops remaining C0 control bytes (stray BEL, backspace, ...) that
// ansi.Strip leaves alone because they're not part of an escape sequence —
// left in, they render as garbage or ring the terminal bell.
func stripC0(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
}

var sectionMarkerPattern = regexp.MustCompile(`^section_(start|end):\d+:\S+$`)

func (m Model) renderJobLog() string {
	height := m.logBodyHeight()
	width := m.contentWidth()

	title := "Job Log"
	if m.jobLog.job != nil {
		title = fmt.Sprintf("Log: %s (#%d)", m.jobLog.job.Name, m.jobLog.job.ID)
	}
	if m.jobLogFollowing {
		title += "  " + footerStyle.Render(refreshTag("following", m.lastJobLogFetch))
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n")
	b.WriteString(rule(width) + "\n")
	if m.jobLog.truncated > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("… %d earlier lines omitted for display; press 's' to save the full log …", m.jobLog.truncated)) + "\n")
	}

	rows, _ := m.logVisibleLines(m.jobLog.lineOffset, height, width)
	for _, row := range rows {
		b.WriteString(row + "\n")
	}
	return b.String()
}

// logVisibleLines renders the job log body starting at startOffset,
// soft-wrapping long lines instead of chopping them, and stops once height
// visual rows are used. It returns the rendered rows and the index just past
// the last source line consumed, so callers can report an accurate position
// (a wrapped line takes more than one row, so "start+height" would overcount).
func (m Model) logVisibleLines(startOffset, height, width int) ([]string, int) {
	lines := m.jobLog.lines
	start := min(startOffset, max(0, len(lines)-1))
	var out []string
	rows := 0
	i := start
	for ; i < len(lines) && rows < height; i++ {
		active := m.jobLog.isActiveMatch(i)
		styled := renderLogLine(lines[i], m.jobLog.isError[i], active, m.jobLog.searchQuery, width)
		for _, wrapped := range strings.Split(styled, "\n") {
			if rows >= height {
				break
			}
			out = append(out, wrapped)
			rows++
		}
	}
	return out, i
}

// renderLogLine styles a line (error lines in errorStyle; a search match gets
// only its matched span highlighted — active in full searchStyle, other
// visible matches dimmer — instead of the whole line being painted over) and
// soft-wraps it to width instead of chopping it, so the tail (where the
// actual error message usually lives) stays reachable.
func renderLogLine(line string, isError, active bool, query string, width int) string {
	base := lipgloss.NewStyle()
	if isError {
		base = errorStyle
	}
	var styled string
	idx := -1
	if query != "" {
		idx = strings.Index(strings.ToLower(line), strings.ToLower(query))
	}
	switch {
	case idx < 0:
		styled = base.Render(line)
	default:
		matchStyle := searchDimStyle
		if active {
			matchStyle = searchStyle
		}
		before, match, after := line[:idx], line[idx:idx+len(query)], line[idx+len(query):]
		styled = base.Render(before) + matchStyle.Render(match) + base.Render(after)
	}
	if width > 0 {
		return ansi.Wrap(styled, width, "")
	}
	return styled
}

func (m Model) jobLogHints() []hint {
	lines := len(m.jobLog.lines)
	height := m.logBodyHeight()
	width := m.contentWidth()
	start := min(m.jobLog.lineOffset, max(0, lines-1))
	_, end := m.logVisibleLines(start, height, width)
	followLabel := "follow"
	if m.jobLogFollowing {
		followLabel = "stop following"
	}
	return []hint{
		{fmt.Sprintf("%d-%d/%d", start+1, end, lines), ""},
		{"j/k", "scroll"},
		{"/", "search"},
		{"n/N", "match"},
		{"e/E", "error"},
		{"g/G", "top/end"},
		{"o", "open job"},
		{"y", "copy line"},
		{"s", "save log"},
		{"f", followLabel},
		{"R", "retry job"},
		{"r", "refresh"},
		{"esc", "back"},
		{"q", "quit"},
	}
}

// currentLineText is what 'y' copies: the active search match if one is
// selected, else the line at the top of the current viewport.
func (l logState) currentLineText() string {
	if idx, ok := l.activeSearchLine(); ok {
		return l.lines[idx]
	}
	if l.lineOffset >= 0 && l.lineOffset < len(l.lines) {
		return l.lines[l.lineOffset]
	}
	return ""
}

func (l logState) activeSearchLine() (int, bool) {
	if len(l.searchMatches) == 0 || l.searchCursor < 0 || l.searchCursor >= len(l.searchMatches) {
		return 0, false
	}
	idx := l.searchMatches[l.searchCursor]
	if idx < 0 || idx >= len(l.lines) {
		return 0, false
	}
	return idx, true
}

func (l *logState) scroll(delta, height int) {
	l.lineOffset = min(max(l.lineOffset+delta, 0), l.maxLineOffset(height))
}

func (l *logState) scrollToEnd(height int) {
	l.lineOffset = l.maxLineOffset(height)
}

func (l *logState) maxLineOffset(height int) int {
	return max(0, len(l.lines)-max(1, height))
}

func (l *logState) search(query string, height int) bool {
	l.searchQuery = strings.TrimSpace(query)
	l.searchMatches = nil
	l.searchCursor = 0
	if l.searchQuery == "" {
		return false
	}

	needle := strings.ToLower(l.searchQuery)
	for i, line := range l.lines {
		if strings.Contains(strings.ToLower(line), needle) {
			l.searchMatches = append(l.searchMatches, i)
		}
	}
	if len(l.searchMatches) == 0 {
		return false
	}

	l.searchCursor = l.firstMatchAtOrAfterCurrent()
	l.applySearchMatch(height)
	return true
}

func (l *logState) moveSearchMatch(delta, height int) bool {
	if len(l.searchMatches) == 0 {
		if l.searchQuery == "" {
			return false
		}
		return l.search(l.searchQuery, height)
	}
	l.searchCursor = (l.searchCursor + delta + len(l.searchMatches)) % len(l.searchMatches)
	l.applySearchMatch(height)
	return true
}

func (l *logState) firstMatchAtOrAfterCurrent() int {
	return firstIndexAtOrAfter(l.searchMatches, l.lineOffset)
}

func (l *logState) applySearchMatch(height int) {
	if len(l.searchMatches) == 0 || l.searchCursor < 0 || l.searchCursor >= len(l.searchMatches) {
		return
	}
	l.lineOffset = min(l.searchMatches[l.searchCursor], l.maxLineOffset(height))
}

func (l logState) isActiveMatch(index int) bool {
	if len(l.searchMatches) == 0 || l.searchCursor < 0 || l.searchCursor >= len(l.searchMatches) {
		return false
	}
	return l.searchMatches[l.searchCursor] == index
}

func (l logState) searchStatus() string {
	if l.searchQuery == "" {
		return ""
	}
	if len(l.searchMatches) == 0 {
		return fmt.Sprintf("No matches for %q", l.searchQuery)
	}
	return fmt.Sprintf("Match %d/%d for %q", l.searchCursor+1, len(l.searchMatches), l.searchQuery)
}

func (l *logState) moveErrorMatch(delta, height int) bool {
	if len(l.errorMatches) == 0 {
		return false
	}
	if l.errorCursor < 0 {
		l.errorCursor = l.firstErrorAtOrAfterCurrent()
	} else {
		l.errorCursor = (l.errorCursor + delta + len(l.errorMatches)) % len(l.errorMatches)
	}
	l.lineOffset = min(l.errorMatches[l.errorCursor], l.maxLineOffset(height))
	return true
}

func (l *logState) firstErrorAtOrAfterCurrent() int {
	return firstIndexAtOrAfter(l.errorMatches, l.lineOffset)
}

// firstIndexAtOrAfter returns the position within indices of the first value
// >= from, or 0 (wrap to the first entry) if none qualifies.
func firstIndexAtOrAfter(indices []int, from int) int {
	for i, line := range indices {
		if line >= from {
			return i
		}
	}
	return 0
}

func (l logState) errorStatus() string {
	if len(l.errorMatches) == 0 {
		return "No error lines found"
	}
	return fmt.Sprintf("Error %d/%d", l.errorCursor+1, len(l.errorMatches))
}
