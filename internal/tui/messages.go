package tui

import (
	"github.com/arturoburigo/gitlab-tui/internal/dashboard"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
	"github.com/arturoburigo/gitlab-tui/internal/history"
)

type dashboardLoadedMsg struct{ ctx *dashboard.Context }

type historyLoadedMsg struct {
	projects []history.Project
	branches []history.Branch
}

type mrListLoadedMsg struct{ mrs []*gitlab.MergeRequest }

type mrDetailLoadedMsg struct{ mr *gitlab.MergeRequest }

type mrDiffLoadedMsg struct{ diffs []*gitlab.MergeRequestDiff }

type pipelineLoadedMsg struct {
	pipeline *gitlab.Pipeline
	jobs     []*gitlab.Job
}

type jobLogLoadedMsg struct {
	job *gitlab.Job
	log string
}

type pipelineActionDoneMsg struct{ status string }

type mrActionDoneMsg struct{ status string }

type mrCheckedOutMsg struct{ branch string }

type discussionsLoadedMsg struct{ discussions []gitlab.Discussion }

type commentPostedMsg struct{ iid int }

type discussionActionDoneMsg struct {
	iid    int
	status string
}

type checkoutPreparedMsg struct {
	iid   int
	dirty bool
}

type statusMsg struct{ text string }

type errMsg struct{ err error }
