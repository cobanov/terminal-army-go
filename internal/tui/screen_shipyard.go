package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// shipyardScreen lists the ship counts on a planet and lets the user queue
// new ones. A small text input toggles when the user presses Enter so they
// can type the quantity.
type shipyardScreen struct {
	root      *rootModel
	planetID  int64
	planet    *svc.Planet
	queues    []svc.QueueItem
	cursor    int
	loading   bool
	countInp  textinput.Model
	editing   bool
}

func newShipyardScreen(root *rootModel, planetID int64) *shipyardScreen {
	in := textinput.New()
	in.Placeholder = "count"
	in.Width = 8
	in.CharLimit = 8
	return &shipyardScreen{root: root, planetID: planetID, loading: true, countInp: in}
}

func (s *shipyardScreen) Init() tea.Cmd {
	return tea.Batch(
		cmdGetPlanet(s.root.client, s.planetID),
		cmdGetQueues(s.root.client, s.planetID),
	)
}

func (s *shipyardScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case planetLoadedMsg:
		s.planet = m.planet
		s.loading = false
	case queuesLoadedMsg:
		s.queues = m.items
	case queueQueuedMsg:
		s.editing = false
		s.countInp.SetValue("")
		s.countInp.Blur()
		return s, tea.Batch(
			cmdStatus(fmt.Sprintf("queued %d x %s", m.item.Count, m.item.ItemKey), statusOK),
			cmdGetPlanet(s.root.client, s.planetID),
			cmdGetQueues(s.root.client, s.planetID),
		)
	case tea.KeyMsg:
		if s.editing {
			switch m.String() {
			case "esc":
				s.editing = false
				s.countInp.Blur()
				return s, nil
			case "enter":
				v, err := strconv.Atoi(s.countInp.Value())
				if err != nil || v <= 0 {
					return s, cmdStatus("count must be a positive integer", statusWarn)
				}
				key := ShipCatalog[s.cursor].Key
				return s, cmdQueueShip(s.root.client, s.planetID, key, v)
			}
			var cmd tea.Cmd
			s.countInp, cmd = s.countInp.Update(msg)
			return s, cmd
		}
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(ShipCatalog)-1 {
				s.cursor++
			}
		case "enter", "u":
			s.editing = true
			s.countInp.Focus()
			s.countInp.SetValue("")
			return s, textinput.Blink
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		case "r":
			return s, s.Init()
		}
	}
	return s, nil
}

func (s *shipyardScreen) View() string {
	st := s.root.styles
	if s.loading || s.planet == nil {
		return st.Muted.Render("loading planet...")
	}
	rows := []string{st.Header.Render("Shipyard - " + s.planet.Name)}
	for i, item := range ShipCatalog {
		have := s.planet.Ships[item.Key]
		line := fmt.Sprintf("%-22s  x %d", item.Label, have)
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	if s.editing {
		rows = append(rows, "", st.Muted.Render("how many "+ShipCatalog[s.cursor].Label+"?"), s.countInp.View())
	}
	left := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	queueRows := []string{st.Header.Render("Shipyard queue")}
	if len(s.queues) == 0 {
		queueRows = append(queueRows, st.Muted.Render("(empty)"))
	}
	for _, q := range s.queues {
		if q.QueueType != "ship" {
			continue
		}
		queueRows = append(queueRows, st.Item.Render(fmt.Sprintf("%s x %d", q.ItemKey, q.Count)))
	}
	right := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, queueRows...))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (s *shipyardScreen) Title() string { return "shipyard" }

func (s *shipyardScreen) Help() []HelpEntry {
	if s.editing {
		return []HelpEntry{
			{Key: "enter", Desc: "queue"},
			{Key: "esc", Desc: "cancel"},
		}
	}
	return []HelpEntry{
		{Key: "↑/↓", Desc: "select"},
		{Key: "enter", Desc: "queue"},
		{Key: "esc", Desc: "back"},
	}
}
