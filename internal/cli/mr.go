package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arturoburigo/gitlab-tui/internal/gitdetect"
	"github.com/arturoburigo/gitlab-tui/internal/gitlab"
)

func newMRCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mr",
		Short: "List and view merge requests for the current repository",
	}
	cmd.AddCommand(newMRListCommand())
	cmd.AddCommand(newMRViewCommand())
	return cmd
}

func newMRListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List open merge requests for the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withAPITimeout(context.Background())
			defer cancel()

			client, project, _, err := resolveProject(ctx)
			if err != nil {
				return err
			}
			mrs, err := client.ListMergeRequests(ctx, project.ID, gitlab.ListMergeRequestsOptions{})
			if err != nil {
				return err
			}
			return printMRList(cmd.OutOrStdout(), mrs)
		},
	}
}

func newMRViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view [iid]",
		Short: "Show a merge request (defaults to the current branch's MR)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withAPITimeout(context.Background())
			defer cancel()

			client, project, branch, err := resolveProject(ctx)
			if err != nil {
				return err
			}

			var mr *gitlab.MergeRequest
			if len(args) == 1 {
				iid, convErr := strconv.Atoi(args[0])
				if convErr != nil {
					return fmt.Errorf("invalid merge request IID %q: %w", args[0], convErr)
				}
				mr, err = client.GetMergeRequest(ctx, project.ID, iid)
			} else {
				mr, err = client.FindMergeRequestForBranch(ctx, project.ID, branch)
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatMRDetail(mr))
			return err
		},
	}
}

// resolveProject detects the current repo, matches it to a profile, and looks
// up its GitLab project — the shared preamble for the mr subcommands.
func resolveProject(ctx context.Context) (gitlab.Client, *gitlab.Project, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, nil, "", err
	}
	repoRoot, err := gitdetect.RepoRoot(dir)
	if err != nil {
		return nil, nil, "", fmt.Errorf("gitlab-tui must be run inside a git repository: %w", err)
	}
	originURL, err := gitdetect.OriginURL(repoRoot)
	if err != nil {
		return nil, nil, "", fmt.Errorf("this repo has no \"origin\" remote configured: %w", err)
	}
	remote, err := gitdetect.ParseRemoteURL(originURL)
	if err != nil {
		return nil, nil, "", err
	}
	branch, err := gitdetect.CurrentBranch(repoRoot)
	if err != nil {
		return nil, nil, "", fmt.Errorf("could not determine the current branch: %w", err)
	}

	_, _, profile, err := loadActiveProfile()
	if err != nil {
		return nil, nil, "", err
	}
	client, err := newClientForProfile(profile)
	if err != nil {
		return nil, nil, "", err
	}
	project, err := client.GetProjectByPath(ctx, remote.Path)
	if err != nil {
		return nil, nil, "", err
	}
	return client, project, branch, nil
}

func printMRList(out io.Writer, mrs []*gitlab.MergeRequest) error {
	if len(mrs) == 0 {
		_, err := fmt.Fprintln(out, "No open merge requests.")
		return err
	}
	for _, mr := range mrs {
		draft := ""
		if mr.Draft {
			draft = " (draft)"
		}
		if _, err := fmt.Fprintf(out, "!%-6d %s%s\n", mr.IID, mr.Title, draft); err != nil {
			return err
		}
	}
	return nil
}

func formatMRDetail(mr *gitlab.MergeRequest) string {
	var b strings.Builder
	draft := ""
	if mr.Draft {
		draft = " (draft)"
	}
	fmt.Fprintf(&b, "!%d %s%s\n", mr.IID, mr.Title, draft)
	fmt.Fprintf(&b, "Branch:    %s -> %s\n", mr.SourceBranch, mr.TargetBranch)
	if mr.Author != "" {
		fmt.Fprintf(&b, "Author:    %s\n", mr.Author)
	}
	fmt.Fprintf(&b, "State:     %s\n", mr.State)
	if mr.Pipeline != nil {
		fmt.Fprintf(&b, "Pipeline:  %s\n", mr.Pipeline.Status)
	}
	if mr.ApprovalsRequired > 0 {
		fmt.Fprintf(&b, "Approvals: %d/%d\n", mr.ApprovalsGiven, mr.ApprovalsRequired)
	}
	if mr.HasConflicts {
		b.WriteString("Conflicts: yes\n")
	}
	if mr.WebURL != "" {
		b.WriteString(mr.WebURL + "\n")
	}
	return b.String()
}
