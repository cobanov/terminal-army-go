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

// fleetDispatchScreen is a small form that builds a FleetDispatchRequest and
// fires it. Fields are entered as text inputs to keep the UI compact. Ships
// are specified as comma separated key=count pairs (e.g. "light_fighter=20,small_cargo=5")
// so we do not need a long row per ship class. Cargo is similar.
type fleetDispatchScreen struct {
	root     *rootModel
	planetID int64
	inputs   []textinput.Model
	focus    int
}

// field order; mirrors the order rendered in View().
const (
	fldGalaxy   = 0
	fldSystem   = 1
	fldPosition = 2
	fldMission  = 3
	fldShips    = 4
	fldCargo    = 5
	fldCount    = 6
)

func newFleetDispatchScreen(root *rootModel, planetID int64) *fleetDispatchScreen {
	mk := func(placeholder string, width int) textinput.Model {
		in := textinput.New()
		in.Placeholder = placeholder
		in.Width = width
		in.CharLimit = 96
		return in
	}
	inputs := make([]textinput.Model, fldCount)
	inputs[fldGalaxy] = mk("galaxy (1-9)", 6)
	inputs[fldSystem] = mk("system (1-499)", 6)
	inputs[fldPosition] = mk("position (1-15)", 6)
	inputs[fldMission] = mk("attack|transport|colonize|deploy|espionage|recycle", 40)
	inputs[fldShips] = mk("light_fighter=20,small_cargo=5", 60)
	inputs[fldCargo] = mk("metal=1000,crystal=500,deuterium=0 (optional)", 60)
	inputs[0].Focus()
	return &fleetDispatchScreen{root: root, planetID: planetID, inputs: inputs}
}

func (s *fleetDispatchScreen) Init() tea.Cmd { return textinput.Blink }

func (s *fleetDispatchScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case fleetDispatchedMsg:
		return s, tea.Batch(
			cmdStatus(fmt.Sprintf("fleet %d dispatched", m.fleet.ID), statusOK),
			func() tea.Msg { return changeScreenMsg{screen: screenFleet} },
		)
	case tea.KeyMsg:
		switch m.String() {
		case "esc":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenFleet} }
		case "tab", "down":
			s.cycle(1)
			return s, nil
		case "shift+tab", "up":
			s.cycle(-1)
			return s, nil
		case "enter":
			if s.focus < fldCount-1 {
				s.cycle(1)
				return s, nil
			}
			return s, s.submit()
		}
	}
	var cmd tea.Cmd
	s.inputs[s.focus], cmd = s.inputs[s.focus].Update(msg)
	return s, cmd
}

func (s *fleetDispatchScreen) cycle(delta int) {
	s.inputs[s.focus].Blur()
	s.focus = (s.focus + delta + fldCount) % fldCount
	s.inputs[s.focus].Focus()
}

func (s *fleetDispatchScreen) submit() tea.Cmd {
	g, err := strconv.Atoi(strings.TrimSpace(s.inputs[fldGalaxy].Value()))
	if err != nil {
		return cmdStatus("galaxy must be an integer", statusErr)
	}
	sys, err := strconv.Atoi(strings.TrimSpace(s.inputs[fldSystem].Value()))
	if err != nil {
		return cmdStatus("system must be an integer", statusErr)
	}
	pos, err := strconv.Atoi(strings.TrimSpace(s.inputs[fldPosition].Value()))
	if err != nil {
		return cmdStatus("position must be an integer", statusErr)
	}
	mission := strings.ToLower(strings.TrimSpace(s.inputs[fldMission].Value()))
	if mission == "" {
		return cmdStatus("mission is required", statusErr)
	}
	ships, err := parseKV(s.inputs[fldShips].Value())
	if err != nil {
		return cmdStatus("ships: "+err.Error(), statusErr)
	}
	if len(ships) == 0 {
		return cmdStatus("at least one ship is required", statusErr)
	}
	cargo := map[string]int{}
	if v := strings.TrimSpace(s.inputs[fldCargo].Value()); v != "" {
		cargo, err = parseKV(v)
		if err != nil {
			return cmdStatus("cargo: "+err.Error(), statusErr)
		}
	}
	req := svc.FleetDispatchRequest{
		OriginPlanetID: s.planetID,
		TargetGalaxy:   g,
		TargetSystem:   sys,
		TargetPosition: pos,
		Mission:        mission,
		Ships:          ships,
		Cargo:          cargo,
	}
	return cmdDispatchFleet(s.root.client, req)
}

// parseKV turns a comma separated key=count list into a map. Whitespace
// around keys/values is ignored. Empty input returns an empty map.
func parseKV(s string) (map[string]int, error) {
	out := map[string]int{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			return nil, fmt.Errorf("missing '=' in %q", pair)
		}
		k := strings.TrimSpace(pair[:eq])
		vStr := strings.TrimSpace(pair[eq+1:])
		v, err := strconv.Atoi(vStr)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("invalid count %q for %q", vStr, k)
		}
		out[k] = v
	}
	return out, nil
}

func (s *fleetDispatchScreen) View() string {
	st := s.root.styles
	labels := []string{"galaxy:", "system:", "position:", "mission:", "ships:", "cargo:"}
	rows := []string{st.Header.Render("Dispatch fleet")}
	for i := 0; i < fldCount; i++ {
		row := st.BarLabel.Render(fmt.Sprintf("%-10s ", labels[i])) + s.inputs[i].View()
		if i == s.focus {
			row = "> " + row
		} else {
			row = "  " + row
		}
		rows = append(rows, row)
	}
	rows = append(rows, "", st.Muted.Render("Tab to move between fields; Enter on the last field submits."))
	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (s *fleetDispatchScreen) Title() string { return "dispatch" }

func (s *fleetDispatchScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "tab", Desc: "next field"},
		{Key: "enter", Desc: "next/submit"},
		{Key: "esc", Desc: "cancel"},
	}
}
