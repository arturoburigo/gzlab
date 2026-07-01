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
	ID        int
	Status    PipelineStatus
	Ref       string
	WebURL    string
	CreatedAt time.Time
}
