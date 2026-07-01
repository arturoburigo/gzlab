// Package version holds build-time metadata injected via -ldflags.
package version

var (
	// Version is the semantic version of the build, set via -ldflags at build time.
	Version = "dev"
	// Commit is the git commit SHA of the build.
	Commit = "none"
	// Date is the build timestamp.
	Date = "unknown"
)

// String returns a human-readable version string.
func String() string {
	return Version + " (commit " + Commit + ", built " + Date + ")"
}
