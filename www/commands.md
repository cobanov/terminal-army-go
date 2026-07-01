# Commands

Commands start with `/`. Type `/help` in the client for this list, and `/q` to
quit. Most commands accept short aliases (shown in parentheses).

## Planet & economy

| Command | Description |
|---------|-------------|
| `/planet` (`/p`) | Show the current planet: resources, buildings, queue. |
| `/switch <n>` | Switch planet by number, code, or name. |
| `/planets` | List your planets. |
| `/resources` | Resource buildings: mines, energy, storage, crawlers. |
| `/facilities` | Facilities: robotics, shipyard, lab, nanite, silo, depot. |
| `/upgrade <key>` (`/u`) | Queue a building upgrade, e.g. `/upgrade metal_mine`. |
| `/queue` | Show the active build/research queue. |
| `/refresh` | Re-fetch the current planet. |

## Research

| Command | Description |
|---------|-------------|
| `/research [key]` (`/r`) | List tech levels, or queue a tech from the current planet. |
| `/tree` | Show the research tree and prerequisites. |

## Fleet & military

| Command | Description |
|---------|-------------|
| `/ships [build <key> <n>]` (`/s`) | List ships, or build them: `/ships build small_cargo 5`. |
| `/defense [build <key> <n>]` (`/def`) | List or build defenses. |
| `/fleet` | Show fleets in flight. |
| `/attack <g:s:p> ship=n` (`/atk`) | Send an attack fleet. |
| `/transport <g:s:p> m=n c=n d=n` (`/tx`) | Transport resources. |
| `/espionage <g:s:p>` (`/spy`) | Send an espionage probe. |
| `/galaxy [g:s]` (`/g`) | View a system. |
| `/reports` | Combat and espionage reports. |

## Social

| Command | Description |
|---------|-------------|
| `/msg <user> <text>` | Send a player message. |
| `/messages` (`/inbox`) | Show your inbox. |
| `/alliance` (`/ally`) | List / create / join / leave alliances. |
| `/leaderboard` (`/rank`, `/lb`) | Global rankings. |

## Help & meta

| Command | Description |
|---------|-------------|
| `/quest` | The next suggested step for your empire. |
| `/info [key]` | List or describe known item keys. |
| `/help` (`/h`) | Command reference. |
| `/logout` | Log out and quit. |
| `/q` (`/quit`, `/exit`) | Quit. |

## Coordinates & arguments

Coordinates are written `galaxy:system:position`, e.g. `2:88:4`. Fleet
composition uses `key=count`, and cargo uses `m=`, `c=`, `d=`:

```
/attack 2:88:4 light_fighter=20 cruiser=5
/transport 3:41:8 large_cargo=10 m=100000 c=50000
```
