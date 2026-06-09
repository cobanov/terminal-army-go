package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/tui/client"
)

// RunConsole starts the rich slash-command client. Non-interactive shells use
// the line REPL so smoke tests and pipes keep working.
func RunConsole(ctx context.Context, serverURL string, logout bool) error {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return RunREPL(ctx, serverURL, logout)
	}
	if serverURL == "" {
		serverURL = DefaultServerURL
	}
	c := client.New(serverURL)
	if logout {
		ClearCreds(c.BaseURL())
		fmt.Printf("key removed for: %s\n", c.BaseURL())
		return nil
	}

	user, err := acquireSession(ctx, c)
	if err != nil {
		return err
	}
	r := &replSession{client: c, user: user}
	if err := r.ensurePlanets(ctx); err != nil {
		return err
	}

	m := newConsoleModel(ctx, r)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

type consoleModel struct {
	ctx         context.Context
	session     *replSession
	input       textinput.Model
	log         []string
	history     []string
	historyAt   int
	suggestions []completion
	selected    int
	width       int
	height      int
	busy        bool
	status      string
	err         error
}

type completion struct {
	value string
	label string
	desc  string
}

type commandDoneMsg struct {
	line string
	out  string
	err  error
}

func newConsoleModel(ctx context.Context, r *replSession) consoleModel {
	ti := textinput.New()
	ti.Prompt = "tarmy> "
	ti.Placeholder = "type / for commands, Tab to autocomplete, /help, /q"
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 96

	m := consoleModel{
		ctx:       ctx,
		session:   r,
		input:     ti,
		historyAt: -1,
		width:     100,
		height:    32,
		status:    "ready",
	}
	m.addLog(titleStyle().Render("terminal.army") + " " + mutedStyle().Render(r.client.BaseURL()))
	m.addLog(mutedStyle().Render("Type / to see commands, Tab to autocomplete, Up/Down to navigate suggestions, Ctrl+L to clear."))
	if out, err := m.capture(func() error { return r.printPlanet(ctx) }); err == nil {
		m.addLog(out)
	}
	return m
}

func (m consoleModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m consoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(24, msg.Width-14)
		return m, nil
	case commandDoneMsg:
		m.busy = false
		m.status = "ready"
		if msg.out != "" {
			m.addLog(msg.out)
		}
		if msg.err != nil {
			if errors.Is(msg.err, errQuit) {
				return m, tea.Quit
			}
			m.addLog(errorStyle().Render("error: " + msg.err.Error()))
		}
		m.trimLog()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+l":
			m.log = nil
			m.status = "cleared"
			return m, nil
		case "esc":
			m.suggestions = nil
			m.selected = 0
			return m, nil
		case "tab":
			if m.acceptSuggestion() {
				return m, nil
			}
		case "up":
			if len(m.suggestions) > 0 {
				m.selected = (m.selected - 1 + len(m.suggestions)) % len(m.suggestions)
				return m, nil
			}
			m.prevHistory()
			m.refreshSuggestions()
			return m, nil
		case "down":
			if len(m.suggestions) > 0 {
				m.selected = (m.selected + 1) % len(m.suggestions)
				return m, nil
			}
			m.nextHistory()
			m.refreshSuggestions()
			return m, nil
		case "enter":
			line := strings.TrimSpace(m.input.Value())
			if line == "" || m.busy {
				return m, nil
			}
			if !strings.HasPrefix(line, "/") {
				line = "/" + line
			}
			if line == "/clear" {
				m.log = nil
				m.input.SetValue("")
				m.suggestions = nil
				return m, nil
			}
			m.addLog(commandStyle().Render(line))
			m.history = append(m.history, line)
			m.historyAt = -1
			m.input.SetValue("")
			m.suggestions = nil
			m.busy = true
			m.status = "running " + strings.Fields(line)[0]
			return m, m.runCommand(line)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refreshSuggestions()
	return m, cmd
}

func (m consoleModel) View() string {
	w := max(72, m.width)
	h := max(24, m.height)
	sideW := 26
	mainW := max(40, w-sideW-4)
	logH := max(9, h-9)

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		titleStyle().Render("// terminal.army"),
		" ",
		tagStyle().Render(m.session.client.BaseURL()),
		" ",
		mutedStyle().Render("user "+m.session.user.Username),
	)
	if m.busy {
		header += " " + warnStyle().Render(m.status)
	}

	main := panelStyle(mainW, logH).Render(m.renderLog(mainW-4, logH-2))
	side := panelStyle(sideW, logH).Render(m.renderNav(sideW - 4))
	body := lipgloss.JoinHorizontal(lipgloss.Top, main, "  ", side)

	suggest := m.renderSuggestions(mainW)
	prompt := inputPanelStyle(w - 2).Render(m.input.View())
	footer := mutedStyle().Render("Tab complete  Up/Down select/history  Esc hide  Ctrl+L clear  Ctrl+C quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, body, suggest, prompt, footer)
}

func (m *consoleModel) runCommand(line string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.capture(func() error { return m.session.exec(m.ctx, line) })
		return commandDoneMsg{line: line, out: out, err: err}
	}
}

func (m *consoleModel) capture(fn func() error) (string, error) {
	var buf bytes.Buffer
	old := m.session.out
	m.session.out = &buf
	err := fn()
	m.session.out = old
	return strings.TrimRight(buf.String(), "\n"), err
}

func (m *consoleModel) addLog(s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		m.log = append(m.log, line)
	}
}

func (m *consoleModel) trimLog() {
	if len(m.log) > 500 {
		m.log = m.log[len(m.log)-500:]
	}
}

func (m consoleModel) renderLog(width, height int) string {
	if len(m.log) == 0 {
		return mutedStyle().Render("log is empty")
	}
	start := max(0, len(m.log)-height)
	lines := make([]string, 0, height)
	for _, line := range m.log[start:] {
		lines = append(lines, clampLine(line, width))
	}
	return strings.Join(lines, "\n")
}

func (m consoleModel) renderNav(width int) string {
	sections := []struct {
		title string
		cmds  []string
	}{
		{"PLANET", []string{"/planet", "/resources", "/upgrade", "/queue", "/switch"}},
		{"RESEARCH", []string{"/research", "/info"}},
		{"FLEET", []string{"/ships", "/defense", "/fleet", "/attack", "/transport", "/espionage"}},
		{"GALAXY", []string{"/galaxy", "/leaderboard"}},
		{"SOCIAL", []string{"/msg", "/messages", "/alliance"}},
		{"SYSTEM", []string{"/quest", "/refresh", "/logout", "/q"}},
	}
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(accentStyle().Render(section.title))
		b.WriteByte('\n')
		for _, cmd := range section.cmds {
			b.WriteString(clampLine("  "+cmd, width))
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m consoleModel) renderSuggestions(width int) string {
	if len(m.suggestions) == 0 {
		return mutedStyle().Render(" ")
	}
	maxRows := min(7, len(m.suggestions))
	rows := make([]string, 0, maxRows)
	for i := 0; i < maxRows; i++ {
		s := m.suggestions[i]
		label := fmt.Sprintf("%-28s %s", s.label, s.desc)
		label = clampLine(label, width-6)
		if i == m.selected {
			rows = append(rows, selectedStyle().Render(label))
			continue
		}
		rows = append(rows, "  "+mutedStyle().Render(label))
	}
	return suggestionsStyle(width - 2).Render(strings.Join(rows, "\n"))
}

func (m *consoleModel) refreshSuggestions() {
	m.suggestions = suggestionsForInput(m.input.Value(), m.session.planets)
	if m.selected >= len(m.suggestions) {
		m.selected = 0
	}
}

func (m *consoleModel) acceptSuggestion() bool {
	if len(m.suggestions) == 0 {
		return false
	}
	if m.selected < 0 || m.selected >= len(m.suggestions) {
		m.selected = 0
	}
	m.input.SetValue(m.suggestions[m.selected].value)
	m.input.CursorEnd()
	m.refreshSuggestions()
	return true
}

func (m *consoleModel) prevHistory() {
	if len(m.history) == 0 {
		return
	}
	if m.historyAt == -1 {
		m.historyAt = len(m.history) - 1
	} else if m.historyAt > 0 {
		m.historyAt--
	}
	m.input.SetValue(m.history[m.historyAt])
	m.input.CursorEnd()
}

func (m *consoleModel) nextHistory() {
	if len(m.history) == 0 || m.historyAt == -1 {
		return
	}
	if m.historyAt < len(m.history)-1 {
		m.historyAt++
		m.input.SetValue(m.history[m.historyAt])
	} else {
		m.historyAt = -1
		m.input.SetValue("")
	}
	m.input.CursorEnd()
}

func suggestionsForInput(input string, planets []svc.Planet) []completion {
	text := strings.TrimLeft(input, " ")
	if text == "" {
		return nil
	}
	if !strings.HasPrefix(text, "/") {
		text = "/" + text
	}
	if strings.Contains(text, " ") {
		cmd, arg, _ := strings.Cut(strings.ToLower(text), " ")
		rawArg := strings.TrimPrefix(text[strings.Index(text, " ")+1:], " ")
		arg = strings.TrimSpace(arg)
		switch cmd {
		case "/upgrade", "/u":
			return keySuggestions("/upgrade", rawArg, catalogKeys(BuildingCatalog), "upgrade building")
		case "/research", "/r":
			return keySuggestions("/research", rawArg, catalogKeys(ResearchCatalog), "research tech")
		case "/info":
			return keySuggestions("/info", rawArg, allCatalogKeys(), "lookup encyclopedia")
		case "/ships":
			return buildSuggestions("/ships", rawArg, catalogKeys(ShipCatalog), "build ship")
		case "/defense":
			return buildSuggestions("/defense", rawArg, catalogKeys(DefenseCatalog), "build defense")
		case "/attack", "/transport", "/espionage":
			return shipArgSuggestions(cmd, rawArg)
		case "/switch":
			return planetSuggestions(rawArg, planets)
		case "/alliance":
			return prefixSuggestions("/alliance", rawArg, []completion{
				{value: "/alliance list", label: "/alliance list", desc: "list alliances"},
				{value: "/alliance create ", label: "/alliance create <TAG> <name>", desc: "found alliance"},
				{value: "/alliance join ", label: "/alliance join <id>", desc: "join by id"},
				{value: "/alliance leave ", label: "/alliance leave <id>", desc: "leave by id"},
			})
		}
		return nil
	}
	return commandSuggestions(strings.ToLower(text))
}

func commandSuggestions(prefix string) []completion {
	specs := []completion{
		{"/help", "/help", "show commands"},
		{"/planet", "/planet", "current planet detail"},
		{"/planets", "/planets", "list planets"},
		{"/switch ", "/switch <planet>", "change active planet"},
		{"/resources", "/resources", "resources and buildings"},
		{"/facilities", "/facilities", "facilities overview"},
		{"/upgrade ", "/upgrade <building>", "queue building"},
		{"/research ", "/research <tech>", "queue research"},
		{"/ships", "/ships", "ship inventory"},
		{"/ships build ", "/ships build <ship> <n>", "build ships"},
		{"/defense", "/defense", "defense inventory"},
		{"/defense build ", "/defense build <type> <n>", "build defenses"},
		{"/galaxy ", "/galaxy <g:s>", "system view"},
		{"/fleet", "/fleet", "active fleet movements"},
		{"/attack ", "/attack <g:s:p> ship=n", "dispatch attack"},
		{"/transport ", "/transport <g:s:p> m=n", "transport resources"},
		{"/espionage ", "/espionage <g:s:p>", "send probe"},
		{"/msg ", "/msg <user> <text>", "send message"},
		{"/messages", "/messages", "inbox"},
		{"/reports", "/reports", "combat and spy reports"},
		{"/alliance", "/alliance", "alliance list/create/join"},
		{"/leaderboard", "/leaderboard", "server rankings"},
		{"/quest", "/quest", "next suggested step"},
		{"/info ", "/info <key>", "item lookup"},
		{"/me", "/me", "account info"},
		{"/refresh", "/refresh", "refresh planet"},
		{"/clear", "/clear", "clear log"},
		{"/logout", "/logout", "delete saved key"},
		{"/q", "/q", "quit"},
	}
	out := make([]completion, 0, len(specs))
	for _, spec := range specs {
		if strings.HasPrefix(strings.ToLower(spec.label), prefix) || strings.HasPrefix(strings.ToLower(spec.value), prefix) {
			out = append(out, spec)
		}
	}
	return out
}

func keySuggestions(cmd, prefix string, keys []string, desc string) []completion {
	out := make([]completion, 0, len(keys))
	p := strings.ToLower(strings.TrimSpace(prefix))
	for _, key := range keys {
		if p == "" || strings.HasPrefix(strings.ToLower(key), p) {
			out = append(out, completion{value: cmd + " " + key, label: cmd + " " + key, desc: desc})
		}
	}
	return out
}

func buildSuggestions(cmd, arg string, keys []string, desc string) []completion {
	parts := strings.Fields(arg)
	if len(parts) == 0 {
		return []completion{{value: cmd + " build ", label: cmd + " build <type> <n>", desc: desc}}
	}
	if parts[0] != "build" {
		return nil
	}
	prefix := ""
	if len(parts) > 1 {
		prefix = parts[1]
	}
	out := keySuggestions(cmd+" build", prefix, keys, desc)
	for i := range out {
		if !strings.HasSuffix(out[i].value, " ") {
			out[i].value += " "
		}
	}
	return out
}

func shipArgSuggestions(cmd, arg string) []completion {
	parts := strings.Fields(arg)
	if len(parts) <= 1 {
		return nil
	}
	last := parts[len(parts)-1]
	if strings.Contains(last, "=") {
		return nil
	}
	keys := catalogKeys(ShipCatalog)
	out := make([]completion, 0, len(keys))
	prefix := strings.ToLower(last)
	base := cmd + " " + strings.Join(parts[:len(parts)-1], " ")
	for _, key := range keys {
		if strings.HasPrefix(strings.ToLower(key), prefix) {
			out = append(out, completion{value: base + " " + key + "=1", label: key + "=1", desc: "ship count"})
		}
	}
	return out
}

func planetSuggestions(prefix string, planets []svc.Planet) []completion {
	out := make([]completion, 0, len(planets))
	p := strings.ToLower(strings.TrimSpace(prefix))
	for i, planet := range planets {
		index := strconv.Itoa(i + 1)
		code := strings.ToUpper(planet.Code)
		name := planet.Name
		if p == "" || strings.HasPrefix(strings.ToLower(code), p) || strings.HasPrefix(strings.ToLower(name), p) || strings.HasPrefix(index, p) {
			out = append(out, completion{
				value: "/switch " + code,
				label: "/switch " + code,
				desc:  fmt.Sprintf("%s #%d %d:%d:%d", name, i+1, planet.Galaxy, planet.System, planet.Position),
			})
		}
	}
	return out
}

func prefixSuggestions(cmd, prefix string, specs []completion) []completion {
	out := make([]completion, 0, len(specs))
	p := strings.ToLower(strings.TrimSpace(prefix))
	for _, spec := range specs {
		if p == "" || strings.HasPrefix(strings.TrimPrefix(strings.ToLower(spec.value), cmd+" "), p) {
			out = append(out, spec)
		}
	}
	return out
}

func catalogKeys(rows []CatalogItem) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys
}

func allCatalogKeys() []string {
	var keys []string
	keys = append(keys, catalogKeys(BuildingCatalog)...)
	keys = append(keys, catalogKeys(ResearchCatalog)...)
	keys = append(keys, catalogKeys(ShipCatalog)...)
	keys = append(keys, catalogKeys(DefenseCatalog)...)
	return keys
}

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
}

func accentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
}

func tagStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("73")).Padding(0, 1)
}

func mutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
}

func warnStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
}

func errorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
}

func commandStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
}

func selectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("111")).Bold(true).Padding(0, 1)
}

func panelStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 1)
}

func inputPanelStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("73")).
		Padding(0, 1)
}

func suggestionsStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("59")).
		Padding(0, 1)
}

func clampLine(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
