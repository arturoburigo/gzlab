package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

var (
	accentColor = lipgloss.Color("12")
	mutedColor  = lipgloss.Color("7")
	borderColor = lipgloss.Color("8")
	goodColor   = lipgloss.Color("10")
	warnColor   = lipgloss.Color("11")
	badColor    = lipgloss.Color("9")

	logoStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accentColor).Padding(0, 1)
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	labelStyle    = lipgloss.NewStyle().Foreground(mutedColor)
	valueStyle    = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	footerKey     = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	footerStyle   = lipgloss.NewStyle().Foreground(mutedColor)
	errorStyle    = lipgloss.NewStyle().Foreground(badColor).Bold(true)
	selectedStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
	tableHead     = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	paneStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(accentColor).Padding(0, 1)
	ruleStyle     = lipgloss.NewStyle().Foreground(borderColor)
	hunkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	additionStyle = lipgloss.NewStyle().Foreground(goodColor)
	deletionStyle = lipgloss.NewStyle().Foreground(badColor)
	searchStyle   = lipgloss.NewStyle().Reverse(true).Bold(true).Foreground(lipgloss.Color("0")).Background(warnColor)
	confirmStyle  = lipgloss.NewStyle().Foreground(warnColor).Bold(true)

	discussHeaderStyle     = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	discussResolvedStyle   = lipgloss.NewStyle().Foreground(goodColor).Bold(true)
	discussUnresolvedStyle = lipgloss.NewStyle().Foreground(warnColor).Bold(true)
	cursorStyle            = lipgloss.NewStyle().Bold(true).Foreground(accentColor)

	pipelineSuccess = lipgloss.NewStyle().Foreground(goodColor).Bold(true)
	pipelineFailed  = lipgloss.NewStyle().Foreground(badColor).Bold(true)
	pipelineRunning = lipgloss.NewStyle().Foreground(warnColor).Bold(true)
	pipelineNeutral = lipgloss.NewStyle().Foreground(mutedColor)
)

func pipelineStyle(status gitlab.PipelineStatus) lipgloss.Style {
	switch status {
	case gitlab.PipelineStatusSuccess:
		return pipelineSuccess
	case gitlab.PipelineStatusFailed, gitlab.PipelineStatusCanceled:
		return pipelineFailed
	case gitlab.PipelineStatusRunning, gitlab.PipelineStatusPending:
		return pipelineRunning
	default:
		return pipelineNeutral
	}
}
