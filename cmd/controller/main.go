package main

import (
	_ "embed"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/systray"

	"github.com/bjaka-max/dispeys/cmd/controller/config"
	appdetector "github.com/bjaka-max/dispeys/pkg/app_detector"
	"github.com/bjaka-max/dispeys/pkg/autostart"
	"github.com/bjaka-max/dispeys/pkg/ulanzid200"
)

//go:embed logo.png
var iconData []byte

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTooltip("dispeys")

	mSettings := systray.AddMenuItem("Settings", "Application settings")

	enabled, _ := autostart.IsEnabled(config.AppName)
	mAutostart := systray.AddMenuItemCheckbox("Autostart", "Execute application on enter", enabled)

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Exit", "Turn application off")

	go func() {
		for {
			select {
			case <-mSettings.ClickedCh:
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
				systray.Quit()
			}
		}
	}()
	
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
			fmt.Printf("keyPressedEvent: %#v\n", keyPressedEvent)
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
				fmt.Printf("command: %#v\n", command)
				if strings.HasPrefix(command, "@") {
					command=strings.TrimSpace(strings.TrimPrefix(command, "@"))
					fmt.Printf("command: %#v\n", command)
					if command == "" {
						stopProcessDetect = false
						setSettings(dev, settings)
					} else {
						stopProcessDetect = true
						settingsNP = appdetector.GetSettingsForProcess(command)
						setSettings(dev, settingsNP)
					}
				} else if strings.HasPrefix(command, "$") {
					command=strings.TrimSpace(strings.TrimPrefix(command, "$"))
					_ = appdetector.FocusOrRun(command)
				} else {
					if err := exec.Command("sh", "-c", command).Run(); err != nil {
						fmt.Printf("Command failed: %v\n", err)
					}
				}
			}
		}
	}()
	appDetector.Start()
	dev.Start()

	// Load and set initial default settings after a short delay
	time.Sleep(500 * time.Millisecond)
	fmt.Println("Loading initial settings...")
	loaded, err := appdetector.LoadAppSettings(config.GetSettingsPath(), config.GetIconsDir())
	fmt.Printf("LoadAppSettings: loaded=%v err=%v\n", loaded, err)
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
		fmt.Println(i, button.Name)
		buttons[i] = ulanzid200.Button{
			Icon: button.Icon,
		}
	}
	dev.SetButtons(buttons, false)
}

func onExit() {
	fmt.Println("Завершение работы")
}

// showOutputOnActiveWindow displays captured output on the active window using xdotool_type
func showOutputOnActiveWindow(output string) error {
	if output == "" {
		return nil
	}

	// Get active window ID
	cmd := exec.Command("xdotool", "getactivewindow")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get active window: %w", err)
	}

	_ = string(out)

	// Type each character with a small delay for readability
	for _, c := range output {
		xdotoolTypeCmd := exec.Command("xdotool", "type", fmt.Sprintf("%c", c))
		if err := xdotoolTypeCmd.Run(); err != nil {
			return fmt.Errorf("failed to type character %q: %w", c, err)
		}
		time.Sleep(50 * time.Millisecond) // Small delay between characters
	}

	// Press Enter at the end to finalize output
	enterCmd := exec.Command("xdotool", "key", "return")
	return enterCmd.Run()
}

func openSettingsWindow() {
	editor := config.GetEditorForTextFile()
	if editor != "" {
		cmdArg := strings.ReplaceAll(editor, "%U", config.ShellEscape(config.GetSettingsPath()))
		_ = exec.Command("sh", "-c", cmdArg).Start()
	}
}
