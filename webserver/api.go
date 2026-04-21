package webserver

import (
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	appdetector "github.com/bjaka-max/dispeys/pkg/app_detector"
	xdraw "golang.org/x/image/draw"
	xpng "image/png"
	xwebp "golang.org/x/image/webp"
)

func handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(SettingsPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)

	case http.MethodPost:
		var apps map[string]*appdetector.Application
		if err := json.NewDecoder(r.Body).Decode(&apps); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appdetector.AppSettings.Applications = apps
		if err := appdetector.SaveAppSettings(SettingsPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleIcons(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	files, err := os.ReadDir(IconsDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var icons []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".png") {
			icons = append(icons, f.Name())
		}
	}
	_ = json.NewEncoder(w).Encode(icons)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	file, header, err := r.FormFile("icon")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	name := filepath.Base(header.Filename)
	if !strings.HasSuffix(strings.ToLower(name), ".png") {
		name += ".png"
	}

	// Decode any image format
	decoded, _, err := image.Decode(file)
	if err != nil {
		_, _ = file.Seek(0, 0)
		decoded, err = xwebp.Decode(file)
		if err != nil {
			http.Error(w, "invalid image", http.StatusBadRequest)
			return
		}
	}
	bounds := decoded.Bounds()
	if bounds.Dx() < 64 || bounds.Dy() < 64 {
		http.Error(w, "image too small (min 64x64)", http.StatusBadRequest)
		return
	}

	resized := image.NewRGBA(image.Rect(0, 0, 196, 196))
	xdraw.BiLinear.Scale(resized, resized.Bounds(), decoded, decoded.Bounds(), xdraw.Over, nil)

	targetPath := filepath.Join(IconsDir, name)
	outFile, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = outFile.Close() }()

	if err := xpng.Encode(outFile, resized); err != nil {
		os.Remove(targetPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": name})
}

