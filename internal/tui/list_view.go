package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// mrScope selects which merge requests the list screen fetches: the current
// project's, or one of GitLab's cross-project views (Épico 11).
type mrScope int

const (
	mrScopeProject mrScope = iota
	mrScopeCreatedByMe
	mrScopeAssignedToMe
	mrScopeToReview
)

func (s mrScope) label() string {
	switch s {
	case mrScopeCreatedByMe:
		return "mine"
	case mrScopeAssignedToMe:
		return "assigned to me"
	case mrScopeToReview:
		return "to review"
	default:
		return "project"
	}
}

func (s mrScope) next() mrScope {
	return (s + 1) % 4
}

// mrQuickFilter narrows the already-loaded list client-side, without a
// refetch — the plan's draft/ready/pipeline-failing quick filters.
type mrQuickFilter int

const (
	mrFilterAll mrQuickFilter = iota
	mrFilterDraft
	mrFilterReady
	mrFilterFailing
)

func (f mrQuickFilter) label() string {
	switch f {
	case mrFilterDraft:
		return "draft"
	case mrFilterReady:
		return "ready"
	case mrFilterFailing:
		return "pipeline failing"
	default:
		return "all"
	}
}

func (f mrQuickFilter) next() mrQuickFilter {
	return (f + 1) % 4
}

// filteredMRs applies the active quick filter to the loaded MR list. Cursor
// indexing on the list screen always goes through this, not m.mrs directly.
func (m Model) filteredMRs() []*gitlab.MergeRequest {
	if m.mrFilter == mrFilterAll {
		return m.mrs
	}
	out := make([]*gitlab.MergeRequest, 0, len(m.mrs))
	for _, mr := range m.mrs {
		switch m.mrFilter {
		case mrFilterDraft:
			if mr.Draft {
				out = append(out, mr)
			}
		case mrFilterReady:
			if !mr.Draft {
				out = append(out, mr)
			}
		case mrFilterFailing:
			if mr.Pipeline != nil && mr.Pipeline.Status == gitlab.PipelineStatusFailed {
				out = append(out, mr)
			}
		}
	}
	return out
}

// anyPipelineLoaded reports whether any MR in mrs carries pipeline data.
// GitLab's list endpoints don't populate Pipeline (only GetMergeRequest
// does), so the "pipeline failing" quick filter can never match against a
// freshly loaded list — this lets the empty state say so instead of just
// looking like a silently broken filter.
func anyPipelineLoaded(mrs []*gitlab.MergeRequest) bool {
	for _, mr := range mrs {
		if mr.Pipeline != nil {
			return true
		}
	}
	return false
}

// listRowWidths sizes the list's columns from the pane's real content width
// instead of a fixed 44/72 that leaves most of a wide terminal blank; author
// and age columns only appear once there's room for them.
func listRowWidths(width int) (titleW, authorW, ageW int, showExtra bool) {
	const iidW, stateW = 7, 10
	showExtra = width >= 100
	if showExtra {
		authorW, ageW = 16, 8
	}
	fixed := iidW + 1 + stateW + 1
	if showExtra {
		fixed += authorW + 1 + ageW + 1
	}
	titleW = max(16, width-fixed)
	return titleW, authorW, ageW, showExtra
}

func mrStateLabel(mr *gitlab.MergeRequest) string {
	if mr.Draft {
		return "DRAFT"
	}
	return string(mr.State)
}

// mrStateTag colors the state so opened/merged/closed stop being visually
// identical plain text. Only safe to use on unselected rows: nesting a
// pre-styled fragment inside the selected row's single inverse-video Render
// call would let this style's own reset cut that highlight short (D1).
func mrStateTag(mr *gitlab.MergeRequest) string {
	if mr.Draft {
		return confirmStyle.Render("DRAFT")
	}
	switch mr.State {
	case gitlab.MergeRequestStateMerged:
		return pipelineSuccess.Render(string(mr.State))
	case gitlab.MergeRequestStateClosed:
		return pipelineFailed.Render(string(mr.State))
	default:
		return pipelineNeutral.Render(string(mr.State))
	}
}

func (m Model) renderList() string {
	title := fmt.Sprintf("Merge Requests - %s", m.mrScope.label())
	if m.mrScope == mrScopeProject && m.dash != nil && m.dash.Project != nil {
		title = fmt.Sprintf("Merge Requests - %s", m.dash.Project.PathWithNamespace)
	}
	if m.mrFilter != mrFilterAll {
		title += "  " + footerStyle.Render("["+m.mrFilter.label()+"]")
	}

	width := m.contentWidth()
	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n")
	b.WriteString(rule(width) + "\n")

	mrs := m.filteredMRs()
	if len(mrs) == 0 {
		empty := "No open merge requests."
		switch {
		case m.mrFilter == mrFilterFailing && !anyPipelineLoaded(m.mrs):
			empty = "Pipeline status is not loaded for the list view."
		case len(m.mrs) > 0:
			empty = "No merge requests match the current filter."
		}
		b.WriteString("\n" + footerStyle.Render(empty) + "\n")
		return b.String()
	}

	titleW, authorW, ageW, showExtra := listRowWidths(width)
	header := fmt.Sprintf("  %-7s %-*s", "MR", titleW, "TITLE")
	if showExtra {
		header += fmt.Sprintf(" %-*s %-*s", authorW, "AUTHOR", ageW, "AGE")
	}
	header += " STATE"
	b.WriteString(tableHead.Render(header) + "\n")

	limit := max(3, m.contentHeight()-4)
	start, end := visibleWindow(len(mrs), m.cursor, limit)
	if start > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  ↑ %d more above", start)) + "\n")
	}
	for i := start; i < end; i++ {
		mr := mrs[i]
		selected := i == m.cursor
		cursor := emptyMarker
		if selected {
			cursor = cursorMarker
		}
		titleCell := lipgloss.NewStyle().Width(titleW).MaxWidth(titleW).Render(truncate(mr.Title, titleW))
		row := fmt.Sprintf("%s!%-6d %s", cursor, mr.IID, titleCell)
		if showExtra {
			row += fmt.Sprintf(" %s %s",
				lipgloss.NewStyle().Width(authorW).MaxWidth(authorW).Render(truncate(mr.Author, authorW)),
				lipgloss.NewStyle().Width(ageW).MaxWidth(ageW).Render(truncate(relTime(mr.UpdatedAt), ageW)))
		}
		if selected {
			// Build the plain row and style it once, full-width, so the
			// selection bar doesn't end ragged wherever the text does (L5)
			// and doesn't nest mrStateTag's own color inside it (D1).
			line := row + " " + mrStateLabel(mr)
			b.WriteString(selectedStyle.Width(width).Render(line) + "\n")
			continue
		}
		b.WriteString(row + " " + mrStateTag(mr) + "\n")
	}
	if hidden := len(mrs) - end; hidden > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  ↓ %d more below", hidden)) + "\n")
	}
	return b.String()
}

func (m Model) listHints() []hint {
	return []hint{
		{"↑↓", "select"}, {"enter", "view"},
		{"f", "scope: " + m.mrScope.next().label()},
		{"F", "filter: " + m.mrFilter.next().label()},
		{"esc", "back"}, {"r", "refresh"}, {"q", "quit"},
	}
}
