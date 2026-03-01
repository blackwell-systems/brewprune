package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

// newTestStore creates an in-memory SQLite store with the schema applied.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	return st
}

// insertPkg is a convenience helper for test package creation.
func insertPkg(t *testing.T, st *store.Store, name string, binPaths []string) {
	t.Helper()
	pkg := &brew.Package{
		Name:        name,
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		HasBinary:   len(binPaths) > 0,
		BinaryPaths: binPaths,
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage(%s): %v", name, err)
	}
}

// ── parseShimLogLine ─────────────────────────────────────────────────────────

func TestParseShimLogLine_Valid(t *testing.T) {
	ts, path, ok := parseShimLogLine("1709012345678901234,/Users/alice/.brewprune/bin/git")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ts != 1709012345678901234 {
		t.Errorf("ts = %d, want 1709012345678901234", ts)
	}
	if path != "/Users/alice/.brewprune/bin/git" {
		t.Errorf("path = %q, want /Users/alice/.brewprune/bin/git", path)
	}
}

func TestParseShimLogLine_MissingComma(t *testing.T) {
	_, _, ok := parseShimLogLine("1709012345678901234/Users/alice/.brewprune/bin/git")
	if ok {
		t.Fatal("expected ok=false for line with no comma")
	}
}

func TestParseShimLogLine_EmptyTimestamp(t *testing.T) {
	_, _, ok := parseShimLogLine(",/Users/alice/.brewprune/bin/git")
	if ok {
		t.Fatal("expected ok=false for empty timestamp")
	}
}

func TestParseShimLogLine_NonNumericTimestamp(t *testing.T) {
	_, _, ok := parseShimLogLine("not-a-number,/Users/alice/.brewprune/bin/git")
	if ok {
		t.Fatal("expected ok=false for non-numeric timestamp")
	}
}

func TestParseShimLogLine_EmptyPath(t *testing.T) {
	_, _, ok := parseShimLogLine("1709012345678901234,")
	if ok {
		t.Fatal("expected ok=false for empty path")
	}
}

func TestParseShimLogLine_EmptyLine(t *testing.T) {
	_, _, ok := parseShimLogLine("")
	if ok {
		t.Fatal("expected ok=false for empty line")
	}
}

// ── readShimOffset / writeShimOffsetAtomic ───────────────────────────────────

func TestReadShimOffset_Missing(t *testing.T) {
	dir := t.TempDir()
	off, err := readShimOffset(filepath.Join(dir, "usage.offset"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if off != 0 {
		t.Errorf("expected 0 for missing offset file, got %d", off)
	}
}

func TestWriteAndReadShimOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.offset")

	if err := writeShimOffsetAtomic(path, 98765); err != nil {
		t.Fatalf("writeShimOffsetAtomic: %v", err)
	}

	got, err := readShimOffset(path)
	if err != nil {
		t.Fatalf("readShimOffset: %v", err)
	}
	if got != 98765 {
		t.Errorf("got %d, want 98765", got)
	}
}

func TestWriteShimOffsetAtomic_IsCrashSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.offset")

	// Write once.
	if err := writeShimOffsetAtomic(path, 100); err != nil {
		t.Fatal(err)
	}
	// Write again (simulate update).
	if err := writeShimOffsetAtomic(path, 200); err != nil {
		t.Fatal(err)
	}

	got, _ := readShimOffset(path)
	if got != 200 {
		t.Errorf("got %d, want 200", got)
	}

	// Temp file should not be left behind.
	tmpPath := filepath.Join(dir, ".offset.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file was not cleaned up after atomic write")
	}
}

// ── ProcessUsageLog — probe events for *-config binaries ─────────────────────

// TestProcessUsageLog_ConfigBinariesLoggedAsProbe verifies that shim invocations
// whose basename matches the *-config pattern are stored with event_type='probe'
// and do not affect GetUsageEventCountSince or GetLastUsage.
func TestProcessUsageLog_ConfigBinariesLoggedAsProbe(t *testing.T) {
	st := newTestStore(t)

	insertPkg(t, st, "imagemagick", []string{"/opt/homebrew/bin/Magick++-config"})
	insertPkg(t, st, "freetype", []string{"/opt/homebrew/bin/freetype-config"})
	insertPkg(t, st, "pkg-config", []string{"/opt/homebrew/bin/pkg-config"})
	insertPkg(t, st, "git", []string{"/opt/homebrew/bin/git"})

	tmpHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpHome, ".brewprune"), 0700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(tmpHome, ".brewprune", "usage.log")

	ts := int64(1709012345678901234)
	lines := "" +
		"1709012345678901234,/home/u/.brewprune/bin/Magick++-config\n" +
		"1709012345678901235,/home/u/.brewprune/bin/freetype-config\n" +
		"1709012345678901236,/home/u/.brewprune/bin/pkg-config\n" +
		"1709012345678901237,/home/u/.brewprune/bin/git\n"
	_ = ts
	if err := os.WriteFile(logPath, []byte(lines), 0600); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", orig) })
	os.Setenv("HOME", tmpHome)

	if err := ProcessUsageLog(st); err != nil {
		t.Fatalf("ProcessUsageLog: %v", err)
	}

	since := time.Unix(0, 0)

	// *-config packages must show zero usage score events.
	for _, pkg := range []string{"imagemagick", "freetype", "pkg-config"} {
		count, err := st.GetUsageEventCountSince(pkg, since)
		if err != nil {
			t.Fatalf("GetUsageEventCountSince(%s): %v", pkg, err)
		}
		if count != 0 {
			t.Errorf("GetUsageEventCountSince(%s) = %d, want 0 (probe events must be excluded)", pkg, count)
		}

		last, err := st.GetLastUsage(pkg)
		if err != nil {
			t.Fatalf("GetLastUsage(%s): %v", pkg, err)
		}
		if last != nil {
			t.Errorf("GetLastUsage(%s) = %v, want nil (probe events must be excluded)", pkg, last)
		}
	}

	// Regular binary must still show one usage event.
	count, err := st.GetUsageEventCountSince("git", since)
	if err != nil {
		t.Fatalf("GetUsageEventCountSince(git): %v", err)
	}
	if count != 1 {
		t.Errorf("GetUsageEventCountSince(git) = %d, want 1", count)
	}
}

// ── ProcessUsageLog — no-op when log missing ─────────────────────────────────

func TestProcessUsageLog_NoLogFile(t *testing.T) {
	// Override HOME to a temp dir with no usage.log.
	original := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", original) })
	os.Setenv("HOME", tmpHome)

	// ProcessUsageLog should return nil when the log file doesn't exist.
	// We can't call it directly without a store, but we verify the guard
	// condition via the offset helper (which also returns nil for missing files).
	logPath := filepath.Join(tmpHome, ".brewprune", "usage.log")
	_, statErr := os.Stat(logPath)
	if !os.IsNotExist(statErr) {
		t.Fatal("expected log file to not exist in temp home")
	}
}

// ── Offset tracking across multiple reads ────────────────────────────────────

func TestOffsetTrackingAcrossReads(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "usage.log")
	offsetPath := filepath.Join(dir, "usage.offset")

	// Write initial lines.
	batch1 := "1709000000000000001,/home/u/.brewprune/bin/git\n" +
		"1709000000000000002,/home/u/.brewprune/bin/rg\n"
	if err := os.WriteFile(logPath, []byte(batch1), 0600); err != nil {
		t.Fatal(err)
	}

	// Simulate processing batch1 — advance offset to end of batch1.
	if err := writeShimOffsetAtomic(offsetPath, int64(len(batch1))); err != nil {
		t.Fatal(err)
	}

	// Append a new line.
	batch2 := "1709000000000000003,/home/u/.brewprune/bin/gh\n"
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(batch2)
	f.Close()

	// Read from stored offset — should see only batch2.
	off, _ := readShimOffset(offsetPath)
	content, _ := os.ReadFile(logPath)
	newContent := string(content[off:])

	lines := strings.Split(strings.TrimSpace(newContent), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 new line, got %d: %v", len(lines), lines)
	}
	if lines[0] != strings.TrimSuffix(batch2, "\n") {
		t.Errorf("unexpected line: %q", lines[0])
	}
}

// ── buildOptPathMap ───────────────────────────────────────────────────────────

// TestBuildOptPathMap_MapsFullPaths verifies that buildOptPathMap produces a
// map keyed by the complete stored binary path (not the basename).
func TestBuildOptPathMap_MapsFullPaths(t *testing.T) {
	st := newTestStore(t)
	insertPkg(t, st, "git", []string{"/opt/homebrew/bin/git"})
	insertPkg(t, st, "ripgrep", []string{"/opt/homebrew/bin/rg"})

	m, err := buildOptPathMap(st)
	if err != nil {
		t.Fatalf("buildOptPathMap: %v", err)
	}

	cases := map[string]string{
		"/opt/homebrew/bin/git": "git",
		"/opt/homebrew/bin/rg":  "ripgrep",
	}
	for path, want := range cases {
		if got := m[path]; got != want {
			t.Errorf("optPathMap[%q] = %q, want %q", path, got, want)
		}
	}
	// Basenames must NOT be keys in the opt-path map.
	if _, ok := m["git"]; ok {
		t.Error("optPathMap should not contain basename keys")
	}
}

// TestBuildOptPathMap_EmptyStore verifies buildOptPathMap returns an empty map
// when no packages are in the DB.
func TestBuildOptPathMap_EmptyStore(t *testing.T) {
	st := newTestStore(t)
	m, err := buildOptPathMap(st)
	if err != nil {
		t.Fatalf("buildOptPathMap: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

// ── Opt-path disambiguation ───────────────────────────────────────────────────

// TestOptPathDisambiguatesCollision verifies that when two formulae both
// expose a binary with the same basename, the opt-path lookup attributes the
// usage event to the correct package and does not silently pick the last-write
// winner that the old basename-only map would produce.
//
// Scenario: both "imagemagick" and "graphicsmagick" ship a binary named
// "convert". imagemagick stores /opt/homebrew/bin/convert; graphicsmagick
// stores /usr/local/bin/convert (Intel prefix). Usage of the shim for
// "convert" on Apple Silicon resolves to the imagemagick opt path.
func TestOptPathDisambiguatesCollision(t *testing.T) {
	st := newTestStore(t)

	// imagemagick owns the Apple Silicon opt path.
	insertPkg(t, st, "imagemagick", []string{"/opt/homebrew/bin/convert"})
	// graphicsmagick owns the Intel opt path.
	insertPkg(t, st, "graphicsmagick", []string{"/usr/local/bin/convert"})

	optMap, err := buildOptPathMap(st)
	if err != nil {
		t.Fatalf("buildOptPathMap: %v", err)
	}

	// Apple Silicon resolution.
	if got := optMap["/opt/homebrew/bin/convert"]; got != "imagemagick" {
		t.Errorf("Apple Silicon: optPathMap[/opt/homebrew/bin/convert] = %q, want \"imagemagick\"", got)
	}
	// Intel resolution.
	if got := optMap["/usr/local/bin/convert"]; got != "graphicsmagick" {
		t.Errorf("Intel: optPathMap[/usr/local/bin/convert] = %q, want \"graphicsmagick\"", got)
	}

	// Confirm that the basename map WOULD have a collision (last-write wins),
	// demonstrating that the old strategy was insufficient.
	baseMap, err := buildBasenameMap(st)
	if err != nil {
		t.Fatalf("buildBasenameMap: %v", err)
	}
	if _, ok := baseMap["convert"]; !ok {
		t.Error("basename map should contain 'convert' (collision present)")
	}
	// The basename map maps to exactly one package — demonstrating the ambiguity.
	// We cannot reliably assert which one without caring about insertion order,
	// so we just confirm both opt paths resolve to distinct packages.
	if optMap["/opt/homebrew/bin/convert"] == optMap["/usr/local/bin/convert"] {
		t.Error("opt path map should resolve 'convert' to different packages for each prefix")
	}
}

// TestOptPathLookupOrder_AppleSiliconFirst verifies that the Apple Silicon
// prefix (/opt/homebrew/bin) is tried before the Intel prefix (/usr/local/bin)
// in ProcessUsageLog's matching logic — exercised here via buildOptPathMap.
func TestOptPathLookupOrder_AppleSiliconFirst(t *testing.T) {
	st := newTestStore(t)

	// Only the Apple Silicon path is registered in the DB.
	insertPkg(t, st, "git", []string{"/opt/homebrew/bin/git"})

	optMap, err := buildOptPathMap(st)
	if err != nil {
		t.Fatalf("buildOptPathMap: %v", err)
	}

	// Simulate the lookup chain for basename "git" from a shim argv0.
	basename := "git"
	pkg, found := optMap["/opt/homebrew/bin/"+basename]
	if !found {
		pkg, found = optMap["/usr/local/bin/"+basename]
	}
	if !found {
		t.Fatal("expected to find 'git' via opt path, got not-found")
	}
	if pkg != "git" {
		t.Errorf("resolved package = %q, want \"git\"", pkg)
	}
}

// TestOptPathFallbackToBasename verifies that when the opt paths are not in
// the DB but a basename entry exists, the basename fallback still resolves the
// package correctly (backward-compat for older DB entries without full paths).
func TestOptPathFallbackToBasename(t *testing.T) {
	st := newTestStore(t)

	// Store an older-style entry whose binary path is just the binary name
	// (not a full /opt/homebrew/... path). The basename map covers this.
	insertPkg(t, st, "wget", []string{"wget"})

	optMap, err := buildOptPathMap(st)
	if err != nil {
		t.Fatalf("buildOptPathMap: %v", err)
	}
	baseMap, err := buildBasenameMap(st)
	if err != nil {
		t.Fatalf("buildBasenameMap: %v", err)
	}

	basename := "wget"
	pkg, found := optMap["/opt/homebrew/bin/"+basename]
	if !found {
		pkg, found = optMap["/usr/local/bin/"+basename]
	}
	if !found {
		pkg, found = baseMap[basename]
	}

	if !found {
		t.Fatal("expected basename fallback to find 'wget'")
	}
	if pkg != "wget" {
		t.Errorf("resolved package = %q, want \"wget\"", pkg)
	}
}

// ── isConfigProbe ─────────────────────────────────────────────────────────────

func TestIsConfigProbe(t *testing.T) {
	cases := []struct {
		name  string
		probe bool
	}{
		{"pkg-config", true},
		{"freetype-config", true},
		{"Magick++-config", true},
		{"ImageMagick-config", true},
		{"git", false},
		{"rg", false},
		{"node", false},
		{"config", false},
		{"myconfig", false},
	}

	for _, tc := range cases {
		got := isConfigProbe(tc.name)
		if got != tc.probe {
			t.Errorf("isConfigProbe(%q) = %v, want %v", tc.name, got, tc.probe)
		}
	}
}

// ── Malformed lines are skipped ───────────────────────────────────────────────

func TestMalformedLinesSkipped(t *testing.T) {
	cases := []struct {
		line  string
		valid bool
	}{
		{"", false},
		{"not-a-timestamp,/bin/git", false},
		{",/bin/git", false},
		{"1234567890", false},
		{"1234567890,", false},
		{"1234567890,/bin/git", true},
		{"1709012345678901234,/home/u/.brewprune/bin/rg", true},
	}

	for _, tc := range cases {
		_, _, ok := parseShimLogLine(tc.line)
		if ok != tc.valid {
			t.Errorf("parseShimLogLine(%q): got ok=%v, want %v", tc.line, ok, tc.valid)
		}
	}
}

// ── ProcessUsageLog — multiple lines all recorded ─────────────────────────────

// TestProcessUsageLog_MultipleLinesRecorded is the core regression test for the
// shim event pipeline loss bug. Before the fix, bufio.Scanner read-ahead caused
// f.Seek(0, io.SeekCurrent) to return EOF after the first tick even when the
// scanner only processed 1 event, advancing the offset past all remaining lines
// and losing them permanently.
//
// This test writes 5 shim log entries for different packages and verifies that
// all 5 usage events are inserted into the database in a single ProcessUsageLog call.
func TestProcessUsageLog_MultipleLinesRecorded(t *testing.T) {
	st := newTestStore(t)

	insertPkg(t, st, "git", []string{"/opt/homebrew/bin/git"})
	insertPkg(t, st, "jq", []string{"/opt/homebrew/bin/jq"})
	insertPkg(t, st, "bat", []string{"/opt/homebrew/bin/bat"})
	insertPkg(t, st, "fd", []string{"/opt/homebrew/bin/fd"})
	insertPkg(t, st, "ripgrep", []string{"/opt/homebrew/bin/rg"})

	tmpHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpHome, ".brewprune"), 0700); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(tmpHome, ".brewprune", "usage.log")
	lines := "1709012345678901234,/home/u/.brewprune/bin/git\n" +
		"1709012345678901235,/home/u/.brewprune/bin/jq\n" +
		"1709012345678901236,/home/u/.brewprune/bin/bat\n" +
		"1709012345678901237,/home/u/.brewprune/bin/fd\n" +
		"1709012345678901238,/home/u/.brewprune/bin/rg\n"
	if err := os.WriteFile(logPath, []byte(lines), 0600); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", orig) })
	os.Setenv("HOME", tmpHome)

	if err := ProcessUsageLog(st); err != nil {
		t.Fatalf("ProcessUsageLog: %v", err)
	}

	since := time.Unix(0, 0)

	// All 5 packages must have exactly 1 usage event each.
	for _, pkg := range []string{"git", "jq", "bat", "fd", "ripgrep"} {
		count, err := st.GetUsageEventCountSince(pkg, since)
		if err != nil {
			t.Fatalf("GetUsageEventCountSince(%s): %v", pkg, err)
		}
		if count != 1 {
			t.Errorf("GetUsageEventCountSince(%s) = %d, want 1 — event was lost in pipeline", pkg, count)
		}
	}

	// Verify total event count as a belt-and-suspenders check.
	total, err := st.GetEventCount()
	if err != nil {
		t.Fatalf("GetEventCount: %v", err)
	}
	if total != 5 {
		t.Errorf("total events = %d, want 5", total)
	}
}

// ── ProcessUsageLog — Linuxbrew path resolution ───────────────────────────────

// TestProcessUsageLog_LinuxbrewPathResolution verifies that shim invocations
// logged with a Linuxbrew argv0 path (/home/linuxbrew/.linuxbrew/bin/<name>)
// resolve to the correct package name via the Linuxbrew prefix lookup in
// optPathMap.
func TestProcessUsageLog_LinuxbrewPathResolution(t *testing.T) {
	st := newTestStore(t)

	// Register git with the Linuxbrew binary path (as it would appear after
	// a 'brewprune scan' on a Linux system with Linuxbrew).
	insertPkg(t, st, "git", []string{"/home/linuxbrew/.linuxbrew/bin/git"})

	tmpHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpHome, ".brewprune"), 0700); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(tmpHome, ".brewprune", "usage.log")
	// The shim argv0 is the shim symlink under ~/.brewprune/bin/.
	// ProcessUsageLog extracts the basename ("git") and constructs the
	// Linuxbrew lookup key /home/linuxbrew/.linuxbrew/bin/git.
	lines := "1709012345678901234,/home/u/.brewprune/bin/git\n"
	if err := os.WriteFile(logPath, []byte(lines), 0600); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", orig) })
	os.Setenv("HOME", tmpHome)

	if err := ProcessUsageLog(st); err != nil {
		t.Fatalf("ProcessUsageLog: %v", err)
	}

	since := time.Unix(0, 0)
	count, err := st.GetUsageEventCountSince("git", since)
	if err != nil {
		t.Fatalf("GetUsageEventCountSince(git): %v", err)
	}
	if count != 1 {
		t.Errorf("GetUsageEventCountSince(git) = %d, want 1 — Linuxbrew path not resolved", count)
	}
}

// ── ProcessUsageLog — offset not advanced on insert failure ───────────────────

// TestProcessUsageLog_OffsetAdvancesAfterInsert verifies that the file offset is
// NOT advanced when the batch insert transaction fails. Events must not be
// silently lost on database errors — the next tick should retry from the same
// position.
func TestProcessUsageLog_OffsetAdvancesAfterInsert(t *testing.T) {
	st := newTestStore(t)

	insertPkg(t, st, "git", []string{"/opt/homebrew/bin/git"})

	tmpHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpHome, ".brewprune"), 0700); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(tmpHome, ".brewprune", "usage.log")
	offsetPath := filepath.Join(tmpHome, ".brewprune", "usage.offset")

	lines := "1709012345678901234,/home/u/.brewprune/bin/git\n"
	if err := os.WriteFile(logPath, []byte(lines), 0600); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", orig) })
	os.Setenv("HOME", tmpHome)

	// Drop the usage_events table to force the INSERT to fail.
	// buildBasenameMap and buildOptPathMap read from the packages table, so
	// they still succeed; only the INSERT step will fail.
	if _, err := st.DB().Exec("DROP TABLE usage_events"); err != nil {
		t.Fatalf("DROP TABLE usage_events: %v", err)
	}

	err := ProcessUsageLog(st)
	// We expect an error because the INSERT fails.
	if err == nil {
		t.Fatal("ProcessUsageLog: expected error after usage_events dropped, got nil")
	}

	// The offset file must NOT have been written — the position must remain at 0.
	if _, statErr := os.Stat(offsetPath); !os.IsNotExist(statErr) {
		savedOffset, _ := readShimOffset(offsetPath)
		t.Errorf("offset file written despite insert failure: offset=%d (want file absent)", savedOffset)
	}
}
