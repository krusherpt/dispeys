# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

dispeys is a Linux tray application that controls Ulanzi D200 stream deck devices, allowing users to assign custom button configurations based on the currently active window's process.

## Building and Running

```bash
# Build the controller binary
go build -o dispeys ./cmd/controller/

# Run the application (requires GUI environment)
./dispeys
```

## Development Workflow

The application runs as a system tray daemon with two main components:

1. **Hardware Layer** (`pkg/ulanzid200/`): Communicates with Ulanzi D200 device via HID USB protocol
2. **Application Detection** (`pkg/app_detector/`): Monitors active window using `xdotool`, `xprop`, and `wmctrl`

## Architecture

### Key Files

- `cmd/controller/main.go` - Main entry point with tray menu (autostart, settings, exit)
- `pkg/ulanzid200/UlanziD200Device.go` - HID device communication, packet building, button/icon updates
- `pkg/app_detector/AppDetector.go` - Active window detection loop
- `pkg/app_detector/AppSelect.go` - Window focus/launch logic (`$command` buttons)
- `pkg/app_detector/Settings.go` - JSON settings loading/saving with embedded defaults

### Configuration System

User settings stored at `~/.config/dispeysController/settings.json`. Structure:

```json
{
  "process_name": {
    "buttons": [
      {"name": "...", "icon": "...png", "command": "..."}
    ]
  }
}
```

Special command prefixes:
- `@<process>` - Switch to another process's button configuration (e.g., `@select_app`)
- `$<process>` - Focus running instance or launch the application (e.g., `$chrome`)
- plain shell command - Execute directly via `sh -c`

### Button Indexing

13 buttons arranged in a 3x5 grid, indexed row-major:
```
0   1   2   3   4
5   6   7   8   9
10  11  12
```

Button 12 (last) toggles small window mode when pressed.

### Device Protocol Details

- Vendor ID: `0x2207`, Product ID: `0x0019`
- USB packets are 1024 bytes with header (`0x7c 0x7c`) and command protocol
- Icons (196x196) sent as ZIP archives embedded in packet data
- Small window updates run every 500ms to keep device alive

### Settings File Locations

```
~/.config/dispeysController/
  settings.json    # User-configured button mappings
  icons/           # Icon files used by buttons
```

Temp directory: `/tmp/dispeysController/` (for ZIP building)

## Dependencies

- `github.com/karalabe/hid` - HID device access
- `github.com/getlantern/systray` - System tray icon
- `gotk3/gotk3` - GTK for editor detection (xdg-mime query)

## External Tools Required

The application depends on these Linux tools:
- `xdotool` - Window manipulation and active window detection
- `xprop` - Process ID lookup from window ID
- `wmctrl` - Window focus control and PID-based window listing
- `xdg-mime` - Default text editor detection

## Common Tasks

### Adding a new button action
1. Define the command in `~/.config/dispeysController/settings.json` for your process
2. Add icon to `pkg/app_detector/icons/` if needed (also embedded via `//go:embed`)

### Debugging device communication
Add `fmt.Printf` statements around the `keyPressedEvent` log in `main.go` where keypress events are logged. The device sends button press data that can be inspected there.

### Settings Loading

Initial default settings are loaded 500ms after startup in `onReady()` via `LoadAppSettings()` + `GetSettingsForProcess("default")`.

When no settings exist for the detected process, `AppDetector` falls back to the `"default"` process configuration.

`setSettings()` now includes a nil guard at the top.

### Device Protocol Constants

New input protocol constants in `UlanziD200Device.go`:
- `IN_BRIGHTNESS_RESPONSE` (`0x030a`) - device brightness acknowledgement
- Unknown inputs `0x0103` and `0x0104` are ignored in `ParsedPacket.go`
