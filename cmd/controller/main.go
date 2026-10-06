package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"

	"github.com/bjaka-max/dispeys/cmd/controller/config"
	appdetector "github.com/bjaka-max/dispeys/pkg/app_detector"
	"github.com/bjaka-max/dispeys/pkg/autostart"
	"github.com/bjaka-max/dispeys/pkg/ulanzid200"
	"github.com/bjaka-max/dispeys/webserver"
)

//go:embed logo.png
var iconData []byte

var webCancel func()
var webPort int

// webui.port lets external launchers (KDE menu entry, scripts) find the
// running UI; the wrapper validates it with /api/health before trusting it.
func writeWebPortFile(port int) {
	path := filepath.Join(filepath.Dir(config.GetSettingsPath()), "webui.port")
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", port)), 0644)
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTooltip("dispeys")

	mSettings := systray.AddMenuItem("Web Settings", "Open web settings UI")
	mSettingsWeb := systray.AddMenuItem("Settings (JSON)", "Edit settings file")

	enabled, _ := autostart.IsEnabled(config.AppName)
	mAutostart := systray.AddMenuItemCheckbox("Autostart", "Execute application on enter", enabled)

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Exit", "Turn application off")

	go func() {
		for {
			select {
			case <-mSettings.ClickedCh:
				// Server is always up (started in onReady); (re)open the
				// browser, and restart the server only if it never came up.
				if webPort == 0 {
					if port, cancel, err := webserver.StartServer(); err == nil {
						webCancel = cancel
						webPort = port
						writeWebPortFile(port)
					} else {
						fmt.Printf("Failed to start web UI: %v\n", err)
						continue
					}
				}
				_ = exec.Command("xdg-open", fmt.Sprintf("http://localhost:%d", webPort)).Start()
			case <-mSettingsWeb.ClickedCh:
				openSettingsWindow()
			case <-mAutostart.ClickedCh:
				if mAutostart.Checked() {
					_ = autostart.Disable(config.AppName)
					mAutostart.Uncheck()
					fmt.Println("Автозапуск отключён")
				} else {
					err := autostart.Enable(config.AppName)
					if err == nil {
						mAutostart.Check()
						fmt.Println("Автозапуск включён")
					} else {
						fmt.Println("Ошибка при включении автозапуска:", err)
					}
				}
			case <-mQuit.ClickedCh:
				// Restart=always: exiting via the tray must stop the unit,
				// otherwise systemd immediately starts a new instance.
				// systemctl's SIGTERM terminates us as part of the stop.
				stop := exec.Command("systemctl", "--user", "stop", "dispeys.service")
				if err := stop.Run(); err != nil {
					fmt.Println("tray quit: systemctl stop failed:", err)
				}
				systray.Quit()
			}
		}
	}()

	// The webserver package reads these package vars when serving
	// /api/settings and /icons/ — without this assignment (lost in the
	// per-instance ServeMux refactor) the UI 404s on empty paths.
	webserver.SettingsPath = config.GetSettingsPath()
	webserver.IconsDir = config.GetIconsDir()

	// Web UI runs for the whole app lifetime: the KDE menu entry and the
	// tray item both just open it in a browser.
	if port, cancel, err := webserver.StartServer(); err == nil {
		webCancel = cancel
		webPort = port
		writeWebPortFile(port)
	} else {
		fmt.Printf("Failed to start web UI: %v\n", err)
	}

	dev := ulanzid200.New(
		ulanzid200.CLOCK,
		config.GetIconsDir(),
		config.GetTempDir(),
	)
	appDetector := appdetector.New(config.GetSettingsPath(), config.GetIconsDir())
	var settings *appdetector.Application
	var settingsNP *appdetector.Application
	var stopProcessDetect bool
	go func() {
		processChangedChan := appDetector.ProcessChangedChan()
		refreshChan := dev.RefreshChan()
		for {
			select {
			case newSettings := <-processChangedChan:
				settings = newSettings
				if !stopProcessDetect {
					setSettings(dev, settings)
				}
			case <-refreshChan:
				if stopProcessDetect && settingsNP != nil {
					setSettings(dev, settingsNP)
				} else if settings != nil {
					setSettings(dev, settings)
				}
			}
		}
	}()
	go func() {
		keyPressedChan := dev.KeyPressedChan()
		for {
			keyPressedEvent := <-keyPressedChan
			var command string
			if stopProcessDetect {
				if settingsNP == nil || len(settingsNP.Buttons) <= keyPressedEvent.Index {
					continue
				}
				command = settingsNP.Buttons[keyPressedEvent.Index].Command
			} else {
				if settings == nil || len(settings.Buttons) <= keyPressedEvent.Index {
					continue
				}
				command = settings.Buttons[keyPressedEvent.Index].Command
			}
			if command != "" {
				if strings.HasPrefix(command, "@") {
					command = strings.TrimSpace(strings.TrimPrefix(command, "@"))
					if command == "" {
						stopProcessDetect = false
						setSettings(dev, settings)
					} else {
						stopProcessDetect = true
						settingsNP = appdetector.GetSettingsForProcess(command)
						setSettings(dev, settingsNP)
					}
				} else if strings.HasPrefix(command, "$") {
					command = strings.TrimSpace(strings.TrimPrefix(command, "$"))
					_ = appdetector.FocusOrRun(command)
					if stopProcessDetect {
						setSettings(dev, settingsNP)
					} else {
						setSettings(dev, settings)
					}
				} else {
					if err := exec.Command("sh", "-c", command).Run(); err != nil {
						fmt.Printf("Command failed: %v\n", err)
					}
					if stopProcessDetect {
						setSettings(dev, settingsNP)
					} else {
						setSettings(dev, settings)
					}
				}
			}
		}
	}()
	appDetector.Start()
	dev.Start()

	// Load and set initial default settings after a short delay
	time.Sleep(500 * time.Millisecond)
	ulanzid200.InitCachedMetrics()
	_, _ = appdetector.LoadAppSettings(config.GetSettingsPath(), config.GetIconsDir())
	initialSettings := appdetector.GetSettingsForProcess("default")
	if initialSettings != nil {
		settings = initialSettings
		setSettings(dev, settings)
	} else {
		fmt.Println("WARNING: No default settings found!")
	}
}

func setSettings(dev *ulanzid200.UlanziD200Device, settings *appdetector.Application) {
	if settings == nil {
		return
	}
	buttons := make(map[int]ulanzid200.Button)
	for i, button := range settings.Buttons {
		buttons[i] = ulanzid200.Button{
			Icon: button.Icon,
		}
	}
	dev.SetButtons(buttons, false)
}

func onExit() {
	fmt.Println("Завершение работы")
}

func openSettingsWindow() {
	editor := config.GetEditorForTextFile()
	if editor != "" {
		cmdArg := strings.ReplaceAll(editor, "%U", config.ShellEscape(config.GetSettingsPath()))
		_ = exec.Command("sh", "-c", cmdArg).Start()
	}
}
