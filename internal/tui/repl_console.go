package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

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
	side        consoleSideState
}

type completion struct {
	value string
	label string
	desc  string
}

type consoleSideState struct {
	planetID int64
	queues   []svc.QueueItem
	fleets   []svc.Fleet
	messages []svc.Message
	err      error
	loadedAt time.Time
}

type commandDoneMsg struct {
	line string
	out  string
	err  error
}

type sideStateLoadedMsg struct {
	state consoleSideState
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
	return tea.Batch(textinput.Blink, m.loadSideState())
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
		return m, m.loadSideState()
	case sideStateLoadedMsg:
		if p := m.currentPlanet(); p != nil && msg.state.planetID != 0 && msg.state.planetID != p.ID {
			return m, nil
		}
		m.side = msg.state
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
	w := max(96, m.width)
	h := max(30, m.height)
	leftW := 26
	rightW := 31
	gap := 1
	mainW := max(50, w-leftW-rightW-(gap*2))
	bodyH := max(18, h-5)
	mainH := bodyH

	top := m.renderTopBar(w)
	left := sideRailStyle(leftW, bodyH).Render(m.renderNav(leftW - 4))
	main := centerStyle(mainW, mainH).Render(m.renderCenter(mainW-2, mainH-2))
	right := sideRailStyle(rightW, bodyH).Render(m.renderRight(rightW - 4))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), main, strings.Repeat(" ", gap), right)
	return lipgloss.JoinVertical(lipgloss.Left, top, body)
}

func (m *consoleModel) runCommand(line string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.capture(func() error { return m.session.exec(m.ctx, line) })
		return commandDoneMsg{line: line, out: out, err: err}
	}
}

func (m consoleModel) loadSideState() tea.Cmd {
	p := m.currentPlanet()
	if p == nil {
		return nil
	}
	planetID := p.ID
	return func() tea.Msg {
		state := consoleSideState{planetID: planetID, loadedAt: time.Now()}
		queues, err := withTimeout(m.ctx, func(ctx context.Context) ([]svc.QueueItem, error) {
			return m.session.client.GetQueues(ctx, planetID)
		})
		if err != nil {
			state.err = err
			return sideStateLoadedMsg{state: state}
		}
		state.queues = queues

		fleets, err := withTimeout(m.ctx, func(ctx context.Context) ([]svc.Fleet, error) {
			return m.session.client.ListFleet(ctx)
		})
		if err == nil {
			state.fleets = fleets
		}

		messages, err := withTimeout(m.ctx, func(ctx context.Context) ([]svc.Message, error) {
			return m.session.client.ListMessages(ctx)
		})
		if err == nil {
			state.messages = messages
		}
		return sideStateLoadedMsg{state: state}
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

func (m consoleModel) renderTopBar(width int) string {
	p := m.currentPlanet()
	left := accentOrange().Render(m.session.user.Username)
	mid := mutedStyle().Render("no planet")
	res := ""
	if p != nil {
		left += mutedStyle().Render(" · U#1 · ")
		left += accentOrange().Render(p.Name)
		left += fmt.Sprintf(" %s", accentYellow().Render(fmt.Sprintf("%d:%d:%d", p.Galaxy, p.System, p.Position)))
		mid = mutedStyle().Render(fmt.Sprintf("fields %d/%d · temp %d/%d°C", p.FieldsUsed, p.FieldsTotal, p.TempMin, p.TempMax))
		res = fmt.Sprintf("M %s   C %s   D %s   E %s",
			accentYellow().Render(formatCompact(p.Metal)),
			accentCyan().Render(formatCompact(p.Crystal)),
			accentMagenta().Render(formatCompact(p.Deuterium)),
			energyStyle(p.EnergyProduced-p.EnergyUsed).Render(fmt.Sprintf("%+d", p.EnergyProduced-p.EnergyUsed)),
		)
	}
	right := accentGreen().Render(time.Now().Format("15:04:05")) + mutedStyle().Render("  ● 1 online   🛡 0 def")
	line1 := fitColumns(width, left+"  "+mid, right)
	line2 := fitColumns(width, res, mutedStyle().Render("▣ inbox"))
	return topBarStyle(width).Render(line1 + "\n" + line2)
}

func (m consoleModel) renderCenter(width, height int) string {
	planetH := min(10, max(7, height/4))
	suggestH := 8
	inputH := 1
	logH := max(6, height-planetH-suggestH-inputH-4)
	planet := m.renderPlanetCard(width, planetH)
	log := m.renderCommandArea(width, logH)
	suggest := m.renderSuggestions(width)
	input := inputLineStyle(width).Render(m.input.View())
	return lipgloss.JoinVertical(
		lipgloss.Left,
		planet,
		rule(width, "─", "238"),
		log,
		rule(width, "─", "178"),
		suggest,
		input,
	)
}

func (m consoleModel) renderPlanetCard(width, height int) string {
	p := m.currentPlanet()
	if p == nil {
		return mutedStyle().Render("no planet")
	}
	globe := renderPlanetGlobe(p.Code, p.Position, min(18, max(12, width/5)), min(9, max(7, height-1)))
	rows := []string{
		accentOrange().Render(strings.ToUpper(p.Name) + " " + fmt.Sprintf("%d:%d:%d", p.Galaxy, p.System, p.Position)),
		mutedStyle().Render(fmt.Sprintf("fields %d/%d   temp %d/%d°C", p.FieldsUsed, p.FieldsTotal, p.TempMin, p.TempMax)),
		"",
		fmt.Sprintf("metal     %s   %s", accentYellow().Render(formatCompact(p.Metal)), mutedStyle().Render("+?/h")),
		fmt.Sprintf("crystal   %s   %s", accentCyan().Render(formatCompact(p.Crystal)), mutedStyle().Render("+?/h")),
		fmt.Sprintf("deut      %s   %s", accentMagenta().Render(formatCompact(p.Deuterium)), mutedStyle().Render("+?/h")),
		"",
		fmt.Sprintf("energy    prod %d / used %d    bal %s", p.EnergyProduced, p.EnergyUsed, energyStyle(p.EnergyProduced-p.EnergyUsed).Render(fmt.Sprintf("%+d", p.EnergyProduced-p.EnergyUsed))),
	}
	infoW := max(24, width-lipgloss.Width(globe)-4)
	info := clampBlock(strings.Join(rows, "\n"), infoW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, globe, "  ", info)
}

func (m consoleModel) renderCommandArea(width, height int) string {
	title := accentOrange().Render("terminal.army")
	help := mutedStyle().Render("Type / to see commands, Tab to autocomplete, Enter to run.")
	body := m.renderLog(width, max(1, height-2))
	return clampBlock(title+"\n"+help+"\n"+body, width, height)
}

func (m consoleModel) renderNav(width int) string {
	sections := []struct {
		title string
		cmds  []string
		style lipgloss.Style
	}{
		{"PLANET", []string{"/planet", "/resources", "/facilities", "/upgrade", "/queue", "/cancel"}, accentGreen()},
		{"RESEARCH", []string{"/research", "/tree"}, accentMagenta()},
		{"FLEET", []string{"/ships", "/defense", "/fleet", "/espionage", "/attack", "/transport", "/reports"}, accentCyan()},
		{"GALAXY", []string{"/galaxy", "/planets", "/switch", "/logs"}, accentViolet()},
		{"SOCIAL", []string{"/msg", "/inbox"}, accentOrange()},
		{"STANDINGS", []string{"/leaderboard", "/alliance"}, accentOrange()},
		{"HELP", []string{"/help", "/quest", "/info", "/options"}, mutedStyle()},
	}
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(accentOrange().Render(section.title))
		b.WriteByte('\n')
		for _, cmd := range section.cmds {
			b.WriteString(section.style.Render(clampLine("  "+cmd, width)))
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(accentOrange().Render("HOTKEYS"))
	b.WriteString("\n")
	for _, row := range []string{"Tab    complete", "↑ ↓    history", "Esc    hide popup", "Ctrl+L clear log", "Ctrl+C quit"} {
		b.WriteString(mutedStyle().Render("  " + clampLine(row, width-2)))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m consoleModel) renderSuggestions(width int) string {
	items := m.suggestions
	if len(items) == 0 {
		items = commandSuggestions("/")
	}
	maxRows := min(7, len(items))
	rows := make([]string, 0, maxRows)
	for i := 0; i < maxRows; i++ {
		s := items[i]
		raw := clampLine(fmt.Sprintf("%-28s  %s", s.label, s.desc), width-3)
		label := accentCyan().Render(fmt.Sprintf("%-28s", s.label)) + "  " + mutedStyle().Render(s.desc)
		label = clampLine(label, width-3)
		if len(m.suggestions) > 0 && i == m.selected {
			rows = append(rows, selectedStyle().Render(raw))
			continue
		}
		rows = append(rows, label)
	}
	return clampBlock(strings.Join(rows, "\n"), width, 7)
}

func (m consoleModel) renderRight(width int) string {
	p := m.currentPlanet()
	var b strings.Builder
	b.WriteString(accentOrange().Render("PLANETS"))
	b.WriteByte('\n')
	for i, planet := range m.session.planets {
		prefix := "  "
		if p != nil && planet.ID == p.ID {
			prefix = "▸ "
		}
		row := fmt.Sprintf("%s%d. %s#%s", prefix, i+1, planet.Code, planet.Name)
		b.WriteString(accentOrange().Render(clampLine(row, width)))
		b.WriteByte('\n')
		b.WriteString(mutedStyle().Render(clampLine(fmt.Sprintf("     %d:%d:%d   M %s", planet.Galaxy, planet.System, planet.Position, formatCompact(planet.Metal)), width)))
		b.WriteByte('\n')
	}
	writeSection(&b, width, fmt.Sprintf("QUEUES (%d/5)", len(m.side.queues)), m.renderQueueRows(width))
	writeSection(&b, width, fmt.Sprintf("MISSIONS (%d)", len(m.side.fleets)), m.renderFleetRows(width))
	writeSection(&b, width, "QUESTS [10/15]", []string{"▸ Send your first fleet"})
	writeSection(&b, width, messageSectionTitle(m.side.messages), m.renderMessageRows(width))
	if m.side.err != nil {
		writeSection(&b, width, "SYNC", []string{"queue unavailable"})
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m consoleModel) renderQueueRows(width int) []string {
	if len(m.side.queues) == 0 {
		return []string{"empty"}
	}
	now := time.Now()
	rows := make([]string, 0, min(len(m.side.queues), 5))
	for _, q := range m.side.queues {
		label := q.ItemKey
		if q.TargetLevel > 0 {
			label = fmt.Sprintf("%s L%d", label, q.TargetLevel)
		} else if q.Count > 0 {
			label = fmt.Sprintf("%s x%d", label, q.Count)
		}
		remaining := formatRemaining(q.FinishedAt.Sub(now))
		rows = append(rows, clampLine(fmt.Sprintf("▸ %s %-14s %s", queueTypeCode(q.QueueType), label, remaining), width-2))
		if len(rows) == 5 {
			break
		}
	}
	return rows
}

func queueTypeCode(queueType string) string {
	switch queueType {
	case "building":
		return "B"
	case "research":
		return "R"
	case "ship":
		return "S"
	case "defense":
		return "D"
	default:
		if queueType == "" {
			return "?"
		}
		return strings.ToUpper(queueType[:1])
	}
}

func (m consoleModel) renderFleetRows(width int) []string {
	if len(m.side.fleets) == 0 {
		return []string{"no active fleets"}
	}
	now := time.Now()
	rows := make([]string, 0, min(len(m.side.fleets), 4))
	for _, f := range m.side.fleets {
		when := f.ArrivalAt
		if f.State == "returning" && f.ReturnAt != nil {
			when = *f.ReturnAt
		}
		rows = append(rows, clampLine(fmt.Sprintf("▸ %s %s %d:%d:%d %s", f.Mission, f.State, f.TargetGalaxy, f.TargetSystem, f.TargetPosition, formatRemaining(when.Sub(now))), width-2))
		if len(rows) == 4 {
			break
		}
	}
	return rows
}

func (m consoleModel) renderMessageRows(width int) []string {
	if len(m.side.messages) == 0 {
		return nil
	}
	rows := make([]string, 0, min(len(m.side.messages), 4))
	for _, msg := range m.side.messages {
		prefix := " "
		if !msg.Read {
			prefix = "*"
		}
		rows = append(rows, clampLine(fmt.Sprintf("%s #%d %s", prefix, msg.ID, msg.Subject), width-2))
		if len(rows) == 4 {
			break
		}
	}
	return rows
}

func writeSection(b *strings.Builder, width int, title string, rows []string) {
	b.WriteString("\n")
	b.WriteString(accentOrange().Render(title))
	b.WriteByte('\n')
	for _, row := range rows {
		b.WriteString(mutedStyle().Render("  " + clampLine(row, width-2)))
		b.WriteByte('\n')
	}
}

func messageSectionTitle(messages []svc.Message) string {
	if len(messages) == 0 {
		return "MESSAGES (none)"
	}
	unread := 0
	for _, msg := range messages {
		if !msg.Read {
			unread++
		}
	}
	if unread > 0 {
		return fmt.Sprintf("MESSAGES (%d new)", unread)
	}
	return fmt.Sprintf("MESSAGES (%d)", len(messages))
}

func formatRemaining(d time.Duration) string {
	if d <= 0 {
		return "done"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", h, m)
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

func (m consoleModel) currentPlanet() *svc.Planet {
	if len(m.session.planets) == 0 {
		return nil
	}
	if m.session.currentIndex < 0 || m.session.currentIndex >= len(m.session.planets) {
		return &m.session.planets[0]
	}
	return &m.session.planets[m.session.currentIndex]
}

func formatCompact(v float64) string {
	n := int64(v)
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return strconv.FormatInt(n, 10)
	}
}

func fitColumns(width int, left, right string) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	if leftW+rightW+1 >= width {
		return clampLine(left+" "+right, width)
	}
	return left + strings.Repeat(" ", width-leftW-rightW) + right
}

func rule(width int, ch, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(strings.Repeat(ch, max(1, width)))
}

func clampBlock(s string, width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = clampLine(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

func renderPlanetGlobe(seed string, position, globeW, globeH int) string {
	if globeW < 8 {
		globeW = 8
	}
	if globeH < 5 {
		globeH = 5
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	rng := rand.New(rand.NewSource(int64(h.Sum64())))
	light := rng.Float64()*0.6 - 0.3
	threshold := rng.Float64()*0.8 - 0.2
	chars := []rune(" ·░▒▓█")
	style := planetStyle(position)
	var b strings.Builder
	halfW := float64(globeW) / 2
	halfH := float64(globeH) / 2
	for y := 0; y < globeH; y++ {
		ny := (float64(y) + 0.5 - halfH) / halfH
		for x := 0; x < globeW; x++ {
			nx := (float64(x) + 0.5 - halfW) / halfW
			if nx*nx+ny*ny > 1 {
				b.WriteRune(' ')
				continue
			}
			nz := math.Sqrt(maxFloat(0, 1-nx*nx-ny*ny))
			lit := maxFloat(0.16, nx*math.Cos(light)+nz*math.Sin(light)+0.45)
			noise := math.Sin(nx*8+float64(h.Sum64()%17)) + math.Cos(ny*11+float64(h.Sum64()%23))
			if noise > threshold {
				lit *= 1.15
			} else {
				lit *= 0.55
			}
			idx := min(len(chars)-1, max(1, int(lit*float64(len(chars)-1))))
			b.WriteString(style.Render(string(chars[idx])))
		}
		if y < globeH-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
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

func accentOrange() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
}

func accentYellow() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
}

func accentGreen() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("118"))
}

func accentCyan() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
}

func accentMagenta() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("198"))
}

func accentViolet() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
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
	return lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("178")).Bold(true).Padding(0, 1)
}

func topBarStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1)
}

func sideRailStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1)
}

func centerStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 0)
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

func inputLineStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Color("250")).
		Padding(0, 1)
}

func suggestionsStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("59")).
		Padding(0, 1)
}

func energyStyle(balance int) lipgloss.Style {
	if balance >= 0 {
		return accentGreen()
	}
	return errorStyle()
}

func planetStyle(position int) lipgloss.Style {
	switch {
	case position <= 3:
		return accentMagenta()
	case position <= 6:
		return accentYellow()
	case position <= 10:
		return accentCyan()
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("159"))
	}
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
