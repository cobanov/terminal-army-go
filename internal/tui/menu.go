package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// groupHeaderStyle gives each menu group its own accent so the sections read at
// a glance.
func groupHeaderStyle(title string) lipgloss.Style {
	switch title {
	case "EMPIRE":
		return lipgloss.NewStyle().Foreground(colGold).Bold(true)
	case "OPS":
		return lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	case "SOCIAL":
		return lipgloss.NewStyle().Foreground(colViolet).Bold(true)
	}
	return stHeader()
}

// viewID identifies the content shown in the center column.
type viewID int

const (
	viewOverview viewID = iota
	viewBuildings
	viewFacilities
	viewResearch
	viewShipyard
	viewDefense
	viewFleet
	viewGalaxy
	viewMessages
	viewReports
	viewAlliance
	viewRanking
)

// menuEntry is one selectable item in the left menu. command is the slash
// command run when the entry is chosen (so mouse and keyboard share one path).
type menuEntry struct {
	id      viewID
	label   string
	short   string // footer hotkey label
	command string
}

type menuGroup struct {
	title   string
	entries []menuEntry
}

// menuGroups is the grouped left menu, per the agreed design.
var menuGroups = []menuGroup{
	{"EMPIRE", []menuEntry{
		{viewOverview, "Overview", "overview", "/planet"},
		{viewBuildings, "Buildings", "build", "/resources"},
		{viewFacilities, "Facilities", "facilities", "/facilities"},
		{viewResearch, "Research", "research", "/tree"},
		{viewShipyard, "Shipyard", "shipyard", "/ships"},
		{viewDefense, "Defense", "defense", "/defense"},
	}},
	{"OPS", []menuEntry{
		{viewFleet, "Fleet", "fleet", "/fleet"},
		{viewGalaxy, "Galaxy", "galaxy", "/galaxy"},
	}},
	{"SOCIAL", []menuEntry{
		{viewMessages, "Messages", "inbox", "/messages"},
		{viewReports, "Reports", "reports", "/reports"},
		{viewAlliance, "Alliance", "alliance", "/alliance"},
		{viewRanking, "Ranking", "ranking", "/leaderboard"},
	}},
}

// allMenuEntries flattens the groups into display order.
func allMenuEntries() []menuEntry {
	var out []menuEntry
	for _, g := range menuGroups {
		out = append(out, g.entries...)
	}
	return out
}

func menuEntryFor(id viewID) menuEntry {
	for _, e := range allMenuEntries() {
		if e.id == id {
			return e
		}
	}
	return menuEntry{id: viewOverview, label: "Overview", command: "/planet"}
}

// renderMenu draws the vertical grouped menu and returns, for every rendered
// line, the viewID it activates (or -1 for headers/blank lines). hover is the
// currently hovered viewID (-1 for none).
func renderMenu(active, hover viewID, width, height int) (string, []int) {
	var lines []string
	var targets []int
	add := func(s string, target int) {
		lines = append(lines, clampLine(s, width))
		targets = append(targets, target)
	}
	for gi, g := range menuGroups {
		if gi > 0 {
			add("", -1)
		}
		add(groupHeaderStyle(g.title).Render(g.title), -1)
		for _, e := range g.entries {
			marker := "  "
			style := stText()
			switch {
			case e.id == active:
				marker = stBrand().Render("▸ ")
				style = stBrand()
			case e.id == hover:
				style = stText().Background(colHoverBg)
			}
			add(marker+style.Render(e.label), int(e.id))
		}
	}
	// Pad/trim to height and keep targets aligned.
	for len(lines) < height {
		lines = append(lines, "")
		targets = append(targets, -1)
	}
	if len(lines) > height {
		lines = lines[:height]
		targets = targets[:height]
	}
	return strings.Join(lines, "\n"), targets
}

// renderMenuTabs draws the narrow horizontal tab bar and returns per-column
// hit targets is handled by the caller via tabZones.
func renderMenuTabs(active viewID, width int) string {
	var parts []string
	for _, e := range allMenuEntries() {
		label := " " + e.label + " "
		if e.id == active {
			parts = append(parts, stSelected().Render(label))
		} else {
			parts = append(parts, stMuted().Render(label))
		}
	}
	return clampLine(strings.Join(parts, "·"), width)
}
