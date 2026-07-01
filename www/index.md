# terminal.army

An **OGame-style multiplayer strategy game you play from a terminal**. Build
mines, research technology, construct fleets, and raid other players — all from
a single, fast, keyboard- and mouse-driven terminal client.

terminal.army ships as one statically-linked Go binary (`tarmy`) backed by a
single Postgres database. No JavaScript build pipeline, no broker, no extra
runtime.

<div class="grid cards" markdown>

- :material-rocket-launch: **[Install & play](install.md)**
  One line to install, then `tarmy` to jump into the public universe.

- :material-console: **[Commands](commands.md)**
  Every slash command and what it does.

- :material-calculator-variant: **[Game mechanics](mechanics.md)**
  The formulas behind mines, energy, build time, research, and fleets.

- :material-server: **[Self-hosting](self-hosting.md)**
  Run your own universe with Docker Compose.

</div>

## What makes it different

- **Terminal-native.** The whole game is a rich TUI: a resource bar, a grouped
  menu, clickable tables, a live command line, and a status rail — with full
  mouse support and a responsive layout.
- **Real mechanics.** Every formula, coefficient, and tech-tree prerequisite is
  sourced from the [OGame Fandom Wiki](https://ogame.fandom.com/wiki/OGame_Wiki).
- **One binary.** The same `tarmy` binary serves the HTTP API, the queue
  scheduler, and the terminal client.

## Quick start

```bash
curl -fsSL https://terminal.army/install.sh | sh
tarmy
```

That's it — `tarmy` opens a browser sign-in, then drops you into your empire.
See [Install & play](install.md) for details.
