package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderDetail() string {
	mr := m.detail

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("!%d — %s", mr.IID, mr.Title)) + "\n\n")
	b.WriteString(labelStyle.Render("Branches:  ") + fmt.Sprintf("%s → %s\n", mr.SourceBranch, mr.TargetBranch))
	b.WriteString(labelStyle.Render("Author:    ") + mr.Author + "\n")
	b.WriteString(labelStyle.Render("Status:    ") + string(mr.State) + draftSuffix(mr) + "\n")
	b.WriteString(labelStyle.Render("Pipeline:  ") + renderPipeline(mr.Pipeline) + "\n")
	if mr.ApprovalsRequired > 0 {
		b.WriteString(labelStyle.Render("Approvals: ") + fmt.Sprintf("%d/%d\n", mr.ApprovalsGiven, mr.ApprovalsRequired))
	}
	if mr.HasConflicts {
		b.WriteString(errorStyle.Render("Has conflicts") + "\n")
	}

	if m.status != "" {
		b.WriteString("\n" + footerStyle.Render(m.status))
	}
	b.WriteString("\n" + joinFooter("o open browser", "y copy link", "esc back", "q quit"))
	return b.String()
}
