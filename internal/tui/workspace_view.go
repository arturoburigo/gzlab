package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gzlab/internal/gitlab"
	"github.com/arturoburigo/gzlab/internal/workspace"
)

type workspaceView struct {
	Name      string
	Profile   string
	UpdatedAt time.Time
	MRs       []workspaceMRView
}

type workspaceMRView struct {
	Ref workspace.MergeRequestRef
}

func (m Model) renderWorkspace() string {
	width := m.contentWidth()
	var b strings.Builder
	b.WriteString(titleStyle.Render("Workspace") + "\n")
	b.WriteString(rule(width) + "\n")
	if len(m.workspaces) == 0 {
		b.WriteString(footerStyle.Render("No workspaces yet. Use W from an MR detail to add it.") + "\n")
		return b.String()
	}

	maxLines := max(3, m.contentHeight()-2)
	start := m.workspaceWindowStart(maxLines)
	lines := 0
	if start > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  ↑ %d more above", start)) + "\n")
		lines++
	}

	shown := 0
	for i := start; i < len(m.workspaces); i++ {
		ws := m.workspaces[i]
		rowsNeeded := 1 + len(ws.MRs)
		if lines+rowsNeeded > maxLines && shown > 0 {
			break
		}
		cursor := emptyMarker
		nameStyle := tableHead
		if i == m.workspaceCursor {
			cursor = cursorMarker
			nameStyle = selectedStyle
		}
		fmt.Fprintf(&b, "%s%s  %s\n", cursor, nameStyle.Render(ws.Name), footerStyle.Render(fmt.Sprintf("%d MR(s)", len(ws.MRs))))
		for _, item := range ws.MRs {
			b.WriteString(renderWorkspaceMRRow(item.Ref) + "\n")
		}
		lines += rowsNeeded
		shown++
	}
	if hidden := len(m.workspaces) - start - shown; hidden > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  ↓ %d more below", hidden)) + "\n")
	}
	return b.String()
}

// workspaceWindowStart is the earliest workspace index to render so the
// cursor's workspace stays visible within maxLines rows — clampBlock's hard
// clip used to just cut the selection off the bottom of the pane on a list
// with several workspaces.
func (m Model) workspaceWindowStart(maxLines int) int {
	total := 0
	for _, ws := range m.workspaces {
		total += 1 + len(ws.MRs)
	}
	if total <= maxLines {
		return 0
	}
	rows := 1 + len(m.workspaces[m.workspaceCursor].MRs)
	start := m.workspaceCursor
	for start > 0 {
		prevRows := 1 + len(m.workspaces[start-1].MRs)
		if rows+prevRows > maxLines {
			break
		}
		rows += prevRows
		start--
	}
	return start
}

const (
	workspacePathW  = 28
	workspaceTitleW = 30
)

func renderWorkspaceMRRow(ref workspace.MergeRequestRef) string {
	path := lipgloss.NewStyle().Width(workspacePathW).MaxWidth(workspacePathW).
		Render(truncatePathLeft(shortPath(ref.ProjectPath), workspacePathW))
	title := lipgloss.NewStyle().Width(workspaceTitleW).MaxWidth(workspaceTitleW).
		Render(truncate(ref.Title, workspaceTitleW))

	status := footerStyle.Render(strings.TrimSpace(ref.Status))
	var extra string
	if ref.Pipeline != "" {
		extra += " " + pipelineStyle(gitlab.PipelineStatus(ref.Pipeline)).Render(ref.Pipeline)
	}
	if ref.Approvals != "" {
		extra += " " + approvalStyle(ref.Approvals).Render(ref.Approvals)
	}
	return fmt.Sprintf("    !%-6d %s %s %s%s", ref.IID, path, title, status, extra)
}

// approvalStyle colors a "given/required" approvals string good once fully
// approved, warn while it's still short — a failed pipeline used to be
// exactly as gray as everything else, the one signal this overview exists to
// surface.
func approvalStyle(s string) lipgloss.Style {
	before, after, ok := strings.Cut(s, "/")
	if !ok {
		return footerStyle
	}
	given, err1 := strconv.Atoi(before)
	required, err2 := strconv.Atoi(after)
	if err1 != nil || err2 != nil {
		return footerStyle
	}
	if given >= required {
		return pipelineSuccess
	}
	return pipelineRunning
}

func (m Model) workspaceHints() []hint {
	return []hint{{"a", "add current MR"}, {"x", "remove current MR"}, {"Y", "copy summary"}, {"j/k", "select"}, {"esc", "back"}, {"r", "refresh"}, {"q", "quit"}}
}

func workspaceSummary(ws workspaceView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Estado do %s\n\n", ws.Name)
	for _, item := range ws.MRs {
		ref := item.Ref
		status := ref.Status
		if ref.Pipeline != "" {
			status = "pipeline " + ref.Pipeline
		}
		if ref.Approvals != "" {
			status += ", approvals " + ref.Approvals
		}
		fmt.Fprintf(&b, "%s !%d - %s\n", shortPath(ref.ProjectPath), ref.IID, strings.TrimSpace(status))
	}
	return strings.TrimRight(b.String(), "\n")
}
