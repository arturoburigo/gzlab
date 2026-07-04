package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/arturoburigo/gzlab/internal/gitdetect"
	"github.com/arturoburigo/gzlab/internal/gitlab"
	"github.com/arturoburigo/gzlab/internal/tui"
)

func newPipelineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "View pipelines and job logs for the current repository",
	}
	cmd.AddCommand(newPipelineListCommand())
	cmd.AddCommand(newPipelineLogsCommand())
	return cmd
}

func newPipelineListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show the current branch's merge request pipeline and its jobs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withAPITimeout(context.Background())
			defer cancel()

			client, project, branch, err := resolveProject(ctx)
			if err != nil {
				return err
			}
			mr, err := client.FindMergeRequestForBranch(ctx, project.ID, branch)
			if err != nil {
				return err
			}
			mr, err = client.GetMergeRequest(ctx, project.ID, mr.IID)
			if err != nil {
				return err
			}
			if mr.Pipeline == nil {
				return fmt.Errorf("merge request !%d has no pipeline", mr.IID)
			}

			repoRoot, err := currentRepoRoot()
			if err != nil {
				return err
			}
			pipeline, jobs, err := fetchPipeline(ctx, repoRoot, mr.Pipeline.ID)
			if err != nil {
				return err
			}
			return printPipeline(cmd.OutOrStdout(), pipeline, jobs)
		},
	}
}

func newPipelineLogsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <job-id>",
		Short: "Print a job's trace log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid job ID %q: %w", args[0], err)
			}

			repoRoot, err := currentRepoRoot()
			if err != nil {
				return err
			}

			ctx, cancel := withAPITimeout(context.Background())
			defer cancel()
			out, err := glabRunner(ctx, repoRoot, "api", fmt.Sprintf("projects/:id/jobs/%d/trace", jobID))
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(out)
			return err
		},
	}
}

// currentRepoRoot resolves the repo root for commands that only need to run
// glab inside it (pipeline commands don't otherwise need config/profile
// resolution — glab already knows its own authenticated host).
func currentRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	repoRoot, err := gitdetect.RepoRoot(dir)
	if err != nil {
		return "", fmt.Errorf("gzlab must be run inside a git repository: %w", err)
	}
	return repoRoot, nil
}

// fetchPipeline shells out to `glab ci get`, reusing internal/tui's response
// parsing so the standalone CLI and the TUI's pipeline screen stay in sync.
func fetchPipeline(ctx context.Context, repoRoot string, pipelineID int) (*gitlab.Pipeline, []*gitlab.Job, error) {
	out, err := glabRunner(ctx, repoRoot, "ci", "get", "--pipeline-id", strconv.Itoa(pipelineID), "--with-job-details", "--output", "json")
	if err != nil {
		return nil, nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, nil, fmt.Errorf("parsing glab ci get JSON: %w", err)
	}
	return tui.ParseGLabPipeline(raw), tui.ParseGLabJobs(raw), nil
}

func printPipeline(out io.Writer, pipeline *gitlab.Pipeline, jobs []*gitlab.Job) error {
	if _, err := fmt.Fprintf(out, "Pipeline #%d — %s\n", pipeline.ID, pipeline.Status); err != nil {
		return err
	}
	if pipeline.Ref != "" {
		if _, err := fmt.Fprintf(out, "Ref: %s\n", pipeline.Ref); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if len(jobs) == 0 {
		_, err := fmt.Fprintln(out, "No jobs returned for this pipeline.")
		return err
	}
	for _, job := range jobs {
		if _, err := fmt.Fprintf(out, "%-10s %-10s %-40s #%d\n", job.Status, job.Stage, job.Name, job.ID); err != nil {
			return err
		}
	}
	return nil
}
