package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// discussThread is a navigable entry on the discussions screen: one rendered
// thread, remembering where its header sits in lines and whether it can be
// resolved. The cursor moves over these, not over raw lines.
type discussThread struct {
	id         string
	resolvable bool
	resolved   bool
	headerLine int
}

// discussState holds the scrollable, pre-rendered view of an MR's discussion
// threads plus the thread cursor used for resolve actions.
type discussState struct {
	discussions []gitlab.Discussion
	lines       []string
	threads     []discussThread
	cursor      int
	lineOffset  int
}

func newDiscussState(discussions []gitlab.Discussion, width int) discussState {
	lines, threads := discussionView(discussions, width)
	return discussState{discussions: discussions, lines: lines, threads: threads}
}

// discussionView is the display policy for the discussions screen. GitLab's
// endpoint mixes human comments with "system" notes (its own activity log) and
// diff threads carry a resolved/unresolved state. The policy here:
//   - hide system notes (drop a thread entirely if it has none left);
//   - mark resolvable threads with ✓ (resolved) or ○ (open);
//   - indent replies (the 2nd+ note in a thread);
//   - word-wrap bodies to width instead of clipping to one ellipsis line;
//   - dim resolved threads and show file:line for code-anchored comments.
//
// It returns the flat display lines and, alongside, a navigable index of the
// threads that survived filtering.
func discussionView(discussions []gitlab.Discussion, width int) ([]string, []discussThread) {
	var lines []string
	var threads []discussThread

	for _, d := range discussions {
		visible := humanNotes(d.Notes)
		if len(visible) == 0 {
			continue
		}

		resolved := d.Resolvable() && d.Resolved()
		threads = append(threads, discussThread{
			id:         d.ID,
			resolvable: d.Resolvable(),
			resolved:   d.Resolved(),
			headerLine: len(lines),
		})

		for i, note := range visible {
			indent := ""
			if i > 0 {
				indent = "  " // replies sit under the thread opener
			}

			authorStyle := discussHeaderStyle
			if resolved {
				authorStyle = footerStyle
			}
			header := indent + resolveMarker(d, i) + authorStyle.Render(note.Author)
			if !note.CreatedAt.IsZero() {
				header += footerStyle.Render("  " + note.CreatedAt.Format("2006-01-02 15:04"))
			}
			lines = append(lines, header)

			if note.PositionPath != "" && note.PositionLine > 0 {
				lines = append(lines, indent+"  "+footerStyle.Render(fmt.Sprintf("%s:%d", note.PositionPath, note.PositionLine)))
			}

			bodyWidth := max(10, width-len(indent))
			for _, wline := range strings.Split(ansi.Wordwrap(note.Body, bodyWidth, ""), "\n") {
				if resolved {
					lines = append(lines, footerStyle.Render(indent+wline))
					continue
				}
				lines = append(lines, indent+wline)
			}
			lines = append(lines, "")
		}
	}
	return lines, threads
}

func humanNotes(notes []gitlab.Note) []gitlab.Note {
	var out []gitlab.Note
	for _, n := range notes {
		if !n.System {
			out = append(out, n)
		}
	}
	return out
}

// resolveMarker returns the ✓/○ status glyph shown on a thread's opening
// note. Non-resolvable threads (plain top-level comments) get a blank
// same-width placeholder instead of nothing, so author columns line up with
// resolvable threads instead of sitting 2 cells further left.
func resolveMarker(d gitlab.Discussion, noteIndex int) string {
	if noteIndex != 0 {
		return ""
	}
	if !d.Resolvable() {
		return "  "
	}
	if d.Resolved() {
		return discussResolvedStyle.Render("✓ ")
	}
	return discussUnresolvedStyle.Render("○ ")
}

func (m Model) renderDiscussions() string {
	height := m.discussBodyHeight()

	lines := m.discuss.lines
	if len(lines) == 0 {
		return footerStyle.Render("No comments yet. Press c to add one.") + "\n"
	}

	selectedHeader := -1
	if thread, ok := m.discuss.currentThread(); ok {
		selectedHeader = thread.headerLine
	}

	var b strings.Builder
	start := min(m.discuss.lineOffset, max(0, len(lines)-1))
	end := min(start+height, len(lines))
	for i := start; i < end; i++ {
		gutter := emptyMarker
		if i == selectedHeader {
			gutter = cursorStyle.Render(cursorMarker)
		}
		b.WriteString(gutter + lines[i] + "\n")
	}
	return b.String()
}

func (m Model) discussHints() []hint {
	hints := []hint{{"j/k", "thread"}, {"g/G", "top/end"}, {"c", "comment"}}
	if thread, ok := m.discuss.currentThread(); ok {
		hints = append(hints, hint{"R", "reply"})
		if thread.resolvable {
			label := "resolve"
			if thread.resolved {
				label = "unresolve"
			}
			hints = append(hints, hint{"t", label})
		}
	}
	return append(hints, hint{"o", "open"}, hint{"r", "refresh"}, hint{"esc", "back"}, hint{"q", "quit"})
}

func (m Model) discussBodyHeight() int {
	return max(3, m.contentHeight())
}

func (d discussState) currentThread() (discussThread, bool) {
	if d.cursor < 0 || d.cursor >= len(d.threads) {
		return discussThread{}, false
	}
	return d.threads[d.cursor], true
}

// moveCursor steps the thread cursor and scrolls to keep the selected thread's
// header on screen. With no navigable threads it falls back to line scrolling.
func (d *discussState) moveCursor(delta, height int) {
	if len(d.threads) == 0 {
		d.scroll(delta, height)
		return
	}
	d.cursor = min(max(d.cursor+delta, 0), len(d.threads)-1)
	header := d.threads[d.cursor].headerLine
	if header < d.lineOffset {
		d.lineOffset = header
	}
	if header >= d.lineOffset+height {
		d.lineOffset = header - height + 1
	}
	d.lineOffset = min(max(d.lineOffset, 0), d.maxLineOffset(height))
}

func (d *discussState) scroll(delta, height int) {
	d.lineOffset = min(max(d.lineOffset+delta, 0), d.maxLineOffset(height))
}

func (d *discussState) scrollToEnd(height int) {
	d.lineOffset = d.maxLineOffset(height)
}

func (d *discussState) maxLineOffset(height int) int {
	return max(0, len(d.lines)-max(1, height))
}
