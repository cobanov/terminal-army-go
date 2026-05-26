package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// loginScreen handles credential entry. It uses two bubbles/textinput models
// (username + password) plus three navigation choices (login, register,
// quit) toggled with Tab/Shift+Tab.
type loginScreen struct {
	root   *rootModel
	inputs []textinput.Model
	focus  int
	submit bool
}

func newLoginScreen(root *rootModel) *loginScreen {
	user := textinput.New()
	user.Placeholder = "username"
	user.CharLimit = 64
	user.Width = 32
	user.Focus()

	pass := textinput.New()
	pass.Placeholder = "password"
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '*'
	pass.CharLimit = 128
	pass.Width = 32

	return &loginScreen{
		root:   root,
		inputs: []textinput.Model{user, pass},
		focus:  0,
	}
}

func (s *loginScreen) Init() tea.Cmd { return textinput.Blink }

func (s *loginScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			s.focusNext(1)
			return s, textinput.Blink
		case "shift+tab", "up":
			s.focusNext(-1)
			return s, textinput.Blink
		case "ctrl+r":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenRegister} }
		case "enter":
			if s.submit || s.focus == 1 {
				return s, s.submitLogin()
			}
			s.focusNext(1)
			return s, textinput.Blink
		}
	}

	var cmds []tea.Cmd
	for i := range s.inputs {
		var cmd tea.Cmd
		s.inputs[i], cmd = s.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return s, tea.Batch(cmds...)
}

func (s *loginScreen) focusNext(delta int) {
	s.inputs[s.focus].Blur()
	s.focus = (s.focus + delta + len(s.inputs)) % len(s.inputs)
	s.inputs[s.focus].Focus()
	s.submit = false
}

func (s *loginScreen) submitLogin() tea.Cmd {
	user := strings.TrimSpace(s.inputs[0].Value())
	pass := s.inputs[1].Value()
	if user == "" || pass == "" {
		return cmdStatus("username and password required", statusWarn)
	}
	// On success the auth cmd installs the token and we cache it locally.
	c := s.root.client
	return tea.Batch(
		cmdLogin(c, user, pass),
		func() tea.Msg {
			// fire-and-forget save - actual save happens in sessionReadyMsg handler
			return nil
		},
	)
}

func (s *loginScreen) View() string {
	st := s.root.styles
	banner := st.Title.Render("Terminal Army")
	tagline := st.Muted.Render("TUI client for the tarmy server")
	form := lipgloss.JoinVertical(lipgloss.Left,
		st.Header.Render("Sign in"),
		"",
		st.Muted.Render("username:"),
		s.inputs[0].View(),
		"",
		st.Muted.Render("password:"),
		s.inputs[1].View(),
		"",
		st.Muted.Render("server: ")+st.Input.Render(s.root.client.BaseURL()),
	)
	form = st.Panel.Render(form)

	return lipgloss.JoinVertical(lipgloss.Left, "", banner, tagline, "", form)
}

func (s *loginScreen) Title() string { return "login" }

func (s *loginScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "tab", Desc: "next field"},
		{Key: "enter", Desc: "submit"},
		{Key: "ctrl+r", Desc: "create account"},
	}
}
