package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

func (m Model) renderPipeline() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Pipeline #%d for !%d", m.pipeline.ID, m.detail.IID)) + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 72)) + "\n\n")
	b.WriteString(kv("status", renderPipeline(m.pipeline)) + "\n")
	b.WriteString(kv("ref", m.pipeline.Ref) + "\n")
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
			cursor := "  "
			style := valueStyle
			if i == m.jobCursor {
				cursor = "> "
				style = selectedStyle
			}
			line := fmt.Sprintf("%s%-11s %-46s %8s", cursor, job.Status, truncate(job.Name, 46), formatSeconds(job.Duration))
			if job.AllowFailure {
				line += " allow_failure"
			}
			if job.FailureReason != "" {
				line += " " + job.FailureReason
			}
			b.WriteString(style.Render(line) + "\n")
		}
	}

	return b.String()
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

func renderJobStatus(status gitlab.JobStatus) string {
	switch status {
	case gitlab.JobStatusSuccess:
		return pipelineSuccess.Render(string(status))
	case gitlab.JobStatusFailed, gitlab.JobStatusCanceled:
		return pipelineFailed.Render(string(status))
	case gitlab.JobStatusRunning, gitlab.JobStatusPending, gitlab.JobStatusCreated:
		return pipelineRunning.Render(string(status))
	default:
		return pipelineNeutral.Render(string(status))
	}
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

func (m Model) visibleJobRange() (int, int) {
	height := m.jobListHeight()
	start := min(m.jobOffset, max(0, len(m.jobs)-1))
	end := min(start+height, len(m.jobs))
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
