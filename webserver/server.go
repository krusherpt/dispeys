package webserver

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
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

// StartServer starts the web UI on a random free port.
// It returns the actual port and a cancel function.
func StartServer() (port int, cancel func(), err error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/settings", handleSettings)
	mux.HandleFunc("/api/icons", handleIcons)
	mux.HandleFunc("/api/upload", handleUpload)
	mux.HandleFunc("/api/health", handleHealth)

	// Serve user icons from disk
	mux.HandleFunc("/icons/", serveIcons)

	// Serve embedded static files
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, nil, fmt.Errorf("listen: %w", err)
	}

	port = listener.Addr().(*net.TCPAddr).Port
	fmt.Printf("Web UI starting at http://localhost:%d\n", port)

	go func() {
		if serveErr := http.Serve(listener, mux); serveErr != nil && serveErr != http.ErrServerClosed {
			fmt.Printf("Web server error: %v\n", serveErr)
		}
	}()

	cancel = func() { listener.Close() }
	return port, cancel, nil
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
