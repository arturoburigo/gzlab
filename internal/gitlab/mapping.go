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
		result.Pipeline = &Pipeline{
			ID:        int(mr.Pipeline.ID),
			Status:    PipelineStatus(mr.Pipeline.Status),
			Ref:       mr.Pipeline.Ref,
			WebURL:    mr.Pipeline.WebURL,
			CreatedAt: timeValue(mr.Pipeline.CreatedAt),
		}
	}
	return result
}
