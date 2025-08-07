package config

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

const AppVersion = "0.0.1"
const AppName = "dispeysController"

func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		panic("Can't get current user.")
	}
	return usr.HomeDir
}

func GetSettingsPath() string {
	return filepath.Join(GetHomeDir(), ".config", AppName, "settings.json")
}

func GetIconsDir() string {
	return filepath.Join(GetHomeDir(), ".config", AppName, "icons")
}

func GetTempDir() string {
	return filepath.Join(os.TempDir(), AppName)
}

func GetEditorForTextFile() (string) {
	desktopOut, err := exec.Command("xdg-mime", "query", "default", "text/plain").Output()
	if err != nil {
		return ""
	}
	desktopFile := strings.TrimSpace(string(desktopOut))

	// 3. Ищем в системных путях Exec=
	paths := []string{
		GetHomeDir() + "/.local/share/applications/" + desktopFile,
		"/usr/local/share/applications/" + desktopFile,
		"/usr/share/applications/" + desktopFile,
	}

	for _, p := range paths {
		text, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(text), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Exec=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "Exec="))
			}
		}
	}

	return ""
}

func ShellEscape(s string) string {
    if s == "" {
        return "''"
    }
    return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}