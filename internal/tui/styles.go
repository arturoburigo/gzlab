package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	footerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

	pipelineSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	pipelineFailed  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	pipelineRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	pipelineNeutral = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
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
