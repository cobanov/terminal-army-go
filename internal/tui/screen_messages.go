package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// messagesScreen displays the inbox. The list pane is on the left and the
// body of the selected message on the right. Reading a message also marks
// it as read on the server (the GET endpoint does this for us).
type messagesScreen struct {
	root    *rootModel
	items   []svc.Message
	current *svc.Message
	cursor  int
	loading bool
}

func newMessagesScreen(root *rootModel) *messagesScreen {
	return &messagesScreen{root: root, loading: true}
}

func (s *messagesScreen) Init() tea.Cmd { return cmdListMessages(s.root.client) }

func (s *messagesScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case messagesLoadedMsg:
		s.items = m.items
		s.loading = false
		if s.cursor >= len(s.items) {
			s.cursor = 0
		}
		if len(s.items) > 0 {
			return s, s.loadCurrent()
		}
		s.current = nil
	case messageLoadedMsg:
		s.current = m.msg
		// reflect read state in list view
		for i := range s.items {
			if s.items[i].ID == m.msg.ID {
				s.items[i].Read = true
			}
		}
	case messageDeletedMsg:
		return s, tea.Batch(
			cmdStatus("message deleted", statusOK),
			cmdListMessages(s.root.client),
		)
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
				return s, s.loadCurrent()
			}
		case "down", "j":
			if s.cursor < len(s.items)-1 {
				s.cursor++
				return s, s.loadCurrent()
			}
		case "r":
			s.loading = true
			return s, cmdListMessages(s.root.client)
		case "x", "delete":
			if s.cursor >= 0 && s.cursor < len(s.items) {
				return s, cmdDeleteMessage(s.root.client, s.items[s.cursor].ID)
			}
		case "esc", "q":
			return s, func() tea.Msg { return changeScreenMsg{screen: screenOverview} }
		}
	}
	return s, nil
}

func (s *messagesScreen) loadCurrent() tea.Cmd {
	if s.cursor < 0 || s.cursor >= len(s.items) {
		return nil
	}
	return cmdGetMessage(s.root.client, s.items[s.cursor].ID)
}

func (s *messagesScreen) View() string {
	st := s.root.styles
	if s.loading {
		return st.Muted.Render("loading messages...")
	}
	rows := []string{st.Header.Render("Inbox")}
	if len(s.items) == 0 {
		rows = append(rows, st.Muted.Render("(empty)"))
	}
	for i, m := range s.items {
		mark := " "
		if !m.Read {
			mark = "*"
		}
		line := fmt.Sprintf("%s [%s] %s", mark, m.Category, trunc(m.Subject, 36))
		if i == s.cursor {
			rows = append(rows, st.Selected.Render(line))
		} else {
			rows = append(rows, st.Item.Render(line))
		}
	}
	left := st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	right := s.renderBody()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (s *messagesScreen) renderBody() string {
	st := s.root.styles
	if s.current == nil {
		return st.Panel.Render(st.Muted.Render("select a message"))
	}
	body := strings.TrimSpace(s.current.Body)
	if body == "" {
		body = "(no body)"
	}
	rows := []string{
		st.Header.Render(s.current.Subject),
		st.Muted.Render(s.current.CreatedAt.Format(time.RFC822)),
		"",
		body,
	}
	return st.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (s *messagesScreen) Title() string { return "messages" }

func (s *messagesScreen) Help() []HelpEntry {
	return []HelpEntry{
		{Key: "↑/↓", Desc: "select"},
		{Key: "x", Desc: "delete"},
		{Key: "r", Desc: "refresh"},
		{Key: "esc", Desc: "back"},
	}
}
