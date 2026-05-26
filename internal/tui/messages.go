// Inter-screen Bubble Tea messages. Keeping every message type in one file
// makes screen-to-screen communication grep-able. Screens convert API
// responses into these messages via tea.Cmd functions.
package tui

import (
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// errMsg wraps an error so screens can report any async failure uniformly.
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// statusMsg is a transient banner the root model displays for ~3 seconds.
type statusMsg struct {
	text   string
	level  statusLevel
}

type statusLevel int

const (
	statusInfo statusLevel = iota
	statusOK
	statusWarn
	statusErr
)

// clearStatusMsg fires after a delay to wipe the status bar.
type clearStatusMsg struct{}

// --- session messages ---

type sessionReadyMsg struct {
	user *svc.User
}

type loggedOutMsg struct{}

// --- screen navigation ---

type changeScreenMsg struct {
	screen screen
	// optional argument used by some screens, e.g. planet ID for buildings view
	arg any
}

// --- data load messages ---

type planetsLoadedMsg struct{ planets []svc.Planet }
type planetLoadedMsg struct{ planet *svc.Planet }
type productionLoadedMsg struct{ report *svc.ProductionReport }
type queuesLoadedMsg struct{ items []svc.QueueItem }
type researchLoadedMsg struct{ levels []svc.ResearchLevel }
type systemLoadedMsg struct{ view *svc.SystemView }
type fleetsLoadedMsg struct{ fleets []svc.Fleet }
type messagesLoadedMsg struct{ items []svc.Message }
type leaderboardLoadedMsg struct{ entries []svc.LeaderboardEntry }
type statsLoadedMsg struct{ stats *svc.StatsOverview }
type universesLoadedMsg struct{ universes []svc.Universe }
type universeJoinedMsg struct{ planet *svc.Planet }
type queueQueuedMsg struct{ item *svc.QueueItem }
type fleetDispatchedMsg struct{ fleet *svc.Fleet }
type messageLoadedMsg struct{ msg *svc.Message }
type messageDeletedMsg struct{ id int64 }
