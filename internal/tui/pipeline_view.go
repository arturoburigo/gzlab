package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

func (m Model) renderPipeline() string {
	width := m.contentWidth()
	var b strings.Builder
	title := fmt.Sprintf("Pipeline #%d for !%d", m.pipeline.ID, m.detail.IID)
	if isPipelineActive(m.pipeline) {
		title += "  " + footerStyle.Render(refreshTag("auto-refreshing", m.lastPipelineFetch))
	}
	b.WriteString(titleStyle.Render(title) + "\n")
	b.WriteString(rule(width) + "\n\n")
	b.WriteString(kv("status", renderPipeline(m.pipeline)) + "\n")
	b.WriteString(kv("ref", m.pipeline.Ref) + "\n")
	if m.pipeline.User != "" {
		b.WriteString(kv("started by", "@"+m.pipeline.User) + "\n")
	}
	if m.pipeline.Source != "" {
		b.WriteString(kv("source", m.pipeline.Source) + "\n")
	}
	if m.pipeline.Duration > 0 {
		b.WriteString(kv("duration", formatSeconds(float64(m.pipeline.Duration))) + "\n")
	}
	if m.pipeline.Coverage != "" {
		b.WriteString(kv("coverage", m.pipeline.Coverage) + "\n")
	}
	if !m.pipeline.CreatedAt.IsZero() {
		b.WriteString(kv("created", formatTime(m.pipeline.CreatedAt)) + "\n")
	}
	b.WriteString("\n")

	if len(m.jobs) == 0 {
		b.WriteString(footerStyle.Render("No jobs returned for this pipeline.") + "\n")
	} else {
		b.WriteString(tableHead.Render(fmt.Sprintf("  %-11s %-46s %8s", "STATUS", "JOB", "TIME")) + "\n")
		start, end := m.visibleJobRange()
		lastStage := ""
		for i := start; i < end; i++ {
			job := m.jobs[i]
			if job.Stage != lastStage {
				if lastStage != "" {
					b.WriteString("\n")
				}
				b.WriteString(renderStageHeader(job.Stage, m.stageJobs(job.Stage)) + "\n")
				lastStage = job.Stage
			}
			b.WriteString(renderJobRow(job, i == m.jobCursor) + "\n")
		}
	}

	return b.String()
}

// refreshTag renders a "[label]" or, once a fetch has actually happened,
// "[label · updated 15:04:05]" so a slow glab call doesn't look frozen —
// nothing else moved between poll ticks before this.
func refreshTag(label string, lastFetch time.Time) string {
	if lastFetch.IsZero() {
		return "[" + label + "]"
	}
	return fmt.Sprintf("[%s · updated %s]", label, lastFetch.Format("15:04:05"))
}

// renderJobRow builds one job list row. The status cell is padded to width
// *before* coloring (fmt pads by bytes, so coloring first would mis-measure
// it) so a failed job actually reads red instead of every row looking
// uniformly bold. The selected row stays a single uniform selectedStyle
// span — nesting the status color inside it would let its own reset cut the
// selection highlight short (the same class of bug as D1).
func renderJobRow(job *gitlab.Job, selected bool) string {
	cursor := emptyMarker
	if selected {
		cursor = cursorMarker
	}
	statusText := fmt.Sprintf("%-11s", string(job.Status))
	rest := fmt.Sprintf("%-46s %8s", truncate(job.Name, 46), formatSeconds(job.Duration))
	if job.AllowFailure {
		rest += " allow_failure"
	}
	if job.FailureReason != "" {
		rest += " " + job.FailureReason
	}
	if selected {
		return selectedStyle.Render(cursor + statusText + " " + rest)
	}
	return cursor + jobStatusStyle(job.Status).Render(statusText) + " " + valueStyle.Render(rest)
}

func renderStageHeader(stage string, jobs []*gitlab.Job) string {
	name := stage
	if name == "" {
		name = "unknown"
	}
	return hunkStyle.Render(fmt.Sprintf("Stage: %s  %s  %d jobs", name, renderJobStatus(stageStatus(jobs)), len(jobs)))
}

func (m Model) stageJobs(stage string) []*gitlab.Job {
	jobs := make([]*gitlab.Job, 0)
	for _, job := range m.jobs {
		if job.Stage == stage {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func stageStatus(jobs []*gitlab.Job) gitlab.JobStatus {
	status := gitlab.JobStatusSuccess
	for _, job := range jobs {
		switch job.Status {
		case gitlab.JobStatusFailed, gitlab.JobStatusCanceled:
			return job.Status
		case gitlab.JobStatusRunning, gitlab.JobStatusPending, gitlab.JobStatusCreated:
			status = job.Status
		case gitlab.JobStatusManual:
			if status == gitlab.JobStatusSuccess {
				status = job.Status
			}
		}
	}
	return status
}

func jobStatusStyle(status gitlab.JobStatus) lipgloss.Style {
	switch status {
	case gitlab.JobStatusSuccess:
		return pipelineSuccess
	case gitlab.JobStatusFailed, gitlab.JobStatusCanceled:
		return pipelineFailed
	case gitlab.JobStatusRunning, gitlab.JobStatusPending, gitlab.JobStatusCreated:
		return pipelineRunning
	default:
		return pipelineNeutral
	}
}

func renderJobStatus(status gitlab.JobStatus) string {
	return jobStatusStyle(status).Render(string(status))
}

func (m Model) pipelineHints() []hint {
	return []hint{
		{"j/k", "select"},
		{"enter", "open log"},
		{"o", "open job"},
		{"y", "copy link"},
		{"R", "retry job"},
		{"P", "retry pipeline"},
		{"t", "trigger manual"},
		{"x", "cancel pipeline"},
		{"g/G", "top/end"},
		{"r", "refresh"},
		{"esc", "back"},
		{"q", "quit"},
	}
}

// pipelineMetaRows counts the kv rows renderPipeline actually emits for p —
// jobListHeight used to assume a flat "9 fixed rows" regardless of which
// optional fields (started-by, source, duration, coverage, created) are
// present, so it either wasted rows or overshot the pane.
func pipelineMetaRows(p *gitlab.Pipeline) int {
	rows := 2 // status, ref
	if p.User != "" {
		rows++
	}
	if p.Source != "" {
		rows++
	}
	if p.Duration > 0 {
		rows++
	}
	if p.Coverage != "" {
		rows++
	}
	if !p.CreatedAt.IsZero() {
		rows++
	}
	return rows
}

func (m Model) jobListHeight() int {
	if m.pipeline == nil {
		return max(3, m.contentHeight()-9)
	}
	// title + rule + blank, metadata rows, blank, column header.
	fixed := 4 + pipelineMetaRows(m.pipeline)
	return max(3, m.contentHeight()-fixed)
}

// visibleJobRange is jobWindowFrom starting at the current scroll offset.
func (m Model) visibleJobRange() (int, int) {
	return m.jobWindowFrom(m.jobOffset)
}

// jobWindowFrom computes the [start, end) job window that fits in the
// available budget starting at offset, accounting for the stage-header and
// blank-separator rows rendered inline with the jobs — a flat
// row-per-job assumption undercounts real cost once a pipeline has more than
// one stage, so `j` could walk the highlight into rows that were cut off.
func (m Model) jobWindowFrom(offset int) (start, end int) {
	if len(m.jobs) == 0 {
		return 0, 0
	}
	height := m.jobListHeight()
	start = min(max(offset, 0), len(m.jobs)-1)
	rows := 0
	lastStage := ""
	end = start
	for end < len(m.jobs) {
		job := m.jobs[end]
		rowsNeeded := 1
		if job.Stage != lastStage {
			rowsNeeded++
			if lastStage != "" {
				rowsNeeded++
			}
		}
		if rows+rowsNeeded > height && end > start {
			break
		}
		rows += rowsNeeded
		lastStage = job.Stage
		end++
	}
	return start, end
}

func formatSeconds(seconds float64) string {
	if seconds <= 0 {
		return "-"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	minutes := int(seconds) / 60
	remaining := int(seconds) % 60
	return fmt.Sprintf("%dm%02ds", minutes, remaining)
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}
