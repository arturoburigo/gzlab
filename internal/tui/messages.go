package tui

import (
	"github.com/arturoburigo/gzlab/internal/dashboard"
	"github.com/arturoburigo/gzlab/internal/gitlab"
	"github.com/arturoburigo/gzlab/internal/history"
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

// pipelinePollTickMsg drives auto-refresh while a pipeline is still running
// (Épico 14). gen must match Model.pollGen when it fires, or it's a stale
// tick from a screen/session that's since moved on and is dropped.
type pipelinePollTickMsg struct{ gen int }

// jobLogFollowTickMsg drives job-log auto-refresh in follow mode (Épico 15),
// with the same gen-matching semantics as pipelinePollTickMsg.
type jobLogFollowTickMsg struct{ gen int }

type mrActionDoneMsg struct{ status string }

type mrCheckedOutMsg struct {
	branch   string
	projects []history.Project
	branches []history.Branch
}

// dashboardStatsLoadedMsg carries the dashboard's best-effort personal-stats
// enrichment (recent commits, MR state breakdown, MRs assigned to the
// current user, recent cross-project activity). Errors are swallowed by the
// command that produces it — like history recording, this is a convenience,
// never a blocker — so this message always "succeeds" with whatever it
// could fetch (possibly nothing).
type dashboardStatsLoadedMsg struct {
	commits     []gitlab.Commit
	mrs         []*gitlab.MergeRequest
	assignedMRs []*gitlab.MergeRequest
	activity    []gitlab.ContributionEvent
}

// spinnerTickMsg advances the dashboard's initial-load spinner. gen must match
// Model.spinnerGen when it fires, or it's a stale tick from a load that's
// already finished (or been superseded by a refresh) and is dropped.
type spinnerTickMsg struct{ gen int }

type commitsLoadedMsg struct{ commits []gitlab.Commit }

type discussionsLoadedMsg struct{ discussions []gitlab.Discussion }

type searchLoadedMsg struct {
	query   string
	results []gitlab.GlobalSearchResult
}

type workspacesLoadedMsg struct{ workspaces []workspaceView }

type workspaceSavedMsg struct {
	status     string
	workspaces []workspaceView
}

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
