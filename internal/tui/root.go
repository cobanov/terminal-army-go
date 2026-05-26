// Package tui hosts the Bubble Tea client. The root model owns the active
// screen, a shared HTTP client, current user state, and a small in-memory
// cache of universes/planets so the user can hop between screens without
// re-fetching on every keystroke.
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/tui/client"
)

// rootModel is the top-level Bubble Tea model. Sub-screens implement the
// Screen interface; the root forwards Update/View calls and renders the
// surrounding chrome (title bar, status line, help footer).
type rootModel struct {
	client   *client.Client
	styles   *Styles
	user     *svc.User
	planets  []svc.Planet
	width    int
	height   int
	active   Screen
	status   statusMsg
	hasError bool
}

// newRootModel constructs the initial model. When cached creds are present
// we start with a quick /me probe; otherwise we drop straight into the
// login screen.
func newRootModel(c *client.Client) *rootModel {
	m := &rootModel{
		client: c,
		styles: NewStyles(),
	}
	m.active = newLoginScreen(m)
	return m
}

// Init kicks off the first tea.Cmd. If we already have a token we verify it
// by calling /me, otherwise we show the login screen straight away.
func (m *rootModel) Init() tea.Cmd {
	if m.client.Token() != "" {
		return cmdMe(m.client)
	}
	return m.active.Init()
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// continue: also forward to active screen below
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+q":
			return m, tea.Quit
		}
	case errMsg:
		m.hasError = true
		return m, cmdStatus(msg.Error(), statusErr)
	case statusMsg:
		m.status = msg
		return m, nil
	case clearStatusMsg:
		m.status = statusMsg{}
		return m, nil
	case sessionReadyMsg:
		m.user = msg.user
		m.hasError = false
		// pick destination based on whether user is in a universe
		if msg.user.CurrentUniverseID == nil {
			m.active = newUniversePickerScreen(m)
		} else {
			m.active = newOverviewScreen(m)
		}
		return m, m.active.Init()
	case loggedOutMsg:
		m.user = nil
		m.planets = nil
		_ = ClearCreds(m.client.BaseURL())
		m.active = newLoginScreen(m)
		return m, m.active.Init()
	case changeScreenMsg:
		switch msg.screen {
		case screenLogin:
			m.active = newLoginScreen(m)
		case screenRegister:
			m.active = newRegisterScreen(m)
		case screenUniversePicker:
			m.active = newUniversePickerScreen(m)
		case screenOverview:
			m.active = newOverviewScreen(m)
		case screenBuildings:
			pid, _ := msg.arg.(int64)
			m.active = newBuildingsScreen(m, pid)
		case screenResearch:
			pid, _ := msg.arg.(int64)
			m.active = newResearchScreen(m, pid)
		case screenShipyard:
			pid, _ := msg.arg.(int64)
			m.active = newShipyardScreen(m, pid)
		case screenDefense:
			pid, _ := msg.arg.(int64)
			m.active = newDefenseScreen(m, pid)
		case screenFleet:
			m.active = newFleetScreen(m)
		case screenFleetDispatch:
			pid, _ := msg.arg.(int64)
			m.active = newFleetDispatchScreen(m, pid)
		case screenGalaxy:
			m.active = newGalaxyScreen(m)
		case screenMessages:
			m.active = newMessagesScreen(m)
		case screenLeaderboard:
			m.active = newLeaderboardScreen(m)
		case screenStats:
			m.active = newStatsScreen(m)
		}
		return m, m.active.Init()
	case planetsLoadedMsg:
		m.planets = msg.planets
	}

	if m.active == nil {
		return m, nil
	}
	next, cmd := m.active.Update(msg)
	m.active = next
	return m, cmd
}

func (m *rootModel) View() string {
	if m.width == 0 {
		return "loading..."
	}
	if m.active == nil {
		return "no screen"
	}

	title := m.styles.Title.Render(fmt.Sprintf(" tarmy  -  %s ", m.active.Title()))
	right := ""
	if m.user != nil {
		right = m.styles.Muted.Render(fmt.Sprintf(" %s ", m.user.Username))
	}
	pad := m.width - lipgloss.Width(title) - lipgloss.Width(right)
	if pad < 0 {
		pad = 0
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Left, title, strings.Repeat(" ", pad), right)

	body := m.active.View()

	help := m.renderHelp()
	status := m.renderStatus()

	return lipgloss.JoinVertical(lipgloss.Left, bar, body, status, help)
}

func (m *rootModel) renderHelp() string {
	entries := append([]HelpEntry{}, m.active.Help()...)
	entries = append(entries, HelpEntry{Key: "ctrl+q", Desc: "quit"})
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, m.styles.Tag.Render(e.Key)+m.styles.Help.Render(" "+e.Desc))
	}
	return m.styles.Help.Render(strings.Join(parts, "  "))
}

func (m *rootModel) renderStatus() string {
	if m.status.text == "" {
		return ""
	}
	switch m.status.level {
	case statusErr:
		return m.styles.Error.Render(" " + m.status.text + " ")
	case statusOK:
		return m.styles.Success.Render(" " + m.status.text + " ")
	case statusWarn:
		return m.styles.Tag.Render(" " + m.status.text + " ")
	default:
		return m.styles.Muted.Render(" " + m.status.text + " ")
	}
}

// Run starts the interactive client pointing at serverURL. The ctx is
// honoured by tea.NewProgram via WithContext so SIGINT cancels the program.
func Run(ctx context.Context, serverURL string) error {
	c := client.New(serverURL)

	if cached := LoadCreds(serverURL); cached != nil {
		c.SetToken(cached.Token)
	}

	m := newRootModel(c)
	p := tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
