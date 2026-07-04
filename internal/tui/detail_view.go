package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

func (m Model) renderDetail() string {
	mr := m.detail
	width := m.contentWidth()

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("!%d %s", mr.IID, mr.Title)) + "\n")
	b.WriteString(rule(width) + "\n\n")
	b.WriteString(kv("branches", fmt.Sprintf("%s → %s", mr.SourceBranch, mr.TargetBranch)) + "\n")
	b.WriteString(kv("author", mr.Author) + "\n")
	b.WriteString(kv("status", string(mr.State)+draftSuffix(mr)) + "\n")
	b.WriteString(kv("pipeline", renderPipeline(mr.Pipeline)) + "\n")
	if mr.ApprovalsRequired > 0 {
		b.WriteString(kv("approvals", fmt.Sprintf("%d/%d", mr.ApprovalsGiven, mr.ApprovalsRequired)) + "\n")
	}
	if len(mr.Labels) > 0 {
		b.WriteString(kv("labels", labelStyle.Render(strings.Join(mr.Labels, ", "))) + "\n")
	}
	if !mr.CreatedAt.IsZero() {
		b.WriteString(kv("created", formatTime(mr.CreatedAt)) + "\n")
	}
	if !mr.UpdatedAt.IsZero() {
		b.WriteString(kv("updated", formatTime(mr.UpdatedAt)) + "\n")
	}
	if mr.HasConflicts {
		b.WriteString("\n" + errorStyle.Render("Has conflicts") + "\n")
	}

	if reasons := whyBlocked(mr); len(reasons) > 0 {
		b.WriteString("\n" + errorStyle.Render("Why blocked?") + "\n")
		for _, reason := range reasons {
			b.WriteString("  - " + reason + "\n")
		}
	}

	if desc := strings.TrimSpace(mr.Description); desc != "" {
		used := strings.Count(b.String(), "\n")
		remaining := max(3, m.contentHeight()-used-2)
		descLines := strings.Split(ansi.Wordwrap(desc, width, ""), "\n")
		if len(descLines) > remaining {
			descLines = descLines[:remaining]
		}
		b.WriteString("\n" + tableHead.Render("Description") + "\n")
		for _, line := range descLines {
			b.WriteString(footerStyle.Render(line) + "\n")
		}
	}

	return b.String()
}

// whyBlocked consolidates the signals already shown individually above (
// pipeline, conflicts, approvals, draft state) into the plan's "Why
// blocked?" panel — empty when the MR is mergeable or already merged/closed.
func whyBlocked(mr *gitlab.MergeRequest) []string {
	if mr.State != gitlab.MergeRequestStateOpened {
		return nil
	}
	var reasons []string
	if mr.Draft {
		reasons = append(reasons, "still a draft")
	}
	if mr.Pipeline != nil {
		switch mr.Pipeline.Status {
		case gitlab.PipelineStatusFailed:
			reasons = append(reasons, "pipeline failed")
		case gitlab.PipelineStatusCanceled:
			reasons = append(reasons, "pipeline canceled")
		}
	}
	if mr.HasConflicts {
		reasons = append(reasons, "has merge conflicts")
	}
	if mr.ApprovalsRequired > 0 && !mr.Approved() {
		reasons = append(reasons, fmt.Sprintf("missing approval (%d/%d)", mr.ApprovalsGiven, mr.ApprovalsRequired))
	}
	return reasons
}

func (m Model) detailHints() []hint {
	if m.detail == nil {
		return []hint{{"esc", "back"}, {"q", "quit"}}
	}
	actions := []hint{{"d", "diff"}}
	if m.detail.Pipeline != nil {
		actions = append(actions, hint{"p", "pipeline"}, hint{"x", "cancel pipeline"})
	}
	draftLabel := "mark draft"
	if m.detail.Draft {
		draftLabel = "mark ready"
	}
	actions = append(actions, hint{"c", "comments"}, hint{"C", "commits"}, hint{"a", "approve"}, hint{"A", "revoke"}, hint{"w", draftLabel}, hint{"W", "workspace"}, hint{"M", "merge"}, hint{"b", "checkout"})
	return append(actions, hint{"o", "open"}, hint{"y", "copy link"}, hint{"Y", "copy summary"}, hint{"T", "summary: " + m.summaryFormat.next().label()}, hint{"esc", "back"}, hint{"q", "quit"})
}
