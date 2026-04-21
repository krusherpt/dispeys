# dispeys

Linux tray application for Ulanzi D200 stream deck devices. Assign custom buttons based on the currently active window.

## Features

- **Window-aware button layouts** — switch button configurations automatically when you switch apps
- **Command buttons** — launch apps, focus windows, switch profiles, or run shell commands
- **Autostart** — toggle on login via tray menu

## Requirements

- Linux with X11
- `xdotool`, `xprop`, `wmctrl`, `xdg-mime`
- Go 1.21+
- GCC (for CGO HID library)

## Build

```bash
go build -o dispeys ./cmd/controller/
./dispeys
```

## Install as System Service

```bash
sudo ./install.sh
sudo systemctl enable dispeys
sudo systemctl start dispeys
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
