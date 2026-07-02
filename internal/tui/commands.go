package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"

	"github.com/arturoburigo/gitlab-tui/internal/dashboard"
	"github.com/arturoburigo/gitlab-tui/internal/gitdetect"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
	"github.com/arturoburigo/gitlab-tui/internal/history"
)

func loadDashboardCmd(deps Deps) tea.Cmd {
	return func() tea.Msg {
		ctx, err := dashboard.Resolve(context.Background(), deps.Config, deps.NewClient, deps.Remote, deps.Branch, deps.ProfileOverride)
		if err != nil {
			return errMsg{err}
		}
		return dashboardLoadedMsg{ctx}
	}
}

// recordHistoryCmd records the resolved project/branch/MR into the local
// history store, off the main loop. It's best-effort: any failure is swallowed
// (history is a convenience, never a blocker) and it emits no message.
func recordHistoryCmd(deps Deps, ctx *dashboard.Context) tea.Cmd {
	if deps.HistoryPath == "" || ctx == nil {
		return nil
	}
	return func() tea.Msg {
		store := recordHistory(deps.HistoryPath, ctx)
		return historyLoadedMsg{
			projects: store.Projects(ctx.ProfileName),
			branches: store.Branches(ctx.ProfileName),
		}
	}
}

func recordHistory(path string, ctx *dashboard.Context) *history.Store {
	store, err := history.Load(path)
	if err != nil {
		return &history.Store{}
	}

	now := time.Now()
	if ctx.Project != nil {
		store.RecordProject(ctx.ProfileName, history.Project{
			Path:       ctx.Project.PathWithNamespace,
			Name:       ctx.Project.Name,
			LastAccess: now,
		})
	}

	branch := history.Branch{Name: ctx.Branch, LastAccess: now}
	if ctx.Project != nil {
		branch.ProjectPath = ctx.Project.PathWithNamespace
	}
	if ctx.MergeRequest != nil {
		branch.MRIID = ctx.MergeRequest.IID
		branch.MRTitle = ctx.MergeRequest.Title
	}
	if branch.Name != "" {
		store.RecordBranch(ctx.ProfileName, branch)
	}

	_ = history.Save(path, store)
	return store
}

func loadMRListCmd(client gitlab.Client, projectID int) tea.Cmd {
	return func() tea.Msg {
		mrs, err := client.ListMergeRequests(context.Background(), projectID, gitlab.ListMergeRequestsOptions{})
		if err != nil {
			return errMsg{err}
		}
		return mrListLoadedMsg{mrs}
	}
}

func loadMRDetailCmd(client gitlab.Client, projectID, iid int) tea.Cmd {
	return func() tea.Msg {
		mr, err := client.GetMergeRequest(context.Background(), projectID, iid)
		if err != nil {
			return errMsg{err}
		}
		return mrDetailLoadedMsg{mr}
	}
}

func loadMRDiffCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		diffs, err := loadDiffFromGLab(context.Background(), deps, iid)
		if err != nil {
			return errMsg{err}
		}
		return mrDiffLoadedMsg{diffs}
	}
}

func loadPipelineCmd(deps Deps, mr *gitlab.MergeRequest) tea.Cmd {
	return func() tea.Msg {
		if mr.Pipeline == nil {
			return errMsg{fmt.Errorf("merge request !%d has no pipeline", mr.IID)}
		}

		pipeline, jobs, err := loadPipelineFromGLab(context.Background(), deps, mr.Pipeline.ID)
		if err != nil {
			return errMsg{err}
		}
		return pipelineLoadedMsg{pipeline: pipeline, jobs: jobs}
	}
}

func loadJobLogCmd(deps Deps, job *gitlab.Job) tea.Cmd {
	return func() tea.Msg {
		log, err := loadJobLogFromGLab(context.Background(), deps, job.ID)
		if err != nil {
			return errMsg{err}
		}
		return jobLogLoadedMsg{job: job, log: log}
	}
}

func retryJobCmd(deps Deps, jobID int) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "ci", "retry", strconv.Itoa(jobID)); err != nil {
			return errMsg{err}
		}
		return pipelineActionDoneMsg{status: fmt.Sprintf("Retried job #%d.", jobID)}
	}
}

func retryPipelineCmd(deps Deps, pipelineID int) tea.Cmd {
	return func() tea.Msg {
		endpoint := fmt.Sprintf("projects/:id/pipelines/%d/retry", pipelineID)
		if err := runGLabAction(context.Background(), deps, "api", "--method", "POST", endpoint); err != nil {
			return errMsg{err}
		}
		return pipelineActionDoneMsg{status: fmt.Sprintf("Retried pipeline #%d.", pipelineID)}
	}
}

func cancelPipelineCmd(deps Deps, pipelineID int) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "ci", "cancel", "pipeline", strconv.Itoa(pipelineID)); err != nil {
			return errMsg{err}
		}
		return pipelineActionDoneMsg{status: fmt.Sprintf("Cancelled pipeline #%d.", pipelineID)}
	}
}

func triggerJobCmd(deps Deps, jobID int) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "ci", "trigger", strconv.Itoa(jobID)); err != nil {
			return errMsg{err}
		}
		return pipelineActionDoneMsg{status: fmt.Sprintf("Triggered job #%d.", jobID)}
	}
}

func editorCommand(deps Deps, path string) *exec.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	return exec.Command(editor, filepath.Join(deps.RepoRoot, path))
}

// openEditorCmd opens path (relative to the repo root) in $EDITOR. It uses
// tea.ExecProcess rather than running the command directly, since Bubble Tea
// owns the terminal (raw mode / alt-screen buffer) and needs to release and
// reclaim it around an interactive child process like vim.
func openEditorCmd(deps Deps, path string) tea.Cmd {
	return tea.ExecProcess(editorCommand(deps, path), func(err error) tea.Msg {
		if err != nil {
			return errMsg{fmt.Errorf("opening editor: %w", err)}
		}
		return statusMsg{"Returned from editor."}
	})
}

func approveMRCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "mr", "approve", strconv.Itoa(iid)); err != nil {
			return errMsg{err}
		}
		return mrActionDoneMsg{status: fmt.Sprintf("Approved !%d.", iid)}
	}
}

func revokeMRApprovalCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "mr", "revoke", strconv.Itoa(iid)); err != nil {
			return errMsg{err}
		}
		return mrActionDoneMsg{status: fmt.Sprintf("Approval removed from !%d.", iid)}
	}
}

func toggleMRDraftCmd(deps Deps, iid int, makeDraft bool) tea.Cmd {
	flag, verb := "--ready", "ready"
	if makeDraft {
		flag, verb = "--draft", "draft"
	}
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "mr", "update", strconv.Itoa(iid), flag); err != nil {
			return errMsg{err}
		}
		return mrActionDoneMsg{status: fmt.Sprintf("Marked !%d as %s.", iid, verb)}
	}
}

func mergeMRCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "mr", "merge", strconv.Itoa(iid), "--yes"); err != nil {
			return errMsg{err}
		}
		return mrActionDoneMsg{status: fmt.Sprintf("Merged !%d.", iid)}
	}
}

// preCheckoutCmd checks the working tree before a checkout so we can warn
// before switching branches would disrupt uncommitted work.
func preCheckoutCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		dirty, err := gitdetect.HasUncommittedChanges(deps.RepoRoot)
		if err != nil {
			return errMsg{err}
		}
		return checkoutPreparedMsg{iid: iid, dirty: dirty}
	}
}

func checkoutMRCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "mr", "checkout", strconv.Itoa(iid)); err != nil {
			return errMsg{err}
		}
		branch, err := gitdetect.CurrentBranch(deps.RepoRoot)
		if err != nil {
			return errMsg{err}
		}
		return mrCheckedOutMsg{branch: branch}
	}
}

func loadDiscussionsCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		discussions, err := loadDiscussionsFromGLab(context.Background(), deps, iid)
		if err != nil {
			return errMsg{err}
		}
		return discussionsLoadedMsg{discussions: discussions}
	}
}

func postCommentCmd(deps Deps, iid int, body string) tea.Cmd {
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "mr", "note", strconv.Itoa(iid), "--message", body); err != nil {
			return errMsg{err}
		}
		return commentPostedMsg{iid: iid}
	}
}

func resolveDiscussionCmd(deps Deps, iid int, discussionID string, resolved bool) tea.Cmd {
	return func() tea.Msg {
		endpoint := fmt.Sprintf("projects/:id/merge_requests/%d/discussions/%s?resolved=%t", iid, discussionID, resolved)
		if err := runGLabAction(context.Background(), deps, "api", "--method", "PUT", endpoint); err != nil {
			return errMsg{err}
		}
		verb := "Resolved"
		if !resolved {
			verb = "Reopened"
		}
		return discussionActionDoneMsg{iid: iid, status: verb + " thread."}
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if err := browser.OpenURL(url); err != nil {
			return errMsg{fmt.Errorf("opening browser: %w", err)}
		}
		return statusMsg{"Opened in browser."}
	}
}

func copyToClipboardCmd(text, success string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil {
			return errMsg{fmt.Errorf("copying to clipboard: %w", err)}
		}
		return statusMsg{success}
	}
}

func copyLinkCmd(url string) tea.Cmd {
	return copyToClipboardCmd(url, "Link copied to clipboard.")
}
