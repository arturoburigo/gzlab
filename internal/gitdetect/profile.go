package gitdetect

import "strings"

// MatchProfileByHost returns the name of the profile whose host matches the
// detected remote host, if any. hosts maps profile name -> configured host.
func MatchProfileByHost(hosts map[string]string, host string) (string, bool) {
	normalized := normalizeHost(host)
	for name, h := range hosts {
		if normalizeHost(h) == normalized {
			return name, true
		}
	}
	return "", false
}

func normalizeHost(h string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "/"))
}
