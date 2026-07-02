package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

var errorLinePattern = regexp.MustCompile(`(?i)\b(error|fail(ed|ure)?|fatal|exception|panic|traceback)\b`)

type logState struct {
	job     *gitlab.Job
	lines   []string
	isError []bool

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
	state := logState{job: job, lines: lines, isError: make([]bool, len(lines)), errorCursor: -1}
	for i, line := range lines {
		if errorLinePattern.MatchString(line) {
			state.isError[i] = true
			state.errorMatches = append(state.errorMatches, i)
		}
	}
	return state
}

// splitLogLines cleans a raw GitLab CI trace into displayable lines: ANSI
// escape codes are stripped (the TUI applies its own styling) and both "\r"
// and "\n" are treated as line breaks, since GitLab uses "\r" to overwrite
// progress-style output (npm/docker pulls) the same way a real terminal would.
func splitLogLines(raw string) []string {
	raw = ansiEscapePattern.ReplaceAllString(raw, "")
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
		lines = append(lines, line)
	}
	return lines
}

var (
	ansiEscapePattern    = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	sectionMarkerPattern = regexp.MustCompile(`^section_(start|end):\d+:\S+$`)
)

func (m Model) renderJobLog() string {
	height := m.logBodyHeight()

	title := "Job Log"
	if m.jobLog.job != nil {
		title = fmt.Sprintf("Log: %s (#%d)", m.jobLog.job.Name, m.jobLog.job.ID)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 72)) + "\n")

	lines := m.jobLog.lines
	start := min(m.jobLog.lineOffset, max(0, len(lines)-1))
	end := min(start+height, len(lines))
	for i := start; i < end; i++ {
		active := m.jobLog.isActiveMatch(i)
		b.WriteString(renderLogLine(lines[i], m.jobLog.isError[i], active) + "\n")
	}
	return b.String()
}

func renderLogLine(line string, isError, active bool) string {
	switch {
	case active:
		return searchStyle.Render(line)
	case isError:
		return errorStyle.Render(line)
	default:
		return line
	}
}

func (m Model) jobLogHints() []hint {
	lines := len(m.jobLog.lines)
	height := m.logBodyHeight()
	start := min(m.jobLog.lineOffset, max(0, lines-1))
	end := min(start+height, lines)
	return []hint{
		{fmt.Sprintf("%d-%d/%d", start+1, end, lines), ""},
		{"j/k", "scroll"},
		{"/", "search"},
		{"n/N", "match"},
		{"e/E", "error"},
		{"g/G", "top/end"},
		{"o", "open job"},
		{"y", "copy link"},
		{"R", "retry job"},
		{"r", "refresh"},
		{"esc", "back"},
		{"q", "quit"},
	}
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
