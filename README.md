# gitlab-tui

A terminal UI for GitLab focused on the daily developer workflow: merge
requests, diffs, pipelines and job logs — across multiple GitLab profiles
(e.g. work and personal), without leaving the terminal.

See [`gitlab-tui-plano-completo.md`](./gitlab-tui-plano-completo.md) for the
full product plan and [`BACKLOG.md`](./BACKLOG.md) for the épico-by-épico
backlog and progress.

## Status

Early development. The current focus is **Fase 1 — Base utilizável**: token
auth, multi-profile support, local project/branch detection, and a minimal
dashboard showing the merge request for your current branch.

## Requirements

- Go 1.25+
- A GitLab personal access token with at least `read_api` and `read_user`
  scopes

## Build

```bash
make build
./bin/gitlab-tui version
```

## Configure

```bash
export GITLAB_EMPRESA_TOKEN=glpat-xxxxx
./bin/gitlab-tui auth login
```

This creates `~/.config/gitlab-tui/config.yaml`. See the plan doc for the
config file shape.

## Usage

```bash
gitlab-tui                 # open the TUI (detects the current repo/branch)
gitlab-tui auth login
gitlab-tui auth status
gitlab-tui version
```
