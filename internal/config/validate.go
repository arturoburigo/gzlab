package config

import "fmt"

// Validate checks the config for internally-consistent, actionable errors.
func (c *Config) Validate() error {
	for name, p := range c.Profiles {
		if p.Host == "" {
			return fmt.Errorf("profile %q is missing a host", name)
		}
		if p.TokenEnv == "" && p.Token == "" {
			return fmt.Errorf("profile %q needs either token_env (recommended) or token", name)
		}
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
