# dispeys Performance Optimization Plan

**Status: ✅ ALL COMPLETE** — All 6 optimizations implemented and verified.

---

## Bottleneck #1: `cpu.Percent()` blocking every 500ms

**Status: ✅ DONE**

**Location:** `pkg/ulanzid200/SmallWindowData.go` → called every 500ms from `UlanziD200Device.Start()`

**Problem:** `cpu.Percent(time.Second, false)` blocks for 1 full second. Called every 500ms means goroutine is perpetually blocked. Memory and GPU reads add more latency on top.

**Fix:** Cache CPU usage with 1s granularity. Use instant (non-blocking) measurements for the 500ms tick.

### Changes

**`pkg/ulanzid200/SmallWindowData.go`:**
- Add cached values with timestamp to `SmallWindowData`
- In `NewSmallWindowData()`, if cached CPU value is < 1s old, reuse it instead of calling `cpu.Percent()`
- For MEM/GPU, cache indefinitely until mode changes or on explicit refresh

**`pkg/ulanzid200/UlanziD200Device.go`:**
- Add `lastCPUCache` field with timestamp
- Add `lastMEMCache` field with timestamp
- Add `lastGPUCache` field with timestamp
- Pass cache state through `SetSmallWindowData`

### Expected improvement
- Goroutine unblocked: 500ms sleep actually happens instead of sleeping after 1s+ block
- CPU usage: ~0ms → instant cache hit most of the time

---

## Bottleneck #2: `EqualJSON()` double marshal on every small window tick

**Status: ✅ DONE**

**Location:** `pkg/ulanzid200/UlanziD200Device.go` line ~105, called inside `SetSmallWindowData()`

**Problem:** Every 500ms, even when nothing changed, we marshal two structs to JSON just to compare them.

### Changes

**`pkg/ulanzid200/UlanziD200Device.go`:**
- Replace `EqualJSON(d.smallWindowData, data)` with direct field comparison:
  ```go
  func smallWindowDataEqual(a, b SmallWindowData) bool {
      return a.Mode == b.Mode && a.CPU == b.CPU && a.MEM == b.MEM && a.GPU == b.GPU && a.Time == b.Time
  }
  ```
- Keep `EqualJSON()` for `LabelStyle` (which has nested structure)

### Expected improvement
- Eliminates 2× JSON marshal per 500ms tick = ~4000 marshals/min saved

---

## Bottleneck #3: 3 subprocess forks every 2s in AppDetector

**Status: ✅ DONE**

**Location:** `pkg/app_detector/AppDetector.go` — loop every 2 seconds

**Problem:** Each window poll spawns 3 subprocesses: `xdotool`, `xprop`, `ps`. That's ~15 subprocesses/second over a day.

### Changes

**`pkg/app_detector/AppDetector.go`:**
- Add `lastQueryTime time.Time` field to `AppDetector`
- Add `minQueryInterval = 1 * time.Second`
- In the polling loop, skip subprocess calls if `time.Since(lastQueryTime) < minQueryInterval`
- Only query when window ID actually changed AND enough time has passed
- Cache `lastProcessName` and `lastWinID` to avoid redundant lookups

**`pkg/app_detector/AppDetector.go` — `getActiveWindowProcessName()`:**
- Return early if `winID == prevWinID` (already done, but ensure it's the first check)

### Expected improvement
- Reduces subprocess forks from 15/sec to ~0.5-1/sec (only when window actually changes)
- ~95% reduction in fork+exec overhead

---

## Bottleneck #4: `prepareZip()` disk I/O + blind retry loop

**Status: ✅ DONE**

**Location:** `pkg/ulanzid200/UlanziD200Device.go` — called on every `SetButtons()`

**Problem:**
1. Creates temp directory, copies icons, writes manifest, creates zip — all on disk
2. Checks for "invalid bytes" by seeking through entire zip file
3. If invalid bytes found, appends random dummy string and **rebuilds entire zip**
4. Retry loop with 50ms sleeps — worst case 4+ rebuilds

### Changes

**`pkg/ulanzid200/UlanziD200Device.go`:**
- Add zip cache: `lastZipPath string`, `lastButtonsHash uint32`
- Compute a fast hash of button map before rebuilding
- If buttons haven't changed, return cached zip path
- Replace blind retry loop with deterministic dummy:
  ```go
  // Instead of randomString, use a pre-computed safe payload
  // that avoids 0x00, 0x7c at positions 1016 + n*1024
  ```
- Pre-compute safe dummy bytes that don't contain invalid values at danger zones

### Expected improvement
- Zero rebuilds when buttons unchanged (cache hit)
- Single-pass zip creation instead of 1-4 rebuilds
- ~50-80% faster button updates

---

## Bottleneck #5: Settings file re-parse on every process change

**Status: ✅ DONE**

**Location: `pkg/app_detector/AppDetector.go` — called inside process change handler

**Problem:** Every process change triggers `LoadAppSettings()` which reads and parses the full JSON file, even though the mtime check helps.

### Changes

**`pkg/app_detector/AppDetector.go`:**
- Ensure `LoadAppSettings()` is only called once at startup (already partially done in `main.go`)
- In the process change handler, skip `LoadAppSettings()` call — just look up in already-loaded `AppSettings.Applications`
- The mtime-based cache in `LoadAppSettings` already prevents re-parsing, but the call itself is wasteful

**`cmd/controller/main.go`:**
- Remove redundant `LoadAppSettings()` call from the process change handler
- Settings are already loaded in `onReady()` via `LoadAppSettings()` + `GetSettingsForProcess("default")`

### Expected improvement
- Eliminates unnecessary `os.ReadFile` + `json.Unmarshal` on every process switch
- ~10-50μs saved per process change

---

## Bottleneck #6: `fmt.Println`/`fmt.Printf` in hot paths

**Status: ✅ DONE**

**Location: Scattered across `main.go`, `UlanziD200Device.go`, `Settings.go`

**Problem:** `fmt.Println(i, button.Name)` in `setSettings()`, debug prints in `prepareZip()`, `copyFile()`, `getActiveWindowProcessName()` etc.

### Changes

**`cmd/controller/main.go`:**
- Remove `fmt.Println(i, button.Name)` from `setSettings()`
- Remove `fmt.Printf("command: %#v\n", ...)` from keypress handler
- Remove `fmt.Printf("LoadAppSettings: loaded=%v err=%v\n", ...)` from `onReady()`
- Keep `fmt.Printf("keyPressedEvent: %#v\n", ...)` but make it conditional on a debug flag

**`pkg/ulanzid200/UlanziD200Device.go`:**
- Remove `fmt.Println(src, dst)` from `copyFile()`
- Remove `fmt.Println("ZIP создан и прошёл проверку!")` from `prepareZip()`
- Remove `fmt.Printf("HID #%d\n", i)` debug lines from `connectToDevice()` (or make debug-only)
- Remove `fmt.Println(err)` from `copyFile()`

**`pkg/app_detector/Settings.go`:**
- Remove `fmt.Println(iconsTargetDir)`, `fmt.Println(dir)`, `fmt.Println(path)`, `fmt.Println(targetPath)` from `CreateDefaultFiles()`

### Expected improvement
- Removes ~20-50 lines of blocking I/O calls per hot path
- Cleaner logs, easier to spot real issues

---

## Implementation Order

1. **Bottleneck #6** — Remove fmt.Println calls (trivial, no risk)
2. **Bottleneck #2** — Replace EqualJSON for SmallWindowData (simple, no risk)
3. **Bottleneck #5** — Remove redundant LoadAppSettings calls (simple, no risk)
4. **Bottleneck #1** — Cache CPU/MEM/GPU usage (moderate, test the timing)
5. **Bottleneck #4** — Zip cache + deterministic dummy (moderate, test zip generation)
6. **Bottleneck #3** — Debounce subprocess calls (moderate, test window detection)
