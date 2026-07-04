package tui

import (
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// syntaxStyle is the chroma color scheme used for diff syntax highlighting,
// picked once to suit the terminal background. We read only each token's
// foreground colour from it (never its background) and render through
// lipgloss, so colours downsample to the terminal's profile exactly like the
// rest of the UI and never fight the diff's own +/- colouring.
var syntaxStyle = sync.OnceValue(func() *chroma.Style {
	if lipgloss.HasDarkBackground() {
		return styles.Get("catppuccin-mocha")
	}
	return styles.Get("catppuccin-latte")
})

var (
	lexerCache   = map[string]chroma.Lexer{}
	lexerCacheMu sync.Mutex
)

// lexerFor resolves (and caches) the chroma lexer for a filename. lexers.Match
// globs the filename against every lexer's patterns, so caching per path keeps
// per-line highlighting cheap. Coalesce merges adjacent same-type tokens.
func lexerFor(filename string) chroma.Lexer {
	lexerCacheMu.Lock()
	defer lexerCacheMu.Unlock()
	if l, ok := lexerCache[filename]; ok {
		return l
	}
	l := lexers.Match(filename)
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)
	lexerCache[filename] = l
	return l
}

// highlightCode syntax-highlights one line of source (already stripped of any
// diff marker) for filename, returning it with lipgloss foreground colours per
// token. Best-effort: an unknown language or a tokeniser error yields the
// plain text, never an error. Highlighting a line in isolation loses
// cross-line context (a string opened on the previous line, say), which is the
// accepted trade-off every per-line diff highlighter makes.
func highlightCode(filename, code string) string {
	if strings.TrimSpace(code) == "" {
		return code
	}
	it, err := lexerFor(filename).Tokenise(nil, code)
	if err != nil {
		return code
	}
	style := syntaxStyle()
	var b strings.Builder
	for _, tok := range it.Tokens() {
		entry := style.Get(tok.Type)
		s := lipgloss.NewStyle()
		if entry.Colour.IsSet() {
			s = s.Foreground(lipgloss.Color(entry.Colour.String()))
		}
		if entry.Bold == chroma.Yes {
			s = s.Bold(true)
		}
		if entry.Italic == chroma.Yes {
			s = s.Italic(true)
		}
		b.WriteString(s.Render(tok.Value))
	}
	return b.String()
}
