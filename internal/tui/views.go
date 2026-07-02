package tui

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// region is a rendered block plus, for every line, the slash command a click on
// that line runs ("" = not clickable). This is the unit of mouse hit-testing:
// the layout gives each region an absolute top row, and a click resolves to
// targets[clickY-top+scroll].
type region struct {
	lines   []string
	targets []string
}

// rowLine is one candidate data row before layout: its rendered text, the
// command a click runs, and whether it is dimmed (unaffordable/locked).
type rowLine struct {
	text string
	cmd  string
}

// makeRegion assembles a scrollable table: a title, an optional column header,
// then data rows. sel is the keyboard-selected row index (into rows); scroll is
// the first visible row. It clamps to width x height and records click targets.
func makeRegion(title, header string, rows []rowLine, width, height, sel, scroll int) region {
	var lines, targets []string
	push := func(s, cmd string) {
		lines = append(lines, clampLine(s, width))
		targets = append(targets, cmd)
	}
	push(stHeader().Render(title), "")
	if header != "" {
		push(stMuted().Render(header), "")
	}
	headerRows := len(lines)
	visible := height - headerRows
	if visible < 1 {
		visible = 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(rows) {
		scroll = len(rows)
	}
	for i := scroll; i < len(rows) && len(lines)-headerRows < visible; i++ {
		text := rows[i].text
		if i == sel {
			text = stSelected().Render(clampLine(rows[i].text, width))
		}
		push(text, rows[i].cmd)
	}
	if len(rows) == 0 {
		push(stMuted().Render("  (nothing here yet)"), "")
	}
	for len(lines) < height {
		lines = append(lines, "")
		targets = append(targets, "")
	}
	return region{lines: lines[:height], targets: targets[:height]}
}

// --- per-view row builders -------------------------------------------------

func buildingRows(items []svc.BuildingView, upgradeCmd string) []rowLine {
	rows := make([]rowLine, 0, len(items))
	for _, it := range items {
		st := affordStyle(it.Affordable)
		name := st.Render(fmt.Sprintf("%-22s", it.Label))
		lvl := stGold().Render(fmt.Sprintf("L%-3d", it.Level))
		cost := fmt.Sprintf("%7s %7s %7s",
			formatCompact(it.NextCost.Metal), formatCompact(it.NextCost.Crystal), formatCompact(it.NextCost.Deuterium))
		cost = st.Render(cost)
		// Fixed-width time column keeps the action column aligned across rows.
		tail := stMuted().Render(fmt.Sprintf("%7s", formatRemaining(time.Duration(it.BuildSeconds)*time.Second)))
		var action, cmd string
		switch {
		case it.Locked:
			action = stBad().Render("🔒 " + compactLockReason(it.LockedReason))
		case it.Affordable:
			action = stButton(true).Render(" build ")
			cmd = upgradeCmd + " " + it.Key
		default:
			action = stButton(false).Render(" build ")
			cmd = upgradeCmd + " " + it.Key
		}
		rows = append(rows, rowLine{
			text: fmt.Sprintf("%s %s  %s  %s  %s", name, lvl, cost, tail, action),
			cmd:  cmd,
		})
	}
	return rows
}

func unitRows(items []svc.UnitView, buildCmd string) []rowLine {
	rows := make([]rowLine, 0, len(items))
	for _, it := range items {
		st := affordStyle(it.BuildableNow > 0 && !it.Locked)
		name := st.Render(fmt.Sprintf("%-20s", it.Label))
		owned := stGold().Render(fmt.Sprintf("have %-6d", it.Owned))
		cost := st.Render(fmt.Sprintf("%7s %7s %7s",
			formatCompact(it.UnitCost.Metal), formatCompact(it.UnitCost.Crystal), formatCompact(it.UnitCost.Deuterium)))
		var action, cmd string
		switch {
		case it.Locked:
			action = stBad().Render("🔒 " + compactLockReason(it.LockedReason))
		default:
			action = stMuted().Render(fmt.Sprintf("build %d", it.BuildableNow))
			cmd = fmt.Sprintf("%s %s 1", buildCmd, it.Key)
		}
		rows = append(rows, rowLine{
			text: fmt.Sprintf("%s %s  %s  %s", name, owned, cost, action),
			cmd:  cmd,
		})
	}
	return rows
}

func researchRows(v *svc.ResearchView) []rowLine {
	if v == nil {
		return nil
	}
	rows := make([]rowLine, 0, len(v.Nodes))
	for _, n := range v.Nodes {
		indent := ""
		if n.Parent != "" {
			indent = "  └ "
		}
		st := affordStyle(n.Affordable && !n.Locked)
		name := st.Render(fmt.Sprintf("%-24s", indent+n.Label))
		lvl := stGold().Render(fmt.Sprintf("L%-3d", n.Level))
		cost := st.Render(fmt.Sprintf("%7s %7s %7s",
			formatCompact(n.NextCost.Metal), formatCompact(n.NextCost.Crystal), formatCompact(n.NextCost.Deuterium)))
		var action, cmd string
		if n.Locked {
			action = stBad().Render("🔒 " + compactLockReason(n.LockedReason))
		} else {
			action = stButton(n.Affordable).Render(" research ")
			cmd = "/research " + n.Key
		}
		rows = append(rows, rowLine{
			text: fmt.Sprintf("%s %s  %s  %s", name, lvl, cost, action),
			cmd:  cmd,
		})
	}
	return rows
}

func fleetRows(fleets []svc.Fleet) []rowLine {
	now := time.Now()
	rows := make([]rowLine, 0, len(fleets))
	for _, f := range fleets {
		when := f.ArrivalAt
		if f.State == "returning" && f.ReturnAt != nil {
			when = *f.ReturnAt
		}
		rows = append(rows, rowLine{
			text: fmt.Sprintf("#%-4d %-10s %-10s %d:%d:%d  %s", f.ID, stText().Render(f.Mission), stMuted().Render(f.State),
				f.TargetGalaxy, f.TargetSystem, f.TargetPosition, stGold().Render(formatRemaining(when.Sub(now)))),
			cmd: "",
		})
	}
	return rows
}

func systemRows(v *svc.SystemView) []rowLine {
	if v == nil {
		return nil
	}
	rows := make([]rowLine, 0, len(v.Planets))
	for _, s := range v.Planets {
		if s.PlanetName == "" {
			rows = append(rows, rowLine{text: stMuted().Render(fmt.Sprintf("%2d  —", s.Position))})
			continue
		}
		tag := ""
		if s.AllianceTag != "" {
			tag = stMuted().Render("[" + s.AllianceTag + "]")
		}
		rows = append(rows, rowLine{
			text: fmt.Sprintf("%2d  %-18s %-16s %s", s.Position, stText().Render(s.PlanetName), stGold().Render(s.OwnerName), tag),
			cmd:  fmt.Sprintf("/espionage %d:%d:%d", v.Galaxy, v.System, s.Position),
		})
	}
	return rows
}

func messageRows(msgs []svc.Message) []rowLine {
	rows := make([]rowLine, 0, len(msgs))
	for _, m := range msgs {
		flag := " "
		if !m.Read {
			flag = stBrand().Render("*")
		}
		rows = append(rows, rowLine{
			text: fmt.Sprintf("%s #%-4d %-30s %s", flag, m.ID, stText().Render(m.Subject), stMuted().Render(m.CreatedAt.Local().Format("01-02 15:04"))),
		})
	}
	return rows
}

func reportRows(reports []svc.Report) []rowLine {
	rows := make([]rowLine, 0, len(reports))
	for _, r := range reports {
		rows = append(rows, rowLine{
			text: fmt.Sprintf("#%-4d %-10s %-28s %s", r.ID, stGold().Render(r.Kind), stText().Render(r.Subject), stMuted().Render(r.CreatedAt.Local().Format("01-02 15:04"))),
		})
	}
	return rows
}

func allianceRows(as []svc.Alliance) []rowLine {
	rows := make([]rowLine, 0, len(as))
	for _, a := range as {
		rows = append(rows, rowLine{
			text: fmt.Sprintf("#%-3d %-8s %-24s %s", a.ID, stBrand().Render("["+a.Tag+"]"), stText().Render(a.Name), stMuted().Render(fmt.Sprintf("%d members", a.MemberCount))),
			cmd:  fmt.Sprintf("/alliance join %d", a.ID),
		})
	}
	return rows
}

func rankingRows(rs []svc.LeaderboardEntry) []rowLine {
	rows := make([]rowLine, 0, len(rs))
	for _, e := range rs {
		rank := fmt.Sprintf("%3d", e.Rank)
		switch e.Rank {
		case 1:
			rank = "🥇 "
		case 2:
			rank = "🥈 "
		case 3:
			rank = "🥉 "
		}
		rows = append(rows, rowLine{
			text: fmt.Sprintf("%s  %-20s %s", rank, stText().Render(e.Username), stGold().Render(formatCompact(float64(e.Score)))),
		})
	}
	return rows
}

// overviewLines renders the dashboard for the active planet: a shaded ASCII
// globe beside the metrics (when wide enough), then the build queue with
// progress bars.
func overviewLines(p *svc.Planet, prod *svc.ProductionReport, queues []svc.QueueItem, width int) []string {
	if p == nil {
		return []string{stMuted().Render("no planet")}
	}
	var info []string
	info = append(info,
		stBrand().Render(strings.ToUpper(p.Name))+stMuted().Render(fmt.Sprintf("  %d:%d:%d", p.Galaxy, p.System, p.Position)),
		stMuted().Render(fmt.Sprintf("fields %d/%d   temp %d/%d°C", p.FieldsUsed, p.FieldsTotal, p.TempMin, p.TempMax)),
		"")
	mRate, cRate, dRate := "", "", ""
	if prod != nil {
		mRate = stMuted().Render(fmt.Sprintf(" +%.0f/h", prod.MetalPerHour))
		cRate = stMuted().Render(fmt.Sprintf(" +%.0f/h", prod.CrystalPerHour))
		dRate = stMuted().Render(fmt.Sprintf(" +%.0f/h", prod.DeuteriumPerHour))
	}
	info = append(info,
		fmt.Sprintf("metal    %s%s", stGold().Render(formatCompact(p.Metal)), mRate),
		fmt.Sprintf("crystal  %s%s", stCyan().Render(formatCompact(p.Crystal)), cRate),
		fmt.Sprintf("deut     %s%s", stViolet().Render(formatCompact(p.Deuterium)), dRate),
		"")
	if prod != nil {
		info = append(info, fmt.Sprintf("energy   %s%s", energyText(prod.EnergyProduced-prod.EnergyUsed),
			stMuted().Render(fmt.Sprintf("   production %.0f%%", prod.ProductionFactor*100))))
		if prod.ProductionFactor < 1 {
			info = append(info, stBad().Render(fmt.Sprintf("⚠ energy deficit — mines throttled to %.0f%%; build a Solar Plant", prod.ProductionFactor*100)))
		}
	} else {
		info = append(info, fmt.Sprintf("energy   %s", energyText(p.EnergyProduced-p.EnergyUsed)))
	}

	// Place the globe to the left of the metrics when there's room for both.
	lines := info
	if width >= 48 {
		const globeW, globeH = 16, 8
		globe := strings.Split(renderPlanetGlobe(p.Code, p.Position, globeW, globeH), "\n")
		rows := max(len(globe), len(info))
		combined := make([]string, 0, rows)
		for i := 0; i < rows; i++ {
			left := strings.Repeat(" ", globeW)
			if i < len(globe) {
				left = padLine(globe[i], globeW)
			}
			right := ""
			if i < len(info) {
				right = info[i]
			}
			combined = append(combined, left+"   "+right)
		}
		lines = combined
	}

	if len(queues) > 0 {
		lines = append(lines, "", stHeader().Render("BUILD QUEUE"))
		now := time.Now()
		for _, q := range queues {
			filled, empty := progressBar(queueFraction(q, now), 12)
			lines = append(lines, fmt.Sprintf("  %s %-16s %s%s %3.0f%%  %s",
				queueCode(q.QueueType), queueLabel(q),
				stGood().Render(filled), stFaint().Render(empty), queueFraction(q, now)*100,
				stGold().Render(formatRemaining(q.FinishedAt.Sub(now)))))
		}
	}
	for i := range lines {
		lines[i] = clampLine(lines[i], width)
	}
	return lines
}

// queueFraction is elapsed/total for a queue item, clamped to [0,1].
func queueFraction(q svc.QueueItem, now time.Time) float64 {
	total := q.FinishedAt.Sub(q.StartedAt).Seconds()
	if total <= 0 {
		return 1
	}
	f := now.Sub(q.StartedAt).Seconds() / total
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// renderPlanetGlobe draws a deterministic shaded sphere for a planet, seeded by
// its code so each world looks stable and unique. Lambertian diffuse lighting
// over a sinusoidal "continent" pattern, ramped through " ·░▒▓█".
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
			nz := math.Sqrt(math.Max(0, 1-nx*nx-ny*ny))
			lit := math.Max(0.16, nx*math.Cos(light)+nz*math.Sin(light)+0.45)
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

// planetStyle colours a planet by slot position: hot inner worlds violet/gold,
// temperate cyan, cold outer worlds icy blue.
func planetStyle(position int) lipgloss.Style {
	switch {
	case position <= 3:
		return stViolet()
	case position <= 6:
		return stGold()
	case position <= 10:
		return stCyan()
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("159"))
	}
}

// compactLockReason shortens the server's prerequisite text so a locked row
// stays on one tidy line: "deuterium_synthesizer level 5 required, energy level
// 3 required" -> "deuterium synthesizer 5 · energy 3".
func compactLockReason(reason string) string {
	r := strings.ReplaceAll(reason, " level ", " ")
	r = strings.ReplaceAll(r, " required", "")
	r = strings.ReplaceAll(r, ", ", " · ")
	r = strings.ReplaceAll(r, "_", " ")
	return r
}

func energyText(balance int) string {
	if balance >= 0 {
		return stGood().Render(fmt.Sprintf("+%d", balance))
	}
	return stBad().Render(fmt.Sprintf("%d", balance))
}

func queueCode(t string) string {
	switch t {
	case "building":
		return stCyan().Render("B")
	case "research":
		return stViolet().Render("R")
	case "ship":
		return stGold().Render("S")
	case "defense":
		return stBrand().Render("D")
	}
	return "?"
}

func queueLabel(q svc.QueueItem) string {
	if q.TargetLevel > 0 {
		return fmt.Sprintf("%s L%d", q.ItemKey, q.TargetLevel)
	}
	if q.Count > 0 {
		return fmt.Sprintf("%s x%d", q.ItemKey, q.Count)
	}
	return q.ItemKey
}
