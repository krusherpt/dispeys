# Lint Fix Plan

19 fixed issues. `golangci-lint run ./...` reports 0 issues.

**All lint issues resolved.**

---

## New lint session — 14 issues fixed

### errcheck (6)
- `webserver/api.go:38` — `w.Write(data)` in handleSettings GET → check return error
- `webserver/api.go:73` — `w.Write([]byte(...))` in handleSettings POST → check return error
- `webserver/api.go:101` — `json.NewEncoder(w).Encode(icons)` → check return error
- `webserver/api.go:122` — `defer file.Close()` → `defer func() { _ = file.Close() }()`
- `webserver/api.go:176` — `json.NewEncoder(w).Encode(map[...])` in upload → check return error
- `webserver/server.go:45` — `w.Write(data)` in static file handler → check return error

### unused (8)
- `webserver/api.go:142` — `draw.Over` → `xdraw.Over` (wrong package prefix)
- `webserver/api.go:184,186,190` — `png.Decode`, `jpeg.Decode`, `gif.Decode` return `(img, error)` not `(img, string, error)` — fixed return signatures
- `webserver/api.go:188` — `webp` import was blank `_` → used import
- `webserver/api.go:199` — `fmt.Errorf` shadowed by `string` var named `fmt` in default case
- `webserver/server.go:6` — unused `net` import removed
- `webserver/server.go:18` — unused `defaultPort` constant removed
- `cmd/controller/main.go:181` — `showOutputOnActiveWindow` unused, removed entirely
- `pkg/app_detector/AppSelect.go` — 6 dead functions removed: `getPIDs`, `windowsForPIDs`, `getActiveWindow`, `toHexWindowID`, `activateWindow`, `indexOf`
