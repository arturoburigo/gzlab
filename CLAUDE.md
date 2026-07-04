# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`gzlab` is a terminal UI (Bubble Tea) + companion CLI (Cobra) for the daily
GitLab review workflow — merge requests, diffs, pipelines, job logs — across
multiple GitLab profiles. Go module: `github.com/arturoburigo/gzlab`,
Go 1.25.7.

## Commands

```bash
make build            # build ./bin/gzlab with version ldflags
make test             # go test ./...
make lint             # golangci-lint run ./... (needs golangci-lint installed)
make fmt              # gofmt -l -w .
make run              # go run ./cmd/gzlab
make install          # go install into $GOBIN (updates the `gzlab` alias target)
make release          # cross-compile darwin/linux/windows archives + checksums into dist/

go test ./internal/tui/ -run TestName   # run a single test
```

Enabled linters (`.golangci.yml`, v2): govet, staticcheck, unused, errcheck.

**After changing code, run in order** (from `AGENTS.md`): `go test ./...` →
`make build` → `make install`, then verify the installed binary
(`$GOBIN/gzlab version`). The binary is `gzlab`, built from `./cmd/gzlab`.

## Architecture

Layering, outer → inner: **`cli` → `tui` / `dashboard` → `gitlab` (+ decorated by `cache`)**.
Leaf packages (`config`, `gitdetect`, `history`, `workspace`, `version`) have no
internal dependencies.

### Two data-access paths to GitLab (the key thing to know)

1. **Typed SDK client** — `internal/gitlab` is the *only* package allowed to
   import the upstream SDK (`gitlab.com/gitlab-org/api/client-go`). Everything
   else depends on the `gitlab.Client` **interface** (`client.go`), implemented
   by `realClient` (`real.go`). Covers cross-cutting reads: current user,
   project, MR list/detail, pipeline, diff file list, global search.

2. **`glab` CLI subprocess** — feature-specific and mutating operations
   (unified diff text, pipeline job details, job log traces, discussions,
   commits, and all mutations: retry/cancel/approve/merge/resolve) shell out to
   the `glab` binary via `internal/tui/glab.go`. This is deliberate (see
   BACKLOG épico 5) — the typed `Client` is kept scoped to genuinely reusable
   reads. `glab` JSON responses are mapped by hand-written parsers (`ParseGLab*`).

When adding a "talk to GitLab" feature, first decide which path it belongs to.

### Caching decorator

`internal/cache.Client` wraps a `gitlab.Client` (decorator pattern) with a
file-backed JSON cache under `~/.cache/gitlab-tui`. Wired in
`cli/context.go:newClientForProfileWithCache` when `cache.enabled`. Per-type
TTLs come from config. It never caches tokens, errors, or job logs. The `glab`
subprocess path is **not** cached.

### TUI: Elm architecture (Bubble Tea)

`internal/tui` is one root `Model` (`model.go`) with an all-in-one struct field
per screen's state. Screens are the `screen` enum (dashboard, list, detail,
diff, pipeline, jobLog, discussions, commits, search, workspace).

- **`Update`** dispatches on message type; the big `tea.KeyMsg` handler is
  `handleKey`, a screen-aware key switch. Modal input states (confirm, command
  palette, diff/log search, comment compose, global search) are checked first
  and routed to dedicated `handle*Key` sub-handlers.
- **All I/O lives in `tea.Cmd`s** in `commands.go` (and `glab.go`) that run off
  the render loop and return a typed `*Msg` (`messages.go`) back into `Update`.
  Never do blocking work inside `Update`/`View`.
- **`View`** (`view.go`) + per-screen `*_view.go` files render; each screen
  exposes a `*Hints()` used by both the footer and the `?` help overlay so they
  can't drift.
- **Polling** (pipeline auto-refresh, job-log follow) uses `tea.Tick` chains
  guarded by a generation counter (`pollGen`) so stale ticks from a left screen
  are no-ops.

### CLI (Cobra)

`internal/cli/root.go` builds the command tree. Running with **no subcommand**
launches the TUI via `tui.go:runTUI`, which does the bootstrap: git detection →
config load → resolve history/workspace paths → build `tui.Deps` → start the
Bubble Tea program. Subcommands: `version`, `config`, `auth`, `profile`, `mr`,
`pipeline`, `cache`. The `pipeline` CLI commands reuse `tui.ParseGLabPipeline`/
`ParseGLabJobs` so CLI and TUI parsing can't diverge.

### Dependency-injection seams (for testability)

- `dashboard.NewClientFunc` — lets tests supply a mock `gitlab.Client` (no token/network).
- `glabRunner` / `tui.Deps.RunGLab` (and the CLI's own `glabRunner`) — lets
  tests inject a fake `glab` subprocess instead of shelling out.
- Tests combine throwaway git repos + an `httptest` GitLab server for full
  resolve→fetch→render coverage.

## Conventions & gotchas

- **Tokens are never persisted.** Config stores only the *name* of the env var
  holding the token (`Profile.TokenEnv`); `Profile.ResolveToken()` reads it at
  runtime. A direct `token` field exists as a fallback but env is preferred.
- **Local state files:** `~/.config/gitlab-tui/config.yaml` (config),
  `history.json` (recent projects/branches), `workspaces.json` (multi-repo
  workspaces); cache under `~/.cache/gitlab-tui` (or `$XDG_CACHE_HOME`).
- **Version metadata** is injected via ldflags (Makefile) into
  `internal/version`; don't hardcode it.
- **Profile resolution:** the TUI matches the git remote's host against
  configured profiles; `--profile` overrides. The CLI uses `--profile` →
  `default_profile`.

## Planning & scope

`BACKLOG.md` is the source of truth for status and what shipped per "épico";
`gzlab-plano-completo.md` is the full product plan (26 épicos, 5 fases).
Code comments frequently reference épico numbers — cross-check `BACKLOG.md` for
what's intentionally deferred vs. a real gap before "fixing" a perceived
omission. Prefer smallest-slice-first when extending a plan item.
