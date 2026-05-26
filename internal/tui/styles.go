package tui

import "github.com/charmbracelet/lipgloss"

// Color palette - kept small on purpose. The TUI runs on any 256-colour
// terminal; we lean on lipgloss adaptive colours so light and dark themes
// stay readable.
var (
	colorPrimary   = lipgloss.AdaptiveColor{Light: "#5e35b1", Dark: "#9575cd"}
	colorAccent    = lipgloss.AdaptiveColor{Light: "#00897b", Dark: "#4db6ac"}
	colorWarn      = lipgloss.AdaptiveColor{Light: "#f57c00", Dark: "#ffb74d"}
	colorError     = lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef9a9a"}
	colorMuted     = lipgloss.AdaptiveColor{Light: "#616161", Dark: "#9e9e9e"}
	colorHighlight = lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#64b5f6"}
)

// Styles bundles every reusable style so screens can reach into one place.
type Styles struct {
	Title      lipgloss.Style
	Header     lipgloss.Style
	Panel      lipgloss.Style
	Item       lipgloss.Style
	Selected   lipgloss.Style
	Help       lipgloss.Style
	Error      lipgloss.Style
	Success    lipgloss.Style
	Muted      lipgloss.Style
	Input      lipgloss.Style
	Tag        lipgloss.Style
	BarLabel   lipgloss.Style
	BarValue   lipgloss.Style
	Background lipgloss.Style
}

// NewStyles returns the default Styles instance.
func NewStyles() *Styles {
	return &Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Underline(true),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1),
		Item: lipgloss.NewStyle().
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("0")).
			Background(colorHighlight).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true),
		Error: lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true),
		Muted: lipgloss.NewStyle().
			Foreground(colorMuted),
		Input: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#212121", Dark: "#fafafa"}),
		Tag: lipgloss.NewStyle().
			Foreground(colorAccent).
			Padding(0, 1),
		BarLabel: lipgloss.NewStyle().
			Foreground(colorMuted),
		BarValue: lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true),
		Background: lipgloss.NewStyle(),
	}
}
