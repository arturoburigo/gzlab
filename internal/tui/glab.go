package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

type glabRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)

func runGLab(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "glab", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GLAB_PAGER=cat", "PAGER=cat", "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("glab %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func loadDiffFromGLab(ctx context.Context, deps Deps, iid int) ([]*gitlab.MergeRequestDiff, error) {
	out, err := deps.glab(ctx, "mr", "diff", strconv.Itoa(iid), "--color=never")
	if err != nil {
		return nil, err
	}
	return []*gitlab.MergeRequestDiff{{Diff: string(out)}}, nil
}

func loadPipelineFromGLab(ctx context.Context, deps Deps, pipelineID int) (*gitlab.Pipeline, []*gitlab.Job, error) {
	out, err := deps.glab(ctx, "ci", "get", "--pipeline-id", strconv.Itoa(pipelineID), "--with-job-details", "--output", "json")
	if err != nil {
		return nil, nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, nil, fmt.Errorf("parsing glab ci get JSON: %w", err)
	}
	return parseGLabPipeline(raw), parseGLabJobs(raw), nil
}

// runGLabAction runs a glab subcommand that mutates state (retry, cancel,
// approve, merge, ...) and reports success/failure only; the caller doesn't
// need the output.
func runGLabAction(ctx context.Context, deps Deps, args ...string) error {
	_, err := deps.glab(ctx, args...)
	return err
}

func loadJobLogFromGLab(ctx context.Context, deps Deps, jobID int) (string, error) {
	out, err := deps.glab(ctx, "api", fmt.Sprintf("projects/:id/jobs/%d/trace", jobID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// loadDiscussionsFromGLab fetches an MR's discussion threads. Pagination is
// deferred: a single per_page=100 page covers all but unusually noisy MRs.
func loadDiscussionsFromGLab(ctx context.Context, deps Deps, iid int) ([]gitlab.Discussion, error) {
	out, err := deps.glab(ctx, "api", fmt.Sprintf("projects/:id/merge_requests/%d/discussions?per_page=100", iid))
	if err != nil {
		return nil, err
	}

	var raw []map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing discussions JSON: %w", err)
	}
	return parseGLabDiscussions(raw), nil
}

func loadCommitsFromGLab(ctx context.Context, deps Deps, iid int) ([]gitlab.Commit, error) {
	out, err := deps.glab(ctx, "api", fmt.Sprintf("projects/:id/merge_requests/%d/commits?per_page=100", iid))
	if err != nil {
		return nil, err
	}

	var raw []map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing commits JSON: %w", err)
	}
	return parseGLabCommits(raw), nil
}

func parseGLabCommits(raw []map[string]any) []gitlab.Commit {
	commits := make([]gitlab.Commit, 0, len(raw))
	for _, item := range raw {
		commits = append(commits, gitlab.Commit{
			ShortID:    stringValue(firstPresent(item, "short_id", "shortId")),
			Title:      stringValue(item["title"]),
			AuthorName: stringValue(firstPresent(item, "author_name", "authorName")),
			CreatedAt:  timeValueFromAny(firstPresent(item, "created_at", "createdAt")),
		})
	}
	return commits
}

func parseGLabDiscussions(raw []map[string]any) []gitlab.Discussion {
	discussions := make([]gitlab.Discussion, 0, len(raw))
	for _, item := range raw {
		notes, _ := item["notes"].([]any)
		discussion := gitlab.Discussion{ID: stringValue(item["id"])}
		for _, entry := range notes {
			noteRaw, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			discussion.Notes = append(discussion.Notes, gitlab.Note{
				ID:         intNumber(noteRaw["id"]),
				Author:     parseNoteAuthor(noteRaw["author"]),
				Body:       stringValue(noteRaw["body"]),
				System:     boolValue(noteRaw["system"]),
				Resolvable: boolValue(noteRaw["resolvable"]),
				Resolved:   boolValue(noteRaw["resolved"]),
				CreatedAt:  timeValueFromAny(firstPresent(noteRaw, "created_at", "createdAt")),
			})
		}
		discussions = append(discussions, discussion)
	}
	return discussions
}

func parseNoteAuthor(value any) string {
	author, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(firstPresent(author, "username", "name"))
}

func (d Deps) glab(ctx context.Context, args ...string) ([]byte, error) {
	runner := d.RunGLab
	if runner == nil {
		runner = runGLab
	}
	return runner(ctx, d.RepoRoot, args...)
}

func parseGLabPipeline(raw map[string]any) *gitlab.Pipeline {
	return &gitlab.Pipeline{
		ID:         intNumber(raw["id"]),
		IID:        intNumber(raw["iid"]),
		Status:     gitlab.PipelineStatus(stringValue(raw["status"])),
		Source:     stringValue(raw["source"]),
		Ref:        stringValue(raw["ref"]),
		SHA:        stringValue(raw["sha"]),
		WebURL:     stringValue(firstPresent(raw, "web_url", "webUrl")),
		CreatedAt:  timeValueFromAny(firstPresent(raw, "created_at", "createdAt")),
		StartedAt:  timeValueFromAny(firstPresent(raw, "started_at", "startedAt")),
		FinishedAt: timeValueFromAny(firstPresent(raw, "finished_at", "finishedAt")),
		Duration:   intNumber(raw["duration"]),
		Coverage:   stringValue(raw["coverage"]),
	}
}

func parseGLabJobs(raw map[string]any) []*gitlab.Job {
	items, ok := raw["jobs"].([]any)
	if !ok {
		items, _ = raw["builds"].([]any)
	}

	jobs := make([]*gitlab.Job, 0, len(items))
	for _, item := range items {
		jobRaw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		jobs = append(jobs, &gitlab.Job{
			ID:             intNumber(jobRaw["id"]),
			Name:           stringValue(jobRaw["name"]),
			Stage:          stringValue(jobRaw["stage"]),
			Status:         gitlab.JobStatus(stringValue(jobRaw["status"])),
			Ref:            stringValue(jobRaw["ref"]),
			WebURL:         stringValue(firstPresent(jobRaw, "web_url", "webUrl")),
			Duration:       floatNumber(jobRaw["duration"]),
			QueuedDuration: floatNumber(firstPresent(jobRaw, "queued_duration", "queuedDuration")),
			AllowFailure:   boolValue(firstPresent(jobRaw, "allow_failure", "allowFailure")),
			FailureReason:  stringValue(firstPresent(jobRaw, "failure_reason", "failureReason")),
			CreatedAt:      timeValueFromAny(firstPresent(jobRaw, "created_at", "createdAt")),
			StartedAt:      timeValueFromAny(firstPresent(jobRaw, "started_at", "startedAt")),
			FinishedAt:     timeValueFromAny(firstPresent(jobRaw, "finished_at", "finishedAt")),
		})
	}
	return jobs
}

func firstPresent(raw map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			return value
		}
	}
	return nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func intNumber(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func floatNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		n, _ := strconv.ParseFloat(v, 64)
		return n
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	default:
		return false
	}
}

func timeValueFromAny(value any) time.Time {
	text := stringValue(value)
	if text == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, text)
	return t
}
