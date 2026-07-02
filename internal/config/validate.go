package config

import (
	"fmt"

	"github.com/arturoburigo/gitlab-tui/internal/gitdetect"
)

// Validate checks the config for internally-consistent, actionable errors.
func (c *Config) Validate() error {
	hostToProfile := make(map[string]string, len(c.Profiles))
	for name, p := range c.Profiles {
		if p.Host == "" {
			return fmt.Errorf("profile %q is missing a host", name)
		}
		if p.TokenEnv == "" && p.Token == "" {
			return fmt.Errorf("profile %q needs either token_env (recommended) or token", name)
		}

		normalized := gitdetect.NormalizeHost(p.Host)
		if other, ok := hostToProfile[normalized]; ok {
			return fmt.Errorf("profiles %q and %q both point at host %q; matching by host would be ambiguous", other, name, p.Host)
		}
		hostToProfile[normalized] = name
	}

	if c.DefaultProfile != "" {
		if _, ok := c.Profiles[c.DefaultProfile]; !ok {
			return fmt.Errorf("default_profile %q does not match any configured profile", c.DefaultProfile)
		}
	}

	switch c.Diff.Mode {
	case "", DiffModeSideBySide, DiffModeUnified:
	default:
		return fmt.Errorf("diff.mode %q must be %q or %q", c.Diff.Mode, DiffModeSideBySide, DiffModeUnified)
	}

	return nil
}
