package gitlab

import (
	"strings"
	"time"

	gl "gitlab.com/gitlab-org/api/client-go"
)

func toProject(p *gl.Project) *Project {
	return &Project{
		ID:                int(p.ID),
		PathWithNamespace: p.PathWithNamespace,
		Name:              p.Name,
		WebURL:            p.WebURL,
		DefaultBranch:     p.DefaultBranch,
	}
}

func toBranch(b *gl.Branch) *Branch {
	return &Branch{
		Name:      b.Name,
		WebURL:    b.WebURL,
		Protected: b.Protected,
		Default:   b.Default,
	}
}

func timeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func authorUsername(a *gl.BasicUser) string {
	if a == nil {
		return ""
	}
	return a.Username
}

// projectPathFromReferences extracts "namespace/project" from a merge
// request's full reference (e.g. "namespace/project!123"), the only place
// GitLab's cross-project MR listing tells us which project an MR belongs to.
func projectPathFromReferences(refs *gl.IssueReferences) string {
	if refs == nil {
		return ""
	}
	if i := strings.LastIndex(refs.Full, "!"); i > 0 {
		return refs.Full[:i]
	}
	return ""
}

func toMergeRequestFromBasic(mr *gl.BasicMergeRequest) *MergeRequest {
	return &MergeRequest{
		IID:          int(mr.IID),
		ProjectID:    int(mr.ProjectID),
		ProjectPath:  projectPathFromReferences(mr.References),
		Title:        mr.Title,
		State:        MergeRequestState(mr.State),
		Draft:        mr.Draft,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Author:       authorUsername(mr.Author),
		WebURL:       mr.WebURL,
		HasConflicts: mr.HasConflicts,
		Labels:       []string(mr.Labels),
		Description:  mr.Description,
		CreatedAt:    timeValue(mr.CreatedAt),
		UpdatedAt:    timeValue(mr.UpdatedAt),
	}
}

func toMergeRequest(mr *gl.MergeRequest) *MergeRequest {
	result := toMergeRequestFromBasic(&mr.BasicMergeRequest)
	if mr.Pipeline != nil {
		result.Pipeline = toPipelineInfo(mr.Pipeline)
	}
	return result
}

// toContributionEvent maps a GitLab contribution event. Push events carry no
// title (title is empty), so Target falls back to the pushed ref.
func toContributionEvent(e *gl.ContributionEvent) ContributionEvent {
	target := e.TargetTitle
	if target == "" {
		target = e.PushData.Ref
	}
	return ContributionEvent{
		Action:    e.ActionName,
		Target:    target,
		CreatedAt: timeValue(e.CreatedAt),
	}
}

func toPipelineInfo(p *gl.PipelineInfo) *Pipeline {
	if p == nil {
		return nil
	}
	return &Pipeline{
		ID:        int(p.ID),
		IID:       int(p.IID),
		Status:    PipelineStatus(p.Status),
		Source:    p.Source,
		Ref:       p.Ref,
		SHA:       p.SHA,
		WebURL:    p.WebURL,
		CreatedAt: timeValue(p.CreatedAt),
	}
}

func toPipeline(p *gl.Pipeline) *Pipeline {
	if p == nil {
		return nil
	}
	return &Pipeline{
		ID:         int(p.ID),
		IID:        int(p.IID),
		Status:     PipelineStatus(p.Status),
		Source:     string(p.Source),
		Ref:        p.Ref,
		SHA:        p.SHA,
		WebURL:     p.WebURL,
		User:       authorUsername(p.User),
		CreatedAt:  timeValue(p.CreatedAt),
		StartedAt:  timeValue(p.StartedAt),
		FinishedAt: timeValue(p.FinishedAt),
		Duration:   int(p.Duration),
		Coverage:   p.Coverage,
	}
}

func toMergeRequestDiff(d *gl.MergeRequestDiff) *MergeRequestDiff {
	return &MergeRequestDiff{
		OldPath:       d.OldPath,
		NewPath:       d.NewPath,
		Diff:          d.Diff,
		NewFile:       d.NewFile,
		RenamedFile:   d.RenamedFile,
		DeletedFile:   d.DeletedFile,
		GeneratedFile: d.GeneratedFile,
		Collapsed:     d.Collapsed,
		TooLarge:      d.TooLarge,
	}
}

func toCommit(c *gl.Commit) Commit {
	return Commit{
		ShortID:    c.ShortID,
		Title:      c.Title,
		AuthorName: c.AuthorName,
		CreatedAt:  timeValue(c.CreatedAt),
	}
}

func toJob(j *gl.Job) *Job {
	return &Job{
		ID:             int(j.ID),
		Name:           j.Name,
		Stage:          j.Stage,
		Status:         JobStatus(j.Status),
		Ref:            j.Ref,
		WebURL:         j.WebURL,
		Duration:       j.Duration,
		QueuedDuration: j.QueuedDuration,
		AllowFailure:   j.AllowFailure,
		FailureReason:  j.FailureReason,
		CreatedAt:      timeValue(j.CreatedAt),
		StartedAt:      timeValue(j.StartedAt),
		FinishedAt:     timeValue(j.FinishedAt),
	}
}
