package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// statsScreen shows a few server-wide counters: universes, players, planets,
// online count, fleets in flight, and uptime.
type statsScreen struct {
	root    *rootModel
	stats   *svc.StatsOverview
	loading bool
}

func newStatsScreen(root *rootModel) *statsScreen {
	return &statsScreen{root: root, loading: true}
}

func (s *statsScreen) Init() tea.Cmd { return cmdStats(s.root.client) }

func (s *statsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case statsLoadedMsg:
		s.stats = m.stats
		s.loading = false
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			s.loading = true
			return s, cmdStats(s.root.client)
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		}
	}
	return s, nil
}

func (s *statsScreen) View() string {
	st := s.root.styles
	if s.loading || s.stats == nil {
		return st.Muted.Render("loading stats...")
	}
	rows := []string{
		st.Header.Render("Server stats"),
		st.BarLabel.Render("universes:        ") + st.BarValue.Render(fmt.Sprintf("%d", s.stats.Universes)),
		st.BarLabel.Render("players:          ") + st.BarValue.Render(fmt.Sprintf("%d", s.stats.Players)),
		st.BarLabel.Render("planets:          ") + st.BarValue.Render(fmt.Sprintf("%d", s.stats.Planets)),
		st.BarLabel.Render("online players:   ") + st.BarValue.Render(fmt.Sprintf("%d", s.stats.OnlinePlayers)),
		st.BarLabel.Render("fleets in flight: ") + st.BarValue.Render(fmt.Sprintf("%d", s.stats.FleetsInFlight)),
		st.BarLabel.Render("uptime:           ") + st.BarValue.Render(fmtDuration(time.Duration(s.stats.UptimeSeconds)*time.Second)),
	}
	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// fmtDuration prints a long duration as Nd Nh Nm.
func fmtDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	mins := int(d / time.Minute)
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func (s *statsScreen) Title() string { return "stats" }

func (s *statsScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "r", Desc: "refresh"},
		{Key: "esc", Desc: "back"},
	}
}
