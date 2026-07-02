package gitlab

import "time"

// User is the authenticated GitLab user.
type User struct {
	ID       int
	Username string
	Name     string
}

// Project is a GitLab project relevant to gitlab-tui.
type Project struct {
	ID                int
	PathWithNamespace string
	Name              string
	WebURL            string
	DefaultBranch     string
}

// MergeRequestState mirrors GitLab's merge request state values.
type MergeRequestState string

const (
	MergeRequestStateOpened MergeRequestState = "opened"
	MergeRequestStateClosed MergeRequestState = "closed"
	MergeRequestStateMerged MergeRequestState = "merged"
)

// MergeRequest is a trimmed-down view of a GitLab merge request.
type MergeRequest struct {
	IID          int
	ProjectID    int
	Title        string
	State        MergeRequestState
	Draft        bool
	SourceBranch string
	TargetBranch string
	Author       string
	WebURL       string
	HasConflicts bool

	ApprovalsRequired int
	ApprovalsGiven    int

	// Pipeline is the MR's head pipeline. Only populated by GetMergeRequest,
	// not by ListMergeRequests (the GitLab list endpoint omits it).
	Pipeline *Pipeline

	CreatedAt time.Time
	UpdatedAt time.Time
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
	ID         int
	IID        int
	Status     PipelineStatus
	Source     string
	Ref        string
	SHA        string
	WebURL     string
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
