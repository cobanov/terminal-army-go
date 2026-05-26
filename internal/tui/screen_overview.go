package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// overviewScreen is the home base. It shows the user's planet list with a
// summary line for each, plus a panel of resources + production for the
// currently selected planet. Single-key shortcuts jump to focused screens
// (buildings, research, etc).
type overviewScreen struct {
	root       *rootModel
	planets    []svc.Planet
	cursor     int
	production *svc.ProductionReport
	queues     []svc.QueueItem
	loading    bool
}

func newOverviewScreen(root *rootModel) *overviewScreen {
	return &overviewScreen{root: root, loading: true}
}

func (s *overviewScreen) Init() tea.Cmd {
	return tea.Batch(cmdListPlanets(s.root.client))
}

func (s *overviewScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case planetsLoadedMsg:
		s.planets = m.planets
		s.loading = false
		if s.cursor >= len(s.planets) {
			s.cursor = 0
		}
		return s, s.loadDetails()
	case productionLoadedMsg:
		s.production = m.report
	case queuesLoadedMsg:
		s.queues = m.items
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
				return s, s.loadDetails()
			}
		case "down", "j":
			if s.cursor < len(s.planets)-1 {
				s.cursor++
				return s, s.loadDetails()
			}
		case "r":
			s.loading = true
			return s, cmdListPlanets(s.root.client)
		case "b":
			if pid, ok := s.currentPlanetID(); ok {
				return s, func() tea.Msg { return changeScreenMsg{screen: screenBuildings, arg: pid} }
			}
		case "t":
			if pid, ok := s.currentPlanetID(); ok {
				return s, func() tea.Msg { return changeScreenMsg{screen: screenResearch, arg: pid} }
			}
		case "y":
			if pid, ok := s.currentPlanetID(); ok {
				return s, func() tea.Msg { return changeScreenMsg{screen: screenShipyard, arg: pid} }
			}
		case "d":
			if pid, ok := s.currentPlanetID(); ok {
				return s, func() tea.Msg { return changeScreenMsg{screen: screenDefense, arg: pid} }
			}
		case "f":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenFleet} }
		case "g":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenGalaxy} }
		case "m":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenMessages} }
		case "l":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenLeaderboard} }
		case "s":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenStats} }
		case "L":
			return s, cmdLogout(s.root.client)
		}
	}
	return s, nil
}

func (s *overviewScreen) loadDetails() tea.Cmd {
	pid, ok := s.currentPlanetID()
	if !ok {
		return nil
	}
	return tea.Batch(
		cmdGetProduction(s.root.client, pid),
		cmdGetQueues(s.root.client, pid),
	)
}

func (s *overviewScreen) currentPlanetID() (int64, bool) {
	if s.cursor < 0 || s.cursor >= len(s.planets) {
		return 0, false
	}
	return s.planets[s.cursor].ID, true
}

func (s *overviewScreen) View() string {
	st := s.root.styles
	if s.loading {
		return st.Muted.Render("loading planets...")
	}
	if len(s.planets) == 0 {
		return st.Muted.Render("no planets - this should not happen; visit your universe to colonise a starter")
	}

	// Left: planet list
	rows := []string{st.Header.Render("Planets")}
	for i, p := range s.planets {
		line := fmt.Sprintf("[%d:%d:%d] %s", p.Galaxy, p.System, p.Position, p.Name)
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	left := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	// Right: details for selected planet
	right := s.renderDetails()

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (s *overviewScreen) renderDetails() string {
	st := s.root.styles
	if s.cursor < 0 || s.cursor >= len(s.planets) {
		return ""
	}
	p := s.planets[s.cursor]

	header := st.Header.Render(fmt.Sprintf("%s  [%d:%d:%d]", p.Name, p.Galaxy, p.System, p.Position))
	resources := lipgloss.JoinVertical(lipgloss.Left,
		st.BarLabel.Render("metal:     ")+st.BarValue.Render(fmtFloat(p.Metal)),
		st.BarLabel.Render("crystal:   ")+st.BarValue.Render(fmtFloat(p.Crystal)),
		st.BarLabel.Render("deuterium: ")+st.BarValue.Render(fmtFloat(p.Deuterium)),
		st.BarLabel.Render("energy:    ")+st.BarValue.Render(fmt.Sprintf("%d/%d", p.EnergyProduced-p.EnergyUsed, p.EnergyProduced)),
		st.BarLabel.Render("fields:    ")+st.BarValue.Render(fmt.Sprintf("%d/%d", p.FieldsUsed, p.FieldsTotal)),
		st.BarLabel.Render("temp:      ")+st.BarValue.Render(fmt.Sprintf("%d to %d C", p.TempMin, p.TempMax)),
	)

	prod := ""
	if s.production != nil {
		prod = st.Muted.Render(fmt.Sprintf("hourly  metal:%s  crystal:%s  deut:%s   factor:%.2f",
			fmtFloat(s.production.MetalPerHour),
			fmtFloat(s.production.CrystalPerHour),
			fmtFloat(s.production.DeuteriumPerHour),
			s.production.ProductionFactor,
		))
	}

	queues := s.renderQueues()

	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		resources,
		"",
		prod,
		"",
		queues,
	))
}

func (s *overviewScreen) renderQueues() string {
	st := s.root.styles
	if len(s.queues) == 0 {
		return st.Muted.Render("queues: idle")
	}
	rows := []string{st.Header.Render("Queue")}
	now := time.Now()
	for _, q := range s.queues {
		remaining := time.Until(q.FinishedAt).Round(time.Second)
		if remaining < 0 {
			remaining = 0
		}
		_ = now
		line := fmt.Sprintf("[%s] %s -> %d  (%s)", q.QueueType, q.ItemKey, q.TargetLevel, remaining)
		if q.QueueType == "ship" || q.QueueType == "defense" {
			line = fmt.Sprintf("[%s] %s x%d  (%s)", q.QueueType, q.ItemKey, q.Count, remaining)
		}
		rows = append(rows, st.Item.Render(line))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func fmtFloat(v float64) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("%.2fM", v/1_000_000)
	}
	if v >= 1_000 {
		return fmt.Sprintf("%.1fk", v/1_000)
	}
	return fmt.Sprintf("%.0f", v)
}

func (s *overviewScreen) Title() string { return "overview" }

func (s *overviewScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "↑/↓", Desc: "planet"},
		{Key: "b", Desc: "build"},
		{Key: "t", Desc: "tech"},
		{Key: "y", Desc: "shipyard"},
		{Key: "d", Desc: "defense"},
		{Key: "f", Desc: "fleet"},
		{Key: "g", Desc: "galaxy"},
		{Key: "m", Desc: "msgs"},
		{Key: "l", Desc: "ldr"},
		{Key: "s", Desc: "stats"},
		{Key: "r", Desc: "refresh"},
		{Key: "L", Desc: "logout"},
	}
}
