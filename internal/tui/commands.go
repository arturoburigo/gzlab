package tui

import (
	"context"
	"fmt"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"

	"github.com/arturoburigo/gitlab-tui/internal/dashboard"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

func loadDashboardCmd(deps Deps) tea.Cmd {
	return func() tea.Msg {
		ctx, err := dashboard.Resolve(context.Background(), deps.Config, deps.NewClient, deps.Remote, deps.Branch, deps.ProfileOverride)
		if err != nil {
			return errMsg{err}
		}
		return dashboardLoadedMsg{ctx}
	}
}

func loadMRListCmd(client gitlab.Client, projectID int) tea.Cmd {
	return func() tea.Msg {
		mrs, err := client.ListMergeRequests(context.Background(), projectID, gitlab.ListMergeRequestsOptions{})
		if err != nil {
			return errMsg{err}
		}
		return mrListLoadedMsg{mrs}
	}
}

func loadMRDetailCmd(client gitlab.Client, projectID, iid int) tea.Cmd {
	return func() tea.Msg {
		mr, err := client.GetMergeRequest(context.Background(), projectID, iid)
		if err != nil {
			return errMsg{err}
		}
		return mrDetailLoadedMsg{mr}
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if err := browser.OpenURL(url); err != nil {
			return errMsg{fmt.Errorf("opening browser: %w", err)}
		}
		return statusMsg{"Opened in browser."}
	}
}

func copyLinkCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(url); err != nil {
			return errMsg{fmt.Errorf("copying to clipboard: %w", err)}
		}
		return statusMsg{"Link copied to clipboard."}
	}
}
