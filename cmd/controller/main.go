package main

import (
	_ "embed"
	"fmt"
	"os/exec"
	"strings"

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
			case settings = <-processChangedChan:
			case <-refreshChan:
				if stopProcessDetect {
					setSettings(dev, settingsNP)
				}
			}
			if !stopProcessDetect {
				setSettings(dev, settings)
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
				command = settingsNP.Buttons[keyPressedEvent.Index].Command
			} else {
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
					appdetector.FocusOrRun(command)
				} else {
					_ = exec.Command("sh", "-c", command).Start()
				}
			}
		}
	}()
	appDetector.Start()
	dev.Start()
}
func setSettings(dev *ulanzid200.UlanziD200Device, settings *appdetector.Application) {
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

func openSettingsWindow() {
	editor := config.GetEditorForTextFile()
	if editor!= "" {
		cmdArg := strings.ReplaceAll(editor, "%U", config.ShellEscape(config.GetSettingsPath()))
		_ = exec.Command("sh", "-c", cmdArg).Start()
	}
}
