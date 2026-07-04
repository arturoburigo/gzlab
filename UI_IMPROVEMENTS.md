# TUI Style & UX Improvement Areas

Audit of `internal/tui` (2026-07-03), prompted by three complaints: the dashboard is
information-sparse with lots of empty space, styling in general is unpolished, and
the diff viewer looks broken. 65 findings were raised by a seven-area code audit and
each was independently verified against the code and `BACKLOG.md`; 63 were confirmed
and deduplicated into the 51 items below. Every item carries a `file:line` anchor,
an impact/effort rating, and whether it is a rendering **bug** or a **design** weakness.

## TL;DR — why it currently feels broken

Four root causes explain most of what you see:

1. **`truncate()` slices styled strings by byte** (`view.go:157`). Every screen funnels
   overflowing lines through it via `clampBlock`, so ANSI escapes get cut mid-sequence:
   colors/backgrounds bleed across the screen, multibyte text turns into `�`, and lines
   stop short of the pane border. This is the #1 "diff viewer is broken" cause.
2. **Screens size themselves against the wrong geometry.** The diff view budgets
   `m.width-10` but the frame only gives it `m.width-25`; `contentHeight()` disagrees
   with the frame by a row; the pipeline job list overflows its pane. Content is then
   mangled by the byte-truncate above.
3. **Tabs are never expanded.** `lipgloss.Width` counts `\t` as 0 columns, but the
   terminal renders it 4–8 wide. Go diffs and CI logs are full of tabs, so all width
   math (clamping, side-by-side padding, pane borders) breaks on real content.
4. **The dashboard (and detail/list) ignore `m.width`/`m.height` entirely** — fixed
   narrow columns, fixed 5-row caps, hardcoded 44/48/72-column widths. On a modern
   terminal ~80% of the pane is blank.

## Start here — top 8 by leverage

| # | Item | Anchor | Kind | Effort |
|---|------|--------|------|--------|
| 1 | [C1](#c1) Replace byte-based `truncate()` with ANSI/unicode-aware truncation | `view.go:157` | bug | small |
| 2 | [F1](#f1) Diff width budget ignores the frame chrome | `diff_view.go:286` | bug | small |
| 3 | [F2](#f2) Expand tabs at diff parse time | `diff_view.go:212` | bug | small |
| 4 | [S1](#s1) Remove `surface` background from container styles (background bleed) | `styles.go:104` | bug | small |
| 5 | [S3](#s3) Default "dark" theme is the neon-green CRT palette | `styles.go:70` | design | small |
| 6 | [C2](#c2) Stop wiping the screen to a full-page "Loading..." on every action | `view.go:14` | design | small |
| 7 | [D1](#d1) Selected recent-branch row loses its highlight (nested ANSI reset) | `dashboard_view.go:51` | bug | small |
| 8 | [L1](#l1) List screens have no scroll window — selection vanishes below the fold | `list_view.go:113` | bug | medium |

Items 1–5 are all small, independent, and together fix most of "the diff viewer is
broken" and "the styles look off". They are a natural first slice.

---

## 1. Cross-cutting rendering core (chrome)

<a id="c1"></a>
### C1. Replace byte-based `truncate()` with ANSI/unicode-aware truncation
`internal/tui/view.go:157` · **bug** · impact high, effort small

`truncate` checks `len(s)` and cuts with `s[:n-1]`, but `clampBlock` (view.go:414) and
`spread` (view.go:387/394) call it on *styled* strings after detecting overflow with
`lipgloss.Width`. The SGR prefix (~16 bytes) eats the width budget and the closing
`\x1b[0m` is dropped — colors/backgrounds bleed into the rest of the line and screen —
and multibyte runes (accented titles, `─` rules) are cut mid-rune into `�` mojibake.
It also byte-truncates plain UTF-8 at `headerView` (branch names), `list_view.go:120`
(MR titles), `dashboard_view.go:53`, and `diff_view.go:500` (`clampStyledLine`).
Latent panic: `truncate(s, 0)` evaluates `s[:-1]`, and `spread` can pass 0.

**Fix:** make `truncate` a thin wrapper over `ansi.Truncate(s, n, "…")` from
`github.com/charmbracelet/x/ansi` (already an indirect dependency) and guard `n <= 0`.
One change fixes every call site at once.

<a id="c2"></a>
### C2. Stop wiping the whole screen to a static "Loading..." page on every action
`internal/tui/view.go:14` · **design** · impact high, effort small

`m.loading` is set on ~25 transitions (open list/diff/pipeline/log, cycle scope with
`f`, refresh after comment/MR/pipeline actions), and `View()` then replaces everything
with a bare "Loading" frame for the duration of a 1–3s `glab` call — a jarring blank
flash between every screen, with no spinner, losing list rows and cursor context.

**Fix:** render the full-screen loading frame only when there is nothing to show yet
(`m.dash == nil`); otherwise keep the current screen and indicate loading via the
existing header-right slot or the footer status (optionally a 3-frame `tea.Tick`
spinner).

### C3. Non-fatal command errors nuke the entire UI into a full-screen Error page
`internal/tui/view.go:16` · **design** · impact high, effort medium

Every `errMsg` — including "saving log", "opening editor", or one failed poll — sets
`m.err`, which replaces the whole interface with an Error frame. `m.err` is only ever
cleared in the `dashboardLoadedMsg` handler (model.go:145), so the page can persist
even after successful non-dashboard refreshes.

**Fix:** route recoverable errors into the footer status rendered with `errorStyle`,
keeping the current screen; reserve the full-screen error frame for boot failure.

### C4. Overlays are force-padded to max height
`internal/tui/view.go:226` · **design** · impact medium, effort small

`withOverlay` clamps the modal body with `clampBlock`, which *pads* to `contentH` — a
3-line confirm dialog renders as a ~18-row mostly-empty box.

**Fix:** for overlays, clamp width and cap height without padding (split `clampBlock`
into clamp-width + cap-height) so the box shrinks to its content.

### C5. Footer hints overflow silently and `?` help is advertised only on the dashboard
`internal/tui/view.go:184` · **design** · impact medium, effort small

`renderHints` drops hints from the first that doesn't fit, with no `…` marker — and
since `ctrl+k commands` is appended last it is the first casualty. The `?` help hint
appears only in `dashboardHints`, so 9 of 10 screens never mention help exists.

**Fix:** append `hint{"?", "help"}` in `screenHints` for all screens; reserve room for
a trailing `…` (or right-anchor `? help`) so truncation is visible and the
help/palette keys always survive.

### C6. Nav mode is enterable while the sidebar is invisible below 120 cols
`internal/tui/view.go:252` · **design** · impact medium, effort medium

The nav sidebar renders only at `width >= 120`, but `navActive` still toggles on
narrower terminals — the user navigates an invisible menu with only the footer text as
feedback. 120 is also a high bar for an 18-col sidebar.

**Fix:** when the sidebar is collapsed and nav mode is active, render a one-line
horizontal nav strip under the header (`Dashboard · MRs · Pipeline · …` with the
cursor item in `sidebarSelectedStyle`); consider lowering the threshold to ~100.

### C7. Section rules are hardcoded to 48/56/72 columns with mixed glyphs
`internal/tui/diff_view.go:299` (pattern; also list 102, dashboard 16, discuss 112, log 99, view 93/116, commits 18, detail 15, pipeline 18, search 13, workspace 25) · **design** · impact low, effort small

Title underlines are fixed-width (72/48/56) so they stop mid-pane on wide terminals,
and search/workspace draw ASCII `-` while every sibling screen draws `─`.

**Fix:** one shared `rule(w int)` helper sized from the pane's real inner width (see
C9), used everywhere with the same glyph.

### C8. Wide layout is one column short with a double-border gutter mid-screen
`internal/tui/view.go:254` · **design** · impact low, effort small

`mainW = width-sideW-5` makes sidebar+pane total `width-1` (ragged right edge vs the
full-width header/footer), and two adjacent rounded borders + padding form a ~4-column
dead `││` gutter down the middle.

**Fix:** `mainW = width-sideW-4`, and drop the sidebar's own border in favor of a
single divider (e.g. `BorderLeft` on the pane only), reclaiming 2–3 content columns.

### C9. `contentHeight()` disagrees with the pane's real inner height
`internal/tui/view.go:438` · **design** · impact low, effort small

The frame gives screens `m.height-4` inner rows, but `contentHeight()` returns
`m.height-5`, leaving a permanently dead row on screens that size from it; siblings
`jobListHeight`/`logBodyHeight` are further eyeballed constants.

**Fix:** derive `contentHeight` from the same expression as `frameView`
(`m.height-4`), define the sibling helpers relative to it, and add the missing
**`contentWidth()`** twin (mirroring `mainPaneView`: `m.width-25` when ≥120, else
`m.width-4`) — F1, L2, and C7 all need it.

---

## 2. Diff viewer

<a id="f1"></a>
### F1. Width budget ignores the frame chrome — content overshoots the pane every render
`internal/tui/diff_view.go:286` · **bug** · impact high, effort small

`diffWidth := max(30, m.width-sidebarWidth-10)` never accounts for the 18-col nav
sidebar + borders that `mainPaneView` takes (real budget at ≥120 cols: `m.width-25`).
At 160 cols the diff composes ~152 columns into a 135-column pane, so `clampBlock`
byte-truncates every long line — cut ~17 columns early, with ANSI-bleed artifacts
(C1) on each.

**Fix:** compute from the frame's real budget: `diffWidth := max(30, m.contentWidth()
- sidebarWidth - 2)` using the new `contentWidth()` helper (C9).

<a id="f2"></a>
### F2. Expand tabs at parse time — tabs measure 0 columns in all width math
`internal/tui/diff_view.go:212` · **bug** · impact high, effort small

Diff lines keep raw `\t` through layout: `lipgloss.Width` counts a tab as 0, so
`clampBlock` never truncates tab-indented lines and `sideBySideCell` over-pads them;
only the outermost `paneStyle.Render` expands tabs (4 cols). For a Go repo — every
indented line — context lines come out 4–8+ columns wider than budgeted: the bordered
pane grows an extra row per offending line (stair-stepped frame) and the side-by-side
`│` separator zigzags.

**Fix:** normalize once in `splitDiffLines`: `raw = strings.ReplaceAll(raw, "\t",
"    ")` before splitting, so measurement, truncation, padding, and rendering agree.
(Same fix for job logs — see P5.)

### F3. Side-by-side toggle silently no-ops below ~194 terminal columns
`internal/tui/diff_view.go:301` · **bug** · impact medium, effort small

Pressing `s` always shows the `[side-by-side]` badge, but the split layout only
engages when `diffWidth >= 150` — i.e. `m.width >= 194`, wider than almost any real
terminal. The user toggles it, the badge flips on, and nothing changes: it looks like
a broken feature.

**Fix:** lower the threshold (e.g. `diffWidth >= 100` → ~48-col cells) and, when the
toggle is on but the terminal is too narrow, say so in the badge or `m.status` so the
state shown always matches the layout drawn.

### F4. `sideBySideCell` compares byte length against column width
`internal/tui/diff_view.go:490` · **bug** · impact medium, effort small

`if len(truncated) > colWidth { truncate(...) }` — bytes vs display columns. Non-ASCII
cells (accented strings, CJK, emoji) get truncated too early and can be sliced
mid-rune into `�` at the cell edge, desyncing the separator. `fitDiffCell` (:507) and
`clampStyledLine` (:500) share the pattern.

**Fix:** width-based check and cut: `if lipgloss.Width(raw) > colWidth { ansi.Truncate(raw,
colWidth, "…") }` in all three helpers.

### F5. Add an old/new line-number gutter to the unified diff
`internal/tui/diff_view.go:589` · **design** · impact medium, effort medium

Lines render as raw `+foo`/`-bar` with no line numbers; the only positional cue is
the raw `@@` header. Reviewers can't tell what line a change lands on, and the flat
left edge makes +/- runs hard to scan — a big part of why the viewer reads as
unpolished next to delta/GitHub.

**Fix:** parse each `@@ -o,+n` header and carry old/new counters (from the start of
`file.lines`, since rendering iterates only the viewport slice); prefix each row with
a fixed-width dim gutter like `%4s %4s `, and shrink the code width by the gutter.

### F6. Highlight the matched substring on all matches, not the whole active line
`internal/tui/diff_view.go:583` · **design** · impact medium, effort small

Only the single active match gets styling — a solid warn-background bar over the whole
line that destroys the +/- coloring; the other N−1 visible matches are
indistinguishable from normal lines, so "Match 3/17" gives no on-screen context.

**Fix:** in `renderDiffLine`, splice `searchStyle` around just the matched span inside
the already diff-colored line; use a dimmer variant for non-active matches.

### F7. File sidebar truncates the tail of paths, hiding the filename
`internal/tui/diff_view.go:331` · **design** · impact medium, effort small

Paths are tail-truncated in an 18–34-col sidebar, so deep paths in the same package
render identically (`internal…`, `internal…`) — the basename, the one identifying
part, is exactly what gets cut.

**Fix:** a `truncatePathLeft` helper that keeps the last segments prefixed with `…`
(`…/diff_view.go`); optionally color the `+N −N` stats with addition/deletion styles.

---

## 3. Dashboard

<a id="d1"></a>
### D1. Selected recent-branch row loses its highlight (nested ANSI reset)
`internal/tui/dashboard_view.go:51` · **bug** · impact high, effort small

`dashGutter(true)` returns `cursorStyle.Render("▶ ")` — a string ending in `\x1b[0m` —
which is then wrapped in `selectedStyle.Render(...)`. The embedded reset terminates
the selection background right after the marker, so only the arrow is highlighted and
the selection is nearly invisible on the dashboard's main interactive element.

**Fix:** never nest a pre-styled fragment inside another `Render`; build the plain
string and style once (`selectedStyle.Render(cursorMarker + text)` for the selected
row).

### D2. Lay out dashboard cards responsively instead of one narrow top-left column
`internal/tui/dashboard_view.go:13` · **design** · impact high, effort medium

`renderDashboard` never references `m.width`/`m.height`: on a 200×50 terminal the
~60-col, ~20-line column leaves ~80% of the pane blank — the literal "a lot of empty
space" complaint. Recent branches and Recent projects stack vertically though each is
≤6 rows × ~50 cols.

**Fix:** pass the pane's inner width in; at ≥~90 cols render the two recent cards
side-by-side with `lipgloss.JoinHorizontal` (each `Width(colW)`), optionally the
context block + MR card too; keep the stacked layout as the narrow fallback. All data
is already in the model.

### D3. Surface already-fetched MR fields on the dashboard MR card
`internal/tui/dashboard_view.go:29` · **design** · impact high, effort small

The current-branch MR card shows only title/status/pipeline/approvals, while
`m.dash.MergeRequest` already carries author, source→target branches, conflicts,
labels, and timestamps — the copyable `Y` summary (summary.go) prints more than the
screen does.

**Fix:** add `kv` rows for author, branches, updated-relative-time; an `errorStyle`
conflict warning; labels in `labelStyle`. Zero new API calls.

### D4. Size the recent lists from available height instead of a fixed cap of 5
`internal/tui/dashboard_view.go:11` · **design** · impact medium, effort small

`dashRecentLimit = 5` while the history store keeps 10; entries 6–10 are invisible
*and* unreachable (`dashMaxCursor` clamps to the same cap), with no "N more"
indicator, on a screen full of blank rows.

**Fix:** derive the per-card limit from remaining `contentHeight()`; when still cut
off, append a muted `3 more…` line (the command palette already does this at
view.go:137).

### D5. Show recency metadata on the recent-branches/projects cards
`internal/tui/dashboard_view.go:73` · **design** · impact medium, effort small

"Recent" cards show no recency: `history.Branch.LastAccess`, `Project.LastAccess`,
and `Project.Name` are persisted but never rendered; rows are visually flat.

**Fix:** a small `relTime()` helper ("2h ago") right-aligned per row via the existing
`spread()`; show `p.Name` with the path muted.

### D6. Stop duplicating the header's project/branch rows on the dashboard body
`internal/tui/dashboard_view.go:18` · **design** · impact low, effort small

Two of the three context rows repeat what the global header already shows on every
screen.

**Fix:** keep only the profile row (optionally move it into the header's right side,
or show the profile host, which is currently invisible everywhere); reclaim the rows
for the enriched MR card.

---

## 4. Styles & theming

<a id="s1"></a>
### S1. Remove `surface` Background from container styles — mid-line background bleed
`internal/tui/styles.go:104` · **bug** · impact high, effort small

`paneStyle`/`sidebarStyle`/`overlayStyle` set `.Background(palette.surface)`, but pane
content is composed of nested styles whose SGR reset kills the outer background
mid-line: plain text following any styled fragment drops back to the terminal default
while leading text and trailing padding keep the surface tint — every screen looks
patchy/striped with any OC theme (where surface ≠ terminal bg).

**Fix:** drop `.Background(palette.surface)` from the three container styles (both
copies — see S5). Surface is only a 4% tint; nothing meaningful is lost. If a real
surface tint is wanted later, apply it per fully-styled line, not on a container
wrapping nested styles.

### S2. Hardcoded `Foreground("0")` on selection/logo/search styles breaks light themes
`internal/tui/styles.go:102` · **bug** · impact high, effort medium

Five styles (`selectedStyle`, `logoStyle`, `sidebarSelectedStyle`, `searchStyle`,
`overlaySelectedStyle`) force ANSI color 0 (terminal black) over theme-colored
backgrounds. On light themes those accents are *dark* (light accent `25`,
gruvbox-light `#076678`, tokyonight-light `#2e7de9`) → black-on-dark-blue selections,
nearly invisible. ANSI 0 is also user-remappable.

**Fix:** add an `onAccent` field to `themePalette`, computed from accent luminance
(the `parseHex` helper already exists in themes.go), and use it in the five styles.

<a id="s3"></a>
### S3. Default "dark" theme is a neon-green CRT palette, unreadable on light terminals
`internal/tui/styles.go:70` · **design** · impact high, effort small

Out of the box (`config.Default()` → `theme: dark`) the app renders in saturated neon
green (`#00ff66` accent, `#3cff87` "muted" — *brighter* than body text, inverting
hierarchy). None of the legacy named themes consult `lipgloss.HasDarkBackground()`,
so on a light terminal the near-white text is unreadable — only the OC branch adapts.

**Fix:** reroute the `"dark"` case (and the default fallthrough) to an adaptive OC
theme (e.g. `lookupOCTheme("opencode")` or `"github"`); keep the CRT palette
reachable via the explicit `terminal`/`retro`/`crt` names. One-line change.

### S4. Diff hunk headers (and pipeline stage headers) bypass the theme with hardcoded color 39
`internal/tui/styles.go:110` · **design** · impact medium, effort small

`hunkStyle` is the single non-palette color in the style table — always DeepSkyBlue
regardless of theme, reused by pipeline stage headers. Theme switching visibly fails
to restyle those two screens. Meanwhile `ocPalette.info` exists per theme but is
dropped by `ocToPalette`.

**Fix:** add `info` to `themePalette`, populate from `firstHex(p.info, p.accent,
p.primary)` (and per legacy palette), and use `palette.info` in `hunkStyle`;
optionally give stage headers their own palette-driven style.

### S5. The entire 34-style table is defined twice
`internal/tui/styles.go:128` · **design** · impact medium, effort small

Every style exists as a var-block initializer (128–166) *and* a `rebuildStyles()`
assignment (94–126), hand-synchronized. Miss one and the app renders differently
before vs after a theme switch — a silent drift hazard (and the var copies are live:
`applyTheme` only runs when config is present).

**Fix:** declare the vars uninitialized and add `func init() { rebuildStyles() }` —
deletes ~35 duplicated lines, single source of truth.

### S6. `secondary` falls back to a color equal to `primary` on several themes
`internal/tui/themes.go:26` · **design** · impact low, effort small

For themes where `accent == primary` (Cursor, Mercury dark), footer keys, table
heads, sidebar selection, discussion headers, and the cursor all collapse into the
title color — the two-accent hierarchy silently disappears.

**Fix:** in `ocToPalette`, skip candidates equal to `p.primary` when picking
`accent2` so those themes fall through to their `info` color.

### S7. `palette.subtle` is computed by every theme but never used
`internal/tui/styles.go:16` · **design** · impact low, effort small

The only low-emphasis background slot is dead weight, while every selection uses
harsh full-accent inverse video — the loudest possible row-cursor treatment.

**Fix:** either use it (e.g. `selectedStyle` with `Background(palette.subtle)` for a
calmer row highlight, keeping accent inverse for sidebar/overlay picks) or delete the
field.

### S8. Legacy `dracula` case is unreachable and diverges from OC Dracula
`internal/tui/styles.go:58` · **design** · impact low, effort small

`lookupOCTheme` wins first, so `case "dracula", "purple"` only fires for `purple` — a
different, non-adaptive palette. Dead code plus two different Dracula looks by alias.

**Fix:** delete the legacy palette and alias `purple` in `lookupOCTheme` (like the
existing `tokyo` alias).

---

## 5. MR list, search, workspace

<a id="l1"></a>
### L1. No scroll window — the selected row vanishes below the fold
`internal/tui/list_view.go:113` · **bug** · impact high, effort medium

`renderList` prints every MR unconditionally; `clampBlock` hard-clips to the pane. On
a list longer than ~`height-7` rows, pressing `j` moves the cursor onto rows that were
cut off: the highlight disappears and Enter opens an MR the user can't see. Same
defect in `search_view.go:31` and `workspace_view.go:30`.

**Fix:** window rows around the cursor exactly like `renderCommandPalette` already
does (view.go:121–127), with `↑/↓ N more` counters; extract as a shared helper and
apply to all three screens.

### L2. Compute MR list columns from terminal width instead of hardcoding 44/72
`internal/tui/list_view.go:103` · **design** · impact high, effort medium

Title column frozen at 44 cells, rule at 72 — on a 180-col terminal the table uses
~60 of ~155 pane cells, a huge dead band. `%-44s` also pads by runes, misaligning
CJK/emoji titles.

**Fix:** size columns from `contentWidth()` (C9); pad display-width-aware with
`lipgloss.NewStyle().Width(titleW).MaxWidth(titleW).Render(title)`.

### L3. Enrich MR rows — author, branches, age, colored state are fetched but never shown
`internal/tui/list_view.go:120` · **design** · impact high, effort medium

Rows show only IID/title/plain-text state; `mapping.go` already populates
SourceBranch/Author/UpdatedAt/Labels for every listed MR. `opened`/`merged`/`closed`
are visually identical, draft is a dangling suffix.

**Fix:** author + relative-age columns (and `source→target` on wide terminals); color
the state (good=merged, bad=closed); render draft as a styled `DRAFT` tag.

### L4. "pipeline failing" quick filter can never match
`internal/tui/list_view.go:84` · **bug** · impact medium, effort small

The filter tests `mr.Pipeline`, which is nil for every MR from the list endpoints
(`types.go:55`: "Only populated by GetMergeRequest") — cycling `F` to it always shows
the empty state. A silently dead feature.

**Fix (minimal honest):** when the filter is active and no loaded MR has pipeline
data, say "Pipeline status is not loaded for the list view." — or skip the filter in
`next()` until head pipelines are backfilled via a `tea.Cmd` fan-out.

### L5. Extend the selection highlight to the full row width
`internal/tui/list_view.go:124` · **design** · impact medium, effort small

`selectedStyle` is applied to the bare text, so the highlight band ends wherever the
row's text ends — ragged selection bars (also search/workspace).

**Fix:** `style.Width(contentW).Render(line)` for a solid full-width bar.

### L6. Show a text cursor in the search input when editing
`internal/tui/search_view.go:18` · **design** · impact medium, effort small

Input mode changes only the prompt text; no insertion marker, so the field doesn't
read as focused — while every keystroke *also* echoes into the footer status,
duplicating the query.

**Fix:** append `cursorStyle.Render("▌")` after the input when active, use
`promptMarker`, drop the footer echo. (Longer term: `bubbles/textinput`.)

### L7. Workspace rows: unbounded paths break the grid; status is one gray blob
`internal/tui/workspace_view.go:47` · **design** · impact low, effort small

`%-28s` is a minimum — long project paths push title/status right for that row only;
the whole status string (`opened pipeline:failed approvals:1/2`) renders in muted
`footerStyle`, so a failed pipeline doesn't stand out — the one signal the overview
exists to show.

**Fix:** truncate the path like the title; color pipeline via the existing
`pipelineStyle()` and approvals via good/warn, keeping only labels muted.

---

## 6. MR detail, discussions, commits

### T1. Word-wrap discussion note bodies to pane width
`internal/tui/discuss_view.go:73` · **bug** · impact high, effort medium

GitLab comments are long single paragraphs; bodies are emitted verbatim and clipped to
one ellipsis line — a 300-char review comment shows ~90 chars with no way to read the
rest. Lines are pre-rendered in `newDiscussState` with no width, so nothing reflows on
resize.

**Fix:** wrap at render time with `ansi.Wordwrap(body, width-indent, "")`, counting
wrapped rows for the header-line bookkeeping (or rebuild on `tea.WindowSizeMsg`).

### T2. Move comment compose out of the footer status line
`internal/tui/model.go:1265` · **bug** · impact high, effort medium

Composing types into the right-aligned footer status: `alt+enter` inserts a raw `\n`
into `m.status`, growing the frame past the terminal height (header scrolls off); and
`footerView` truncates the status to `width/2` keeping the *head*, so past ~half a
screen of text the user types blind.

**Fix:** when composing, render a dedicated full-width input line in place of the
hints row (tail-anchored slice, `⏎` for newlines) or reuse `withOverlay` as a compose
modal. At minimum show the tail and strip newlines.

### T3. Densify the MR detail screen: description, labels, dates
`internal/tui/detail_view.go:14` · **design** · impact high, effort medium

Detail is a flat 5-row label:value dump in a ~28-row pane — >80% empty. The MR
description is not shown anywhere in the app; Labels/CreatedAt/UpdatedAt are mapped
but never rendered.

**Fix:** add labels + created/updated rows (zero cost); map the SDK's `Description`
in `internal/gitlab/mapping.go` (same payload, no extra call) and render it dimmed,
word-wrapped, clipped to remaining height.

### T4. Dim or collapse resolved discussion threads
`internal/tui/discuss_view.go:96` · **design** · impact medium, effort small

Resolved vs open differs by a 2-char glyph only; on a reviewed MR the screen is a wall
of equally bright text. Non-resolvable threads get no marker, so author columns
misalign by 2 cells.

**Fix:** render resolved threads through `footerStyle`; return a 2-space placeholder
for non-resolvable threads; optionally collapse resolved bodies to one line.

### T5. Show file:line context for positioned diff comments
`internal/tui/glab.go:121` · **design** · impact medium, effort medium

Code-anchored comments render identically to general ones — the `position` object
(`new_path`/`new_line`) is in the fetched JSON but the parser drops it, forcing the
browser to understand any positioned thread.

**Fix:** parse `position.new_path`/`new_line` into the discussion and emit one dim
`file:line` row under the thread header when present.

### T6. Column-align commit rows and truncate titles before the metadata
`internal/tui/commits_view.go:29` · **design** · impact medium, effort small

SHA + title + `(author · date)` are one run — long titles push author/date off the
pane and `clampBlock` chops the tail, so long-titled commits show no metadata at all.
The screen is also scroll-only while the sibling discussions screen has a `▶` cursor.

**Fix:** fixed columns (8-char SHA, width-aware truncated title, right-aligned
`author · date`); reuse the discussions gutter pattern for a row cursor.

### T7. Drop duplicated in-body titles and size rules to the pane
`internal/tui/commits_view.go:17` (also discuss_view.go:111, detail_view.go:15) · **design** · impact medium, effort small

All three screens spend their top two rows repeating the frame header ("Commits !42"
header + "Commits on !42" body title + 72-col rule).

**Fix:** remove the redundant body title+rule from commits/discussions and reclaim the
rows; where a rule stays, size it via the shared helper (C7).

---

## 7. Pipeline & job log

### P1. Pipeline job list overflows the pane; selection can scroll out of view
`internal/tui/view.go:445` (`jobListHeight`) · **bug** · impact high, effort medium

`jobListHeight = m.height-9` ignores the metadata block's variable height (5–8 kv
rows) and the stage-header + blank lines emitted *inside* the job loop; at
`height=30` with 3 stages, ~37 body lines go into a 26-line pane. `ensureJobVisible`
uses the same oversized number, so `j` walks the highlight into clipped rows.

**Fix:** budget from what is actually emitted — count metadata + stage lines while
rendering and stop at `contentHeight()`, using the same number in `ensureJobVisible`.

### P2. Auto-refresh clobbers navigation state
`internal/tui/model.go:188` · **bug** · impact high, effort small

The 10s pipeline poll unconditionally resets `jobCursor/jobOffset = 0` — selection
jumps to the top mid-navigation. `jobLogLoadedMsg` rebuilds `newLogState`, so `r`
loses the user's place in a 20k-line log and every 5s follow tick silently erases the
active search query and match counter.

**Fix:** when the reloaded pipeline/job IDs match the current ones, carry
cursor/offset (clamped) and re-run the previous search across the refresh.

### P3. Color-code job status in the job list — failed jobs are not red
`internal/tui/pipeline_view.go:59` · **design** · impact high, effort small

Every job row renders in uniform bold text; the status color language exists but only
on stage headers, so a failed job in a 20-job stage looks identical to a passing one.

**Fix:** pad first, then style the status cell (`fmt` pads by bytes, so style *after*
`%-11s`) with the existing `renderJobStatus` colors; optionally add a glyph
(`✔ ✖ ● ⏸`) inside the cell. Keep the selected row fully `selectedStyle`.

### P4. ANSI stripping regex misses private CSI and OSC sequences
`internal/tui/log_view.go:82` · **bug** · impact medium, effort small

`\x1b\[[0-9;]*[a-zA-Z]` doesn't match `\x1b[?25l` (cursor hide), OSC titles/hyperlinks
(`\x1b]0;...\x07`), or BEL — viewing an npm/docker trace can literally hide the
terminal cursor, retitle the window, beep, or render garbage.

**Fix:** `ansi.Strip(raw)` from `charmbracelet/x/ansi` (handles CSI private markers +
OSC), and drop other C0 control bytes when splitting.

### P5. Expand tabs in log lines
`internal/tui/log_view.go:63` · **bug** · impact medium, effort small

Same class as F2: Go panics and Makefile output are full of tabs; `clampBlock`
under-measures them, `paneStyle.Render` then expands and wraps the over-wide line,
growing the frame or chopping the bottom border.

**Fix:** normalize tabs in `splitLogLines`.

### P6. Long log lines are chopped with no wrap; only the active search match highlighted
`internal/tui/log_view.go:107` · **design** · impact medium, effort large

Lines >200 cols are truncated with `…` — the tail (where the error message lives) is
unreachable on screen. Non-active search matches get no styling; the active match is a
whole-line bar.

**Fix:** soft-wrap the visible slice at render time (`ansi.Wrap`), counting wrapped
rows against the height budget; highlight the matched substring in every visible
matching line, dimmer for non-active.

### P7. No visible refresh feedback while polling
`internal/tui/pipeline_view.go:14` · **design** · impact medium, effort medium

`[auto-refreshing]`/`[following]` are static tags; no fetch timestamp is stored,
nothing moves between 10s/5s ticks — a slow `glab` call looks frozen.

**Fix:** record `time.Now()` in the loaded-msg handlers and render `[auto-refreshing ·
updated 15:04:05]`; optionally a small braille spinner on a 300ms tick guarded by the
existing `pollGen` pattern while active.

---

## Verified but intentionally deferred (do not "fix" without deciding to un-defer)

- **Modal compositing blanks the full-width band behind the overlay**
  (`view.go:242-246` rebuilds each overlay row as spaces + box + padding, discarding
  the base row — on wide terminals modal rows render as blank full-width stripes that
  break the pane/sidebar borders). Confirmed real, but `BACKLOG.md` Épico 22
  explicitly defers "true modal overlays". Worth revisiting once C4 lands, since the
  two touch the same code.

Rejected in verification (intentional design, not a gap): always showing the
"press m / /" hint in the no-MR empty state — BACKLOG Épico 7 deliberately scopes that
hint to first run, and the footer already shows those keys permanently.

## Not covered by this audit — candidate future polish

- **Syntax highlighting inside diff hunks** (e.g. `alecthomas/chroma` keyed by file
  extension) — the single biggest visual upgrade after the correctness fixes, but a
  new dependency and a deliberate scope decision.
- **Adopting `bubbles` components** (`viewport`, `textinput`, `spinner`, `table`)
  instead of the hand-rolled equivalents — would delete much of the scroll/input code
  this document patches (L1, L6, P6, T2), at the cost of a bigger refactor.
- **Color-blind safety**: added/removed rely on green/red plus the `+`/`-` prefix;
  the prefix keeps it usable, but the line-number gutter (F5) and status glyphs (P3)
  further reduce color dependence.

## Suggested slices (smallest first)

1. **Rendering core** — C1, F2/P5 (tabs), S1, S5, C9 (`contentWidth`/height unify).
   Small, independent, all bugs; fixes most of "broken".
2. **Diff correctness** — F1, F3, F4, D1, P4. The diff viewer now *works*.
3. **Default look** — S3, S2, S4, C7, C8, L5. The app now looks coherent out of the box.
4. **Dashboard density** — D2, D3, D4, D5, D6 (+ C2 loading). The dashboard now earns
   its screen space.
5. **List/detail density** — L1, L2, L3, T3, T7, P3.
6. **Interaction polish** — P1, P2, T1, T2, C3, C5, remainder.

Each slice is independently shippable and testable (`go test ./internal/tui/` — the
render functions are pure string builders, so every fix here can get a focused test).
