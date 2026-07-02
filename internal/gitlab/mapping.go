package gitlab

import (
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

func toMergeRequestFromBasic(mr *gl.BasicMergeRequest) *MergeRequest {
	return &MergeRequest{
		IID:          int(mr.IID),
		ProjectID:    int(mr.ProjectID),
		Title:        mr.Title,
		State:        MergeRequestState(mr.State),
		Draft:        mr.Draft,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Author:       authorUsername(mr.Author),
		WebURL:       mr.WebURL,
		HasConflicts: mr.HasConflicts,
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
