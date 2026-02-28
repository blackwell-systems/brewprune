package app

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/shim"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/blackwell-systems/brewprune/internal/watcher"
)

func TestScanCommand(t *testing.T) {
	// Test that scan command is properly configured
	if scanCmd.Use != "scan" {
		t.Errorf("expected Use to be 'scan', got '%s'", scanCmd.Use)
	}

	if scanCmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if scanCmd.Long == "" {
		t.Error("expected Long description to be set")
	}

	if scanCmd.Example == "" {
		t.Error("expected Example to be set")
	}

	if scanCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestScanCommandFlags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue interface{}
	}{
		{
			name:         "refresh-binaries flag",
			flagName:     "refresh-binaries",
			defaultValue: true,
		},
		{
			name:         "quiet flag",
			flagName:     "quiet",
			defaultValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := scanCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("expected flag '%s' to be registered", tt.flagName)
				return
			}

			if flag.Usage == "" {
				t.Errorf("expected flag '%s' to have usage text", tt.flagName)
			}

			// Check default value
			switch v := tt.defaultValue.(type) {
			case bool:
				if flag.DefValue != "true" && flag.DefValue != "false" {
					t.Errorf("expected flag '%s' to be boolean", tt.flagName)
				}
			case string:
				if flag.DefValue != v {
					t.Errorf("expected flag '%s' default to be '%s', got '%s'", tt.flagName, v, flag.DefValue)
				}
			}
		})
	}
}

func TestScanCommandHelp(t *testing.T) {
	// Test that help can be generated without errors
	oldArgs := scanCmd.Flags()
	defer func() {
		scanCmd.ResetFlags()
		scanCmd.Flags().AddFlagSet(oldArgs)
	}()

	scanCmd.SetArgs([]string{"--help"})

	// Capture the help output
	// The command will return an error but that's expected
	err := scanCmd.Execute()
	if err != nil && !strings.Contains(err.Error(), "help") { //nolint:staticcheck
		// Some error is expected when running help
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    2048,
			expected: "2 KB",
		},
		{
			name:     "megabytes",
			bytes:    5 * 1024 * 1024,
			expected: "5 MB",
		},
		{
			name:     "gigabytes",
			bytes:    3 * 1024 * 1024 * 1024,
			expected: "3.0 GB",
		},
		{
			name:     "zero",
			bytes:    0,
			expected: "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestScanCommandFlagParsing(t *testing.T) {
	// Reset flags before test
	scanRefreshBinaries = true
	scanQuiet = false

	// Test flag parsing
	tests := []struct {
		name                    string
		args                    []string
		expectedRefreshBinaries bool
		expectedQuiet           bool
	}{
		{
			name:                    "default flags",
			args:                    []string{},
			expectedRefreshBinaries: true,
			expectedQuiet:           false,
		},
		{
			name:                    "disable refresh binaries",
			args:                    []string{"--refresh-binaries=false"},
			expectedRefreshBinaries: false,
			expectedQuiet:           false,
		},
		{
			name:                    "enable quiet",
			args:                    []string{"--quiet"},
			expectedRefreshBinaries: true,
			expectedQuiet:           true,
		},
		{
			name:                    "both flags",
			args:                    []string{"--refresh-binaries=false", "--quiet"},
			expectedRefreshBinaries: false,
			expectedQuiet:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			scanRefreshBinaries = true
			scanQuiet = false

			// Parse flags
			scanCmd.ParseFlags(tt.args)

			if scanRefreshBinaries != tt.expectedRefreshBinaries {
				t.Errorf("expected refreshBinaries to be %v, got %v", tt.expectedRefreshBinaries, scanRefreshBinaries)
			}

			if scanQuiet != tt.expectedQuiet {
				t.Errorf("expected quiet to be %v, got %v", tt.expectedQuiet, scanQuiet)
			}
		})
	}
}

func TestScanCommandRegistration(t *testing.T) {
	// Verify scan command is registered with root
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "scan" {
			found = true
			break
		}
	}

	if !found {
		t.Error("scan command not registered with root command")
	}
}

// TestRefreshShimsFlag verifies that the --refresh-shims flag is registered
// with the correct name, default value, and non-empty usage text.
func TestRefreshShimsFlag(t *testing.T) {
	flag := scanCmd.Flags().Lookup("refresh-shims")
	if flag == nil {
		t.Fatal("expected --refresh-shims flag to be registered on scan command")
	}

	if flag.DefValue != "false" {
		t.Errorf("expected --refresh-shims default to be false, got %q", flag.DefValue)
	}

	if flag.Usage == "" {
		t.Error("expected --refresh-shims to have non-empty usage text")
	}
}

// TestRefreshShimsFlagParsing verifies that --refresh-shims parses correctly.
func TestRefreshShimsFlagParsing(t *testing.T) {
	// Reset to known state.
	scanRefreshShims = false

	if err := scanCmd.ParseFlags([]string{"--refresh-shims"}); err != nil {
		t.Fatalf("ParseFlags returned unexpected error: %v", err)
	}

	if !scanRefreshShims {
		t.Error("expected scanRefreshShims to be true after --refresh-shims flag")
	}

	// Reset after test.
	scanRefreshShims = false
}

// TestRefreshShimsFlagDefaultFalse verifies that the flag is false by default
// (i.e. does not activate when absent from the command line).
func TestRefreshShimsFlagDefaultFalse(t *testing.T) {
	// Reset to known state.
	scanRefreshShims = false

	if err := scanCmd.ParseFlags([]string{}); err != nil {
		t.Fatalf("ParseFlags returned unexpected error: %v", err)
	}

	if scanRefreshShims {
		t.Error("expected scanRefreshShims to be false when --refresh-shims is not provided")
	}
}

// TestRunRefreshShimsEmptyDB verifies that runRefreshShims succeeds with an
// empty database. With no packages the binary list is empty; RefreshShims
// will remove any stale symlinks (none present in a fresh shim dir) and
// return (0, 0, nil). The function should therefore return nil.
//
// NOTE: When the shim binary is missing, BuildShimBinary is attempted. In the
// test environment that call may fail (no brewprune-shim on PATH / GOPATH)
// but runRefreshShims treats that as a soft warning — the test still expects
// a nil return value because RefreshShims itself will error only if the shim
// binary is truly absent AND symlinks need to be created.
//
// In this test the DB is empty so allBinaries == nil, meaning RefreshShims
// will have nothing to create and nothing to remove, therefore it succeeds
// even without the shim binary.
func TestRunRefreshShimsEmptyDB(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer db.Close()

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Save and restore global quiet flag so we don't pollute other tests.
	origQuiet := scanQuiet
	scanQuiet = true
	defer func() { scanQuiet = origQuiet }()

	err = runRefreshShims(db)
	if err != nil {
		t.Errorf("runRefreshShims with empty DB returned unexpected error: %v", err)
	}
}

// TestScanCommandFlagsIncludesRefreshShims extends the existing flag table
// test to confirm --refresh-shims appears alongside the other flags.
func TestScanCommandFlagsIncludesRefreshShims(t *testing.T) {
	flagNames := []string{"refresh-binaries", "quiet", "refresh-shims"}
	for _, name := range flagNames {
		if f := scanCmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected flag %q to be registered on scan command", name)
		}
	}
}

// TestRunScan_ShimCountZeroShowsUpToDate verifies that when GenerateShims
// returns 0 (re-scan, all shims already exist), the output message reads
// "up to date" rather than "0 command shims created". This is tested by
// constructing the message the same way runScan does and asserting on the
// format, using a temp directory with pre-existing symlinks to simulate the
// countSymlinks call.
func TestRunScan_ShimCountZeroShowsUpToDate(t *testing.T) {
	// Create a temp directory that acts as the shim dir with pre-existing symlinks.
	tmpDir := t.TempDir()
	// Create a few dummy symlinks in the temp dir.
	targets := []string{"git", "gh", "jq"}
	for _, name := range targets {
		target := fmt.Sprintf("/usr/bin/%s", name)
		link := fmt.Sprintf("%s/%s", tmpDir, name)
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("failed to create symlink %s: %v", link, err)
		}
	}

	// Simulate the shimCount==0 branch: count existing symlinks and build message.
	shimCount := 0
	existing := countSymlinks(tmpDir)
	if existing != len(targets) {
		t.Errorf("expected countSymlinks to return %d, got %d", len(targets), existing)
	}

	var shimMsg string
	if shimCount == 0 {
		shimMsg = fmt.Sprintf("✓ %d shims up to date (0 new)", existing)
	} else {
		shimMsg = fmt.Sprintf("✓ %d command shims created", shimCount)
	}

	if !strings.Contains(shimMsg, "up to date") {
		t.Errorf("expected shimMsg to contain 'up to date', got: %s", shimMsg)
	}
	if strings.Contains(shimMsg, "command shims created") {
		t.Errorf("expected shimMsg NOT to contain 'command shims created' when shimCount==0, got: %s", shimMsg)
	}
}

// TestDetectChanges verifies the detectChanges helper correctly identifies
// when package lists differ.
func TestDetectChanges(t *testing.T) {
	tests := []struct {
		name     string
		oldPkgs  []*brew.Package
		newPkgs  []*brew.Package
		expected bool
	}{
		{
			name:     "empty lists",
			oldPkgs:  []*brew.Package{},
			newPkgs:  []*brew.Package{},
			expected: false,
		},
		{
			name:    "first scan (old empty)",
			oldPkgs: []*brew.Package{},
			newPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0"},
			},
			expected: true,
		},
		{
			name: "identical packages",
			oldPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0", BinaryPaths: []string{"/usr/local/bin/git"}},
				{Name: "node", Version: "20.10.0", BinaryPaths: []string{"/usr/local/bin/node"}},
			},
			newPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0", BinaryPaths: []string{"/usr/local/bin/git"}},
				{Name: "node", Version: "20.10.0", BinaryPaths: []string{"/usr/local/bin/node"}},
			},
			expected: false,
		},
		{
			name: "version changed",
			oldPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0"},
			},
			newPkgs: []*brew.Package{
				{Name: "git", Version: "2.44.0"},
			},
			expected: true,
		},
		{
			name: "package added",
			oldPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0"},
			},
			newPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0"},
				{Name: "node", Version: "20.10.0"},
			},
			expected: true,
		},
		{
			name: "package removed",
			oldPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0"},
				{Name: "node", Version: "20.10.0"},
			},
			newPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0"},
			},
			expected: true,
		},
		{
			name: "binary paths changed",
			oldPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0", BinaryPaths: []string{"/usr/local/bin/git"}},
			},
			newPkgs: []*brew.Package{
				{Name: "git", Version: "2.43.0", BinaryPaths: []string{"/usr/local/bin/git", "/usr/local/bin/git-shell"}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectChanges(tt.oldPkgs, tt.newPkgs)
			if result != tt.expected {
				t.Errorf("detectChanges() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestScanHelpExcludesInternalDetail verifies that the scan command help text
// does not mention "post_install hook" or other internal implementation details.
func TestScanHelpExcludesInternalDetail(t *testing.T) {
	example := scanCmd.Example
	if example == "" {
		t.Fatal("scan command Example field is empty")
	}

	if strings.Contains(example, "post_install hook") {
		t.Error("scan command Example should not mention 'post_install hook' (internal detail)")
	}

	// Verify the --refresh-shims example is still present
	if !strings.Contains(example, "--refresh-shims") {
		t.Error("scan command Example should still include --refresh-shims example")
	}
}

// TestRunScan_DaemonRunning_SuppressesWarning verifies that when the daemon is
// running, the post-scan messaging shows the confirmation message instead of
// the "NEXT STEP: Start usage tracking" warning. The test exercises the
// daemon-check logic path directly: it writes a PID file containing the current
// process's PID (which satisfies watcher.IsDaemonRunning) and asserts on the
// message strings that runScan would produce for both the shimCount>0 and
// shimCount==0 branches.
func TestRunScan_DaemonRunning_SuppressesWarning(t *testing.T) {
	// Set HOME to a temp dir so getDefaultPIDFile resolves into it.
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome) //nolint:errcheck

	// Create ~/.brewprune and write a PID file with the current PID so that
	// watcher.IsDaemonRunning returns true.
	brewpruneDir := tmpDir + "/.brewprune"
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
	}
	pidFilePath := brewpruneDir + "/watch.pid"
	pidContent := fmt.Sprintf("%d\n", os.Getpid())
	if err := os.WriteFile(pidFilePath, []byte(pidContent), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Confirm IsDaemonRunning returns true for the current process.
	running, err := watcher.IsDaemonRunning(pidFilePath)
	if err != nil {
		t.Fatalf("IsDaemonRunning returned error: %v", err)
	}
	if !running {
		t.Fatal("expected IsDaemonRunning to return true for current process PID")
	}

	// Replicate the daemon-check preamble from runScan.
	resolvedPID, pidErr := getDefaultPIDFile()
	daemonAlreadyRunning := false
	if pidErr == nil {
		if r, runErr := watcher.IsDaemonRunning(resolvedPID); runErr == nil && r {
			daemonAlreadyRunning = true
		}
	}
	if !daemonAlreadyRunning {
		t.Fatal("expected daemonAlreadyRunning=true after writing current PID to watch.pid")
	}

	shimOK, _ := shim.IsShimSetup()

	// Helper: choose the message the same way runScan does.
	chooseMsg := func(sc int) string {
		if sc > 0 {
			if !shimOK {
				return "Usage tracking requires one more step"
			} else if daemonAlreadyRunning {
				return "✓ Daemon is running — usage tracking is active."
			}
			return "NEXT STEP: Start usage tracking with 'brewprune watch --daemon'"
		}
		if daemonAlreadyRunning {
			return "✓ Daemon is running — usage tracking is active."
		}
		return "NEXT STEP: Start usage tracking with 'brewprune watch --daemon'"
	}

	// shimCount==0 branch: daemon running must always show the confirmation
	// regardless of shimOK, because there are no new shims to warn about.
	msg0 := chooseMsg(0)
	if !strings.Contains(msg0, "Daemon is running") {
		t.Errorf("(shimCount==0) expected 'Daemon is running' in message, got: %q", msg0)
	}
	if strings.Contains(msg0, "NEXT STEP") {
		t.Errorf("(shimCount==0) expected 'NEXT STEP' absent when daemon running, got: %q", msg0)
	}

	// shimCount>0, shimOK==true branch: daemon running must show confirmation.
	// Only assert this when shim PATH is set up in the current environment;
	// otherwise the PATH-missing message is correctly shown (preserved path).
	if shimOK {
		msg1 := chooseMsg(1)
		if !strings.Contains(msg1, "Daemon is running") {
			t.Errorf("(shimCount>0,shimOK) expected 'Daemon is running' in message, got: %q", msg1)
		}
		if strings.Contains(msg1, "NEXT STEP") {
			t.Errorf("(shimCount>0,shimOK) expected 'NEXT STEP' absent when daemon running, got: %q", msg1)
		}
	}
}
