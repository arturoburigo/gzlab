# Backlog

Working tracker for the 26 épicos in [`gitlab-tui-plano-completo.md`](./gitlab-tui-plano-completo.md).
That file is the source of truth for full task lists per épico; this file
tracks status and links each épico to what actually shipped, so it won't
duplicate — and drift from — the plan doc's own checklists.

Status legend: ✅ done for this scope · 🚧 partial (some tasks trimmed, see note) · ⬜ not started

## Fase 1 — Base utilizável (this session)

Delivered the plan's "Primeira Slice Recomendada" (§13): run `gitlab-tui`
inside a repo and see the merge request for your current branch, with
pipeline status and approvals, plus a basic MR list/detail. Verified live
against `gitlab.services.betha.cloud`.

| Épico | Status | Notes |
|---|---|---|
| 1 — Setup do Projeto | ✅ | Go + Cobra + golangci-lint + Makefile. `help`/`version` commands. |
| 2 — Configuração Local | ✅ | `~/.config/gitlab-tui/config.yaml`, `config show`/`edit`. |
| 3 — Autenticação por Token | ✅ | `auth login`/`status`/`logout`; token validated before saving, never persisted (only its env var name is). |
| 4 — Profiles | 🚧 | `--profile` flag, `default_profile`, `profile list`/`remove`. **Trimmed:** `profile add`/`rename`/`test` — `auth login` already covers validated creation, `auth status --profile <name>` covers testing. `add`/`rename` can be added later without architecture changes. **Deferred:** recent-projects/recent-branches persistence — that's Épico 8/9's job, nothing consumes it yet. |
| 5 — Cliente GitLab | 🚧 | `Client` interface wrapping `gitlab.com/gitlab-org/api/client-go`, scoped to what Fase 1 uses: `CurrentUser`, `GetProjectByPath`, `ListMergeRequests`, `GetMergeRequest`, `FindMergeRequestForBranch`. **Deferred:** diff/pipeline-jobs/logs/retry/cancel/approve/discussions methods — these land with the épicos that actually call them (13, 14, 15, 16, 17), not speculatively now. |
| 6 — Detecção de Projeto Local | ✅ | `internal/gitdetect`: repo root, origin URL, current branch, SSH/HTTPS remote parsing, profile-by-host matching. |
| 7 — Dashboard Inicial | 🚧 | Single screen: profile, project, branch, current-branch MR, pipeline, approvals. **Deferred:** recent-projects/recent-branches cards, `?` help overlay, empty/error states beyond the basic error screen — full multi-card dashboard needs Épicos 8/9 data first. |
| 11 — Listagem de Merge Requests | 🚧 | Open MRs for the current project (`m` key), IID/title/state/draft. **Deferred:** cross-project filters (minhas MRs, atribuídas a mim, para revisar, por label/autor) — those need the global `/merge_requests` scope and labels, not just project-scoped listing. |
| 12 — Detalhe de Merge Request | 🚧 | Branches, author, status, pipeline, approvals, conflicts. **Deferred:** discussions/commits/changed-files tabs and the "Why blocked?" panel — need Épicos 13 (diff) and 16 (discussions) first. |
| 25 — Testes | 🚧 | Unit tests alongside every package (config, gitlab mapping, gitdetect — including real throwaway git repos, dashboard with a mock client, TUI `Update`/`View` with injected messages), now also covering the diff/side-by-side/whitespace parsers and the job-log trace parser (ANSI/section-marker stripping) added in Fase 2. **Deferred:** CLI-level integration tests. |

Untouched this session, all tasks from the plan doc still apply as written:

- 🚧 **8 — Projetos Recentes** — store (`internal/history`: single `~/.config/gitlab-tui/history.json`, keyed by profile, recency-ordered, bounded to 10, best-effort) records the current project on every dashboard load, and the dashboard shows a read-only "Recent projects" card (current project excluded). **Deferred:** selecting a recent project to switch to it — that's cross-repo navigation, which overlaps Épico 19's workspace.
- 🚧 **9 — Branches Recentes** — the store records the current branch with its MR association (IID + title); the dashboard shows a navigable "Recent branches" card (current branch excluded). `j`/`k` move a cursor spanning the current-MR slot (`-1`) and the recent list; `enter` opens the selected branch's MR when it's in the current project. **Deferred:** opening a recent branch whose MR is in another project (needs cross-project resolution, Épico 19), and recording on checkout (today the card refreshes on the next dashboard load / `r`).
- **10 — Busca Global** — backlog.

## Fase 2 — Valor real para review

- 🚧 **13 — Diff Viewer** — unified and side-by-side rendering (`s` toggle), file/hunk navigation, search (`/`, `n`/`N`), whitespace toggle (`w`), open file in `$EDITOR` (`e`). Side-by-side pairs removal/addition runs block-wise (not full intraline/LCS diffing) and reuses the unified scroll offset as an approximation, so it can drift a little inside hunks with uneven +/- counts; search-match highlighting only applies in unified mode. **Deferred:** context expansion, copy hunk/line, per-file loading state, large-diff fallback.
- 🚧 **14 — Pipeline da MR** — stages, jobs, status/duration/ref/source, retry job (`R`), retry pipeline (`P`, via `glab api` — `glab ci` has no pipeline-level retry subcommand), cancel pipeline (`x`, confirms), manual job trigger (`t`). **Deferred:** who started the pipeline (`Pipeline` has no `User` field yet), real-time status updates (refresh is manual via `r`, no polling).
- 🚧 **15 — Logs de Job** — log viewer (`enter` from the pipeline job list), search (`/`, `n`/`N`), jump-to-error (`e`/`E`) with highlighting, retry job (`R`, shared with the pipeline screen). Trace fetched via `glab api projects/:id/jobs/<id>/trace` (one-shot, not `ci trace`'s follow mode). **Deferred:** follow mode (live tail for a running job — needs async polling in the Bubble Tea loop, not just a one-shot fetch), copy line/block, save log to file, paginated download for very large logs.

## Fase 3 — Histórico e produtividade (in progress)

- 🚧 **17 — Ações de MR** — approve (`a`), revoke approval (`A`, confirms), draft/ready toggle (`w`), merge (`M`, confirms), checkout branch (`b`, re-detects the local branch via `gitdetect.CurrentBranch` after switching and updates `dash.Branch`). Confirmation required for merge/revoke, matching the plan's "Segurança UX" list. Retry/cancel pipeline and manual job trigger reuse the pipeline-screen actions (Épico 14/15) instead of duplicating them here — there's no job list on the detail screen to pick a specific manual job from. **Deferred:** reviewer/assignee/label management (explicitly future per the plan).
- 🚧 **16 — Discussões e Comentários** — discussions screen (`c` from MR detail) via `glab api projects/:id/merge_requests/:iid/discussions`. Threads are navigable (`j`/`k` moves a thread cursor, `g`/`G` top/end); the display policy (`discussionView` in `discuss_view.go`) hides system notes, marks resolvable threads `✓`/`○`, and indents replies. Add a comment (`c` to compose, single-line, posts via `glab mr note`); resolve/reopen the selected thread (`t`, via `glab api PUT .../discussions/:id?resolved=`). **Deferred:** multi-line comment composition, reply-to-a-specific-thread (vs. a new MR-level discussion), pagination beyond the first per_page=100.
- 🚧 **18 — Checkout de Branch/MR** — `b` (from MR detail) now inspects the working tree first via `gitdetect.HasUncommittedChanges` and, if it's dirty, confirms before `glab mr checkout` so a switch can't silently clobber uncommitted work; a clean tree checks out directly. `glab mr checkout` already creates/tracks the local branch. **Deferred:** refreshing a recent-branches list (needs Épico 9's persistence store, which doesn't exist yet).
- **19 — Workspace Multi-Repo**
- 🚧 **20 — Resumo Copiável** — `Y` copies a paste-ready plain-text MR summary (IID/title, project, branches, author, status, pipeline, approvals, conflicts, URL) to the clipboard, available wherever an MR is in context (dashboard + detail + diff/pipeline/log/discussions). Shares the clipboard path with `y` (copy link) via `copyToClipboardCmd`. **Deferred:** Markdown/Slack-flavored variants and including diff stats or the discussion count.
- **21 — Cache Local**

## Fase 4/5 and polish (backlog)

- **22 — UI Base** (multi-panel layout, themes, mouse) — current TUI is a single-panel screen sequence, not the three-area layout from §4 of the plan; revisit once there's enough screens to justify it.
- **23 — Atalhos** — most of the plan's suggested table is bound now (global: `o`/`y`/`Y`/`r`/`m`/`q`/`esc`/`j`/`k`/`enter`; diff: `h`/`l`/`[`/`]`/`/`/`n`/`N`/`s`/`w`/`e`; pipeline: `R`/`P`/`t`/`x`; job log: `/`/`n`/`N`/`e`/`E`/`R`; MR detail: `c`/`a`/`A`/`w`/`M`/`b`/`x`; discussions: `c` (compose)/`t` (resolve)/`j`/`k`/`g`/`G`). Deliberately kept `r` = refresh everywhere instead of the plan's per-screen "r = retry job" suggestion, to stay consistent across screens — see BACKLOG notes on 14/15. Still missing: `?` help overlay, `Tab`/`Shift+Tab` panel switching (no multi-panel layout yet — see 22). Comment shortcuts landed with Épico 16 (`c`).
- **24 — CLI Complementar** (`mr list`, `mr view`, `mr checkout`, `pipeline list/logs` as standalone commands, not just inside the TUI).
- **26 — Distribuição** (cross-platform binaries, releases, install script).

## Explicitly future (per the plan, not re-scoped here)

- **Issues e Boards (MVP 7)** — plan §2.3 explicitly puts this after MVPs 1-6.
- Command palette, AI features — plan §10, explicitly not MVP.
