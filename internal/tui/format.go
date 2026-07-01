package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Pure text-layout helpers shared by the interactive client and the headless
// line client. No game logic, no I/O.

// formatCompact renders a resource amount as e.g. 209.0k or 1.2M.
func formatCompact(v float64) string {
	n := int64(v)
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// formatRemaining renders a countdown like 3m04s or 1h05m; "done" once elapsed.
func formatRemaining(d time.Duration) string {
	if d <= 0 {
		return "done"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", h, m)
}

// clampLine truncates s (with an ellipsis) so its display width fits width.
func clampLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return ansi.Truncate(s, width, "…")
}

// clampBlock forces s into exactly width x height cells, truncating and
// padding as needed. Used to keep every region inside its allotted rectangle.
func clampBlock(s string, width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = clampLine(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

// blockHeight counts the rendered lines of s.
func blockHeight(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(strings.TrimRight(s, "\n"), "\n"))
}

// fitColumns places left and right on one row width cells wide, right-aligning
// right. Falls back to a clamped join when the two would collide.
func fitColumns(width int, left, right string) string {
	leftW := ansi.StringWidth(left)
	rightW := ansi.StringWidth(right)
	if leftW+rightW+1 >= width {
		return clampLine(left+" "+right, width)
	}
	return left + strings.Repeat(" ", width-leftW-rightW) + right
}

// padLine clamps s to width and right-pads with spaces to exactly width cells,
// so columns line up when joined horizontally.
func padLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = clampLine(s, width)
	if gap := width - ansi.StringWidth(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

// column normalises lines into exactly height rows, each padded to width.
func column(lines []string, width, height int) []string {
	out := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out[i] = padLine(lines[i], width)
		} else {
			out[i] = strings.Repeat(" ", width)
		}
	}
	return out
}

// clampInt bounds v to [low, high].
func clampInt(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

// min returns the smaller of a and b. (Kept as a package helper for older call
// sites; Go's builtin min is used for float paths.)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
