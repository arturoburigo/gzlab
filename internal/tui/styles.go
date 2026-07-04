package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

// themePalette is the full set of colors a theme contributes. onAccent is the
// foreground used for text drawn on top of an accent-colored background
// (selections, the logo badge) — it must be picked per theme, since a light
// theme's accent is often a dark hue that a hardcoded black foreground would
// vanish against.
type themePalette struct {
	accent    lipgloss.Color
	secondary lipgloss.Color
	text      lipgloss.Color
	muted     lipgloss.Color
	subtle    lipgloss.Color
	border    lipgloss.Color
	good      lipgloss.Color
	warn      lipgloss.Color
	bad       lipgloss.Color
	info      lipgloss.Color
	onAccent  lipgloss.Color
}

const (
	cursorMarker = "▶ "
	activeMarker = "▸ "
	emptyMarker  = "  "
	promptMarker = "› "
)

// defaultAdaptiveTheme is what the "dark" name (and any unrecognized theme
// name) resolves to: an OC theme that adapts to the terminal's actual
// background via lipgloss.HasDarkBackground(), rather than a fixed palette
// that's unreadable on the "wrong" terminal.
const defaultAdaptiveTheme = "opencode"

func applyTheme(theme string) {
	name := strings.ToLower(strings.TrimSpace(theme))
	if oc, ok := lookupOCTheme(name); ok {
		sub := oc.dark
		if !lipgloss.HasDarkBackground() {
			sub = oc.light
		}
		palette = ocToPalette(sub)
		rebuildStyles()
		return
	}

	switch name {
	case "light":
		palette = themePalette{
			accent: lipgloss.Color("25"), secondary: lipgloss.Color("166"),
			text: lipgloss.Color("235"), muted: lipgloss.Color("240"), subtle: lipgloss.Color("254"),
			border: lipgloss.Color("110"), good: lipgloss.Color("28"), warn: lipgloss.Color("130"), bad: lipgloss.Color("124"),
			info: lipgloss.Color("39"), onAccent: lipgloss.Color("255"),
		}
	case "terminal", "retro", "crt", "old-terminal":
		palette = terminalPalette()
	case "gitlab", "orange":
		palette = themePalette{
			accent: lipgloss.Color("208"), secondary: lipgloss.Color("141"),
			text: lipgloss.Color("252"), muted: lipgloss.Color("245"), subtle: lipgloss.Color("236"),
			border: lipgloss.Color("95"), good: lipgloss.Color("77"), warn: lipgloss.Color("214"), bad: lipgloss.Color("203"),
			info: lipgloss.Color("39"), onAccent: lipgloss.Color("0"),
		}
	default:
		// "dark" and any unrecognized name land here. The CRT palette used to
		// be the default, but it's a saturated neon-green look that inverts
		// hierarchy (muted brighter than body text) and is unreadable on
		// light terminals; the adaptive OC theme is a safer default. The CRT
		// look stays reachable via its explicit names above.
		oc := ocByID[defaultAdaptiveTheme]
		sub := oc.dark
		if !lipgloss.HasDarkBackground() {
			sub = oc.light
		}
		palette = ocToPalette(sub)
	}
	rebuildStyles()
}

func terminalPalette() themePalette {
	return themePalette{
		accent:    lipgloss.Color("#00ff66"),
		secondary: lipgloss.Color("#ffcc00"),
		text:      lipgloss.Color("#e7ffe7"),
		muted:     lipgloss.Color("#3cff87"),
		subtle:    lipgloss.Color("#003311"),
		border:    lipgloss.Color("#00cc55"),
		good:      lipgloss.Color("#39ff14"),
		warn:      lipgloss.Color("#ffcc00"),
		bad:       lipgloss.Color("#ff2d55"),
		info:      lipgloss.Color("#00ccff"),
		onAccent:  lipgloss.Color("#000000"),
	}
}

func lookupOCTheme(name string) (ocTheme, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "tokyo", "tokyo-night":
		n = "tokyonight"
	case "purple":
		n = "dracula"
	}
	oc, ok := ocByID[n]
	return oc, ok
}

func init() {
	rebuildStyles()
}

func rebuildStyles() {
	logoStyle = lipgloss.NewStyle().Bold(true).Foreground(palette.onAccent).Background(palette.accent).Padding(0, 1)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(palette.accent)
	labelStyle = lipgloss.NewStyle().Foreground(palette.muted)
	valueStyle = lipgloss.NewStyle().Bold(true).Foreground(palette.text)
	footerKey = lipgloss.NewStyle().Bold(true).Foreground(palette.secondary)
	footerStyle = lipgloss.NewStyle().Foreground(palette.muted)
	errorStyle = lipgloss.NewStyle().Foreground(palette.bad).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(palette.onAccent).Background(palette.accent).Bold(true)
	tableHead = lipgloss.NewStyle().Bold(true).Foreground(palette.secondary)
	// Panes no longer paint a surface background: nested styled fragments
	// reset to the terminal default mid-line, leaving the surface tint only
	// on unstyled runs — a patchy/striped look on every OC theme (S1).
	paneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(palette.border).Padding(0, 1)
	sidebarStyle = lipgloss.NewStyle().Padding(0, 1)
	sidebarSelectedStyle = lipgloss.NewStyle().Foreground(palette.onAccent).Background(palette.secondary).Bold(true)
	sidebarActiveStyle = lipgloss.NewStyle().Foreground(palette.accent).Background(palette.subtle).Bold(true)
	sidebarMutedStyle = lipgloss.NewStyle().Foreground(palette.muted)
	ruleStyle = lipgloss.NewStyle().Foreground(palette.border)
	hunkStyle = lipgloss.NewStyle().Foreground(palette.info)
	additionStyle = lipgloss.NewStyle().Foreground(palette.good)
	deletionStyle = lipgloss.NewStyle().Foreground(palette.bad)
	searchStyle = lipgloss.NewStyle().Bold(true).Foreground(palette.onAccent).Background(palette.warn)
	searchDimStyle = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(palette.warn)
	confirmStyle = lipgloss.NewStyle().Foreground(palette.warn).Bold(true)
	overlayStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(palette.accent).Padding(1, 2)
	overlayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(palette.accent)
	overlaySelectedStyle = lipgloss.NewStyle().Foreground(palette.onAccent).Background(palette.secondary).Bold(true)
	discussHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(palette.secondary)
	discussResolvedStyle = lipgloss.NewStyle().Foreground(palette.good).Bold(true)
	discussUnresolvedStyle = lipgloss.NewStyle().Foreground(palette.warn).Bold(true)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(palette.secondary)
	pipelineSuccess = lipgloss.NewStyle().Foreground(palette.good).Bold(true)
	pipelineFailed = lipgloss.NewStyle().Foreground(palette.bad).Bold(true)
	pipelineRunning = lipgloss.NewStyle().Foreground(palette.warn).Bold(true)
	pipelineNeutral = lipgloss.NewStyle().Foreground(palette.muted)
}

var (
	palette = terminalPalette()

	logoStyle      lipgloss.Style
	titleStyle     lipgloss.Style
	labelStyle     lipgloss.Style
	valueStyle     lipgloss.Style
	footerKey      lipgloss.Style
	footerStyle    lipgloss.Style
	errorStyle     lipgloss.Style
	selectedStyle  lipgloss.Style
	tableHead      lipgloss.Style
	paneStyle      lipgloss.Style
	sidebarStyle   lipgloss.Style
	ruleStyle      lipgloss.Style
	hunkStyle      lipgloss.Style
	additionStyle  lipgloss.Style
	deletionStyle  lipgloss.Style
	searchStyle    lipgloss.Style
	searchDimStyle lipgloss.Style
	confirmStyle   lipgloss.Style
	overlayStyle   lipgloss.Style

	overlayTitleStyle    lipgloss.Style
	overlaySelectedStyle lipgloss.Style

	sidebarSelectedStyle lipgloss.Style
	sidebarActiveStyle   lipgloss.Style
	sidebarMutedStyle    lipgloss.Style

	discussHeaderStyle     lipgloss.Style
	discussResolvedStyle   lipgloss.Style
	discussUnresolvedStyle lipgloss.Style
	cursorStyle            lipgloss.Style

	pipelineSuccess lipgloss.Style
	pipelineFailed  lipgloss.Style
	pipelineRunning lipgloss.Style
	pipelineNeutral lipgloss.Style
)

func pipelineStyle(status gitlab.PipelineStatus) lipgloss.Style {
	switch status {
	case gitlab.PipelineStatusSuccess:
		return pipelineSuccess
	case gitlab.PipelineStatusFailed, gitlab.PipelineStatusCanceled:
		return pipelineFailed
	case gitlab.PipelineStatusRunning, gitlab.PipelineStatusPending:
		return pipelineRunning
	default:
		return pipelineNeutral
	}
}
