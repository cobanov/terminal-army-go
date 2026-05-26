package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// leaderboardScreen shows the global top-N players by score. Read-only; the
// only action is refresh.
type leaderboardScreen struct {
	root    *rootModel
	entries []svc.LeaderboardEntry
	cursor  int
	loading bool
}

func newLeaderboardScreen(root *rootModel) *leaderboardScreen {
	return &leaderboardScreen{root: root, loading: true}
}

func (s *leaderboardScreen) Init() tea.Cmd { return cmdLeaderboard(s.root.client) }

func (s *leaderboardScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case leaderboardLoadedMsg:
		s.entries = m.entries
		s.loading = false
		if s.cursor >= len(s.entries) {
			s.cursor = 0
		}
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.entries)-1 {
				s.cursor++
			}
		case "r":
			s.loading = true
			return s, cmdLeaderboard(s.root.client)
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		}
	}
	return s, nil
}

func (s *leaderboardScreen) View() string {
	st := s.root.styles
	if s.loading {
		return st.Muted.Render("loading leaderboard...")
	}
	rows := []string{
		st.Header.Render("Leaderboard"),
		st.BarLabel.Render(fmt.Sprintf("%-5s %-20s %-10s %-12s", "rank", "player", "alliance", "score")),
	}
	if len(s.entries) == 0 {
		rows = append(rows, st.Muted.Render("(empty)"))
	}
	for i, e := range s.entries {
		line := fmt.Sprintf("%-5d %-20s %-10s %-12d",
			e.Rank, trunc(e.Username, 20), trunc(e.Alliance, 10), e.Score)
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (s *leaderboardScreen) Title() string { return "leaderboard" }

func (s *leaderboardScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "↑/↓", Desc: "scroll"},
		{Key: "r", Desc: "refresh"},
		{Key: "esc", Desc: "back"},
	}
}
