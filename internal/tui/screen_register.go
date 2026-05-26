package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// registerScreen mirrors loginScreen but with three fields.
type registerScreen struct {
	root   *rootModel
	inputs []textinput.Model
	focus  int
}

func newRegisterScreen(root *rootModel) *registerScreen {
	user := textinput.New()
	user.Placeholder = "username (3-32)"
	user.CharLimit = 32
	user.Width = 32
	user.Focus()

	email := textinput.New()
	email.Placeholder = "email"
	email.CharLimit = 128
	email.Width = 32

	pass := textinput.New()
	pass.Placeholder = "password (>=8)"
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '*'
	pass.CharLimit = 128
	pass.Width = 32

	return &registerScreen{
		root:   root,
		inputs: []textinput.Model{user, email, pass},
	}
}

func (s *registerScreen) Init() tea.Cmd { return textinput.Blink }

func (s *registerScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "tab", "down":
			s.focusNext(1)
			return s, textinput.Blink
		case "shift+tab", "up":
			s.focusNext(-1)
			return s, textinput.Blink
		case "esc":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenLogin} }
		case "enter":
			if s.focus == len(s.inputs)-1 {
				return s, s.submit()
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

func (s *registerScreen) focusNext(delta int) {
	s.inputs[s.focus].Blur()
	s.focus = (s.focus + delta + len(s.inputs)) % len(s.inputs)
	s.inputs[s.focus].Focus()
}

func (s *registerScreen) submit() tea.Cmd {
	u := strings.TrimSpace(s.inputs[0].Value())
	e := strings.TrimSpace(s.inputs[1].Value())
	p := s.inputs[2].Value()
	if u == "" || e == "" || len(p) < 8 {
		return cmdStatus("all fields required, password >= 8 chars", statusWarn)
	}
	return cmdRegister(s.root.client, u, e, p)
}

func (s *registerScreen) View() string {
	st := s.root.styles
	form := lipgloss.JoinVertical(lipgloss.Left,
		st.Header.Render("Create an account"),
		"",
		st.Muted.Render("username:"),
		s.inputs[0].View(),
		"",
		st.Muted.Render("email:"),
		s.inputs[1].View(),
		"",
		st.Muted.Render("password:"),
		s.inputs[2].View(),
	)
	return st.Panel.Render(form)
}

func (s *registerScreen) Title() string { return "register" }

func (s *registerScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "tab", Desc: "next field"},
		{Key: "enter", Desc: "submit"},
		{Key: "esc", Desc: "back to login"},
	}
}
