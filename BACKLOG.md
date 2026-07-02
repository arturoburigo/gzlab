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
| 25 — Testes | 🚧 | Unit tests alongside every package built this session (config, gitlab mapping, gitdetect — including real throwaway git repos, dashboard with a mock client, TUI `Update`/`View` with injected messages). **Deferred:** CLI-level integration tests, diff/log parser tests (those parsers don't exist yet). |

Untouched this session, all tasks from the plan doc still apply as written:

- **8 — Projetos Recentes** — backlog, needs a local history store keyed by profile.
- **9 — Branches Recentes** — backlog, same as above plus MR association.
- **10 — Busca Global** — backlog.

## Fase 2 — Valor real para review (backlog)

- **13 — Diff Viewer** — side-by-side/unified rendering, hunk navigation, search.
- **14 — Pipeline da MR** — stage/job breakdown beyond the current one-line status, retry/cancel.
- **15 — Logs de Job** — log viewer, search, jump-to-error, retry, follow mode.

## Fase 3 — Histórico e produtividade (backlog)

- **16 — Discussões e Comentários**
- **17 — Ações de MR** (approve, draft/ready, merge, checkout)
- **18 — Checkout de Branch/MR**
- **19 — Workspace Multi-Repo**
- **20 — Resumo Copiável**
- **21 — Cache Local**

## Fase 4/5 and polish (backlog)

- **22 — UI Base** (multi-panel layout, themes, mouse) — current TUI is a single-panel screen sequence, not the three-area layout from §4 of the plan; revisit once there's enough screens to justify it.
- **23 — Atalhos** — current bindings (`o`/`y`/`r`/`m`/`q`/`esc`/`j`/`k`/`enter`) are a subset of the plan's full shortcut table; the rest depend on features not built yet (diff, pipeline, comments).
- **24 — CLI Complementar** (`mr list`, `mr view`, `mr checkout`, `pipeline list/logs` as standalone commands, not just inside the TUI).
- **26 — Distribuição** (cross-platform binaries, releases, install script).

## Explicitly future (per the plan, not re-scoped here)

- **Issues e Boards (MVP 7)** — plan §2.3 explicitly puts this after MVPs 1-6.
- Command palette, AI features — plan §10, explicitly not MVP.
