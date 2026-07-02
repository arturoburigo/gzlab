package tui

import (
	"github.com/arturoburigo/gitlab-tui/internal/dashboard"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

type dashboardLoadedMsg struct{ ctx *dashboard.Context }

type mrListLoadedMsg struct{ mrs []*gitlab.MergeRequest }

type mrDetailLoadedMsg struct{ mr *gitlab.MergeRequest }

type statusMsg struct{ text string }

type errMsg struct{ err error }
