package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// universePickerScreen lists every universe and lets the user join one.
// Shown right after registration or when /me reports no current universe.
type universePickerScreen struct {
	root      *rootModel
	universes []svc.Universe
	cursor    int
	loading   bool
}

func newUniversePickerScreen(root *rootModel) *universePickerScreen {
	return &universePickerScreen{root: root, loading: true}
}

func (s *universePickerScreen) Init() tea.Cmd { return cmdListUniverses(s.root.client) }

func (s *universePickerScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case universesLoadedMsg:
		s.universes = m.universes
		s.loading = false
	case universeJoinedMsg:
		// session has changed; re-fetch /me to land in the overview
		return s, cmdMe(s.root.client)
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.universes)-1 {
				s.cursor++
			}
		case "enter":
			if s.cursor >= 0 && s.cursor < len(s.universes) {
				return s, cmdJoinUniverse(s.root.client, s.universes[s.cursor].ID)
			}
		case "r":
			s.loading = true
			return s, cmdListUniverses(s.root.client)
		}
	}
	return s, nil
}

func (s *universePickerScreen) View() string {
	st := s.root.styles
	if s.loading {
		return st.Muted.Render("loading universes...")
	}
	if len(s.universes) == 0 {
		return st.Muted.Render("no universes available - ask an admin to create one")
	}

	rows := make([]string, 0, len(s.universes))
	rows = append(rows, st.Header.Render("Choose a universe"))
	rows = append(rows, "")
	for i, u := range s.universes {
		line := fmt.Sprintf("%-24s  econ x%d  fleet x%d  research x%d  %d/%d players",
			u.Name, u.SpeedEconomy, u.SpeedFleet, u.SpeedResearch, u.PlayerCount, u.GalaxiesCount*u.SystemsCount*15)
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...)))
}

func (s *universePickerScreen) Title() string { return "join a universe" }

func (s *universePickerScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "↑/↓", Desc: "select"},
		{Key: "enter", Desc: "join"},
		{Key: "r", Desc: "refresh"},
	}
}
