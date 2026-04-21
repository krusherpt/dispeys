# dispeys

Linux tray application for Ulanzi D200 stream deck devices. Assign custom buttons based on the currently active window.

## Features

- **Window-aware button layouts** — switch button configurations automatically when you switch apps
- **Web UI** — configure buttons via a browser at `http://localhost:19876` (tray menu: "Web Settings")
- **Command buttons** — launch apps, focus windows, switch profiles, or run shell commands
- **Autostart** — toggle on login via tray menu

## Quick Start

```bash
go build -o dispeys ./cmd/controller/
./dispeys
```

A tray icon appears. Click **"Web Settings"** to open the web UI in your browser.

## Requirements

- Linux with X11
- `xdotool`, `xprop`, `wmctrl`, `xdg-mime`

## Build

```bash
go build -o dispeys ./cmd/controller/
```

## Settings

Settings are stored in `~/.config/dispeysController/settings.json`.

Command prefixes:
- `@<process>` — switch to another process's layout
- `$<process>` — focus running instance or launch the app
- plain text — execute as a shell command

## Architecture

| Package | Role |
|---------|------|
| `cmd/controller` | Tray menu, main loop |
| `pkg/ulanzid200` | HID device communication |
| `pkg/app_detector` | Active window detection, settings |
| `webserver` | Embedded web UI for configuration |
