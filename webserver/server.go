package webserver

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed static/*
var staticFS embed.FS

// IconsDir is the user's icon directory (set from main)
var IconsDir string

// SettingsPath is the settings file path (set from main)
var SettingsPath string

// StartServer starts the web UI on the given port.
func StartServer(port int) {
	http.HandleFunc("/api/settings", handleSettings)
	http.HandleFunc("/api/icons", handleIcons)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/health", handleHealth)

	// Serve user icons from disk
	http.HandleFunc("/icons/", serveIcons)

	// Serve embedded static files
	staticSub, _ := fs.Sub(staticFS, "static")
	http.Handle("/", http.FileServer(http.FS(staticSub)))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Web UI starting at http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Web server error: %v\n", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

func serveIcons(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/icons/"):]
	if name == "" || containsDotDot(name) {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(IconsDir, name)
	// Ensure path is within IconsDir
	if !filepath.IsAbs(path) {
		path = filepath.Join(IconsDir, path)
	}
	realIconsDir, _ := filepath.Abs(IconsDir)
	realPath, _ := filepath.Abs(path)
	if len(realPath) < len(realIconsDir) || realPath[:len(realIconsDir)] != realIconsDir {
		http.NotFound(w, r)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(data)
}

func containsDotDot(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return true
		}
	}
	return false
}
