// Package config models and persists gitlab-tui's local configuration file.
package config

// Config is the root of ~/.config/gitlab-tui/config.yaml.
type Config struct {
	DefaultProfile string             `yaml:"default_profile"`
	Profiles       map[string]Profile `yaml:"profiles"`
	UI             UIConfig           `yaml:"ui"`
	Diff           DiffConfig         `yaml:"diff"`
	Cache          CacheConfig        `yaml:"cache"`
}

// Profile is a single named GitLab connection (e.g. "empresa", "pessoal").
type Profile struct {
	Host string `yaml:"host"`
	// TokenEnv names the environment variable holding the access token.
	TokenEnv string `yaml:"token_env,omitempty"`
	// Token is a fallback for storing the token directly in the config file.
	// Prefer TokenEnv; this exists only for cases where env vars aren't practical.
	Token string `yaml:"token,omitempty"`
}

// UIConfig controls general interface behavior.
type UIConfig struct {
	Theme   string `yaml:"theme"`
	Mouse   bool   `yaml:"mouse"`
	Editor  string `yaml:"editor"`
	Browser string `yaml:"browser"`
}

// DiffMode selects how file diffs are rendered.
type DiffMode string

const (
	DiffModeSideBySide DiffMode = "side_by_side"
	DiffModeUnified    DiffMode = "unified"
)

// DiffConfig controls the diff viewer (Épico 13).
type DiffConfig struct {
	Mode             DiffMode `yaml:"mode"`
	IgnoreWhitespace bool     `yaml:"ignore_whitespace"`
	ContextLines     int      `yaml:"context_lines"`
}

// CacheConfig controls local response caching (Épico 21).
type CacheConfig struct {
	Enabled bool `yaml:"enabled"`
}

// Default returns a new Config with no profiles and sane defaults for
// everything else, suitable as a starting point for `auth login`.
func Default() *Config {
	return &Config{
		Profiles: map[string]Profile{},
		UI: UIConfig{
			Theme:   "dark",
			Mouse:   true,
			Editor:  "$EDITOR",
			Browser: "default",
		},
		Diff: DiffConfig{
			Mode:             DiffModeSideBySide,
			IgnoreWhitespace: false,
			ContextLines:     3,
		},
		Cache: CacheConfig{
			Enabled: true,
		},
	}
}
