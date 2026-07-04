package tui

import (
	"fmt"
	"strings"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// summaryFormat selects the copyable-summary flavor (Épico 20): plain text,
// Markdown, or a Slack/Google Chat-style single line.
type summaryFormat int

const (
	summaryFormatPlain summaryFormat = iota
	summaryFormatMarkdown
	summaryFormatSlack
)

func (f summaryFormat) label() string {
	switch f {
	case summaryFormatMarkdown:
		return "markdown"
	case summaryFormatSlack:
		return "slack"
	default:
		return "plain"
	}
}

func (f summaryFormat) next() summaryFormat {
	return (f + 1) % 3
}

// summaryExtras carries data that's only available once the diff or
// discussions screens have been visited for the MR in context — diff stats
// and discussion counts are included when present, omitted otherwise.
type summaryExtras struct {
	hasDiff              bool
	additions, deletions int
	hasDiscussions       bool
	discussionTotal      int
	discussionResolved   int
}

// summaryExtras computes what's available from the model's current state for
// the MR in context. Diff/discussion data is only trustworthy when it was
// loaded for this same MR (detail screen or beyond), not a stale prior one.
func (m Model) summaryExtras(mr *gitlab.MergeRequest) summaryExtras {
	var e summaryExtras
	if mr == nil || m.detail == nil || mr.IID != m.detail.IID {
		return e
	}
	if len(m.diff.files) > 0 {
		e.hasDiff = true
		for _, f := range m.diff.files {
			e.additions += f.additions
			e.deletions += f.deletions
		}
	}
	if len(m.discuss.threads) > 0 {
		e.hasDiscussions = true
		e.discussionTotal = len(m.discuss.threads)
		for _, t := range m.discuss.threads {
			if t.resolvable && t.resolved {
				e.discussionResolved++
			}
		}
	}
	return e
}

// mrSummary renders a paste-ready summary of a merge request in format, for
// dropping into Slack, a standup note, a Markdown PR description, or a
// review request.
func mrSummary(mr *gitlab.MergeRequest, projectPath string, format summaryFormat, extra summaryExtras) string {
	switch format {
	case summaryFormatMarkdown:
		return mrSummaryMarkdown(mr, projectPath, extra)
	case summaryFormatSlack:
		return mrSummarySlack(mr, projectPath, extra)
	default:
		return mrSummaryPlain(mr, projectPath, extra)
	}
}

func mrSummaryPlain(mr *gitlab.MergeRequest, projectPath string, extra summaryExtras) string {
	var b strings.Builder
	fmt.Fprintf(&b, "!%d %s\n", mr.IID, mr.Title)
	if projectPath != "" {
		fmt.Fprintf(&b, "Project:   %s\n", projectPath)
	}
	fmt.Fprintf(&b, "Branch:    %s → %s\n", mr.SourceBranch, mr.TargetBranch)
	if mr.Author != "" {
		fmt.Fprintf(&b, "Author:    %s\n", mr.Author)
	}
	fmt.Fprintf(&b, "Status:    %s%s\n", mr.State, draftSuffix(mr))
	if mr.Pipeline != nil {
		fmt.Fprintf(&b, "Pipeline:  %s\n", mr.Pipeline.Status)
	}
	if mr.ApprovalsRequired > 0 {
		fmt.Fprintf(&b, "Approvals: %d/%d\n", mr.ApprovalsGiven, mr.ApprovalsRequired)
	}
	if mr.HasConflicts {
		b.WriteString("Conflicts: yes\n")
	}
	if extra.hasDiff {
		fmt.Fprintf(&b, "Diff:      +%d -%d\n", extra.additions, extra.deletions)
	}
	if extra.hasDiscussions {
		fmt.Fprintf(&b, "Discussions: %d (%d resolved)\n", extra.discussionTotal, extra.discussionResolved)
	}
	if mr.WebURL != "" {
		b.WriteString(mr.WebURL)
	}
	return strings.TrimRight(b.String(), "\n")
}

func mrSummaryMarkdown(mr *gitlab.MergeRequest, projectPath string, extra summaryExtras) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## !%d %s\n\n", mr.IID, mr.Title)
	if projectPath != "" {
		fmt.Fprintf(&b, "- **Project:** %s\n", projectPath)
	}
	fmt.Fprintf(&b, "- **Branch:** %s → %s\n", mr.SourceBranch, mr.TargetBranch)
	if mr.Author != "" {
		fmt.Fprintf(&b, "- **Author:** %s\n", mr.Author)
	}
	fmt.Fprintf(&b, "- **Status:** %s%s\n", mr.State, draftSuffix(mr))
	if mr.Pipeline != nil {
		fmt.Fprintf(&b, "- **Pipeline:** %s\n", mr.Pipeline.Status)
	}
	if mr.ApprovalsRequired > 0 {
		fmt.Fprintf(&b, "- **Approvals:** %d/%d\n", mr.ApprovalsGiven, mr.ApprovalsRequired)
	}
	if mr.HasConflicts {
		b.WriteString("- **Conflicts:** yes\n")
	}
	if extra.hasDiff {
		fmt.Fprintf(&b, "- **Diff:** +%d -%d\n", extra.additions, extra.deletions)
	}
	if extra.hasDiscussions {
		fmt.Fprintf(&b, "- **Discussions:** %d (%d resolved)\n", extra.discussionTotal, extra.discussionResolved)
	}
	if mr.WebURL != "" {
		fmt.Fprintf(&b, "\n%s", mr.WebURL)
	}
	return strings.TrimRight(b.String(), "\n")
}

func mrSummarySlack(mr *gitlab.MergeRequest, projectPath string, extra summaryExtras) string {
	var b strings.Builder
	b.WriteString(summaryEmoji(mr) + " ")
	if projectPath != "" {
		fmt.Fprintf(&b, "%s ", projectPath)
	}
	fmt.Fprintf(&b, "!%d %s — %s%s", mr.IID, mr.Title, summarySlackStatus(mr), draftSuffix(mr))
	if extra.hasDiff {
		fmt.Fprintf(&b, " (+%d -%d)", extra.additions, extra.deletions)
	}
	if mr.WebURL != "" {
		fmt.Fprintf(&b, "\n%s", mr.WebURL)
	}
	return b.String()
}

// summaryEmoji picks the plan's Google Chat/Slack-style status glyph:
// ✅ merged/passing and approved, ❌ failed/conflicted, 🟡 anything pending.
func summaryEmoji(mr *gitlab.MergeRequest) string {
	switch {
	case mr.State == gitlab.MergeRequestStateMerged:
		return "✅"
	case mr.HasConflicts:
		return "❌"
	case mr.Pipeline != nil && (mr.Pipeline.Status == gitlab.PipelineStatusFailed || mr.Pipeline.Status == gitlab.PipelineStatusCanceled):
		return "❌"
	case mr.Pipeline != nil && (mr.Pipeline.Status == gitlab.PipelineStatusRunning || mr.Pipeline.Status == gitlab.PipelineStatusPending || mr.Pipeline.Status == gitlab.PipelineStatusCreated):
		return "🟡"
	case mr.ApprovalsRequired > 0 && !mr.Approved():
		return "🟡"
	default:
		return "✅"
	}
}

func summarySlackStatus(mr *gitlab.MergeRequest) string {
	if mr.Pipeline != nil {
		return "pipeline " + string(mr.Pipeline.Status)
	}
	return string(mr.State)
}

// summaryMR returns the merge request currently in context, if any: the open
// detail, else the dashboard's current-branch MR.
func (m Model) summaryMR() *gitlab.MergeRequest {
	if m.detail != nil {
		return m.detail
	}
	if m.dash != nil {
		return m.dash.MergeRequest
	}
	return nil
}

func (m Model) projectPath() string {
	if m.dash != nil && m.dash.Project != nil {
		return m.dash.Project.PathWithNamespace
	}
	return ""
}
