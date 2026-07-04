// Package config models and persists gzlab's local configuration file.
package config

import "time"

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
	Enabled bool           `yaml:"enabled"`
	TTL     CacheTTLConfig `yaml:"ttl"`
}

// CacheTTLConfig lets each cached data type use its own retention window.
// Values are duration strings (e.g. "45s", "24h"); unset fields keep the
// defaults set by Default() below.
type CacheTTLConfig struct {
	CurrentUser        Duration `yaml:"current_user"`
	Project            Duration `yaml:"project"`
	MergeRequestList   Duration `yaml:"merge_request_list"`
	MergeRequestDetail Duration `yaml:"merge_request_detail"`
	Diff               Duration `yaml:"diff"`
	Pipeline           Duration `yaml:"pipeline"`
	PipelineJobs       Duration `yaml:"pipeline_jobs"`
	Commits            Duration `yaml:"commits"`
	ContributionEvents Duration `yaml:"contribution_events"`
}

// Default returns a new Config with no profiles and sane defaults for
// everything else, suitable as a starting point for `auth login`.
func Default() *Config {
	return &Config{
		Profiles: map[string]Profile{},
		UI: UIConfig{
			Theme: "dark",
			Mouse: true,
			// Empty means "use $EDITOR, falling back to vi" — see
			// internal/cli.resolveEditorCommand. Set this to override,
			// e.g. "code --wait".
			Editor:  "",
			Browser: "default",
		},
		Diff: DiffConfig{
			Mode:             DiffModeSideBySide,
			IgnoreWhitespace: false,
			ContextLines:     3,
		},
		Cache: CacheConfig{
			Enabled: true,
			TTL: CacheTTLConfig{
				CurrentUser:        Duration(24 * time.Hour),
				Project:            Duration(24 * time.Hour),
				MergeRequestList:   Duration(45 * time.Second),
				MergeRequestDetail: Duration(45 * time.Second),
				Diff:               Duration(5 * time.Minute),
				Pipeline:           Duration(15 * time.Second),
				PipelineJobs:       Duration(15 * time.Second),
				Commits:            Duration(5 * time.Minute),
				ContributionEvents: Duration(5 * time.Minute),
			},
		},
	}
}
