package config

import (
	"fmt"
	"os"
)

// ResolveToken returns the profile's access token, preferring the
// environment variable named by TokenEnv over the config-file fallback.
func (p Profile) ResolveToken() (string, error) {
	if p.TokenEnv != "" {
		if v := os.Getenv(p.TokenEnv); v != "" {
			return v, nil
		}
		if p.Token == "" {
			return "", fmt.Errorf("environment variable %s is not set and no fallback token is configured", p.TokenEnv)
		}
	}
	if p.Token != "" {
		return p.Token, nil
	}
	return "", fmt.Errorf("profile has no token_env or token configured")
}
