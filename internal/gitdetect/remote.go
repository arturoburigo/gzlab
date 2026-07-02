package gitdetect

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// RemoteInfo is a git remote URL split into a GitLab host and the
// namespace/project path.
type RemoteInfo struct {
	// Host is the scheme+hostname, e.g. "https://gitlab.example.com".
	Host string
	// Path is the "namespace/subgroup/project" path, without ".git".
	Path string
}

// scpLikeRe matches SCP-style SSH remotes: [user@]host:path (no scheme).
var scpLikeRe = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// ParseRemoteURL extracts the host and namespace/project path from a git
// remote URL. It supports SCP-style SSH (git@host:namespace/project.git),
// ssh://, https:// and git:// forms.
func ParseRemoteURL(remote string) (*RemoteInfo, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return nil, fmt.Errorf("empty remote URL")
	}

	if !strings.Contains(remote, "://") {
		if m := scpLikeRe.FindStringSubmatch(remote); m != nil {
			return &RemoteInfo{
				Host: "https://" + m[1],
				Path: strings.TrimSuffix(m[2], ".git"),
			}, nil
		}
	}

	u, err := url.Parse(remote)
	if err != nil {
		return nil, fmt.Errorf("parsing remote URL %q: %w", remote, err)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("could not determine host from remote URL %q", remote)
	}

	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return nil, fmt.Errorf("could not determine project path from remote URL %q", remote)
	}

	// http(s):// remotes use the same port for git and the API, so keep it
	// (u.Host, not u.Hostname()) along with the original scheme. ssh://,
	// git:// and scp-like remotes carry an SSH-specific port that has no
	// relation to the API's port, so those fall back to a plain https host.
	scheme := u.Scheme
	host := u.Hostname()
	if scheme == "http" || scheme == "https" {
		host = u.Host
	} else {
		scheme = "https"
	}

	return &RemoteInfo{
		Host: scheme + "://" + host,
		Path: path,
	}, nil
}
