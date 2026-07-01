package tui

import (
	"bytes"
	"context"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/tui/client"
)

// RunConsole starts the interactive, mouse-first client. Non-interactive shells
// (pipes, smoke tests) fall back to the headless line REPL so scripting keeps
// working.
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
	p := tea.NewProgram(newAppModel(ctx, r),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
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

// appModel is the root Bubble Tea model for the interactive client.
type appModel struct {
	ctx     context.Context
	session *replSession

	width, height int

	active   viewID
	hover    viewID // menu entry currently under the mouse; -1 for none
	data     viewData
	rail     railData
	rowSel   int
	tblScrl  int
	logScrl  int
	log      []string
	cmdHist  []string
	cmdAt    int
	sugg     []completion
	selSug   int
	busy     bool
	status   string
	input    textinput.Model
	lastTick time.Time
}

// viewData holds whatever the active view last loaded from the read-model.
type viewData struct {
	loaded    viewID
	buildings []svc.BuildingView
	research  *svc.ResearchView
	units     []svc.UnitView
	fleets    []svc.Fleet
	system    *svc.SystemView
	messages  []svc.Message
	reports   []svc.Report
	alliances []svc.Alliance
	ranks     []svc.LeaderboardEntry
	planet    *svc.Planet
	prod      *svc.ProductionReport
	queues    []svc.QueueItem
	err       error
}

type viewLoadedMsg struct {
	id   viewID
	data viewData
}
type railLoadedMsg struct{ rail railData }
type cmdDoneMsg struct {
	line string
	out  string
	err  error
}
type tickMsg time.Time

func newAppModel(ctx context.Context, r *replSession) appModel {
	ti := textinput.New()
	ti.Prompt = "tarmy> "
	ti.Placeholder = "type / for commands, Tab to autocomplete"
	ti.CharLimit = 512
	ti.Focus()
	return appModel{
		ctx:     ctx,
		session: r,
		active:  viewOverview,
		hover:   -1,
		input:   ti,
		width:   120,
		height:  36,
		status:  "ready",
		cmdAt:   -1,
		log:     []string{stMuted().Render("Welcome to terminal.army. Click the menu or type a /command.")},
	}
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadView(m.active), m.loadRail(), tick())
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.lastTick = time.Time(msg)
		return m, tick()
	case viewLoadedMsg:
		if msg.id == m.active {
			m.data = msg.data
			m.data.loaded = msg.id
			m.rowSel = 0
			m.tblScrl = 0
		}
		return m, nil
	case railLoadedMsg:
		m.rail = msg.rail
		return m, nil
	case cmdDoneMsg:
		m.busy = false
		m.status = "ready"
		m.appendLog(stMuted().Render("› ") + msg.line)
		if msg.out != "" {
			m.appendLog(msg.out)
		}
		if msg.err != nil {
			m.appendLog(stBad().Render("error: " + msg.err.Error()))
		}
		return m, tea.Batch(m.loadView(m.active), m.loadRail())
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m appModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		return m, tea.Quit
	case "ctrl+l":
		m.log = nil
		return m, nil
	case "esc":
		m.sugg = nil
		m.selSug = 0
		return m, nil
	case "tab":
		if len(m.sugg) > 0 {
			m.input.SetValue(m.sugg[m.selSug].value)
			m.input.CursorEnd()
			m.refreshSugg()
		}
		return m, nil
	case "up":
		if len(m.sugg) > 0 {
			m.selSug = (m.selSug - 1 + len(m.sugg)) % len(m.sugg)
			return m, nil
		}
		m.historyPrev()
		return m, nil
	case "down":
		if len(m.sugg) > 0 {
			m.selSug = (m.selSug + 1) % len(m.sugg)
			return m, nil
		}
		m.historyNext()
		return m, nil
	case "pgup":
		m.tblScrl = max(0, m.tblScrl-3)
		return m, nil
	case "pgdown":
		m.tblScrl += 3
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
			m.sugg = nil
			return m, nil
		}
		m.cmdHist = append(m.cmdHist, line)
		m.cmdAt = -1
		m.input.SetValue("")
		m.sugg = nil
		if v, ok := viewForCommand(line); ok {
			m.active = v
		}
		m.busy = true
		m.status = "running " + strings.Fields(line)[0]
		return m, m.runCommand(line)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refreshSugg()
	return m, cmd
}

func (m appModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	lay := m.layout()
	// Wheel scrolls the region under the cursor.
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inRect(msg.X, msg.Y, lay.centerX, lay.tableTop, lay.centerW, lay.tableH) {
			m.tblScrl = max(0, m.tblScrl-2)
		} else {
			m.logScrl = max(0, m.logScrl-2)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if inRect(msg.X, msg.Y, lay.centerX, lay.tableTop, lay.centerW, lay.tableH) {
			m.tblScrl += 2
		} else {
			m.logScrl += 2
		}
		return m, nil
	}
	if msg.Action == tea.MouseActionMotion {
		m.hover = m.menuHitTest(lay, msg.X, msg.Y)
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	// Menu / tabs → switch view.
	if v, ok := m.menuHitView(lay, msg.X, msg.Y); ok {
		if v != m.active {
			m.active = v
			m.tblScrl, m.rowSel = 0, 0
			return m, m.loadView(v)
		}
		return m, nil
	}
	// Table / rail → run the line's command.
	if cmd, ok := m.clickCommand(lay, msg.X, msg.Y); ok && cmd != "" && !m.busy {
		if v, isNav := viewForCommand(cmd); isNav {
			m.active = v
			m.tblScrl, m.rowSel = 0, 0
			return m, m.loadView(v)
		}
		m.busy = true
		m.status = "running " + strings.Fields(cmd)[0]
		return m, m.runCommand(cmd)
	}
	return m, nil
}

// --- command + history helpers ---------------------------------------------

func (m *appModel) appendLog(s string) {
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		m.log = append(m.log, line)
	}
	if len(m.log) > 500 {
		m.log = m.log[len(m.log)-500:]
	}
	m.logScrl = 0
}

func (m *appModel) refreshSugg() {
	m.sugg = suggestionsForInput(m.input.Value(), m.session.planets)
	if m.selSug >= len(m.sugg) {
		m.selSug = 0
	}
}

func (m *appModel) historyPrev() {
	if len(m.cmdHist) == 0 {
		return
	}
	if m.cmdAt == -1 {
		m.cmdAt = len(m.cmdHist) - 1
	} else if m.cmdAt > 0 {
		m.cmdAt--
	}
	m.input.SetValue(m.cmdHist[m.cmdAt])
	m.input.CursorEnd()
}

func (m *appModel) historyNext() {
	if len(m.cmdHist) == 0 || m.cmdAt == -1 {
		return
	}
	if m.cmdAt < len(m.cmdHist)-1 {
		m.cmdAt++
		m.input.SetValue(m.cmdHist[m.cmdAt])
	} else {
		m.cmdAt = -1
		m.input.SetValue("")
	}
	m.input.CursorEnd()
}

func (m appModel) runCommand(line string) tea.Cmd {
	sess := m.session
	ctx := m.ctx
	return func() tea.Msg {
		var buf bytes.Buffer
		old := sess.out
		sess.out = &buf
		err := sess.exec(ctx, line)
		sess.out = old
		return cmdDoneMsg{line: line, out: strings.TrimRight(buf.String(), "\n"), err: err}
	}
}

// viewForCommand maps a nav command to the view it should focus. The bool is
// false for non-nav commands (builds, dispatches) which leave the view as-is.
func viewForCommand(line string) (viewID, bool) {
	switch strings.TrimPrefix(strings.Fields(line)[0], "/") {
	case "planet", "p", "overview", "refresh":
		return viewOverview, true
	case "resources":
		return viewBuildings, true
	case "facilities":
		return viewFacilities, true
	case "tree":
		return viewResearch, true
	case "ships", "ship", "s":
		return viewShipyard, true
	case "defense", "def":
		return viewDefense, true
	case "fleet", "fleets":
		return viewFleet, true
	case "galaxy", "g":
		return viewGalaxy, true
	case "messages", "inbox", "msg":
		return viewMessages, true
	case "reports":
		return viewReports, true
	case "alliance", "ally":
		return viewAlliance, true
	case "leaderboard", "rank", "lb":
		return viewRanking, true
	}
	return viewOverview, false
}

// --- data loaders ----------------------------------------------------------

func (m appModel) loadRail() tea.Cmd {
	sess := m.session
	ctx := m.ctx
	p, err := sess.currentPlanet()
	if err != nil {
		return nil
	}
	pid := p.ID
	return func() tea.Msg {
		var rail railData
		if qs, err := sess.client.GetQueues(ctx, pid); err == nil {
			rail.queues = qs
		}
		if fs, err := sess.client.ListFleet(ctx); err == nil {
			rail.fleets = fs
		}
		if ms, err := sess.client.ListMessages(ctx); err == nil {
			rail.messages = ms
		}
		return railLoadedMsg{rail: rail}
	}
}

func (m appModel) loadView(v viewID) tea.Cmd {
	sess := m.session
	ctx := m.ctx
	p, perr := sess.currentPlanet()
	var pid int64
	var galaxy, system int
	if perr == nil {
		pid, galaxy, system = p.ID, p.Galaxy, p.System
	}
	return func() tea.Msg {
		d := viewData{}
		cl := sess.client
		switch v {
		case viewOverview:
			if pl, err := cl.GetPlanet(ctx, pid); err == nil {
				d.planet = pl
			}
			if pr, err := cl.GetProduction(ctx, pid); err == nil {
				d.prod = pr
			}
			if qs, err := cl.GetQueues(ctx, pid); err == nil {
				d.queues = qs
			}
		case viewBuildings:
			d.buildings, d.err = cl.PlanetBuildings(ctx, pid)
		case viewFacilities:
			d.buildings, d.err = cl.PlanetFacilities(ctx, pid)
		case viewResearch:
			d.research, d.err = cl.PlanetResearch(ctx, pid)
		case viewShipyard:
			d.units, d.err = cl.PlanetShipyard(ctx, pid)
		case viewDefense:
			d.units, d.err = cl.PlanetDefense(ctx, pid)
		case viewFleet:
			d.fleets, d.err = cl.ListFleet(ctx)
		case viewGalaxy:
			d.system, d.err = cl.ViewSystem(ctx, galaxy, system)
		case viewMessages:
			d.messages, d.err = cl.ListMessages(ctx)
		case viewReports:
			d.reports, d.err = cl.ListReports(ctx)
		case viewAlliance:
			d.alliances, d.err = cl.ListAlliances(ctx)
		case viewRanking:
			d.ranks, d.err = cl.Leaderboard(ctx)
		}
		return viewLoadedMsg{id: v, data: d}
	}
}
