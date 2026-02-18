# PAI Agent Dashboard TUI

Real-time orchestration and observability terminal dashboard for [PAI](https://github.com/danielmiessler/PAI) agents. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Features

- **Live agent table** — Status, phase, progress bars, token throughput, and current process for every agent
- **PAI Algorithm phase tracking** — OBSERVE > THINK > PLAN > BUILD > EXECUTE > VERIFY > LEARN with visual timeline
- **Detail pane** — Token metrics, phase timeline, ISC criteria pass/fail, and recent event log per agent
- **Real-time simulation** — 2-second tick with agent state transitions, throughput fluctuation, and spawn/GC
- **Tokyo Night color palette** — Consistent with PAI design conventions
- **Keyboard-driven** — Vim-style navigation (j/k), start/stop agents, toggle detail view

## Screenshot

Run the built-in screenshot mode for a static capture:

```bash
go run main.go --screenshot
```

## Prerequisites

- Go 1.22+

## Setup

```bash
git clone https://github.com/your-username/pai-tui.git
cd pai-tui
go mod tidy
```

## Usage

```bash
go run main.go
```

Or build and run the binary:

```bash
go build -o pai-dashboard && ./pai-dashboard
```

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `Enter` | Toggle detail pane |
| `r` | Refresh |
| `s` | Start/stop selected agent |
| `q` / `Ctrl+C` | Quit |

## Project Structure

```
pai-tui/
  main.go          # Application (model, update, view, simulation)
  go.mod           # Module definition and dependencies
  go.sum           # Dependency checksums
  .gitignore       # Ignores compiled binary and OS files
```

## Tech Stack

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — Elm-architecture TUI framework
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** — Styling and layout
- **[Bubbles](https://github.com/charmbracelet/bubbles)** — Spinner, help, and key binding components

## Current Status

v0.2.0 — Simulation mode with realistic agent state transitions. Currently uses randomized data to demonstrate the dashboard layout and interactions. Future versions will connect to live PAI agent sessions.
