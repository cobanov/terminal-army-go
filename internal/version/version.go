// Package version exposes build metadata stamped via -ldflags at build time.
package version

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
