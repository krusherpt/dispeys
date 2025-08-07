package appdetector

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type Settings struct {
	lastModifiedTime   time.Time
	Applications map[string]*Application
}

type Button struct {
	Name string      `json:"name"`
	Icon string      `json:"icon"`
	Command string   `json:"command"`
}

type Application struct {
	Name string      `json:"name"`
	Buttons []Button `json:"buttons"`
}

//go:embed settings_default.json
var defaultSettings []byte

//go:embed icons/*
var iconsFS embed.FS

var AppSettings Settings

func CreateDefaultFiles(path, iconsTargetDir string) (created bool, err error) {
	fmt.Println(iconsTargetDir)
	dir := filepath.Dir(path)
	fmt.Println(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("mkdir error: %w", err)
	}
	if err := os.MkdirAll(iconsTargetDir, 0o755); err != nil {
		return false, fmt.Errorf("mkdir error: %w", err)
	}

	var tmp map[string]Application
	if err := json.Unmarshal(defaultSettings, &tmp); err != nil {
		return false, fmt.Errorf("embedded default JSON is invalid: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, defaultSettings, 0o644); err != nil {
		return false, fmt.Errorf("write temp file error: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return false, fmt.Errorf("rename temp to final error: %w", err)
	}

	if fi, err := os.Stat(path); err == nil {
		AppSettings.lastModifiedTime = fi.ModTime().UTC()
	}

	err = fs.WalkDir(iconsFS, "icons", func(path string, d fs.DirEntry, walkErr error) error {
		fmt.Println(path)
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relName := filepath.Base(path)
		targetPath := filepath.Join(iconsTargetDir, relName)
		fmt.Println(targetPath)

		if _, err := os.Stat(targetPath); err == nil {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stat error для %s: %w", targetPath, err)
		}
		inFile, err := iconsFS.Open(path)
		if err != nil {
			return fmt.Errorf("ошибка открытия встроенного файла %s: %w", path, err)
		}
		defer inFile.Close()

		tmpTarget := targetPath + ".tmp"
		outFile, err := os.OpenFile(tmpTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("ошибка создания временного файла %s: %w", tmpTarget, err)
		}

		if _, err := io.Copy(outFile, inFile); err != nil {
			outFile.Close()
			os.Remove(tmpTarget)
			return fmt.Errorf("ошибка записи файла %s: %w", tmpTarget, err)
		}
		outFile.Close()

		fmt.Println(targetPath)
		if err := os.Rename(tmpTarget, targetPath); err != nil {
			_ = os.Remove(tmpTarget)
			return fmt.Errorf("не удалось переименовать %s -> %s: %w", tmpTarget, targetPath, err)
		}

		return nil
	})

	if err != nil {
		return true, fmt.Errorf("ошибка при разворачивании иконок: %w", err)
	}

	return true, nil
}

func LoadAppSettings(path, iconPath string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			_, err = CreateDefaultFiles(path, iconPath)
			if err != nil {
				return false, fmt.Errorf("не удалось создать файл: %w", err)
			}
			return LoadAppSettings(path, iconPath)
		}
		return false, fmt.Errorf("не удалось stat файла: %w", err)
	}

	modTime := fi.ModTime().UTC()

	if !AppSettings.lastModifiedTime.IsZero() && modTime.Equal(AppSettings.lastModifiedTime) {
		return false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	var apps map[string]*Application
	if err := json.Unmarshal(data, &apps); err != nil {
		return false, fmt.Errorf("ошибка парсинга JSON (ожидается map[string]Application): %w", err)
	}

	AppSettings.Applications = apps
	AppSettings.lastModifiedTime = modTime

	return true, nil
}

func SaveAppSettings(path string) error {
	if AppSettings.Applications == nil {
		AppSettings.Applications = make(map[string]*Application)
	}

	data, err := json.MarshalIndent(AppSettings.Applications, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create dir %s: %w", dir, err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp file error: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file error: %w", err)
	}

	fi, err := os.Stat(path)
	if err == nil {
		AppSettings.lastModifiedTime = fi.ModTime().UTC()
	} else {
		return fmt.Errorf("saved but stat failed: %w", err)
	}

	return nil
}

func GetSettingsForProcess(process string) (result *Application) {
	var ok bool
	if result, ok = AppSettings.Applications[process]; !ok {
		if result, ok = AppSettings.Applications["default"]; !ok {
			result = nil
		}
	}
	return
}