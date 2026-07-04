package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gzlab/internal/gitlab"
	"github.com/arturoburigo/gzlab/internal/history"
)

// dashCardsSideBySideMinWidth is the content width at which the recent
// branches/projects cards render as two columns instead of stacking — each
// card is at most ~50 columns wide, so anything reasonably modern-terminal
// sized can fit both.
const dashCardsSideBySideMinWidth = 90

func (m Model) renderDashboard() string {
	width := m.contentWidth()
	var b strings.Builder
	b.WriteString(titleStyle.Render("GitLab TUI") + "\n")
	b.WriteString(rule(min(48, width)) + "\n\n")
	b.WriteString(kv("profile", valueStyle.Render(m.dash.ProfileName)) + "\n")

	if m.dash.MergeRequest == nil {
		b.WriteString("\n" + footerStyle.Render("No open merge request for this branch.") + "\n")
		if len(m.recentBranchItems()) == 0 && len(m.recentProjectItems()) == 0 {
			b.WriteString(footerStyle.Render("Press 'm' to browse this project's merge requests, or '/' to search across GitLab.") + "\n")
		}
	} else {
		b.WriteString("\n" + m.renderDashboardMRCard() + "\n")
	}

	if len(m.dashCommits) > 0 || len(m.dashMRs) > 0 {
		stats := m.renderCardPair(width,
			func(w int) string { return m.renderRecentCommitsCard(w) },
			func(w int) string { return m.renderMRStatsCard(w) },
		)
		if stats != "" {
			b.WriteString("\n" + stats + "\n")
		}
	}

	if assigned := m.renderAssignedMRsCard(width); assigned != "" {
		b.WriteString("\n" + assigned + "\n")
	}

	if strip := m.renderContributionSplitCard(width); strip != "" {
		b.WriteString("\n" + strip + "\n")
	}

	limit := m.dashCardRowLimit()
	recent := m.renderCardPair(width,
		func(w int) string { return m.renderRecentBranchesCard(w, limit) },
		func(w int) string { return m.renderRecentProjectsCard(w, limit) },
	)
	if recent != "" {
		b.WriteString("\n" + recent)
	}
	return b.String()
}

// renderCardPair lays two dashboard cards side by side when the pane is wide
// enough and both have content, or stacks them otherwise. buildLeft/buildRight
// receive the width their card will actually occupy — building at the full
// pane width and only afterward squeezing the result into a half-width
// lipgloss.Style.Width() column would wrap the overflow onto an extra row.
func (m Model) renderCardPair(width int, buildLeft, buildRight func(cardWidth int) string) string {
	const gap = 2
	cardWidth := width
	if width >= dashCardsSideBySideMinWidth {
		cardWidth = max(20, (width-gap)/2)
	}
	left := buildLeft(cardWidth)
	right := buildRight(cardWidth)

	switch {
	case left == "" && right == "":
		return ""
	case width >= dashCardsSideBySideMinWidth && left != "" && right != "":
		l := lipgloss.NewStyle().Width(cardWidth).Render(left)
		r := lipgloss.NewStyle().Width(cardWidth).Render(right)
		return lipgloss.JoinHorizontal(lipgloss.Top, l, strings.Repeat(" ", gap), r)
	default:
		var b strings.Builder
		if left != "" {
			b.WriteString(left)
		}
		if right != "" {
			if left != "" {
				b.WriteString("\n")
			}
			b.WriteString(right)
		}
		return b.String()
	}
}

// renderRecentCommitsCard shows the user's own recent commits on this
// project — best-effort data fetched alongside the dashboard load; empty
// (rather than an error) when it couldn't be determined.
func (m Model) renderRecentCommitsCard(width int) string {
	if len(m.dashCommits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(tableHead.Render("Your recent commits") + "\n")
	for _, c := range m.dashCommits {
		sha := discussHeaderStyle.Render(fmt.Sprintf("%-8s", truncate(c.ShortID, 8)))
		age := footerStyle.Render(relTime(c.CreatedAt))
		titleW := max(10, width-8-1-lipgloss.Width(age)-2)
		line := sha + " " + truncate(c.Title, titleW)
		b.WriteString(spread(line, age, width) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// mrStateBarWidth is the fixed bar width (in block characters) for the MR
// state gauge.
const mrStateBarWidth = 10

// renderMRStatsCard shows a small bar per MR state (opened/merged/closed)
// across every project the user has authored a merge request in, scaled to
// the largest bucket — a compact "usage"-style gauge instead of a bare count.
func (m Model) renderMRStatsCard(_ int) string {
	if len(m.dashMRs) == 0 {
		return ""
	}
	counts := map[gitlab.MergeRequestState]int{}
	for _, mr := range m.dashMRs {
		counts[mr.State]++
	}
	rows := []struct {
		label string
		state gitlab.MergeRequestState
		style lipgloss.Style
	}{
		{"opened", gitlab.MergeRequestStateOpened, pipelineNeutral},
		{"merged", gitlab.MergeRequestStateMerged, pipelineSuccess},
		{"closed", gitlab.MergeRequestStateClosed, pipelineFailed},
	}

	maxCount := 1
	for _, r := range rows {
		if c := counts[r.state]; c > maxCount {
			maxCount = c
		}
	}

	var b strings.Builder
	b.WriteString(tableHead.Render("Your merge requests") + "\n")
	for _, r := range rows {
		count := counts[r.state]
		filled := count * mrStateBarWidth / maxCount
		if filled == 0 && count > 0 {
			filled = 1
		}
		bar := r.style.Render(strings.Repeat("█", filled)) + footerStyle.Render(strings.Repeat("░", mrStateBarWidth-filled))
		fmt.Fprintf(&b, "  %-7s %s  %d\n", r.label, bar, count)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderAssignedMRsCard shows open MRs assigned to the current user across
// projects — cross-project, unlike the current-branch MR card, so each row
// names its project (from ProjectPath, the one thing ListMyMergeRequests can
// tell us that a same-project listing wouldn't need to).
func (m Model) renderAssignedMRsCard(width int) string {
	if len(m.dashAssignedMRs) == 0 {
		return ""
	}
	const projectColWidth = 28
	var b strings.Builder
	b.WriteString(tableHead.Render("Assigned to you") + "\n")
	for _, mr := range m.dashAssignedMRs {
		proj := footerStyle.Render(fmt.Sprintf("%-*s", projectColWidth, truncate(shortPath(mr.ProjectPath), projectColWidth)))
		age := footerStyle.Render(relTime(mr.UpdatedAt))
		titleW := max(10, width-projectColWidth-1-lipgloss.Width(age)-2)
		title := truncate(fmt.Sprintf("!%d %s%s", mr.IID, mr.Title, draftSuffix(mr)), titleW)
		b.WriteString(spread(proj+" "+title, age, width) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// activityHeatLevel buckets one day's count into a 0-4 heat level, scaled
// relative to the busiest day (max) rather than fixed absolute cutoffs.
// Counting every contribution (comments, approvals, pushes — not just commits)
// means an actively-reviewing day can easily clear 20-50+ events, so a
// GitHub-style fixed "6+ = max" threshold would saturate every workday to the
// brightest color and collapse the whole gradient into a flat blob.
func activityHeatLevel(count, max int) int {
	if count == 0 || max == 0 {
		return 0
	}
	switch t := float64(count) / float64(max); {
	case t > 0.75:
		return 4
	case t > 0.5:
		return 3
	case t > 0.25:
		return 2
	default:
		return 1
	}
}

// activityLevelColor renders a heatmap level (0-4) as a color mixed from the
// active theme's own subtle→good colors — the strip re-themes with
// everything else instead of hardcoding a fixed green.
func activityLevelColor(level int) lipgloss.Color {
	return lipgloss.Color(mixHex(string(palette.subtle), string(palette.good), float64(level)/4.0))
}

// renderDashboardLoading is the full-screen "load as one" state: an animated
// spinner plus a hint at what's being fetched, shown until the dashboard
// context and its stats have both arrived so the whole screen paints at once.
func (m Model) renderDashboardLoading() string {
	frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
	spinner := lipgloss.NewStyle().Foreground(palette.accent).Bold(true).Render(frame)
	return spinner + "  " + valueStyle.Render("Loading…") + "\n\n" +
		footerStyle.Render("Fetching your merge requests, commits & activity")
}

// dashContributionSplitMinWidth is the content width at which the contribution
// calendar and the activity feed sit side by side; below it, they stack.
const dashContributionSplitMinWidth = 72

// renderContributionSplitCard lays the current-month contribution calendar on
// the left and the recent-activity feed on the right — "more information" in
// the same space — stacking them when the pane is too narrow to sit side by
// side.
func (m Model) renderContributionSplitCard(width int) string {
	cal := m.renderContributionCalendar()
	if cal == "" {
		return ""
	}
	const gap = 4
	calW := lipgloss.Width(cal)
	if feedW := width - calW - gap; width >= dashContributionSplitMinWidth && feedW >= 32 {
		if feed := m.renderActivityFeed(feedW); feed != "" {
			left := lipgloss.NewStyle().Width(calW).Render(cal)
			right := lipgloss.NewStyle().Width(feedW).Render(feed)
			return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
		}
	}
	if feed := m.renderActivityFeed(width); feed != "" {
		return cal + "\n\n" + feed
	}
	return cal
}

// weekdayLabels are the calendar's column labels, Sunday-first to match
// GitHub's and GitLab's contribution calendars.
var weekdayLabels = [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}

// activityMonthStats derives the header stats from a month's per-day counts:
// the total, the busiest day and its count, and the current streak of
// consecutive contribution days ending today — or yesterday, since an
// activity-free morning shouldn't read as a broken streak while the day is
// still in progress.
func activityMonthStats(counts map[int]int, today int) (total, peakDay, peakCount, streak int) {
	for day, c := range counts {
		total += c
		if c > peakCount || (c == peakCount && (peakDay == 0 || day < peakDay)) {
			peakDay, peakCount = day, c
		}
	}
	start := today
	if counts[start] == 0 {
		start--
	}
	for d := start; d >= 1 && counts[d] > 0; d-- {
		streak++
	}
	return total, peakDay, peakCount, streak
}

// renderContributionCalendar is the left half of the contribution card: the
// current month laid out like `cal` — weekday columns, week rows — where each
// elapsed day is a tile colored by its heat level with the day number inside.
// Future days render as dim plain numbers (a level-0 tile would read as data),
// and today is bold. A stats line (peak day, streak) sits under the header,
// GitHub-profile-style.
func (m Model) renderContributionCalendar() string {
	if len(m.dashActivity) == 0 {
		return ""
	}
	loc := time.Local
	now := time.Now().In(loc)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, -1)
	dayStart := func(t time.Time) time.Time {
		y, mo, d := t.In(loc).Date()
		return time.Date(y, mo, d, 0, 0, 0, 0, loc)
	}

	counts := make(map[int]int, 31) // day-of-month → contribution count
	for _, ev := range m.dashActivity {
		d := dayStart(ev.CreatedAt)
		if d.Before(monthStart) || d.After(monthEnd) {
			continue
		}
		counts[d.Day()]++
	}
	total, peakDay, peakCount, streak := activityMonthStats(counts, now.Day())
	maxCount := peakCount

	var b strings.Builder
	b.WriteString(tableHead.Render("Contribution activity") + "  " +
		footerStyle.Render(fmt.Sprintf("%s · %d total", now.Format("January"), total)) + "\n")
	if peakCount > 0 {
		stats := fmt.Sprintf("peak %d (%s %d)", peakCount, now.Format("Jan"), peakDay)
		if streak > 0 {
			stats += fmt.Sprintf(" · streak %dd", streak)
		}
		b.WriteString(footerStyle.Render(stats) + "\n")
	} else {
		b.WriteString(footerStyle.Render("no contributions yet this month") + "\n")
	}

	for _, wd := range weekdayLabels {
		b.WriteString(footerStyle.Render(fmt.Sprintf(" %-2s ", wd)) + " ")
	}
	b.WriteString("\n")

	// Week rows: blanks lead in until the 1st's weekday, then one 4-wide tile
	// per day (" %2d "), separated by a 1-column gutter.
	weekday := int(monthStart.Weekday())
	b.WriteString(strings.Repeat(" ", weekday*5))
	for day := 1; day <= monthEnd.Day(); day++ {
		b.WriteString(m.renderCalendarTile(day, counts[day], maxCount, now.Day()) + " ")
		weekday++
		if weekday == 7 && day < monthEnd.Day() {
			weekday = 0
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	b.WriteString(footerStyle.Render("Less "))
	for lvl := 0; lvl <= 4; lvl++ {
		b.WriteString(lipgloss.NewStyle().Background(activityLevelColor(lvl)).Render("  ") + " ")
	}
	b.WriteString(footerStyle.Render("More"))

	return b.String()
}

// renderCalendarTile renders one day of the contribution calendar. Elapsed
// days are 4-wide background-colored heat tiles with the day number inside
// (foreground picked for contrast against the tile); future days are dim
// plain numbers; today is bold — accent-colored when it has no tile yet.
func (m Model) renderCalendarTile(day, count, maxCount, today int) string {
	cell := fmt.Sprintf(" %2d ", day)
	if day > today {
		return footerStyle.Render(cell)
	}
	lvl := activityHeatLevel(count, maxCount)
	bg := activityLevelColor(lvl)
	style := lipgloss.NewStyle().Background(bg)
	switch {
	case lvl > 0:
		style = style.Foreground(lipgloss.Color(onColorFor(string(bg))))
	case day == today:
		style = style.Foreground(palette.accent)
	default:
		style = style.Foreground(palette.muted)
	}
	if day == today {
		style = style.Bold(true)
	}
	return style.Render(cell)
}

// dashboardActivityFeedLimit bounds how many recent-activity rows the feed
// beside the contribution strip shows.
const dashboardActivityFeedLimit = 6

// renderActivityFeed is the right half of the contribution card: a GitLab-style
// feed of the current user's most recent actions across projects — merged,
// opened, commented, pushed — with relative times. It reuses the activity
// already fetched for the strip, so it costs nothing extra.
func (m Model) renderActivityFeed(width int) string {
	if len(m.dashActivity) == 0 {
		return ""
	}
	const verbW = 8
	shown := min(dashboardActivityFeedLimit, len(m.dashActivity))

	var b strings.Builder
	b.WriteString(tableHead.Render("Recent activity") + "\n")
	for i := 0; i < shown; i++ {
		ev := m.dashActivity[i]
		marker := lipgloss.NewStyle().Foreground(palette.good).Render(strings.TrimSpace(activeMarker))
		verb := lipgloss.NewStyle().Foreground(activityVerbColor(ev.Action)).
			Render(fmt.Sprintf("%-*s", verbW, activityVerb(ev.Action)))
		age := footerStyle.Render(relTime(ev.CreatedAt))
		targetW := max(6, width-verbW-lipgloss.Width(age)-4)
		left := marker + " " + verb + " " + valueStyle.Render(truncate(ev.Target, targetW))
		b.WriteString(spread(left, age, width) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// activityVerb shortens GitLab's action_name into a compact, fixed-vocabulary
// verb for the feed ("commented on" → "comment", "pushed to" → "pushed");
// single-word actions (opened/closed/approved) pass through unchanged.
func activityVerb(action string) string {
	switch action {
	case "merged", "accepted":
		return "merged"
	case "commented on":
		return "comment"
	case "pushed to", "pushed new", "pushed":
		return "pushed"
	default:
		if i := strings.IndexByte(action, ' '); i > 0 {
			return action[:i]
		}
		return action
	}
}

// activityVerbColor tints the feed's verb by outcome: green for
// merged/accepted/approved, red for closed/deleted, the accent for everything
// else — a glanceable signal, like the pipeline gauges.
func activityVerbColor(action string) lipgloss.Color {
	switch action {
	case "merged", "accepted", "approved":
		return palette.good
	case "closed", "deleted":
		return palette.bad
	default:
		return palette.secondary
	}
}

// renderDashboardMRCard surfaces the current-branch MR's already-fetched
// fields — the copyable 'Y' summary (summary.go) prints more than this
// screen used to.
func (m Model) renderDashboardMRCard() string {
	mr := m.dash.MergeRequest
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s !%d %s\n", dashMarker(m.dashCursor < 0), tableHead.Render("MR"), mr.IID, mr.Title)
	b.WriteString(kv("status", string(mr.State)+draftSuffix(mr)) + "\n")
	b.WriteString(kv("pipeline", renderPipeline(mr.Pipeline)) + "\n")
	if mr.ApprovalsRequired > 0 {
		b.WriteString(kv("approvals", fmt.Sprintf("%d/%d", mr.ApprovalsGiven, mr.ApprovalsRequired)) + "\n")
	}
	b.WriteString(kv("author", mr.Author) + "\n")
	b.WriteString(kv("branches", fmt.Sprintf("%s → %s", mr.SourceBranch, mr.TargetBranch)) + "\n")
	if !mr.UpdatedAt.IsZero() {
		b.WriteString(kv("updated", relTime(mr.UpdatedAt)) + "\n")
	}
	if mr.HasConflicts {
		b.WriteString(errorStyle.Render("Has conflicts") + "\n")
	}
	if len(mr.Labels) > 0 {
		b.WriteString(kv("labels", labelStyle.Render(strings.Join(mr.Labels, ", "))) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderRecentBranchesCard(width, limit int) string {
	items := m.recentBranchItems()
	if len(items) == 0 {
		return ""
	}
	limit = max(1, limit)
	shown := min(limit, len(items))

	var b strings.Builder
	b.WriteString(tableHead.Render("Recent branches") + "\n")
	for i := 0; i < shown; i++ {
		br := items[i]
		left := dashMarker(m.dashCursor == i) + br.Name
		if br.MRIID != 0 {
			left += fmt.Sprintf("  !%d %s", br.MRIID, truncate(br.MRTitle, 40))
		}
		// Build the full row as plain text and style it once at the end —
		// styling the gutter marker separately and nesting it inside another
		// Render() call would let its reset code cut the row's own
		// highlight short right after the arrow (D1).
		line := spread(left, relTime(br.LastAccess), width)
		style := valueStyle
		if m.dashCursor == i {
			style = selectedStyle
		}
		b.WriteString(style.Render(line) + "\n")
	}
	if hidden := len(items) - shown; hidden > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  %d more…", hidden)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderRecentProjectsCard(width, limit int) string {
	items := m.recentProjectItems()
	if len(items) == 0 {
		return ""
	}
	limit = max(1, limit)
	shown := min(limit, len(items))

	var b strings.Builder
	b.WriteString(tableHead.Render("Recent projects") + "\n")
	for i := 0; i < shown; i++ {
		p := items[i]
		name := p.Name
		if name == "" {
			name = shortPath(p.Path)
		}
		left := "  " + valueStyle.Render(name) + "  " + footerStyle.Render(shortPath(p.Path))
		right := footerStyle.Render(relTime(p.LastAccess))
		b.WriteString(spread(left, right, width) + "\n")
	}
	if hidden := len(items) - shown; hidden > 0 {
		b.WriteString(footerStyle.Render(fmt.Sprintf("  %d more…", hidden)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) dashboardHints() []hint {
	actions := []hint{{"m", "merge requests"}, {"/", "search"}, {"tab", "workspace"}}
	if m.dash.MergeRequest != nil || len(m.recentBranchItems()) > 0 {
		actions = append(actions, hint{"enter", "detail"})
	}
	if len(m.recentBranchItems()) > 0 {
		actions = append(actions, hint{"j/k", "select"})
	}
	if m.currentURL() != "" {
		actions = append(actions, hint{"o", "open"}, hint{"y", "copy link"})
	}
	if m.dash.MergeRequest != nil {
		actions = append(actions, hint{"Y", "copy summary"}, hint{"T", "summary: " + m.summaryFormat.next().label()})
	}
	return append(actions, hint{"r", "refresh"}, hint{"q", "quit"})
}

// recentBranchItems is the recent-branches list the dashboard shows: recorded
// branches other than the current one, which the MR panel above already covers.
func (m Model) recentBranchItems() []history.Branch {
	current := ""
	if m.dash != nil {
		current = m.dash.Branch
	}
	items := make([]history.Branch, 0, len(m.recentBranches))
	for _, br := range m.recentBranches {
		if br.Name != current {
			items = append(items, br)
		}
	}
	return items
}

func (m Model) recentProjectItems() []history.Project {
	current := ""
	if m.dash != nil && m.dash.Project != nil {
		current = m.dash.Project.PathWithNamespace
	}
	items := make([]history.Project, 0, len(m.recentProjects))
	for _, p := range m.recentProjects {
		if p.Path != current {
			items = append(items, p)
		}
	}
	return items
}

// dashMinCursor is -1 when a current-branch MR occupies the top slot, else 0.
func (m Model) dashMinCursor() int {
	if m.dash != nil && m.dash.MergeRequest != nil {
		return -1
	}
	return 0
}

func (m Model) dashMaxCursor() int {
	limit := m.dashCardRowLimit()
	if n := len(m.recentBranchItems()); n < limit {
		return n - 1
	}
	return limit - 1
}

// dashCardRowLimit is how many rows each recent-branch/recent-project card
// shows, derived from the pane's available height instead of a fixed cap —
// a fixed cap of 5 left rows 6-10 (the history store keeps 10) both
// invisible and unreachable on a screen that was otherwise mostly blank.
func (m Model) dashCardRowLimit() int {
	overhead := 9 // title+rule+blank, profile, blank, "no MR" message, card header
	if m.dash != nil && m.dash.MergeRequest != nil {
		overhead = 16 // ...or the fuller MR card in its place
	}
	if len(m.dashCommits) > 0 || len(m.dashMRs) > 0 {
		overhead += 4 + dashboardCommitLimit // stat card pair (bounded by the taller: commits) + separator
	}
	if len(m.dashAssignedMRs) > 0 {
		overhead += 2 + dashboardAssignedMRLimit // "assigned to you" card + separator
	}
	if len(m.dashActivity) > 0 {
		overhead += 11 // contribution calendar (header + stats + labels + up to 6 week rows + legend) + separator
	}
	return max(3, m.contentHeight()-overhead)
}

func dashMarker(selected bool) string {
	if selected {
		return cursorMarker
	}
	return emptyMarker
}

// relTime renders a short "2h ago"-style relative timestamp, falling back to
// an absolute date once it's more than a month old.
func relTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Local().Format("2006-01-02")
	}
}
