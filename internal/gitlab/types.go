package gitlab

import "time"

// User is the authenticated GitLab user.
type User struct {
	ID       int
	Username string
	Name     string
}

// Project is a GitLab project relevant to gzlab.
type Project struct {
	ID                int
	PathWithNamespace string
	Name              string
	WebURL            string
	DefaultBranch     string
}

// Branch is a repository branch returned by GitLab.
type Branch struct {
	Name      string
	WebURL    string
	Protected bool
	Default   bool
}

// MergeRequestState mirrors GitLab's merge request state values.
type MergeRequestState string

const (
	MergeRequestStateOpened MergeRequestState = "opened"
	MergeRequestStateClosed MergeRequestState = "closed"
	MergeRequestStateMerged MergeRequestState = "merged"
	// MergeRequestStateAll requests every state instead of GitLab's default
	// of "opened" — for callers that want a full state breakdown (e.g. the
	// dashboard's merge-request stats card).
	MergeRequestStateAll MergeRequestState = "all"
)

// MergeRequest is a trimmed-down view of a GitLab merge request.
type MergeRequest struct {
	IID          int
	ProjectID    int
	ProjectPath  string // "namespace/project"; only populated by cross-project listings (ListMyMergeRequests)
	Title        string
	State        MergeRequestState
	Draft        bool
	SourceBranch string
	TargetBranch string
	Author       string
	WebURL       string
	HasConflicts bool
	Labels       []string
	Description  string

	ApprovalsRequired int
	ApprovalsGiven    int

	// Pipeline is the MR's head pipeline. Only populated by GetMergeRequest,
	// not by ListMergeRequests (the GitLab list endpoint omits it).
	Pipeline *Pipeline

	CreatedAt time.Time
	UpdatedAt time.Time
}

// MergeRequestsScope selects which merge requests ListMyMergeRequests returns,
// mirroring GitLab's global /merge_requests "scope" values.
type MergeRequestsScope string

const (
	// MergeRequestsScopeCreatedByMe returns MRs authored by the token's user.
	MergeRequestsScopeCreatedByMe MergeRequestsScope = "created_by_me"
	// MergeRequestsScopeAssignedToMe returns MRs assigned to the token's user.
	MergeRequestsScopeAssignedToMe MergeRequestsScope = "assigned_to_me"
)

// ListMyMergeRequestsOptions filters MRs returned by ListMyMergeRequests,
// GitLab's global (cross-project) merge requests endpoint.
type ListMyMergeRequestsOptions struct {
	// State defaults to MergeRequestStateOpened when empty.
	State MergeRequestState
	// Scope selects created-by-me/assigned-to-me. Leave empty together with
	// ReviewerUsername set to list MRs where the user is a reviewer.
	Scope MergeRequestsScope
	// ReviewerUsername, when set, restricts results to MRs where that user
	// is a requested reviewer ("MRs para revisar").
	ReviewerUsername string
	// Limit, when > 0, fetches a single page of at most this many results
	// instead of paginating through everything — for callers that want a
	// recent snapshot (e.g. a dashboard stat) rather than the full list.
	Limit int
}

// GlobalSearchOptions controls the lightweight global search used by the TUI.
type GlobalSearchOptions struct {
	Query   string
	Limit   int
	Project *Project
}

// GlobalSearchResult is a project, MR, or branch match.
type GlobalSearchResult struct {
	Type    string
	Project *Project
	MR      *MergeRequest
	Branch  *Branch
}

// Approved reports whether the merge request has enough approvals.
func (m MergeRequest) Approved() bool {
	return m.ApprovalsRequired > 0 && m.ApprovalsGiven >= m.ApprovalsRequired
}

// PipelineStatus mirrors GitLab's pipeline status values.
type PipelineStatus string

const (
	PipelineStatusCreated            PipelineStatus = "created"
	PipelineStatusWaitingForResource PipelineStatus = "waiting_for_resource"
	PipelineStatusPreparing          PipelineStatus = "preparing"
	PipelineStatusPending            PipelineStatus = "pending"
	PipelineStatusRunning            PipelineStatus = "running"
	PipelineStatusSuccess            PipelineStatus = "success"
	PipelineStatusFailed             PipelineStatus = "failed"
	PipelineStatusCanceled           PipelineStatus = "canceled"
	PipelineStatusSkipped            PipelineStatus = "skipped"
	PipelineStatusManual             PipelineStatus = "manual"
)

// Pipeline is a trimmed-down view of a GitLab pipeline.
type Pipeline struct {
	ID     int
	IID    int
	Status PipelineStatus
	Source string
	Ref    string
	SHA    string
	WebURL string
	// User is the username of whoever started the pipeline. Only populated
	// by GetPipeline (the full pipeline screen); GitLab's embedded MR
	// pipeline summary doesn't include it.
	User       string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	Duration   int
	Coverage   string
}

// MergeRequestDiff is a single changed file in an MR diff.
type MergeRequestDiff struct {
	OldPath       string
	NewPath       string
	Diff          string
	NewFile       bool
	RenamedFile   bool
	DeletedFile   bool
	GeneratedFile bool
	Collapsed     bool
	TooLarge      bool
}

// Commit is one commit in a merge request.
type Commit struct {
	ShortID    string
	Title      string
	AuthorName string
	CreatedAt  time.Time
}

// ContributionEvent is a single action the current user took anywhere in
// GitLab — pushing, commenting, opening/closing/merging an MR or issue — the
// data behind the dashboard's activity feed and contribution-activity strip.
type ContributionEvent struct {
	Action    string // GitLab's own action_name, e.g. "opened", "commented on", "approved"
	Target    string // the MR/issue title, or the branch ref for a push
	CreatedAt time.Time
}

// ListContributionEventsOptions filters ListMyContributionEvents.
type ListContributionEventsOptions struct {
	// After restricts results to events on/after this time; zero means no
	// lower bound.
	After time.Time
	// Limit caps how many events are fetched (most-recent-first); 0 means a
	// single default page.
	Limit int
}

// Note is a single comment within a merge request discussion thread. System
// notes (System == true) are GitLab's own activity log entries — "changed the
// description", "added 3 commits", "marked as ready" — not human comments.
type Note struct {
	ID         int
	Author     string
	Body       string
	System     bool
	Resolvable bool
	Resolved   bool
	CreatedAt  time.Time
	// PositionPath/PositionLine locate a code-anchored diff comment ("new_path"
	// / "new_line" in GitLab's position object); PositionLine is 0 for notes
	// with no position (general MR comments).
	PositionPath string
	PositionLine int
}

// Discussion is a thread of notes on a merge request. A one-note discussion is
// a plain comment; multiple notes form a reply thread (or a diff conversation).
type Discussion struct {
	ID    string
	Notes []Note
}

// Resolvable reports whether the thread can be resolved — true for diff/review
// threads, false for plain comments and system notes.
func (d Discussion) Resolvable() bool {
	for _, n := range d.Notes {
		if n.Resolvable {
			return true
		}
	}
	return false
}

// Resolved reports whether every resolvable note in the thread is resolved.
// It is only meaningful when Resolvable is true.
func (d Discussion) Resolved() bool {
	seen := false
	for _, n := range d.Notes {
		if n.Resolvable {
			seen = true
			if !n.Resolved {
				return false
			}
		}
	}
	return seen
}

// JobStatus mirrors GitLab job status values.
type JobStatus string

const (
	JobStatusCreated  JobStatus = "created"
	JobStatusPending  JobStatus = "pending"
	JobStatusRunning  JobStatus = "running"
	JobStatusSuccess  JobStatus = "success"
	JobStatusFailed   JobStatus = "failed"
	JobStatusCanceled JobStatus = "canceled"
	JobStatusSkipped  JobStatus = "skipped"
	JobStatusManual   JobStatus = "manual"
)

// Job is a trimmed-down view of a GitLab CI job.
type Job struct {
	ID             int
	Name           string
	Stage          string
	Status         JobStatus
	Ref            string
	WebURL         string
	Duration       float64
	QueuedDuration float64
	AllowFailure   bool
	FailureReason  string
	CreatedAt      time.Time
	StartedAt      time.Time
	FinishedAt     time.Time
}
