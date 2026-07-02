package tui

import (
	"fmt"
	"time"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// railData is the always-on live status shown in the right rail. It also
// carries the HUD inputs (planet snapshot, production, online count, sync time)
// so the top bar shows live figures on every view, not just the overview.
type railData struct {
	queues   []svc.QueueItem
	fleets   []svc.Fleet
	messages []svc.Message
	planet   *svc.Planet
	prod     *svc.ProductionReport
	online   int
	syncedAt time.Time
}

// renderRail builds the right-rail region. Section headers and rows are
// clickable and jump to the matching view (via its slash command).
func renderRail(d railData, width, height int) region {
	var lines, targets []string
	push := func(s, cmd string) {
		lines = append(lines, clampLine(s, width))
		targets = append(targets, cmd)
	}
	now := time.Now()

	push(stHeader().Render(fmt.Sprintf("QUEUE (%d)", len(d.queues))), "/queue")
	if len(d.queues) == 0 {
		push(stMuted().Render("  idle"), "")
	}
	for i, q := range d.queues {
		if i >= 4 {
			break
		}
		filled, empty := progressBar(queueFraction(q, now), 5)
		push(fmt.Sprintf(" %s %-8s %s%s %s",
			queueCode(q.QueueType), clampLine(queueLabel(q), 8),
			stGood().Render(filled), stFaint().Render(empty),
			stGold().Render(formatRemaining(q.FinishedAt.Sub(now)))), "/queue")
	}

	push("", "")
	push(stHeader().Render(fmt.Sprintf("FLEETS (%d)", len(d.fleets))), "/fleet")
	if len(d.fleets) == 0 {
		push(stMuted().Render("  none"), "")
	}
	for i, f := range d.fleets {
		if i >= 4 {
			break
		}
		when := f.ArrivalAt
		if f.State == "returning" && f.ReturnAt != nil {
			when = *f.ReturnAt
		}
		push(fmt.Sprintf(" %-8s %d:%d:%d %s", f.Mission, f.TargetGalaxy, f.TargetSystem, f.TargetPosition, stGold().Render(formatRemaining(when.Sub(now)))), "/fleet")
	}

	unread := 0
	for _, m := range d.messages {
		if !m.Read {
			unread++
		}
	}
	push("", "")
	push(stHeader().Render(fmt.Sprintf("INBOX (%d)", unread)), "/messages")
	shown := 0
	for _, m := range d.messages {
		if shown >= 4 {
			break
		}
		flag := " "
		if !m.Read {
			flag = stBrand().Render("*")
		}
		push(fmt.Sprintf("%s %s", flag, clampLine(m.Subject, width-3)), "/messages")
		shown++
	}
	if len(d.messages) == 0 {
		push(stMuted().Render("  empty"), "")
	}

	for len(lines) < height {
		lines = append(lines, "")
		targets = append(targets, "")
	}
	return region{lines: lines[:height], targets: targets[:height]}
}

// railStrip is the compact one-line summary shown under the top bar in
// two-column mode, where the full rail is hidden.
func railStrip(d railData, width int) string {
	unread := 0
	for _, m := range d.messages {
		if !m.Read {
			unread++
		}
	}
	s := fmt.Sprintf("%s queue %d   %s fleets %d   %s inbox %d",
		stCyan().Render("▪"), len(d.queues),
		stGold().Render("▪"), len(d.fleets),
		stBrand().Render("▪"), unread)
	return clampLine(s, width)
}
