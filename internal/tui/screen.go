// Screen enumeration and the small Screen interface every screen implements.
// The root model dispatches Update and View calls based on the active screen.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// screen identifies one of the top-level views the TUI can show.
type screen int

const (
	screenLogin screen = iota
	screenRegister
	screenUniversePicker
	screenOverview
	screenBuildings
	screenResearch
	screenShipyard
	screenDefense
	screenFleet
	screenFleetDispatch
	screenGalaxy
	screenMessages
	screenLeaderboard
	screenStats
)

// Screen is the contract every focused view implements. The root model
// forwards size, key, and async messages here, then renders View() into the
// app frame.
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View() string
	Title() string
	Help() []HelpEntry
}

// HelpEntry is one item in the contextual help footer.
type HelpEntry struct {
	Key  string
	Desc string
}
