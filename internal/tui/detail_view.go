package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderDetail() string {
	mr := m.detail

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("!%d %s", mr.IID, mr.Title)) + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 72)) + "\n\n")
	b.WriteString(kv("branches", fmt.Sprintf("%s -> %s", mr.SourceBranch, mr.TargetBranch)) + "\n")
	b.WriteString(kv("author", mr.Author) + "\n")
	b.WriteString(kv("status", string(mr.State)+draftSuffix(mr)) + "\n")
	b.WriteString(kv("pipeline", renderPipeline(mr.Pipeline)) + "\n")
	if mr.ApprovalsRequired > 0 {
		b.WriteString(kv("approvals", fmt.Sprintf("%d/%d", mr.ApprovalsGiven, mr.ApprovalsRequired)) + "\n")
	}
	if mr.HasConflicts {
		b.WriteString("\n" + errorStyle.Render("Has conflicts") + "\n")
	}

	return b.String()
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
	actions = append(actions, hint{"c", "comments"}, hint{"a", "approve"}, hint{"A", "revoke"}, hint{"w", draftLabel}, hint{"M", "merge"}, hint{"b", "checkout"})
	return append(actions, hint{"o", "open"}, hint{"y", "copy link"}, hint{"Y", "copy summary"}, hint{"esc", "back"}, hint{"q", "quit"})
}
