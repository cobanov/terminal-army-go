package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// completion is one autocomplete candidate for the command line.
type completion struct {
	value string // text inserted into the input when accepted
	label string // human-readable label shown in the popup
	desc  string // short description shown next to the label
}

// suggestionsForInput returns the autocomplete candidates for the current input
// buffer. It understands both bare command prefixes ("gal" -> "/galaxy") and
// argument position ("/upgrade metal" -> building keys).
func suggestionsForInput(input string, planets []svc.Planet) []completion {
	text := strings.TrimLeft(input, " ")
	if text == "" {
		return nil
	}
	if !strings.HasPrefix(text, "/") {
		text = "/" + text
	}
	if strings.Contains(text, " ") {
		cmd, _, _ := strings.Cut(strings.ToLower(text), " ")
		rawArg := strings.TrimPrefix(text[strings.Index(text, " ")+1:], " ")
		cmd = strings.TrimSpace(cmd)
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
		{"/queue", "/queue", "active build and research queue"},
		{"/research ", "/research <tech>", "queue research"},
		{"/tree", "/tree", "research tree and prerequisites"},
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
