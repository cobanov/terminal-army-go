package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// buildingsScreen shows the current building levels for a planet and lets
// the user enqueue the next upgrade.
type buildingsScreen struct {
	root     *rootModel
	planetID int64
	planet   *svc.Planet
	queues   []svc.QueueItem
	cursor   int
	loading  bool
}

func newBuildingsScreen(root *rootModel, planetID int64) *buildingsScreen {
	return &buildingsScreen{root: root, planetID: planetID, loading: true}
}

func (s *buildingsScreen) Init() tea.Cmd {
	return tea.Batch(
		cmdGetPlanet(s.root.client, s.planetID),
		cmdGetQueues(s.root.client, s.planetID),
	)
}

func (s *buildingsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case planetLoadedMsg:
		s.planet = m.planet
		s.loading = false
	case queuesLoadedMsg:
		s.queues = m.items
	case queueQueuedMsg:
		return s, tea.Batch(
			cmdStatus(fmt.Sprintf("queued %s -> %d", m.item.ItemKey, m.item.TargetLevel), statusOK),
			cmdGetPlanet(s.root.client, s.planetID),
			cmdGetQueues(s.root.client, s.planetID),
		)
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(BuildingCatalog)-1 {
				s.cursor++
			}
		case "enter", "u":
			if s.cursor >= 0 && s.cursor < len(BuildingCatalog) {
				return s, cmdQueueBuilding(s.root.client, s.planetID, BuildingCatalog[s.cursor].Key)
			}
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		case "r":
			return s, s.Init()
		}
	}
	return s, nil
}

func (s *buildingsScreen) View() string {
	st := s.root.styles
	if s.loading || s.planet == nil {
		return st.Muted.Render("loading planet...")
	}

	rows := []string{st.Header.Render("Buildings on " + s.planet.Name)}
	for i, item := range BuildingCatalog {
		level := s.planet.Buildings[item.Key]
		line := fmt.Sprintf("%-22s  lvl %d", item.Label, level)
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	left := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	right := s.renderQueuePanel()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (s *buildingsScreen) renderQueuePanel() string {
	st := s.root.styles
	rows := []string{st.Header.Render("Queue")}
	if len(s.queues) == 0 {
		rows = append(rows, st.Muted.Render("(empty)"))
	}
	for _, q := range s.queues {
		if q.QueueType != "building" {
			continue
		}
		line := fmt.Sprintf("%s -> %d  (%s)", q.ItemKey, q.TargetLevel, q.FinishedAt.Sub(q.StartedAt))
		rows = append(rows, st.Item.Render(line))
	}
	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (s *buildingsScreen) Title() string {
	if s.planet != nil {
		return "buildings - " + s.planet.Name
	}
	return "buildings"
}

func (s *buildingsScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "↑/↓", Desc: "select"},
		{Key: "enter", Desc: "upgrade"},
		{Key: "r", Desc: "refresh"},
		{Key: "esc", Desc: "back"},
	}
}
