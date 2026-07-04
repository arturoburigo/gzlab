package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"

	"github.com/arturoburigo/gzlab/internal/dashboard"
	"github.com/arturoburigo/gzlab/internal/gitdetect"
	"github.com/arturoburigo/gzlab/internal/gitlab"
	"github.com/arturoburigo/gzlab/internal/history"
	"github.com/arturoburigo/gzlab/internal/workspace"
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

// dashboardCommitLimit/dashboardMRStatsLimit/dashboardAssignedMRLimit/
// dashboardActivityLimit bound the dashboard's best-effort personal-stats
// enrichment — a recent snapshot for a small visual, not an exhaustive
// history. The contribution calendar covers the current month, so activity is
// fetched with an After bound at the month's 1st (ListMyContributionEvents'
// filter scopes this server-side, and lets pagination stop once it walks past
// the month); the limit is just a safety ceiling for a very busy month.
const (
	dashboardCommitLimit     = 8
	dashboardMRStatsLimit    = 100
	dashboardAssignedMRLimit = 6
	dashboardActivityLimit   = 500
)

// currentMonthStart is midnight on the 1st of now's month, local time — the
// lower bound for the contribution calendar's activity fetch.
func currentMonthStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

// loadDashboardStatsCmd fetches the dashboard's personal-stats enrichment
// (recent commits authored by the current user on this project, a
// cross-project MR state breakdown, open MRs assigned to the current user
// across projects, and the current user's recent cross-project activity).
// It's best-effort like recordHistoryCmd: any failure just means the cards
// don't show, never an error banner.
//
// The four fetches run concurrently — they're independent (only commits waits
// on CurrentUser for the author name), so a WaitGroup collapses what used to
// be five back-to-back round-trips into roughly one. Each goroutine writes a
// distinct field of msg, so there's no shared-memory race.
func loadDashboardStatsCmd(client gitlab.Client, ctx *dashboard.Context) tea.Cmd {
	if client == nil || ctx == nil || ctx.Project == nil {
		return nil
	}
	return func() tea.Msg {
		background := context.Background()
		var (
			msg dashboardStatsLoadedMsg
			wg  sync.WaitGroup
		)

		wg.Add(1)
		go func() {
			defer wg.Done()
			user, err := client.CurrentUser(background)
			if err != nil || user == nil {
				return
			}
			if commits, err := client.ListCommits(background, ctx.Project.ID, gitlab.ListCommitsOptions{
				Author: user.Name,
				Limit:  dashboardCommitLimit,
			}); err == nil {
				msg.commits = commits
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			if mrs, err := client.ListMyMergeRequests(background, gitlab.ListMyMergeRequestsOptions{
				State: gitlab.MergeRequestStateAll,
				Scope: gitlab.MergeRequestsScopeCreatedByMe,
				Limit: dashboardMRStatsLimit,
			}); err == nil {
				msg.mrs = mrs
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			if assigned, err := client.ListMyMergeRequests(background, gitlab.ListMyMergeRequestsOptions{
				State: gitlab.MergeRequestStateOpened,
				Scope: gitlab.MergeRequestsScopeAssignedToMe,
				Limit: dashboardAssignedMRLimit,
			}); err == nil {
				msg.assignedMRs = assigned
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			if activity, err := client.ListMyContributionEvents(background, gitlab.ListContributionEventsOptions{
				After: currentMonthStart(time.Now()).AddDate(0, 0, -1), // inclusive of the 1st (After is date-exclusive)
				Limit: dashboardActivityLimit,
			}); err == nil {
				msg.activity = activity
			}
		}()

		wg.Wait()
		return msg
	}
}

// spinnerFrames/spinnerInterval drive the dashboard's initial-load spinner —
// a tiny hand-rolled cycler, since bubbles/spinner isn't a dependency and the
// tea.Tick idiom is already how this package animates (pipeline/job polling).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 90 * time.Millisecond

func spinnerTickCmd(gen int) tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg {
		return spinnerTickMsg{gen: gen}
	})
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

// loadMRScopeCmd fetches merge requests for scope: either the current
// project, or one of GitLab's cross-project views (Épico 11). The "to
// review" scope needs the caller's own username, fetched (and cached) via
// CurrentUser.
func loadMRScopeCmd(client gitlab.Client, scope mrScope, projectID int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		switch scope {
		case mrScopeCreatedByMe:
			mrs, err := client.ListMyMergeRequests(ctx, gitlab.ListMyMergeRequestsOptions{Scope: gitlab.MergeRequestsScopeCreatedByMe})
			if err != nil {
				return errMsg{err}
			}
			return mrListLoadedMsg{mrs}
		case mrScopeAssignedToMe:
			mrs, err := client.ListMyMergeRequests(ctx, gitlab.ListMyMergeRequestsOptions{Scope: gitlab.MergeRequestsScopeAssignedToMe})
			if err != nil {
				return errMsg{err}
			}
			return mrListLoadedMsg{mrs}
		case mrScopeToReview:
			user, err := client.CurrentUser(ctx)
			if err != nil {
				return errMsg{err}
			}
			mrs, err := client.ListMyMergeRequests(ctx, gitlab.ListMyMergeRequestsOptions{ReviewerUsername: user.Username})
			if err != nil {
				return errMsg{err}
			}
			return mrListLoadedMsg{mrs}
		default:
			mrs, err := client.ListMergeRequests(ctx, projectID, gitlab.ListMergeRequestsOptions{})
			if err != nil {
				return errMsg{err}
			}
			return mrListLoadedMsg{mrs}
		}
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

func loadSearchCmd(client gitlab.Client, query string, project *gitlab.Project) tea.Cmd {
	return func() tea.Msg {
		results, err := client.Search(context.Background(), gitlab.GlobalSearchOptions{
			Query:   query,
			Limit:   20,
			Project: project,
		})
		if err != nil {
			return errMsg{err}
		}
		return searchLoadedMsg{query: query, results: results}
	}
}

func loadWorkspacesCmd(deps Deps, client gitlab.Client) tea.Cmd {
	return func() tea.Msg {
		items, err := loadWorkspaceViews(context.Background(), deps, client)
		if err != nil {
			return errMsg{err}
		}
		return workspacesLoadedMsg{workspaces: items}
	}
}

func addCurrentMRToWorkspaceCmd(deps Deps, ctx *dashboard.Context, mr *gitlab.MergeRequest) tea.Cmd {
	return func() tea.Msg {
		items, status, err := saveCurrentMRToWorkspace(context.Background(), deps, ctx, mr, true)
		if err != nil {
			return errMsg{err}
		}
		return workspaceSavedMsg{status: status, workspaces: items}
	}
}

func removeCurrentMRFromWorkspaceCmd(deps Deps, ctx *dashboard.Context, mr *gitlab.MergeRequest) tea.Cmd {
	return func() tea.Msg {
		items, status, err := saveCurrentMRToWorkspace(context.Background(), deps, ctx, mr, false)
		if err != nil {
			return errMsg{err}
		}
		return workspaceSavedMsg{status: status, workspaces: items}
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

func loadWorkspaceViews(ctx context.Context, deps Deps, client gitlab.Client) ([]workspaceView, error) {
	store, err := workspace.Load(deps.WorkspacePath)
	if err != nil {
		return nil, err
	}
	profile := deps.ProfileOverride
	if profile == "" {
		profile = deps.Config.DefaultProfile
	}
	var views []workspaceView
	for _, ws := range store.List(profile) {
		view := workspaceView{Name: ws.Name, Profile: ws.Profile, UpdatedAt: ws.UpdatedAt}
		for _, ref := range ws.MergeRequests {
			item := workspaceMRView{Ref: ref}
			if client != nil && ref.ProjectID != 0 && ref.IID != 0 {
				if mr, err := client.GetMergeRequest(ctx, ref.ProjectID, ref.IID); err == nil && mr != nil {
					item.Ref.Title = mr.Title
					item.Ref.WebURL = mr.WebURL
					item.Ref.Status = string(mr.State) + draftSuffix(mr)
					item.Ref.Branch = mr.SourceBranch
					if mr.Pipeline != nil {
						item.Ref.Pipeline = string(mr.Pipeline.Status)
					}
					if mr.ApprovalsRequired > 0 {
						item.Ref.Approvals = fmt.Sprintf("%d/%d", mr.ApprovalsGiven, mr.ApprovalsRequired)
					}
				}
			}
			view.MRs = append(view.MRs, item)
		}
		views = append(views, view)
	}
	return views, nil
}

func saveCurrentMRToWorkspace(ctx context.Context, deps Deps, dash *dashboard.Context, mr *gitlab.MergeRequest, add bool) ([]workspaceView, string, error) {
	if dash == nil || dash.Project == nil || mr == nil {
		return nil, "", fmt.Errorf("no merge request in context")
	}
	store, err := workspace.Load(deps.WorkspacePath)
	if err != nil {
		return nil, "", err
	}
	name := workspaceNameFromBranch(mr.SourceBranch)
	ref := workspace.MergeRequestRef{
		ProjectPath: dash.Project.PathWithNamespace,
		ProjectID:   dash.Project.ID,
		IID:         mr.IID,
		Title:       mr.Title,
		WebURL:      mr.WebURL,
		Status:      string(mr.State) + draftSuffix(mr),
		Branch:      mr.SourceBranch,
	}
	if mr.Pipeline != nil {
		ref.Pipeline = string(mr.Pipeline.Status)
	}
	if mr.ApprovalsRequired > 0 {
		ref.Approvals = fmt.Sprintf("%d/%d", mr.ApprovalsGiven, mr.ApprovalsRequired)
	}
	if add {
		store.UpsertMR(dash.ProfileName, name, ref)
	} else {
		store.RemoveMR(dash.ProfileName, name, ref.ProjectPath, ref.IID)
	}
	if err := workspace.Save(deps.WorkspacePath, store); err != nil {
		return nil, "", err
	}
	views, err := loadWorkspaceViews(ctx, deps, nil)
	if err != nil {
		return nil, "", err
	}
	verb := "Added"
	if !add {
		verb = "Removed"
	}
	return views, fmt.Sprintf("%s !%d %s workspace %s.", verb, mr.IID, map[bool]string{true: "to", false: "from"}[add], name), nil
}

func workspaceNameFromBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "default"
	}
	for _, sep := range []string{"/", "_"} {
		if i := strings.Index(branch, sep); i >= 0 && i < len(branch)-1 {
			branch = branch[i+1:]
		}
	}
	parts := strings.Split(branch, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], "-")
	}
	return branch
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

// pipelinePollInterval is how often the pipeline screen auto-refreshes while
// the pipeline is still running (Épico 14).
const pipelinePollInterval = 10 * time.Second

// isPipelineActive reports whether p hasn't reached a terminal status yet —
// the condition under which the pipeline screen keeps auto-polling.
func isPipelineActive(p *gitlab.Pipeline) bool {
	if p == nil {
		return false
	}
	switch p.Status {
	case gitlab.PipelineStatusCreated, gitlab.PipelineStatusWaitingForResource,
		gitlab.PipelineStatusPreparing, gitlab.PipelineStatusPending, gitlab.PipelineStatusRunning:
		return true
	default:
		return false
	}
}

func pipelinePollTickCmd(gen int) tea.Cmd {
	return tea.Tick(pipelinePollInterval, func(time.Time) tea.Msg {
		return pipelinePollTickMsg{gen: gen}
	})
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

// jobLogFollowInterval is how often follow mode re-fetches the trace.
const jobLogFollowInterval = 5 * time.Second

// isJobActive reports whether job hasn't reached a terminal status yet.
func isJobActive(status gitlab.JobStatus) bool {
	switch status {
	case gitlab.JobStatusCreated, gitlab.JobStatusPending, gitlab.JobStatusRunning:
		return true
	default:
		return false
	}
}

func jobLogFollowTickCmd(gen int) tea.Cmd {
	return tea.Tick(jobLogFollowInterval, func(time.Time) tea.Msg {
		return jobLogFollowTickMsg{gen: gen}
	})
}

// followJobLogCmd re-fetches both the job's current status and its trace —
// unlike loadJobLogCmd, which trusts the Job passed in, follow mode needs a
// fresh status to notice when a running job has finished.
func followJobLogCmd(deps Deps, jobID int) tea.Cmd {
	return func() tea.Msg {
		job, err := loadJobFromGLab(context.Background(), deps, jobID)
		if err != nil {
			return errMsg{err}
		}
		log, err := loadJobLogFromGLab(context.Background(), deps, jobID)
		if err != nil {
			return errMsg{err}
		}
		return jobLogLoadedMsg{job: job, log: log}
	}
}

// saveJobLogCmd writes the job's full (untruncated) trace to a file in the
// repo root, so very large logs are still fully retrievable even though the
// viewer only keeps the tail in memory (Épico 15).
func saveJobLogCmd(deps Deps, job *gitlab.Job, raw string) tea.Cmd {
	return func() tea.Msg {
		name := fmt.Sprintf("job-%d.log", job.ID)
		path := filepath.Join(deps.RepoRoot, name)
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			return errMsg{fmt.Errorf("saving log: %w", err)}
		}
		return statusMsg{"Saved log to " + path}
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

// checkoutMRCmd checks out mr's branch and, on success, records it into
// history immediately (Épico 18) rather than waiting for the next dashboard
// load — dash and mr describe what's being checked out, for that record.
func checkoutMRCmd(deps Deps, dash *dashboard.Context, mr *gitlab.MergeRequest) tea.Cmd {
	iid := mr.IID
	return func() tea.Msg {
		if err := runGLabAction(context.Background(), deps, "mr", "checkout", strconv.Itoa(iid)); err != nil {
			return errMsg{err}
		}
		branch, err := gitdetect.CurrentBranch(deps.RepoRoot)
		if err != nil {
			return errMsg{err}
		}

		msg := mrCheckedOutMsg{branch: branch}
		if deps.HistoryPath == "" || dash == nil {
			return msg
		}
		checkedOut := &dashboard.Context{
			ProfileName:  dash.ProfileName,
			Profile:      dash.Profile,
			Project:      dash.Project,
			Branch:       branch,
			MergeRequest: mr,
		}
		store := recordHistory(deps.HistoryPath, checkedOut)
		msg.projects = store.Projects(dash.ProfileName)
		msg.branches = store.Branches(dash.ProfileName)
		return msg
	}
}

func loadCommitsCmd(deps Deps, iid int) tea.Cmd {
	return func() tea.Msg {
		commits, err := loadCommitsFromGLab(context.Background(), deps, iid)
		if err != nil {
			return errMsg{err}
		}
		return commitsLoadedMsg{commits: commits}
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

// replyDiscussionCmd posts body as a reply within an existing thread
// (discussionID), rather than opening a new MR-level discussion the way
// postCommentCmd does.
func replyDiscussionCmd(deps Deps, iid int, discussionID, body string) tea.Cmd {
	return func() tea.Msg {
		endpoint := fmt.Sprintf("projects/:id/merge_requests/%d/discussions/%s/notes", iid, discussionID)
		if err := runGLabAction(context.Background(), deps, "api", "--method", "POST", endpoint, "--field", "body="+body); err != nil {
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
