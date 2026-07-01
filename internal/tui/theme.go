package tui

import "github.com/charmbracelet/lipgloss"

// Colour tokens for the interactive client. We deliberately avoid forcing a
// background colour so the client blends into both light and dark terminals;
// only foreground accents are set. Values are 256-colour codes chosen to read
// on either theme.
const (
	colBrand   = lipgloss.Color("203") // red-orange primary accent
	colGold    = lipgloss.Color("220") // metal / headline numbers
	colCyan    = lipgloss.Color("81")  // crystal
	colViolet  = lipgloss.Color("177") // deuterium
	colGreen   = lipgloss.Color("78")  // ready / affordable / positive
	colRed     = lipgloss.Color("203") // locked / error / negative
	colMuted   = lipgloss.Color("244") // secondary text
	colFaint   = lipgloss.Color("240") // disabled / unaffordable
	colBorder  = lipgloss.Color("238")
	colHeader  = lipgloss.Color("111") // section / column headers
	colText    = lipgloss.Color("252")
	colSelFg   = lipgloss.Color("231")
	colSelBg   = lipgloss.Color("237")
	colHoverBg = lipgloss.Color("236")
)

func stBrand() lipgloss.Style  { return lipgloss.NewStyle().Foreground(colBrand).Bold(true) }
func stGold() lipgloss.Style   { return lipgloss.NewStyle().Foreground(colGold) }
func stCyan() lipgloss.Style   { return lipgloss.NewStyle().Foreground(colCyan) }
func stViolet() lipgloss.Style { return lipgloss.NewStyle().Foreground(colViolet) }
func stText() lipgloss.Style   { return lipgloss.NewStyle().Foreground(colText) }
func stMuted() lipgloss.Style  { return lipgloss.NewStyle().Foreground(colMuted) }
func stFaint() lipgloss.Style  { return lipgloss.NewStyle().Foreground(colFaint) }
func stHeader() lipgloss.Style { return lipgloss.NewStyle().Foreground(colHeader).Bold(true) }
func stGood() lipgloss.Style   { return lipgloss.NewStyle().Foreground(colGreen).Bold(true) }
func stBad() lipgloss.Style    { return lipgloss.NewStyle().Foreground(colRed).Bold(true) }

// stSelected renders the highlighted (keyboard-selected) table/menu row.
func stSelected() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colSelFg).Background(colSelBg).Bold(true)
}

// stHover renders the mouse-hovered row.
func stHover() lipgloss.Style {
	return lipgloss.NewStyle().Background(colHoverBg)
}

// stButton renders a clickable [Build] style cell.
func stButton(enabled bool) lipgloss.Style {
	if enabled {
		return lipgloss.NewStyle().Foreground(colSelFg).Background(colGreen).Bold(true)
	}
	return lipgloss.NewStyle().Foreground(colFaint)
}

// affordStyle returns the style for a value cell: plain white when the action
// is affordable now, faint grey when it is not (per the agreed visual rules).
func affordStyle(affordable bool) lipgloss.Style {
	if affordable {
		return stText()
	}
	return stFaint()
}

func topBarStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(colBorder)
}

func railStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colBorder).
		Padding(0, 1)
}

func menuStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1)
}

func footerStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colBorder).
		Foreground(colMuted)
}
