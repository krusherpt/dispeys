package webserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
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

	// Decode any raster format; SVG icon packs are rasterized through
	// rsvg-convert/magick because the device only accepts PNG.
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "cannot read upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	decoded, err := decodeIcon(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	bounds := decoded.Bounds()
	if bounds.Dx() < 16 || bounds.Dy() < 16 {
		http.Error(w, fmt.Sprintf("image too small (%dx%d, min 16x16)", bounds.Dx(), bounds.Dy()), http.StatusBadRequest)
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
		_ = os.Remove(targetPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": name})
}

// decodeIcon turns upload bytes into an image, accepting PNG/JPEG/GIF/WebP and
// rasterizing SVG (common in icon packs) with whatever converter is installed.
func decodeIcon(data []byte) (image.Image, error) {
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := xwebp.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if looksLikeSVG(data) {
		img, err := rasterizeSVG(data)
		if err != nil {
			return nil, fmt.Errorf("SVG conversion failed: %v", err)
		}
		return img, nil
	}
	return nil, errors.New("unsupported image format - send PNG, JPEG, WebP or SVG")
}

func looksLikeSVG(data []byte) bool {
	head := data
	if len(head) > 2048 {
		head = head[:2048]
	}
	s := strings.ToLower(string(head))
	return strings.Contains(s, "<svg") || (strings.HasPrefix(s, "<?xml") && strings.Contains(s, "svg"))
}

func rasterizeSVG(data []byte) (image.Image, error) {
	candidates := []struct {
		bin  string
		args []string
	}{
		{"rsvg-convert", []string{"-w", "196", "-h", "196", "--format=png"}},
		{"magick", []string{"svg:-", "-resize", "196x196", "png:-"}},
		{"convert", []string{"svg:-", "-resize", "196x196", "png:-"}},
	}
	var lastErr error
	for _, c := range candidates {
		bin, err := exec.LookPath(c.bin)
		if err != nil {
			lastErr = err
			continue
		}
		cmd := exec.Command(bin, c.args...)
		cmd.Stdin = bytes.NewReader(data)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("%s: %v", c.bin, err)
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(out.Bytes()))
		if err != nil {
			lastErr = fmt.Errorf("%s output: %v", c.bin, err)
			continue
		}
		return img, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no SVG rasterizer found")
	}
	return nil, lastErr
}

