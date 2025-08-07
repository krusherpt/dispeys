package appdetector

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type AppDetector struct {
	settingsFilePath   string
	iconsDirPath       string
	processChangedChan chan *Application
	stopped					   bool
}

func New(
	SettingsFilePath, IconsDirPath string,
) AppDetector {
	return AppDetector{
		settingsFilePath: SettingsFilePath,
		iconsDirPath: IconsDirPath,
		processChangedChan: make(chan *Application),
	}
}

var processName string
var winID string

func (a *AppDetector) ProcessChangedChan() chan *Application {
	return a.processChangedChan
}

func (a *AppDetector) Start() {
	go func() {
		for {
			currentProcessName, currentWinID, err := getActiveWindowProcessName(winID)
			if err == nil {
				if currentProcessName != "" && (processName == "" || processName != currentProcessName) {
					processName = currentProcessName
					winID = currentWinID
					_, err = LoadAppSettings(a.settingsFilePath, a.iconsDirPath)
					if err != nil {
						fmt.Println(err)
					}
					fmt.Printf("%s, %#v\n", processName, AppSettings)
					settings := GetSettingsForProcess(processName)
					if settings !=nil {
						a.processChangedChan <- settings

						select {
						case a.processChangedChan <- settings:
							fmt.Println("process changed to ", processName, " done")
						default:
							fmt.Println("process changed without receiver")
						}
					}
				}
			} else {
				fmt.Println(err)
			}
			time.Sleep(2 * time.Second)
				
			if a.stopped {
				break
			}
		}
	}()
}

func (a *AppDetector) Stop() {
	a.stopped = true
}

func getActiveWindowProcessName(prevWinID string) (processName string, winID string, err error) {
	// Получаем ID активного окна
	winIDRaw, err := exec.Command("xdotool", "getactivewindow").Output()
	if err != nil {
		err = fmt.Errorf("не удалось получить активное окно: %w", err)
		return
	}
	winID = strings.TrimSpace(string(winIDRaw))
	if winID == prevWinID {
		return
	}

	// Получаем PID по окну
	cmd := exec.Command("xprop", "-id", winID, "_NET_WM_PID")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("xprop не смог получить PID: %w", err)
		return
	}

	// Парсим PID
	line := out.String()
	parts := strings.Split(line, " = ")
	if len(parts) != 2 {
		err = fmt.Errorf("неожиданный формат xprop: %s", line)
		return
	}
	pid := strings.TrimSpace(parts[1])

	// Получаем команду по PID
	cmdlineRaw, err := exec.Command("ps", "-p", pid, "-o", "comm=").Output()
	if err != nil {
		err = fmt.Errorf("не удалось получить имя процесса: %w", err)
		return
	}
	processName = strings.TrimSpace(string(cmdlineRaw))

	return 
}