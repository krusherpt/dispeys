package autostart

import (
	"os"
	"os/user"
	"path/filepath"
)

func Enable(appName string) error {
  execPath, _ := os.Executable()
	usr, err := user.Current()
	if err != nil {
		return err
	}

	autostartDir := filepath.Join(usr.HomeDir, ".config", "autostart")
	err = os.MkdirAll(autostartDir, 0755)
	if err != nil {
		return err
	}

	desktopFile := `[Desktop Entry]
Type=Application
Name=` + appName + `
Exec=` + execPath + `
X-GNOME-Autostart-enabled=true
Terminal=false
`

	filePath := filepath.Join(autostartDir, appName+".desktop")
	return os.WriteFile(filePath, []byte(desktopFile), 0644)
}

func Disable(appName string) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}
	desktopPath := filepath.Join(usr.HomeDir, ".config", "autostart", appName+".desktop")
	return os.Remove(desktopPath)
}

func IsEnabled(appName string) (bool, error) {
	usr, err := user.Current()
	if err != nil {
		return false, err
	}
	desktopPath := filepath.Join(usr.HomeDir, ".config", "autostart", appName+".desktop")
	_, err = os.Stat(desktopPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}