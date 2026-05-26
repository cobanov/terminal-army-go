package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// galaxyScreen displays one star system as a 15 slot table. Galaxy and
// system numbers can be edited via small text inputs; pressing Enter loads
// the new system. The screen also remembers the last position the cursor
// sat at so re-loading the same system keeps focus.
type galaxyScreen struct {
	root    *rootModel
	view    *svc.SystemView
	loading bool
	cursor  int
	editing bool
	galInp  textinput.Model
	sysInp  textinput.Model
	focus   int // 0 = galaxy input, 1 = system input
}

func newGalaxyScreen(root *rootModel) *galaxyScreen {
	gIn := textinput.New()
	gIn.Placeholder = "g"
	gIn.Width = 4
	gIn.CharLimit = 3
	sIn := textinput.New()
	sIn.Placeholder = "s"
	sIn.Width = 5
	sIn.CharLimit = 4

	g, s := 1, 1
	if len(root.planets) > 0 {
		g = root.planets[0].Galaxy
		s = root.planets[0].System
	}
	gIn.SetValue(strconv.Itoa(g))
	sIn.SetValue(strconv.Itoa(s))
	return &galaxyScreen{
		root:    root,
		loading: true,
		galInp:  gIn,
		sysInp:  sIn,
	}
}

func (s *galaxyScreen) Init() tea.Cmd {
	g, _ := strconv.Atoi(s.galInp.Value())
	sy, _ := strconv.Atoi(s.sysInp.Value())
	return cmdViewSystem(s.root.client, g, sy)
}

func (s *galaxyScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case systemLoadedMsg:
		s.view = m.view
		s.loading = false
	case tea.KeyMsg:
		if s.editing {
			switch m.String() {
			case "esc":
				s.editing = false
				s.galInp.Blur()
				s.sysInp.Blur()
				return s, nil
			case "enter":
				g, err := strconv.Atoi(strings.TrimSpace(s.galInp.Value()))
				if err != nil || g < 1 {
					return s, cmdStatus("galaxy must be a positive integer", statusWarn)
				}
				sy, err := strconv.Atoi(strings.TrimSpace(s.sysInp.Value()))
				if err != nil || sy < 1 {
					return s, cmdStatus("system must be a positive integer", statusWarn)
				}
				s.editing = false
				s.galInp.Blur()
				s.sysInp.Blur()
				s.loading = true
				return s, cmdViewSystem(s.root.client, g, sy)
			case "tab", "right":
				if s.focus == 0 {
					s.galInp.Blur()
					s.sysInp.Focus()
					s.focus = 1
				} else {
					s.sysInp.Blur()
					s.galInp.Focus()
					s.focus = 0
				}
				return s, nil
			}
			var cmd tea.Cmd
			if s.focus == 0 {
				s.galInp, cmd = s.galInp.Update(msg)
			} else {
				s.sysInp, cmd = s.sysInp.Update(msg)
			}
			return s, cmd
		}
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < 14 {
				s.cursor++
			}
		case "left", "h":
			g, _ := strconv.Atoi(s.galInp.Value())
			sy, _ := strconv.Atoi(s.sysInp.Value())
			if sy > 1 {
				sy--
				s.sysInp.SetValue(strconv.Itoa(sy))
				s.loading = true
				return s, cmdViewSystem(s.root.client, g, sy)
			}
		case "right", "l":
			g, _ := strconv.Atoi(s.galInp.Value())
			sy, _ := strconv.Atoi(s.sysInp.Value())
			sy++
			s.sysInp.SetValue(strconv.Itoa(sy))
			s.loading = true
			return s, cmdViewSystem(s.root.client, g, sy)
		case "G":
			s.editing = true
			s.focus = 0
			s.galInp.Focus()
			return s, textinput.Blink
		case "r":
			g, _ := strconv.Atoi(s.galInp.Value())
			sy, _ := strconv.Atoi(s.sysInp.Value())
			s.loading = true
			return s, cmdViewSystem(s.root.client, g, sy)
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		}
	}
	return s, nil
}

func (s *galaxyScreen) View() string {
	st := s.root.styles
	g, _ := strconv.Atoi(s.galInp.Value())
	sy, _ := strconv.Atoi(s.sysInp.Value())

	header := st.Header.Render(fmt.Sprintf("Galaxy %d  System %d", g, sy))
	if s.editing {
		header = lipgloss.JoinHorizontal(lipgloss.Left,
			st.Header.Render("Jump to  "),
			st.BarLabel.Render("galaxy:"), s.galInp.View(),
			st.BarLabel.Render("  system:"), s.sysInp.View(),
		)
	}

	if s.loading || s.view == nil {
		return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left,
			header, "", st.Muted.Render("loading system..."),
		))
	}

	// Build a map of position -> slot so empty positions still render.
	occupied := map[int]svc.SystemPlanetView{}
	for _, p := range s.view.Planets {
		occupied[p.Position] = p
	}
	rows := []string{header, ""}
	rows = append(rows, st.BarLabel.Render(" pos  planet                owner            alliance  status"))
	for i := 1; i <= 15; i++ {
		var line string
		if p, ok := occupied[i]; ok {
			status := ""
			if p.Online {
				status = "online"
			}
			line = fmt.Sprintf(" %2d   %-20s  %-15s  %-8s  %s",
				i, trunc(p.PlanetName, 20), trunc(p.OwnerName, 15), trunc(p.AllianceTag, 8), status)
		} else {
			line = fmt.Sprintf(" %2d   %s", i, st.Muted.Render("(empty)"))
		}
		idx := i - 1
		if idx == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "."
}

func (s *galaxyScreen) Title() string { return "galaxy" }

func (s *galaxyScreen) Help() []HelpEntry {
	if s.editing {
		return []HelpEntry{
			{Key: "tab", Desc: "switch"},
			{Key: "enter", Desc: "jump"},
			{Key: "esc", Desc: "cancel"},
		}
	}
	return []HelpEntry{
		{Key: "↑/↓", Desc: "slot"},
		{Key: "←/→", Desc: "system -/+"},
		{Key: "G", Desc: "goto"},
		{Key: "r", Desc: "refresh"},
		{Key: "esc", Desc: "back"},
	}
}
