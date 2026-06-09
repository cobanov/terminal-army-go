package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cobanov/terminal-army-go/internal/game"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/tui/client"
)

const DefaultServerURL = "https://terminal.army"

// RunREPL starts the Python-compatible slash-command client.
func RunREPL(ctx context.Context, serverURL string, logout bool) error {
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
	fmt.Printf("terminal.army %s\n", c.BaseURL())
	fmt.Println("type /help for commands, /q to quit")
	if err := r.printPlanet(ctx); err != nil {
		return err
	}
	return r.loop(ctx)
}

type replSession struct {
	client       *client.Client
	user         *svc.User
	planets      []svc.Planet
	currentIndex int
}

func acquireSession(ctx context.Context, c *client.Client) (*svc.User, error) {
	if cached := LoadCreds(c.BaseURL()); cached != nil {
		c.SetToken(cached.Token)
		user, err := withTimeout(ctx, func(ctx context.Context) (*svc.User, error) {
			return c.AuthMe(ctx)
		})
		if err == nil {
			return user, nil
		}
		_ = ClearCreds(c.BaseURL())
	}

	start, err := withTimeout(ctx, func(ctx context.Context) (*svc.DeviceAuthStart, error) {
		return c.StartDeviceAuth(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("start browser auth: %w", err)
	}
	loginURL := c.BaseURL() + "/login?code=" + url.QueryEscape(start.AuthCode)
	fmt.Println()
	fmt.Println("Open this URL in your browser:")
	fmt.Println("  " + loginURL)
	fmt.Println("Sign in or create an account. Waiting for the terminal session...")
	openBrowser(loginURL)

	deadline := time.Now().Add(time.Duration(start.ExpiresIn) * time.Second)
	interval := time.Duration(start.PollingInterval) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for time.Now().Before(deadline) {
		poll, err := withTimeout(ctx, func(ctx context.Context) (*svc.DeviceAuthPoll, error) {
			return c.PollDeviceAuth(ctx, start.AuthCode)
		})
		if err == nil && poll.Token != "" {
			c.SetToken(poll.Token)
			user, err := withTimeout(ctx, func(ctx context.Context) (*svc.User, error) {
				return c.AuthMe(ctx)
			})
			if err != nil {
				return nil, fmt.Errorf("validate browser token: %w", err)
			}
			_ = SaveCreds(&CachedCreds{ServerURL: c.BaseURL(), Token: poll.Token, Username: user.Username})
			fmt.Println("authentication complete")
			return user, nil
		}
		var apiErr *client.APIError
		if err != nil && errors.As(err, &apiErr) && apiErr.Status != 202 {
			return nil, err
		}
		time.Sleep(interval)
	}
	return nil, errors.New("auth timed out")
}

func (r *replSession) loop(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("tarmy> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "/") {
			fmt.Println("commands start with /. type /help")
			continue
		}
		if err := r.exec(ctx, line); err != nil {
			if errors.Is(err, errQuit) {
				return nil
			}
			fmt.Println("error:", err)
		}
	}
}

var errQuit = errors.New("quit")

func (r *replSession) exec(ctx context.Context, line string) error {
	fields := strings.Fields(line)
	cmd := strings.TrimPrefix(fields[0], "/")
	args := fields[1:]
	switch cmd {
	case "q", "quit", "exit":
		return errQuit
	case "help", "h":
		printHelp()
	case "planet", "p":
		return r.printPlanet(ctx)
	case "switch":
		return r.switchPlanet(ctx, args)
	case "upgrade", "u":
		return r.queueBuilding(ctx, args)
	case "research", "r":
		return r.research(ctx, args)
	case "ships", "ship", "s":
		return r.ships(ctx, args)
	case "defense", "def":
		return r.defense(ctx, args)
	case "galaxy", "g":
		return r.galaxy(ctx, args)
	case "fleet", "fleets":
		return r.fleets(ctx)
	case "attack", "atk":
		return r.dispatch(ctx, "attack", args)
	case "transport", "tx":
		return r.dispatch(ctx, "transport", args)
	case "espionage", "spy":
		return r.dispatch(ctx, "espionage", args)
	case "msg", "message":
		return r.message(ctx, args)
	case "messages", "inbox":
		return r.messages(ctx)
	case "alliance", "ally":
		return r.alliance(ctx, args)
	case "leaderboard", "rank", "lb":
		return r.leaderboard(ctx)
	case "quest":
		return r.quest(ctx)
	case "info":
		return r.info(args)
	default:
		fmt.Println("unknown command. type /help")
	}
	return nil
}

func (r *replSession) ensurePlanets(ctx context.Context) error {
	ps, err := withTimeout(ctx, func(ctx context.Context) ([]svc.Planet, error) {
		return r.client.ListPlanets(ctx)
	})
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		us, err := withTimeout(ctx, func(ctx context.Context) ([]svc.Universe, error) {
			return r.client.ListUniverses(ctx)
		})
		if err != nil {
			return err
		}
		if len(us) == 0 {
			return errors.New("no universe available; ask the operator to run seed-universe")
		}
		p, err := withTimeout(ctx, func(ctx context.Context) (*svc.Planet, error) {
			return r.client.JoinUniverse(ctx, us[0].ID)
		})
		if err != nil {
			return err
		}
		ps = []svc.Planet{*p}
	}
	r.planets = ps
	if r.currentIndex >= len(r.planets) {
		r.currentIndex = 0
	}
	return nil
}

func (r *replSession) currentPlanet() (*svc.Planet, error) {
	if len(r.planets) == 0 {
		return nil, errors.New("no planets")
	}
	return &r.planets[r.currentIndex], nil
}

func (r *replSession) refreshCurrent(ctx context.Context) (*svc.Planet, error) {
	p, err := r.currentPlanet()
	if err != nil {
		return nil, err
	}
	out, err := withTimeout(ctx, func(ctx context.Context) (*svc.Planet, error) {
		return r.client.GetPlanet(ctx, p.ID)
	})
	if err != nil {
		return nil, err
	}
	r.planets[r.currentIndex] = *out
	return out, nil
}

func (r *replSession) printPlanet(ctx context.Context) error {
	p, err := r.refreshCurrent(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%s [%s] %d:%d:%d\n", p.Name, p.Code, p.Galaxy, p.System, p.Position)
	fmt.Printf("resources  metal %.0f  crystal %.0f  deuterium %.0f\n", p.Metal, p.Crystal, p.Deuterium)
	fmt.Printf("energy     %d / %d\n", p.EnergyUsed, p.EnergyProduced)
	printLevels("buildings", p.Buildings)
	if len(p.Ships) > 0 {
		printLevels("ships", p.Ships)
	}
	if len(p.Defense) > 0 {
		printLevels("defense", p.Defense)
	}
	qs, err := withTimeout(ctx, func(ctx context.Context) ([]svc.QueueItem, error) {
		return r.client.GetQueues(ctx, p.ID)
	})
	if err == nil && len(qs) > 0 {
		fmt.Println("queue")
		for _, q := range qs {
			fmt.Printf("  #%d %s %s x%d -> %s\n", q.ID, q.QueueType, q.ItemKey, max(q.Count, q.TargetLevel), q.FinishedAt.Local().Format(time.Kitchen))
		}
	}
	return nil
}

func (r *replSession) switchPlanet(ctx context.Context, args []string) error {
	if len(args) == 0 {
		for i, p := range r.planets {
			fmt.Printf("%d. %s [%s] %d:%d:%d\n", i+1, p.Name, p.Code, p.Galaxy, p.System, p.Position)
		}
		return nil
	}
	want := strings.ToLower(args[0])
	for i, p := range r.planets {
		if strconv.Itoa(i+1) == want || strings.ToLower(p.Code) == want || strings.HasPrefix(strings.ToLower(p.Name), want) {
			r.currentIndex = i
			return r.printPlanet(ctx)
		}
	}
	return errors.New("planet not found")
}

func (r *replSession) queueBuilding(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printCatalog("buildings", BuildingCatalog)
		return nil
	}
	p, err := r.currentPlanet()
	if err != nil {
		return err
	}
	item, err := withTimeout(ctx, func(ctx context.Context) (*svc.QueueItem, error) {
		return r.client.QueueBuilding(ctx, p.ID, args[0])
	})
	if err != nil {
		return err
	}
	fmt.Printf("queued %s level %d, finishes %s\n", item.ItemKey, item.TargetLevel, item.FinishedAt.Local().Format(time.Kitchen))
	return nil
}

func (r *replSession) research(ctx context.Context, args []string) error {
	if len(args) == 0 {
		rows, err := withTimeout(ctx, func(ctx context.Context) ([]svc.ResearchLevel, error) {
			return r.client.ListResearch(ctx)
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			fmt.Printf("%-22s %d\n", row.Tech, row.Level)
		}
		return nil
	}
	p, err := r.currentPlanet()
	if err != nil {
		return err
	}
	item, err := withTimeout(ctx, func(ctx context.Context) (*svc.QueueItem, error) {
		return r.client.QueueResearch(ctx, p.ID, args[0])
	})
	if err != nil {
		return err
	}
	fmt.Printf("queued %s level %d, finishes %s\n", item.ItemKey, item.TargetLevel, item.FinishedAt.Local().Format(time.Kitchen))
	return nil
}

func (r *replSession) ships(ctx context.Context, args []string) error {
	p, err := r.currentPlanet()
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		p, err := r.refreshCurrent(ctx)
		if err != nil {
			return err
		}
		printLevels("ships", p.Ships)
		return nil
	}
	if args[0] != "build" || len(args) < 3 {
		fmt.Println("usage: /ships build small_cargo 5")
		return nil
	}
	count, err := strconv.Atoi(args[2])
	if err != nil {
		return err
	}
	item, err := withTimeout(ctx, func(ctx context.Context) (*svc.QueueItem, error) {
		return r.client.QueueShip(ctx, p.ID, args[1], count)
	})
	if err != nil {
		return err
	}
	fmt.Printf("queued %s x%d, finishes %s\n", item.ItemKey, item.Count, item.FinishedAt.Local().Format(time.Kitchen))
	return nil
}

func (r *replSession) defense(ctx context.Context, args []string) error {
	p, err := r.currentPlanet()
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		p, err := r.refreshCurrent(ctx)
		if err != nil {
			return err
		}
		printLevels("defense", p.Defense)
		return nil
	}
	if args[0] != "build" || len(args) < 3 {
		fmt.Println("usage: /defense build rocket_launcher 10")
		return nil
	}
	count, err := strconv.Atoi(args[2])
	if err != nil {
		return err
	}
	item, err := withTimeout(ctx, func(ctx context.Context) (*svc.QueueItem, error) {
		return r.client.QueueDefense(ctx, p.ID, args[1], count)
	})
	if err != nil {
		return err
	}
	fmt.Printf("queued %s x%d, finishes %s\n", item.ItemKey, item.Count, item.FinishedAt.Local().Format(time.Kitchen))
	return nil
}

func (r *replSession) galaxy(ctx context.Context, args []string) error {
	p, err := r.currentPlanet()
	if err != nil {
		return err
	}
	g, s := p.Galaxy, p.System
	if len(args) > 0 {
		parts := strings.Split(args[0], ":")
		if len(parts) >= 2 {
			g, _ = strconv.Atoi(parts[0])
			s, _ = strconv.Atoi(parts[1])
		}
	}
	view, err := withTimeout(ctx, func(ctx context.Context) (*svc.SystemView, error) {
		return r.client.ViewSystem(ctx, g, s)
	})
	if err != nil {
		return err
	}
	fmt.Printf("galaxy %d:%d\n", view.Galaxy, view.System)
	for _, slot := range view.Planets {
		if slot.PlanetName == "" {
			fmt.Printf("%2d  --\n", slot.Position)
			continue
		}
		fmt.Printf("%2d  %-18s %-16s %s\n", slot.Position, slot.PlanetName, slot.OwnerName, slot.AllianceTag)
	}
	return nil
}

func (r *replSession) dispatch(ctx context.Context, mission string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: /%s g:s:p ship=count [m=0 c=0 d=0]", mission)
	}
	g, s, pos, err := parseCoord(args[0])
	if err != nil {
		return err
	}
	ships, cargo := parseKVArgs(args[1:])
	if mission == "espionage" && len(ships) == 0 {
		ships[string(game.ShipEspionageProbe)] = 1
	}
	if mission == "transport" && len(ships) == 0 {
		ships[string(game.ShipSmallCargo)] = 1
	}
	if len(ships) == 0 {
		return errors.New("add ships, for example small_cargo=1")
	}
	p, err := r.currentPlanet()
	if err != nil {
		return err
	}
	f, err := withTimeout(ctx, func(ctx context.Context) (*svc.Fleet, error) {
		return r.client.DispatchFleet(ctx, svc.FleetDispatchRequest{
			OriginPlanetID: p.ID,
			TargetGalaxy:   g,
			TargetSystem:   s,
			TargetPosition: pos,
			Mission:        mission,
			Ships:          ships,
			Cargo:          cargo,
			SpeedPercent:   100,
		})
	})
	if err != nil {
		return err
	}
	fmt.Printf("fleet #%d %s to %d:%d:%d, arrives %s\n", f.ID, f.Mission, g, s, pos, f.ArrivalAt.Local().Format(time.Kitchen))
	return nil
}

func (r *replSession) fleets(ctx context.Context) error {
	rows, err := withTimeout(ctx, func(ctx context.Context) ([]svc.Fleet, error) {
		return r.client.ListFleet(ctx)
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("no fleets in flight")
		return nil
	}
	for _, f := range rows {
		fmt.Printf("#%d %-10s %-9s %d:%d:%d arrives %s\n", f.ID, f.Mission, f.State, f.TargetGalaxy, f.TargetSystem, f.TargetPosition, f.ArrivalAt.Local().Format(time.Kitchen))
	}
	return nil
}

func (r *replSession) message(ctx context.Context, args []string) error {
	if len(args) < 2 {
		fmt.Println("usage: /msg username hello")
		return nil
	}
	body := strings.Join(args[1:], " ")
	_, err := withTimeout(ctx, func(ctx context.Context) (*svc.Message, error) {
		return r.client.SendMessage(ctx, args[0], body)
	})
	if err != nil {
		return err
	}
	fmt.Println("sent")
	return nil
}

func (r *replSession) messages(ctx context.Context) error {
	rows, err := withTimeout(ctx, func(ctx context.Context) ([]svc.Message, error) {
		return r.client.ListMessages(ctx)
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("inbox empty")
		return nil
	}
	for _, m := range rows {
		read := " "
		if !m.Read {
			read = "*"
		}
		fmt.Printf("%s #%d %-18s %s\n", read, m.ID, m.Subject, m.CreatedAt.Local().Format(time.Kitchen))
	}
	return nil
}

func (r *replSession) alliance(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "list" {
		rows, err := withTimeout(ctx, func(ctx context.Context) ([]svc.Alliance, error) {
			return r.client.ListAlliances(ctx)
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			fmt.Println("no alliances yet")
			return nil
		}
		for _, a := range rows {
			fmt.Printf("#%d [%s] %-24s members:%d\n", a.ID, a.Tag, a.Name, a.MemberCount)
		}
		return nil
	}
	switch args[0] {
	case "create":
		if len(args) < 3 {
			fmt.Println("usage: /alliance create TAG Name")
			return nil
		}
		a, err := withTimeout(ctx, func(ctx context.Context) (*svc.Alliance, error) {
			return r.client.CreateAlliance(ctx, args[1], args[2], strings.Join(args[3:], " "))
		})
		if err != nil {
			return err
		}
		fmt.Printf("created [%s] %s\n", a.Tag, a.Name)
	case "join":
		if len(args) < 2 {
			return errors.New("usage: /alliance join <id>")
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}
		return r.client.JoinAlliance(ctx, id)
	case "leave":
		if len(args) < 2 {
			return errors.New("usage: /alliance leave <id>")
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}
		return r.client.LeaveAlliance(ctx, id)
	default:
		fmt.Println("usage: /alliance [list|create|join|leave]")
	}
	return nil
}

func (r *replSession) leaderboard(ctx context.Context) error {
	rows, err := withTimeout(ctx, func(ctx context.Context) ([]svc.LeaderboardEntry, error) {
		return r.client.Leaderboard(ctx)
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%3d %-18s %d\n", row.Rank, row.Username, row.Score)
	}
	return nil
}

func (r *replSession) quest(ctx context.Context) error {
	p, err := r.refreshCurrent(ctx)
	if err != nil {
		return err
	}
	if p.Buildings[string(game.BuildingMetalMine)] == 0 {
		fmt.Println("current quest: build a metal mine with /upgrade metal_mine")
		return nil
	}
	if p.Buildings[string(game.BuildingCrystalMine)] == 0 {
		fmt.Println("current quest: build a crystal mine with /upgrade crystal_mine")
		return nil
	}
	if p.Buildings[string(game.BuildingResearchLab)] == 0 {
		fmt.Println("current quest: build a research lab with /upgrade research_lab")
		return nil
	}
	fmt.Println("current quest: scout the galaxy with /galaxy")
	return nil
}

func (r *replSession) info(args []string) error {
	if len(args) == 0 {
		printCatalog("buildings", BuildingCatalog)
		printCatalog("research", ResearchCatalog)
		printCatalog("ships", ShipCatalog)
		printCatalog("defense", DefenseCatalog)
		return nil
	}
	key := args[0]
	for _, group := range [][]CatalogItem{BuildingCatalog, ResearchCatalog, ShipCatalog, DefenseCatalog} {
		for _, item := range group {
			if item.Key == key {
				fmt.Printf("%s: %s\n", item.Key, item.Label)
				return nil
			}
		}
	}
	return errors.New("unknown item")
}

func parseCoord(s string) (int, int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, 0, 0, errors.New("coordinate must be g:s:p")
	}
	g, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}
	sys, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, err
	}
	pos, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, err
	}
	return g, sys, pos, nil
}

func parseKVArgs(args []string) (map[string]int, map[string]int) {
	ships := map[string]int{}
	cargo := map[string]int{}
	for _, arg := range args {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			continue
		}
		switch k {
		case "m", "metal":
			cargo["metal"] = n
		case "c", "crystal":
			cargo["crystal"] = n
		case "d", "deuterium":
			cargo["deuterium"] = n
		default:
			ships[k] = n
		}
	}
	return ships, cargo
}

func printHelp() {
	fmt.Println(`/planet                 show current planet
/switch 2               switch planet by number, code, or name
/upgrade metal_mine     queue a building
/research energy        queue research from current planet
/ships build key n      build ships; /ships lists inventory
/defense build key n    build defenses; /defense lists inventory
/galaxy [g:s]           show a system
/attack g:s:p ship=n    send an attack fleet
/transport g:s:p m=n    transport resources
/espionage g:s:p        send one espionage probe
/fleet                  show active fleets
/msg user text          send a player message
/messages               show inbox
/alliance               list/create/join/leave alliances
/leaderboard            show rankings
/quest                  show the next suggested step
/info [key]             list or describe known item keys
/q                      quit`)
}

func printCatalog(title string, rows []CatalogItem) {
	fmt.Println(title)
	for _, item := range rows {
		fmt.Printf("  %-24s %s\n", item.Key, item.Label)
	}
}

func printLevels(title string, rows map[string]int) {
	if len(rows) == 0 {
		fmt.Printf("%s: none\n", title)
		return
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println(title)
	for _, k := range keys {
		fmt.Printf("  %-24s %d\n", k, rows[k])
	}
}

func withTimeout[T any](ctx context.Context, f func(context.Context) (T, error)) (T, error) {
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return f(callCtx)
}

func openBrowser(rawURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	_ = cmd.Start()
}
