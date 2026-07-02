package gitdetect

import "strings"

// MatchProfileByHost returns the name of the profile whose host matches the
// detected remote host, if any. hosts maps profile name -> configured host.
//
// Callers are responsible for ensuring hosts has no two entries that
// normalize to the same value (config.Validate enforces this) — with a
// duplicate, map iteration order would make the match nondeterministic.
func MatchProfileByHost(hosts map[string]string, host string) (string, bool) {
	normalized := NormalizeHost(host)
	for name, h := range hosts {
		if NormalizeHost(h) == normalized {
			return name, true
		}
	}
	return "", false
}

// NormalizeHost puts a configured or detected host into a comparable form:
// lowercased, with no trailing slash.
func NormalizeHost(h string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "/"))
}
