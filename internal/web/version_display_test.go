package web

import (
	"strings"
	"testing"
)

func TestDisplayBuildDate(t *testing.T) {
	if got := displayBuildDate(""); got != "unknown" {
		t.Errorf("empty -> %q, want unknown", got)
	}
	if got := displayBuildDate("unknown"); got != "unknown" {
		t.Errorf("unknown -> %q, want unknown", got)
	}
	if got := displayBuildDate("2026-07-01T22:44:00Z"); !strings.HasPrefix(got, "2026-07-01") {
		t.Errorf("rfc3339 -> %q, want a 2026-07-01 date", got)
	}
	// Non-RFC3339 falls through unchanged.
	if got := displayBuildDate("v1.2.3-build"); got != "v1.2.3-build" {
		t.Errorf("raw -> %q, want passthrough", got)
	}
}
