package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// researchScreen shows tech levels and lets the user queue the next tech
// level using the named planet's research lab.
type researchScreen struct {
	root     *rootModel
	planetID int64
	levels   map[string]int
	queues   []svc.QueueItem
	cursor   int
	loading  bool
}

func newResearchScreen(root *rootModel, planetID int64) *researchScreen {
	return &researchScreen{root: root, planetID: planetID, loading: true, levels: map[string]int{}}
}

func (s *researchScreen) Init() tea.Cmd {
	return tea.Batch(
		cmdListResearch(s.root.client),
		cmdGetQueues(s.root.client, s.planetID),
	)
}

func (s *researchScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case researchLoadedMsg:
		s.levels = map[string]int{}
		for _, r := range m.levels {
			s.levels[r.Tech] = r.Level
		}
		s.loading = false
	case queuesLoadedMsg:
		s.queues = m.items
	case queueQueuedMsg:
		return s, tea.Batch(
			cmdStatus(fmt.Sprintf("queued %s -> %d", m.item.ItemKey, m.item.TargetLevel), statusOK),
			cmdListResearch(s.root.client),
			cmdGetQueues(s.root.client, s.planetID),
		)
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(ResearchCatalog)-1 {
				s.cursor++
			}
		case "enter", "u":
			if s.cursor >= 0 && s.cursor < len(ResearchCatalog) {
				return s, cmdQueueResearch(s.root.client, s.planetID, ResearchCatalog[s.cursor].Key)
			}
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		case "r":
			return s, s.Init()
		}
	}
	return s, nil
}

func (s *researchScreen) View() string {
	st := s.root.styles
	if s.loading {
		return st.Muted.Render("loading research...")
	}
	rows := []string{st.Header.Render("Research")}
	for i, item := range ResearchCatalog {
		line := fmt.Sprintf("%-24s  lvl %d", item.Label, s.levels[item.Key])
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	left := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	queueRows := []string{st.Header.Render("Lab queue")}
	if len(s.queues) == 0 {
		queueRows = append(queueRows, st.Muted.Render("(empty)"))
	}
	for _, q := range s.queues {
		if q.QueueType != "research" {
			continue
		}
		queueRows = append(queueRows, st.Item.Render(fmt.Sprintf("%s -> %d", q.ItemKey, q.TargetLevel)))
	}
	right := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, queueRows...))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (s *researchScreen) Title() string { return "research" }

func (s *researchScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "↑/↓", Desc: "select"},
		{Key: "enter", Desc: "queue"},
		{Key: "esc", Desc: "back"},
	}
}
