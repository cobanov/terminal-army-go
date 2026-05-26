package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// fleetScreen lists every fleet that belongs to the user. The selected fleet
// can be recalled. From the home planet selection the user can also jump to
// the dispatch screen via "d" using whichever planet is currently selected
// in the overview. Since this screen does not own a planet cursor, dispatch
// here falls back to the first owned planet.
type fleetScreen struct {
	root    *rootModel
	fleets  []svc.Fleet
	cursor  int
	loading bool
}

func newFleetScreen(root *rootModel) *fleetScreen {
	return &fleetScreen{root: root, loading: true}
}

func (s *fleetScreen) Init() tea.Cmd {
	return tea.Batch(
		cmdListFleet(s.root.client),
		cmdListPlanets(s.root.client),
	)
}

func (s *fleetScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case fleetsLoadedMsg:
		s.fleets = m.fleets
		s.loading = false
		if s.cursor >= len(s.fleets) {
			s.cursor = 0
		}
	case fleetDispatchedMsg:
		return s, tea.Batch(
			cmdStatus(fmt.Sprintf("fleet %d updated", m.fleet.ID), statusOK),
			cmdListFleet(s.root.client),
		)
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.fleets)-1 {
				s.cursor++
			}
		case "r":
			s.loading = true
			return s, cmdListFleet(s.root.client)
		case "x":
			if s.cursor >= 0 && s.cursor < len(s.fleets) {
				f := s.fleets[s.cursor]
				if f.State == "outbound" {
					return s, cmdRecallFleet(s.root.client, f.ID)
				}
				return s, cmdStatus("only outbound fleets can be recalled", statusWarn)
			}
		case "d":
			if len(s.root.planets) == 0 {
				return s, cmdStatus("no planets to dispatch from", statusWarn)
			}
			pid := s.root.planets[0].ID
			return s, func() tea.Msg {
				return changeScreenMsg{screen: screenFleetDispatch, arg: pid}
			}
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		}
	}
	return s, nil
}

func (s *fleetScreen) View() string {
	st := s.root.styles
	if s.loading {
		return st.Muted.Render("loading fleets...")
	}
	rows := []string{st.Header.Render("Fleets")}
	if len(s.fleets) == 0 {
		rows = append(rows, st.Muted.Render("(none in flight)"))
	}
	now := time.Now()
	for i, f := range s.fleets {
		eta := time.Until(f.ArrivalAt).Round(time.Second)
		if eta < 0 {
			eta = 0
		}
		retEta := ""
		if f.ReturnAt != nil {
			r := time.Until(*f.ReturnAt).Round(time.Second)
			if r < 0 {
				r = 0
			}
			retEta = fmt.Sprintf("  ret %s", r)
		}
		line := fmt.Sprintf("#%d  %-9s  %-9s  -> [%d:%d:%d]  eta %s%s",
			f.ID, f.Mission, f.State,
			f.TargetGalaxy, f.TargetSystem, f.TargetPosition,
			eta, retEta,
		)
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
		_ = now
	}
	left := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	right := s.renderDetails()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (s *fleetScreen) renderDetails() string {
	st := s.root.styles
	if s.cursor < 0 || s.cursor >= len(s.fleets) {
		return ""
	}
	f := s.fleets[s.cursor]
	rows := []string{
		st.Header.Render(fmt.Sprintf("Fleet #%d", f.ID)),
		st.BarLabel.Render("mission:   ") + st.BarValue.Render(f.Mission),
		st.BarLabel.Render("state:     ") + st.BarValue.Render(f.State),
		st.BarLabel.Render("departure: ") + st.BarValue.Render(f.DepartureAt.Format(time.RFC822)),
		st.BarLabel.Render("arrival:   ") + st.BarValue.Render(f.ArrivalAt.Format(time.RFC822)),
	}
	if f.ReturnAt != nil {
		rows = append(rows, st.BarLabel.Render("return:    ")+st.BarValue.Render(f.ReturnAt.Format(time.RFC822)))
	}
	rows = append(rows, "", st.Header.Render("Ships"))
	if len(f.Ships) == 0 {
		rows = append(rows, st.Muted.Render("(none)"))
	}
	for k, v := range f.Ships {
		rows = append(rows, st.Item.Render(fmt.Sprintf("%-18s x %d", k, v)))
	}
	if len(f.Cargo) > 0 {
		rows = append(rows, "", st.Header.Render("Cargo"))
		for k, v := range f.Cargo {
			rows = append(rows, st.Item.Render(fmt.Sprintf("%-10s %d", k, v)))
		}
	}
	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (s *fleetScreen) Title() string { return "fleet" }

func (s *fleetScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "↑/↓", Desc: "select"},
		{Key: "d", Desc: "dispatch"},
		{Key: "x", Desc: "recall"},
		{Key: "r", Desc: "refresh"},
		{Key: "esc", Desc: "back"},
	}
}
