# gzlab

A terminal UI for GitLab focused on the daily developer workflow: merge
requests, diffs, pipelines and job logs — across multiple GitLab profiles
(e.g. work and personal), without leaving the terminal.

## Demo

<video src="https://github.com/arturoburigo/gzlab/raw/main/docs/presentation.mov" controls width="100%"></video>

▶️ If the player above doesn't load, [watch the demo](./docs/presentation.mov).

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
./bin/gzlab version
```

Cross-platform release artifacts:

```bash
make release
ls dist/
```

This produces macOS, Linux and Windows archives plus `checksums.txt`.

## Install

From a published release:

```bash
curl -fsSL https://raw.githubusercontent.com/arturoburigo/gzlab/main/scripts/install.sh | sh
```

Useful overrides:

```bash
GITLAB_TUI_VERSION=v0.1.0 sh scripts/install.sh
GITLAB_TUI_INSTALL_DIR=/usr/local/bin sh scripts/install.sh
GITLAB_TUI_REPO=owner/repo sh scripts/install.sh
```

## Configure

```bash
export GITLAB_EMPRESA_TOKEN=glpat-xxxxx
./bin/gzlab auth login
```

This creates `~/.config/gitlab-tui/config.yaml`. See the plan doc for the
config file shape.

Available UI themes include `dark` (black terminal with vibrant colors),
`terminal`, `retro`, `light`, `gitlab`, `dracula`, `tokyonight`, `catppuccin`,
`gruvbox`, `nord`, `rosepine`, `solarized`, and the other built-in themes from
`ku`:

```yaml
ui:
  theme: dark
  mouse: true
```

## Usage

```bash
gzlab                 # open the TUI (detects the current repo/branch)
gzlab auth login
gzlab auth status
gzlab version
```

Inside the TUI, use `/` for global search, `tab` for workspaces, and `W` from
an MR detail to add the current MR to a branch-derived workspace.
