# Install & play

## Install the client

```bash
curl -fsSL https://terminal.army/install.sh | sh
```

The installer picks the right binary for your OS and CPU (Linux or macOS,
amd64 or arm64), verifies its SHA256 when the release ships one, and drops
`tarmy` into `/usr/local/bin` (or `~/.local/bin` if you are not root).

Verify it:

```bash
tarmy --version
```

## Play the public universe

```bash
tarmy
```

`tarmy` defaults to the public shard at `https://terminal.army`. On first run it
opens a browser sign-in URL; create an account or log in, and the terminal
client picks up the session automatically.

## Play on another server

If a friend runs their own universe:

```bash
tarmy --remote https://their-server.example
```

## Update

```bash
tarmy update            # update to the latest release
tarmy update --to vX.Y.Z
```

## Log out

```bash
tarmy --logout          # delete the saved session key for the current server
```

## The interface

The client is a full-screen TUI with a resource bar on top, a grouped menu on
the left (Empire · Ops · Social), clickable tables in the center, a live status
rail on the right, and an always-available command line at the bottom.

- **Mouse:** click a menu item to switch views, click a table row to build or
  upgrade, scroll to page through lists.
- **Keyboard:** type slash commands (see [Commands](commands.md)); `Tab`
  autocompletes, `↑`/`↓` walk history.

Every mouse action has a command equivalent, so you can play entirely by
keyboard or entirely by mouse.
