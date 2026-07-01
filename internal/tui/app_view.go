package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// layout holds the computed geometry for the current terminal size. It is a
// pure function of width/height (plus column count), so both View and the mouse
// hit-testers derive the same rectangles.
type layout struct {
	cols      int
	bodyTop   int
	bodyH     int
	footerTop int

	menuX, menuW     int
	centerX, centerW int
	railX, railW     int

	tableTop, tableH int
	histTop, histH   int
	inputTop         int

	stripTop int // two-column rail strip row (-1 if none)
	tabsTop  int // narrow tab-bar row (-1 if none)
}

const (
	topBarH = 3 // two content rows + bottom border
	footerH = 2 // top border + one content row
	sepW    = 3 // " │ " column separator
)

func (m appModel) layout() layout {
	w, h := m.width, m.height
	l := layout{stripTop: -1, tabsTop: -1}
	switch {
	case w >= 120:
		l.cols = 3
	case w >= 80:
		l.cols = 2
	default:
		l.cols = 1
	}
	bodyTop := topBarH
	switch l.cols {
	case 3:
		l.menuW = clampInt(w/7, 14, 22)
		l.railW = clampInt(w/5, 22, 30)
		l.centerW = w - l.menuW - l.railW - sepW*2
		l.menuX = 0
		l.centerX = l.menuW + sepW
		l.railX = l.centerX + l.centerW + sepW
	case 2:
		l.stripTop = topBarH
		bodyTop = topBarH + 1
		l.menuW = clampInt(w/6, 14, 22)
		l.centerW = w - l.menuW - sepW
		l.menuX = 0
		l.centerX = l.menuW + sepW
	default:
		l.tabsTop = topBarH
		bodyTop = topBarH + 1
		l.centerW = w
		l.centerX = 0
	}
	if l.centerW < 8 {
		l.centerW = 8
	}
	l.bodyTop = bodyTop
	l.bodyH = max(1, h-bodyTop-footerH)
	l.footerTop = bodyTop + l.bodyH
	l.tableTop = bodyTop
	l.inputTop = bodyTop + l.bodyH - 1
	l.histH = clampInt(l.bodyH-8, 0, 5)
	l.histTop = l.inputTop - l.histH
	l.tableH = max(1, l.histTop-l.tableTop)
	return l
}

func (m appModel) View() string {
	if m.width < 8 || m.height < 8 {
		return "terminal too small"
	}
	l := m.layout()
	var out []string

	// Top bar.
	out = append(out, topBarStyle(m.width).Render(m.renderTopBar(m.width)))
	if l.stripTop >= 0 {
		out = append(out, padLine(railStrip(m.rail, m.width), m.width))
	}
	if l.tabsTop >= 0 {
		out = append(out, padLine(renderMenuTabs(m.active, m.width), m.width))
	}

	// Center column: table + history/suggestions + input.
	center := m.buildCenterRegion(l.centerW, l.tableH)
	centerLines := append([]string{}, center.lines...)
	centerLines = append(centerLines, m.centerLower(l)...)
	centerCol := column(centerLines, l.centerW, l.bodyH)

	sep := stMuted().Render(" │ ")
	var bodyRows []string
	switch l.cols {
	case 3:
		menuLines, _ := renderMenu(m.active, m.hover, l.menuW, l.bodyH)
		menuCol := column(strings.Split(menuLines, "\n"), l.menuW, l.bodyH)
		railCol := column(renderRail(m.rail, l.railW, l.bodyH).lines, l.railW, l.bodyH)
		for i := 0; i < l.bodyH; i++ {
			bodyRows = append(bodyRows, menuCol[i]+sep+centerCol[i]+sep+railCol[i])
		}
	case 2:
		menuLines, _ := renderMenu(m.active, m.hover, l.menuW, l.bodyH)
		menuCol := column(strings.Split(menuLines, "\n"), l.menuW, l.bodyH)
		for i := 0; i < l.bodyH; i++ {
			bodyRows = append(bodyRows, menuCol[i]+sep+centerCol[i])
		}
	default:
		bodyRows = centerCol
	}
	out = append(out, bodyRows...)

	// Footer.
	footer, _ := m.footer(m.width)
	out = append(out, footerStyle(m.width).Render(footer))

	return clampBlock(strings.Join(out, "\n"), m.width, m.height)
}

// centerLower renders the history/suggestion strip plus the command input line.
func (m appModel) centerLower(l layout) []string {
	var lines []string
	if l.histH > 0 {
		if len(m.sugg) > 0 {
			lines = append(lines, m.suggLines(l.centerW, l.histH)...)
		} else {
			lines = append(lines, m.logLines(l.centerW, l.histH)...)
		}
	}
	inp := m.input
	inp.Width = max(4, l.centerW-len(inp.Prompt)-1)
	prompt := padLine(inp.View(), l.centerW)
	if m.busy {
		prompt = padLine(stMuted().Render("… "+m.status), l.centerW)
	}
	lines = append(lines, prompt)
	return lines
}

func (m appModel) logLines(width, height int) []string {
	src := m.log
	if len(src) == 0 {
		return []string{stMuted().Render(clampLine("history is empty", width))}
	}
	end := len(src) - m.logScrl
	end = clampInt(end, 1, len(src))
	start := max(0, end-height)
	out := make([]string, 0, height)
	for _, line := range src[start:end] {
		out = append(out, clampLine(line, width))
	}
	return out
}

func (m appModel) suggLines(width, height int) []string {
	out := make([]string, 0, height)
	for i := 0; i < len(m.sugg) && i < height; i++ {
		s := m.sugg[i]
		row := clampLine(fmt.Sprintf("%-26s %s", s.label, s.desc), width)
		if i == m.selSug {
			row = stSelected().Render(clampLine(fmt.Sprintf("%-26s %s", s.label, s.desc), width))
		}
		out = append(out, row)
	}
	return out
}

func (m appModel) renderTopBar(width int) string {
	p := m.curPlanet()
	name, coords, res := stMuted().Render("no planet"), "", ""
	if p != nil {
		name = stBrand().Render(strings.ToUpper(p.Name))
		coords = stMuted().Render(fmt.Sprintf("%d:%d:%d", p.Galaxy, p.System, p.Position))
		mRate, cRate, dRate := "", "", ""
		bal := p.EnergyProduced - p.EnergyUsed
		if m.data.prod != nil && m.data.loaded == viewOverview {
			mRate = stMuted().Render(fmt.Sprintf("+%.0f", m.data.prod.MetalPerHour))
			cRate = stMuted().Render(fmt.Sprintf("+%.0f", m.data.prod.CrystalPerHour))
			dRate = stMuted().Render(fmt.Sprintf("+%.0f", m.data.prod.DeuteriumPerHour))
		}
		res = fmt.Sprintf("M %s %s   C %s %s   D %s %s   E %s",
			stGold().Render(formatCompact(p.Metal)), mRate,
			stCyan().Render(formatCompact(p.Crystal)), cRate,
			stViolet().Render(formatCompact(p.Deuterium)), dRate,
			energyText(bal))
	}
	clock := time.Now().Format("15:04:05")
	line1 := fitColumns(width, stBrand().Render("tarmy")+"  "+name+"  "+coords,
		stMuted().Render(fmt.Sprintf("%d planets  ⌚%s", len(m.session.planets), clock)))
	line2 := fitColumns(width, res, stMuted().Render(m.session.user.Username))
	return line1 + "\n" + line2
}

// curPlanet prefers the freshly-loaded overview planet, else the session's
// current planet snapshot.
func (m appModel) curPlanet() *svc.Planet {
	if m.data.planet != nil {
		return m.data.planet
	}
	p, err := m.session.currentPlanet()
	if err != nil {
		return nil
	}
	return p
}

// buildCenterRegion renders the active view's table into a region of the given
// size, carrying per-line click commands for mouse hit-testing.
func (m appModel) buildCenterRegion(width, height int) region {
	d := m.data
	switch m.active {
	case viewOverview:
		lines := overviewLines(m.curPlanet(), d.prod, d.queues, width)
		return makeRegionFromLines("OVERVIEW", lines, width, height)
	case viewBuildings:
		return makeRegion("BUILDINGS · Resources", buildingsHeader(), buildingRows(d.buildings, "/upgrade"), width, height, m.rowSel, m.tblScrl)
	case viewFacilities:
		return makeRegion("FACILITIES", buildingsHeader(), buildingRows(d.buildings, "/upgrade"), width, height, m.rowSel, m.tblScrl)
	case viewResearch:
		return makeRegion("RESEARCH", researchHeader(m.data.research), researchRows(d.research), width, height, m.rowSel, m.tblScrl)
	case viewShipyard:
		return makeRegion("SHIPYARD", unitsHeader(), unitRows(d.units, "/ships build"), width, height, m.rowSel, m.tblScrl)
	case viewDefense:
		return makeRegion("DEFENSE", unitsHeader(), unitRows(d.units, "/defense build"), width, height, m.rowSel, m.tblScrl)
	case viewFleet:
		return makeRegion("FLEET MOVEMENTS", "id    mission    state      target      eta", fleetRows(d.fleets), width, height, m.rowSel, m.tblScrl)
	case viewGalaxy:
		title := "GALAXY"
		if d.system != nil {
			title = fmt.Sprintf("GALAXY %d:%d", d.system.Galaxy, d.system.System)
		}
		return makeRegion(title, "pos planet             owner            alliance", systemRows(d.system), width, height, m.rowSel, m.tblScrl)
	case viewMessages:
		return makeRegion("INBOX", "", messageRows(d.messages), width, height, m.rowSel, m.tblScrl)
	case viewReports:
		return makeRegion("REPORTS", "", reportRows(d.reports), width, height, m.rowSel, m.tblScrl)
	case viewAlliance:
		return makeRegion("ALLIANCES", "", allianceRows(d.alliances), width, height, m.rowSel, m.tblScrl)
	case viewRanking:
		return makeRegion("LEADERBOARD", "rank player               score", rankingRows(d.ranks), width, height, m.rowSel, m.tblScrl)
	}
	return makeRegionFromLines("", nil, width, height)
}

func makeRegionFromLines(title string, body []string, width, height int) region {
	var lines []string
	if title != "" {
		lines = append(lines, clampLine(stHeader().Render(title), width))
	}
	for _, b := range body {
		lines = append(lines, clampLine(b, width))
	}
	targets := make([]string, height)
	padded := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			padded[i] = lines[i]
		}
	}
	return region{lines: padded, targets: targets}
}

func buildingsHeader() string {
	return stMuted().Render(fmt.Sprintf("%-22s %-4s %7s %7s %7s", "building", "lvl", "metal", "crystal", "deut"))
}
func unitsHeader() string {
	return stMuted().Render(fmt.Sprintf("%-20s %-11s %7s %7s %7s", "unit", "owned", "metal", "crystal", "deut"))
}
func researchHeader(v *svc.ResearchView) string {
	lab := 0
	if v != nil {
		lab = v.LabLevel
	}
	return stMuted().Render(fmt.Sprintf("Research Lab L%d — tech, level, next cost", lab))
}

// footer renders the clickable hotkey bar and its click zones.
func (m appModel) footer(width int) (string, []zone) {
	items := []struct {
		label string
		cmd   string
	}{
		{"F1 overview", "/planet"},
		{"F2 build", "/resources"},
		{"F3 research", "/tree"},
		{"F4 fleet", "/fleet"},
		{"F5 galaxy", "/galaxy"},
		{"/help", "/help"},
	}
	var b strings.Builder
	var zones []zone
	x := 0
	for i, it := range items {
		if i > 0 {
			b.WriteString(stMuted().Render("  "))
			x += 2
		}
		seg := it.label
		zones = append(zones, zone{x0: x, x1: x + len(seg), cmd: it.cmd})
		b.WriteString(stMuted().Render(seg))
		x += len(seg)
	}
	return clampLine(b.String(), width), zones
}

// zone is a clickable horizontal span on a single row.
type zone struct {
	x0, x1 int
	cmd    string
}

// --- hit-testing -----------------------------------------------------------

func inRect(x, y, rx, ry, rw, rh int) bool {
	return x >= rx && x < rx+rw && y >= ry && y < ry+rh
}

func (m appModel) menuHitTest(l layout, x, y int) viewID {
	if l.cols < 2 || !inRect(x, y, l.menuX, l.bodyTop, l.menuW, l.bodyH) {
		return -1
	}
	_, targets := renderMenu(m.active, m.hover, l.menuW, l.bodyH)
	idx := y - l.bodyTop
	if idx >= 0 && idx < len(targets) && targets[idx] >= 0 {
		return viewID(targets[idx])
	}
	return -1
}

func (m appModel) menuHitView(l layout, x, y int) (viewID, bool) {
	if l.tabsTop >= 0 && y == l.tabsTop {
		return m.tabHit(x)
	}
	if v := m.menuHitTest(l, x, y); v >= 0 {
		return v, true
	}
	return 0, false
}

func (m appModel) tabHit(x int) (viewID, bool) {
	col := 0
	for _, e := range allMenuEntries() {
		seg := len(" " + e.label + " ")
		if x >= col && x < col+seg {
			return e.id, true
		}
		col += seg + 1 // +1 for the "·" separator
	}
	return 0, false
}

func (m appModel) clickCommand(l layout, x, y int) (string, bool) {
	// Table rows.
	if inRect(x, y, l.centerX, l.tableTop, l.centerW, l.tableH) {
		reg := m.buildCenterRegion(l.centerW, l.tableH)
		idx := y - l.tableTop
		if idx >= 0 && idx < len(reg.targets) {
			return reg.targets[idx], true
		}
	}
	// Right rail.
	if l.cols == 3 && inRect(x, y, l.railX, l.bodyTop, l.railW, l.bodyH) {
		reg := renderRail(m.rail, l.railW, l.bodyH)
		idx := y - l.bodyTop
		if idx >= 0 && idx < len(reg.targets) {
			return reg.targets[idx], true
		}
	}
	// Footer.
	if y == l.footerTop {
		_, zones := m.footer(m.width)
		for _, z := range zones {
			if x >= z.x0 && x < z.x1 {
				return z.cmd, true
			}
		}
	}
	return "", false
}
