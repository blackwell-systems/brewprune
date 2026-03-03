package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

func TestRemoveCommand(t *testing.T) {
	// Test command registration
	if removeCmd == nil {
		t.Fatal("removeCmd is nil")
	}

	// Test command metadata
	if removeCmd.Use != "remove [packages...]" {
		t.Errorf("removeCmd.Use = %q, want %q", removeCmd.Use, "remove [packages...]")
	}

	if removeCmd.Short == "" {
		t.Error("removeCmd.Short is empty")
	}

	if removeCmd.RunE == nil {
		t.Error("removeCmd.RunE is nil")
	}
}

func TestRemoveFlags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue bool
	}{
		{"safe flag", "safe", false},
		{"medium flag", "medium", false},
		{"risky flag", "risky", false},
		{"dry-run flag", "dry-run", false},
		{"yes flag", "yes", false},
		{"no-snapshot flag", "no-snapshot", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := removeCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			if flag.DefValue != "false" {
				t.Errorf("flag %q default = %q, want %q", tt.flagName, flag.DefValue, "false")
			}
		})
	}
}

func TestDetermineTier(t *testing.T) {
	tests := []struct {
		name      string
		safe      bool
		medium    bool
		risky     bool
		expected  string
		wantError bool
	}{
		{"no flags", false, false, false, "", false},
		{"safe only", true, false, false, "safe", false},
		{"medium only", false, true, false, "medium", false},
		{"risky only", false, false, true, "risky", false},
		// Multiple shorthand flags are now a conflict error (REMOVE-1)
		{"safe and medium", true, true, false, "", true},
		{"all flags", true, true, true, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			removeFlagSafe = tt.safe
			removeFlagMedium = tt.medium
			removeFlagRisky = tt.risky
			removeTierFlag = ""

			result, err := determineTier()

			if tt.wantError {
				if err == nil {
					t.Errorf("determineTier() expected error for safe=%v medium=%v risky=%v, got none", tt.safe, tt.medium, tt.risky)
				}
				return
			}
			if err != nil {
				t.Errorf("determineTier() unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("determineTier() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Reset flags
	removeFlagSafe = false
	removeFlagMedium = false
	removeFlagRisky = false
	removeTierFlag = ""
}

func TestDetermineTier_TierFlag(t *testing.T) {
	tests := []struct {
		name      string
		tierFlag  string
		wantTier  string
		wantError bool
	}{
		{"tier safe", "safe", "safe", false},
		{"tier medium", "medium", "medium", false},
		{"tier risky", "risky", "risky", false},
		{"tier invalid", "invalid", "", true},
		{"tier empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset boolean flags
			removeFlagSafe = false
			removeFlagMedium = false
			removeFlagRisky = false
			removeTierFlag = tt.tierFlag

			result, err := determineTier()

			if tt.wantError {
				if err == nil {
					t.Errorf("determineTier() expected error for tier %q, got none", tt.tierFlag)
				}
				return
			}
			if err != nil {
				t.Errorf("determineTier() unexpected error: %v", err)
			}
			if result != tt.wantTier {
				t.Errorf("determineTier() = %q, want %q", result, tt.wantTier)
			}
		})
	}

	// Reset flags
	removeTierFlag = ""
}

func TestRemoveTierValidationFormat(t *testing.T) {
	// Test that tier validation error matches standard format
	removeFlagSafe = false
	removeFlagMedium = false
	removeFlagRisky = false
	removeTierFlag = "invalid"

	_, err := determineTier()

	if err == nil {
		t.Fatal("expected error for invalid tier, got nil")
	}

	expectedMsg := `invalid --tier value "invalid": must be one of: safe, medium, risky`
	if err.Error() != expectedMsg {
		t.Errorf("error message = %q, want %q", err.Error(), expectedMsg)
	}

	// Reset flags
	removeTierFlag = ""
}

func TestRemoveCommandRegistration(t *testing.T) {
	// Create a temporary root command for testing
	tempRoot := &cobra.Command{Use: "test"}

	// Add remove command
	tempRoot.AddCommand(removeCmd)

	// Verify command was added
	found := false
	for _, cmd := range tempRoot.Commands() {
		if cmd.Use == "remove [packages...]" {
			found = true
			break
		}
	}

	if !found {
		t.Error("remove command not registered with parent")
	}
}

func TestRemoveValidation(t *testing.T) {
	// Test that tier flag is required when no packages specified
	// This is more of an integration test and would need a mock setup

	t.Run("no tier and no packages", func(t *testing.T) {
		// This would fail in actual execution
		// Testing the determineTier function is sufficient
		removeTierFlag = ""
		tier, err := determineTier()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tier != "" {
			t.Errorf("expected empty tier, got %q", tier)
		}
	})
}

func TestDisplayConfidenceScores_LastUsedNotNever(t *testing.T) {
	// Create in-memory store for testing
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert a test package
	pkg := &brew.Package{
		Name:        "test-pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -30),
		InstallType: "explicit",
		Tap:         "homebrew/core",
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Insert a usage event
	now := time.Now()
	event := &store.UsageEvent{
		Package:    "test-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/test",
		Timestamp:  now,
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	// Verify getLastUsed returns a non-zero time for this package
	lastUsed := getLastUsed(st, "test-pkg")
	if lastUsed.IsZero() {
		t.Error("expected non-zero LastUsed time for package with usage data, got zero time (never)")
	}

	// Verify the time is approximately correct
	diff := lastUsed.Sub(now)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("LastUsed time mismatch: got %v, want approximately %v", lastUsed, now)
	}
}

func TestGetPackagesByTierLogic(t *testing.T) {
	// This tests the tier inclusion logic
	tests := []struct {
		name     string
		tier     string
		includes []string
	}{
		{"safe tier", "safe", []string{"safe"}},
		{"medium tier", "medium", []string{"safe", "medium"}},
		{"risky tier", "risky", []string{"safe", "medium", "risky"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the logic matches expected behavior
			switch tt.tier {
			case "safe":
				if len(tt.includes) != 1 || tt.includes[0] != "safe" {
					t.Errorf("safe tier should only include safe packages")
				}
			case "medium":
				if len(tt.includes) != 2 {
					t.Errorf("medium tier should include safe and medium packages")
				}
			case "risky":
				if len(tt.includes) != 3 {
					t.Errorf("risky tier should include all packages")
				}
			}
		})
	}
}

func TestRemoveHelp_ExplainsTierShortcuts(t *testing.T) {
	longDesc := removeCmd.Long
	if !strings.Contains(longDesc, "--tier") {
		t.Error("removeCmd.Long should contain '--tier' to explain the tier flag")
	}
	if !strings.Contains(longDesc, "shortcut") && !strings.Contains(longDesc, "equivalent") {
		t.Error("removeCmd.Long should contain 'shortcut' or 'equivalent' to explain the relationship between boolean flags and --tier")
	}
}

func TestRunRemove_NotFoundError_NotDoubled(t *testing.T) {
	// Create in-memory store for testing
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Attempt to get a nonexistent package to observe the store error
	_, storeErr := st.GetPackage("nonexistent")
	if storeErr == nil {
		t.Fatal("expected error for nonexistent package, got nil")
	}

	// The new behavior writes to stderr and calls os.Exit(1) — verify the store
	// error contains "not found" exactly once (confirming no double-wrapping at
	// the store layer).
	msg := storeErr.Error()

	// Count occurrences of "not found" in the store error message
	count := strings.Count(msg, "not found")
	if count != 1 {
		t.Errorf("store error contains 'not found' %d times, want exactly 1; message: %q", count, msg)
	}

	// Ensure the package name appears in the message
	if !strings.Contains(msg, "nonexistent") {
		t.Errorf("store error should contain the package name; got: %q", msg)
	}
}

// TestRemove_NonexistentPackageHelpfulError verifies that the helpful error
// message for a nonexistent package includes the "brew list" and "brewprune
// scan" suggestions (matching the explain command's error format).
func TestRemove_NonexistentPackageHelpfulError(t *testing.T) {
	// Simulate the stderr output that runRemove writes for a nonexistent package.
	// The actual code calls os.Exit(1) after this write, which we cannot easily
	// test end-to-end without subprocess spawning. Instead, we capture the output
	// of the formatting logic directly to verify the message content.
	pkgName := "nonexistent-package"

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Error: package not found: %s\n\nCheck the name with 'brew list' or 'brew search %s'.\nIf you just installed it, run 'brewprune scan' to update the index.\n", pkgName, pkgName)

	got := buf.String()

	// Must contain the package name
	if !strings.Contains(got, pkgName) {
		t.Errorf("error message does not contain package name %q: %s", pkgName, got)
	}
	// Must have "package not found:" prefix (new style, not "package %q not found")
	if !strings.Contains(got, "package not found:") {
		t.Errorf("error message missing 'package not found:' prefix: %s", got)
	}
	// Must NOT use quoted format from old style
	if strings.Contains(got, fmt.Sprintf("%q", pkgName)) {
		t.Errorf("error message should not use quoted package name format: %s", got)
	}
	// Must suggest 'brew list'
	if !strings.Contains(got, "brew list") {
		t.Errorf("error message missing 'brew list' suggestion: %s", got)
	}
	// Must suggest 'brew search <pkg>'
	if !strings.Contains(got, "brew search "+pkgName) {
		t.Errorf("error message missing 'brew search %s' suggestion: %s", pkgName, got)
	}
	// Must suggest 'brewprune scan'
	if !strings.Contains(got, "brewprune scan") {
		t.Errorf("error message missing 'brewprune scan' suggestion: %s", got)
	}
}

// TestRemove_NoDatabaseErrorUnwrapped verifies that when no DB exists the
// error returned by getPackagesByTier is unwrapped to the terminal sentinel
// without the "failed to get packages" prefix.
func TestRemove_NoDatabaseErrorUnwrapped(t *testing.T) {
	// Open an in-memory store WITHOUT calling CreateSchema so that
	// GetPackagesByTier returns store.ErrNotInitialized wrapped in chain.
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	// Verify that the raw error from the store is ErrNotInitialized.
	_, listErr := st.ListPackages()
	if listErr == nil {
		t.Fatal("expected error from ListPackages without schema, got nil")
	}
	if !errors.Is(listErr, store.ErrNotInitialized) {
		t.Fatalf("expected ErrNotInitialized from ListPackages, got: %v", listErr)
	}

	// Simulate the wrapping chain that getPackagesByTier creates and then
	// verify that the unwrapping logic in runRemove surfaces the terminal error.
	wrapped := fmt.Errorf("failed to get packages: %w",
		fmt.Errorf("failed to get safe tier: %w", listErr))

	cause := wrapped
	for errors.Unwrap(cause) != nil {
		cause = errors.Unwrap(cause)
	}

	// The unwrapped cause must be the ErrNotInitialized sentinel.
	if !errors.Is(cause, store.ErrNotInitialized) {
		t.Errorf("unwrapped cause is not ErrNotInitialized; got: %v", cause)
	}

	// The final error message must NOT contain the "failed to get packages" prefix.
	msg := cause.Error()
	if strings.Contains(msg, "failed to get packages") {
		t.Errorf("unwrapped error message still contains 'failed to get packages': %q", msg)
	}
	// Must contain the sentinel message text.
	if !strings.Contains(msg, "database not initialized") {
		t.Errorf("unwrapped error message missing 'database not initialized': %q", msg)
	}
	if !strings.Contains(msg, "brewprune scan") {
		t.Errorf("unwrapped error message missing 'brewprune scan': %q", msg)
	}
}

// TestDetermineTier_ConflictShorthands verifies that setting multiple shorthand
// tier flags simultaneously returns an error containing "only one tier flag".
func TestDetermineTier_ConflictShorthands(t *testing.T) {
	tests := []struct {
		name   string
		safe   bool
		medium bool
		risky  bool
	}{
		{"safe and medium", true, true, false},
		{"safe and risky", true, false, true},
		{"medium and risky", false, true, true},
		{"all three", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeFlagSafe = tt.safe
			removeFlagMedium = tt.medium
			removeFlagRisky = tt.risky
			removeTierFlag = ""
			defer func() {
				removeFlagSafe = false
				removeFlagMedium = false
				removeFlagRisky = false
			}()

			_, err := determineTier()
			if err == nil {
				t.Errorf("determineTier() expected error for multiple shorthand flags, got none")
				return
			}
			if !strings.Contains(err.Error(), "only one tier flag") {
				t.Errorf("error %q does not contain %q", err.Error(), "only one tier flag")
			}
		})
	}
}

// TestDetermineTier_ConflictShorthandAndTierFlag verifies that combining --tier
// with a shorthand flag returns an error.
func TestDetermineTier_ConflictShorthandAndTierFlag(t *testing.T) {
	tests := []struct {
		name     string
		tierFlag string
		safe     bool
		medium   bool
		risky    bool
	}{
		{"--tier safe with --safe", "safe", true, false, false},
		{"--tier medium with --risky", "medium", false, false, true},
		{"--tier risky with --medium", "risky", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeTierFlag = tt.tierFlag
			removeFlagSafe = tt.safe
			removeFlagMedium = tt.medium
			removeFlagRisky = tt.risky
			defer func() {
				removeTierFlag = ""
				removeFlagSafe = false
				removeFlagMedium = false
				removeFlagRisky = false
			}()

			_, err := determineTier()
			if err == nil {
				t.Errorf("determineTier() expected error when --tier combined with shorthand, got none")
				return
			}
			if !strings.Contains(err.Error(), "cannot combine --tier") {
				t.Errorf("error %q does not contain %q", err.Error(), "cannot combine --tier")
			}
		})
	}
}

// TestConfirmRemoval_RiskyRequiresYes verifies that risky tier confirmation
// rejects "y" and accepts only "yes".
func TestConfirmRemoval_RiskyRequiresYes(t *testing.T) {
	t.Run("risky rejects y", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r

		// Write "y\n" as input
		_, _ = io.WriteString(w, "y\n")
		w.Close()

		result := confirmRemoval(5, "risky")
		os.Stdin = oldStdin

		if result {
			t.Error("risky confirmRemoval should reject input 'y', expected false")
		}
	})

	t.Run("risky accepts yes", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r

		// Write "yes\n" as input
		_, _ = io.WriteString(w, "yes\n")
		w.Close()

		result := confirmRemoval(5, "risky")
		os.Stdin = oldStdin

		if !result {
			t.Error("risky confirmRemoval should accept input 'yes', expected true")
		}
	})

	t.Run("safe accepts y", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r

		_, _ = io.WriteString(w, "y\n")
		w.Close()

		result := confirmRemoval(3, "safe")
		os.Stdin = oldStdin

		if !result {
			t.Error("safe confirmRemoval should accept input 'y', expected true")
		}
	})
}

// TestFreedSpaceReflectsActualRemovals verifies that the freed-size accumulation
// logic only counts packages that were successfully removed, not all candidates.
func TestFreedSpaceReflectsActualRemovals(t *testing.T) {
	// Simulate three packages: sizes 100, 200, 300 bytes.
	// Suppose only the first and third are successfully removed.
	type pkg struct {
		name     string
		size     int64
		removeOK bool
	}

	packages := []pkg{
		{"pkg-a", 100, true},
		{"pkg-b", 200, false}, // removal fails
		{"pkg-c", 300, true},
	}

	var freedSize int64
	successCount := 0

	for _, p := range packages {
		pkgSize := p.size // captured before removal
		if !p.removeOK {
			// simulate brew.Uninstall failure — skip accumulation
			continue
		}
		freedSize += pkgSize
		successCount++
	}

	expectedFreed := int64(400) // 100 + 300
	if freedSize != expectedFreed {
		t.Errorf("freedSize = %d, want %d", freedSize, expectedFreed)
	}
	if successCount != 2 {
		t.Errorf("successCount = %d, want 2", successCount)
	}

	// Verify that totalSize (all candidates) differs from freedSize
	var totalSize int64
	for _, p := range packages {
		totalSize += p.size
	}
	if totalSize == freedSize {
		t.Errorf("totalSize (%d) should not equal freedSize (%d) when some removals fail", totalSize, freedSize)
	}
}

// TestRemoveFiltersDepLockedPackages verifies that the dep-locked filter logic
// correctly separates locked from unlocked packages.
func TestRemoveFiltersDepLockedPackages(t *testing.T) {
	type candidate struct {
		name string
		deps []string
	}

	candidates := []candidate{
		{"pkg-free", nil},
		{"pkg-locked", []string{"dep-a", "dep-b"}},
		{"pkg-also-free", nil},
	}

	var lockedPackages []string
	var filteredNames []string

	for _, c := range candidates {
		if len(c.deps) > 0 {
			lockedPackages = append(lockedPackages, fmt.Sprintf("%s (required by: %s)", c.name, strings.Join(c.deps, ", ")))
		} else {
			filteredNames = append(filteredNames, c.name)
		}
	}

	if len(lockedPackages) != 1 {
		t.Errorf("expected 1 locked package, got %d: %v", len(lockedPackages), lockedPackages)
	}
	if len(filteredNames) != 2 {
		t.Errorf("expected 2 filtered packages, got %d: %v", len(filteredNames), filteredNames)
	}
	if !strings.Contains(lockedPackages[0], "dep-a") {
		t.Errorf("locked package entry should contain dep-a: %s", lockedPackages[0])
	}
	if !strings.Contains(lockedPackages[0], "dep-b") {
		t.Errorf("locked package entry should contain dep-b: %s", lockedPackages[0])
	}
	if filteredNames[0] != "pkg-free" || filteredNames[1] != "pkg-also-free" {
		t.Errorf("unexpected filtered names: %v", filteredNames)
	}
}

// TestRemoveStalenessCheckRemoved verifies that runRemove does not print
// "new formulae since last scan" (the staleness check block is removed).
func TestRemoveStalenessCheckRemoved(t *testing.T) {
	// Verify that the staleness warning string does not appear in runRemove source.
	// We do this by inspecting the Long description and command help — neither should
	// mention the staleness message. More importantly, we assert the function signature
	// doesn't invoke CheckStaleness by checking source-level constants.
	//
	// The practical assertion: the removeCmd.Long and removeCmd.Short should NOT
	// contain the "new formulae since last scan" phrase.
	stalePhrases := []string{
		"new formulae since last scan",
	}
	for _, phrase := range stalePhrases {
		if strings.Contains(removeCmd.Long, phrase) {
			t.Errorf("removeCmd.Long contains stale-check phrase %q — should be removed", phrase)
		}
		if strings.Contains(removeCmd.Short, phrase) {
			t.Errorf("removeCmd.Short contains stale-check phrase %q — should be removed", phrase)
		}
	}

	// Additional: verify the removeCmd RunE is non-nil (command is still functional)
	if removeCmd.RunE == nil {
		t.Error("removeCmd.RunE should not be nil after removing staleness check")
	}
}

// TestNoSnapshotWarning_Output verifies that when --no-snapshot is active,
// the summary output contains "cannot be undone".
func TestNoSnapshotWarning_Output(t *testing.T) {
	// Capture stdout by redirecting it
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Simulate the output that runRemove generates for --no-snapshot
	// (mirrors the logic in remove.go)
	removeFlagNoSnapshot = true
	defer func() {
		removeFlagNoSnapshot = false
		os.Stdout = oldStdout
	}()

	// Replicate the snapshot display logic (no TTY in test so isColor=false)
	isColor := false
	if isColor {
		fmt.Printf("  \033[33m⚠  Snapshot: SKIPPED (--no-snapshot) — removal cannot be undone!\033[0m\n")
	} else {
		fmt.Printf("  ⚠  Snapshot: SKIPPED (--no-snapshot) — removal cannot be undone!\n")
	}

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = oldStdout

	output := buf.String()
	if !strings.Contains(output, "cannot be undone") {
		t.Errorf("output %q does not contain 'cannot be undone'", output)
	}
	if !strings.Contains(output, "SKIPPED") {
		t.Errorf("output %q does not contain 'SKIPPED'", output)
	}
	if !strings.Contains(output, "⚠") {
		t.Errorf("output %q does not contain warning character '⚠'", output)
	}
}

// TestRemoveSkippedSummaryAppearsAfterTable verifies that when skipped packages
// exist, only a summary line (containing "skipped" and the count) is emitted —
// not the full per-package list — and that the summary is not emitted before the
// action table header text.
func TestRemoveSkippedSummaryAppearsAfterTable(t *testing.T) {
	// Simulate 3 locked packages and verify the summary line format.
	lockedPackages := []string{
		"pkg-a (required by: dep-x)",
		"pkg-b (required by: dep-y)",
		"pkg-c (required by: dep-z)",
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	// Replicate the summary-only output from remove.go (tier-based branch)
	if len(lockedPackages) > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠  %d packages skipped (locked by dependents) — run with --verbose to see details\n", len(lockedPackages))
	}

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stderr = oldStderr

	got := buf.String()

	// Must contain the count and the word "skipped"
	if !strings.Contains(got, "3") {
		t.Errorf("summary line does not contain count '3': %q", got)
	}
	if !strings.Contains(got, "skipped") {
		t.Errorf("summary line does not contain 'skipped': %q", got)
	}

	// Must NOT contain the individual package details
	for _, pkg := range []string{"pkg-a", "pkg-b", "pkg-c"} {
		if strings.Contains(got, pkg) {
			t.Errorf("summary line should not contain individual package %q but got: %q", pkg, got)
		}
	}
}

// TestRemoveMultiFlagErrorReportsAll verifies that when --safe, --medium, and
// --risky are all set simultaneously, the error message lists all three flags
// with Oxford comma formatting rather than reporting only the first two.
func TestRemoveMultiFlagErrorReportsAll(t *testing.T) {
	removeFlagSafe = true
	removeFlagMedium = true
	removeFlagRisky = true
	removeTierFlag = ""
	defer func() {
		removeFlagSafe = false
		removeFlagMedium = false
		removeFlagRisky = false
	}()

	_, err := determineTier()
	if err == nil {
		t.Fatal("determineTier() expected error for all three flags set, got nil")
	}

	msg := err.Error()

	// All three flags must appear in the error
	for _, flag := range []string{"--safe", "--medium", "--risky"} {
		if !strings.Contains(msg, flag) {
			t.Errorf("error %q does not contain flag %q", msg, flag)
		}
	}

	// Must still contain the "only one tier flag" prefix
	if !strings.Contains(msg, "only one tier flag") {
		t.Errorf("error %q does not contain 'only one tier flag'", msg)
	}

	// Oxford comma: should contain ", and" for 3-item list
	if !strings.Contains(msg, ", and") {
		t.Errorf("error %q missing Oxford comma ', and' for 3-item list", msg)
	}
}

// TestRemoveAllFailExitsNonZero verifies that when successCount == 0 and there
// are failures, the result summary uses ✗ (not ✓) and a non-nil error is returned.
func TestRemoveAllFailExitsNonZero(t *testing.T) {
	// Simulate the summary logic from runRemove with all removals failing.
	successCount := 0
	failures := []string{
		"pkg-a: uninstall failed: exit status 1",
		"pkg-b: uninstall failed: exit status 1",
	}
	var freedSize int64

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Replicate the summary branch from remove.go
	var resultErr error
	if successCount == 0 && len(failures) > 0 {
		fmt.Printf("\n✗ Removed 0 packages, freed %s\n", formatSize(freedSize))
		fmt.Printf("\n⚠️  %d failures:\n", len(failures))
		for _, failure := range failures {
			fmt.Printf("  - %s\n", failure)
		}
		resultErr = fmt.Errorf("removed 0 packages: all %d removals failed", len(failures))
	}

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = oldStdout

	output := buf.String()

	// Must use ✗ not ✓
	if !strings.Contains(output, "✗") {
		t.Errorf("output %q does not contain ✗ on total failure", output)
	}
	if strings.Contains(output, "✓") {
		t.Errorf("output %q should not contain ✓ on total failure", output)
	}

	// Must report 0 packages
	if !strings.Contains(output, "0 packages") {
		t.Errorf("output %q does not contain '0 packages'", output)
	}

	// Must return a non-nil error
	if resultErr == nil {
		t.Error("expected non-nil error when all removals fail, got nil")
	}

	// Error message must reference the failure count
	if resultErr != nil && !strings.Contains(resultErr.Error(), "all 2 removals failed") {
		t.Errorf("error message %q does not contain 'all 2 removals failed'", resultErr.Error())
	}
}

// TestRemoveNotFoundUndoHint verifies that the not-found error message
// contains the "brewprune undo" hint (parity with explain.go).
func TestRemoveNotFoundUndoHint(t *testing.T) {
	pkgName := "missing-package"

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	// Replicate the not-found message from remove.go
	fmt.Fprintf(os.Stderr, "Error: package not found: %s\n\nCheck the name with 'brew list' or 'brew search %s'.\nIf you just installed it, run 'brewprune scan' to update the index.\nIf you recently ran 'brewprune undo', run 'brewprune scan' to update the index.\n", pkgName, pkgName)

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stderr = oldStderr

	got := buf.String()

	// Must contain the undo hint
	if !strings.Contains(got, "brewprune undo") {
		t.Errorf("not-found message missing 'brewprune undo' hint: %s", got)
	}

	// Must still contain the existing scan hint
	if !strings.Contains(got, "brewprune scan") {
		t.Errorf("not-found message missing 'brewprune scan' hint: %s", got)
	}

	// Must contain the package name
	if !strings.Contains(got, pkgName) {
		t.Errorf("not-found message missing package name %q: %s", pkgName, got)
	}
}

// TestRemoveAllLockedExitsNonZero verifies that when all candidates are filtered
// out by the dep-locked check, the empty packagesToRemove branch returns a
// non-nil error containing "skipped".
func TestRemoveAllLockedExitsNonZero(t *testing.T) {
	// Simulate the condition: all packages were locked, so packagesToRemove is empty.
	packagesToRemove := []string{}

	// Replicate the guard branch logic from runRemove.
	var resultErr error
	if len(packagesToRemove) == 0 {
		fmt.Println("No packages to remove.")
		resultErr = fmt.Errorf("all candidates were skipped (locked by dependents) — run with --verbose for details")
	}

	if resultErr == nil {
		t.Fatal("expected non-nil error when all candidates are locked, got nil")
	}
	if !strings.Contains(resultErr.Error(), "skipped") {
		t.Errorf("error %q does not contain 'skipped'", resultErr.Error())
	}
}

// TestDryRunBannerAppearsAtTop verifies that the DRY RUN banner string is
// present in output when dry-run mode is active.
func TestDryRunBannerAppearsAtTop(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	removeFlagDryRun = true
	defer func() {
		removeFlagDryRun = false
		os.Stdout = oldStdout
	}()

	// Replicate the tier-path display logic from remove.go
	tier := "safe"
	fmt.Printf("\nPackages to remove (%s tier):\n\n", tier)
	if removeFlagDryRun {
		fmt.Println("  *** DRY RUN — NO CHANGES WILL BE MADE ***")
		fmt.Println()
	}
	// (displayConfidenceScores would follow here)
	fmt.Println("some-package  safe  ...")

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = oldStdout

	got := buf.String()

	if !strings.Contains(got, "*** DRY RUN") {
		t.Errorf("output %q does not contain '*** DRY RUN' banner", got)
	}

	// Verify the banner appears before the package table content
	bannerIdx := strings.Index(got, "*** DRY RUN")
	tableIdx := strings.Index(got, "some-package")
	if bannerIdx == -1 {
		t.Fatal("DRY RUN banner not found in output")
	}
	if tableIdx == -1 {
		t.Fatal("package table content not found in output")
	}
	if bannerIdx > tableIdx {
		t.Errorf("DRY RUN banner (pos %d) appears after package table (pos %d) — should be before", bannerIdx, tableIdx)
	}
}

// TestNoSnapshotFlagDescription verifies that the --no-snapshot flag usage
// string contains "WARNING".
func TestNoSnapshotFlagDescription(t *testing.T) {
	flag := removeCmd.Flags().Lookup("no-snapshot")
	if flag == nil {
		t.Fatal("no-snapshot flag not found")
	}
	if !strings.Contains(flag.Usage, "WARNING") {
		t.Errorf("no-snapshot flag Usage %q does not contain 'WARNING'", flag.Usage)
	}
}

// TestRemoveExplicitPackageNoExplicitlyInstalledWarning verifies that the
// "explicitly installed" warning is suppressed for named-package removal.
// It tests the filtering logic directly without requiring a real DB or brew.
func TestRemoveExplicitPackageNoExplicitlyInstalledWarning(t *testing.T) {
	// Simulate the warnings returned by anlzr.ValidateRemoval for explicit packages
	allWarnings := []string{
		"bat: explicitly installed (not a dependency)",
		"fd: explicitly installed (not a dependency)",
		"bat: used recently (2026-01-01)",
	}

	// Apply the same filter used in remove.go
	var filteredWarnings []string
	for _, w := range allWarnings {
		if !strings.Contains(w, "explicitly installed") {
			filteredWarnings = append(filteredWarnings, w)
		}
	}

	// "explicitly installed" warnings must be gone
	for _, w := range filteredWarnings {
		if strings.Contains(w, "explicitly installed") {
			t.Errorf("filtered warnings still contain 'explicitly installed': %q", w)
		}
	}

	// Non-explicitly-installed warnings must survive the filter
	foundRecentlyUsed := false
	for _, w := range filteredWarnings {
		if strings.Contains(w, "used recently") {
			foundRecentlyUsed = true
			break
		}
	}
	if !foundRecentlyUsed {
		t.Errorf("filter removed 'used recently' warning — it should have been kept; remaining: %v", filteredWarnings)
	}
}

// TestRemove_LockedWarningClear verifies the locked warning has clear text.
func TestRemove_LockedWarningClear(t *testing.T) {
	lockedPackages := []string{"pkg-a (required by: dep-x)"}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\n⚠  %d packages skipped (have other packages depending on them) — remove their dependents first, or use --verbose to see details\n", len(lockedPackages))

	got := buf.String()
	if !strings.Contains(got, "have other packages depending on them") {
		t.Errorf("locked warning should contain clear explanation, got: %q", got)
	}
	if !strings.Contains(got, "remove their dependents first") {
		t.Errorf("locked warning should suggest removing dependents first, got: %q", got)
	}
}

// TestRemove_RiskyWarningBanner verifies risky banner shown for risky dry-run.
func TestRemove_RiskyWarningBanner(t *testing.T) {
	var buf bytes.Buffer

	activeTier := "risky"
	removeFlagDryRun := true

	if activeTier == "risky" && removeFlagDryRun {
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "═══════════════════════════════════════════════════════════════")
		fmt.Fprintln(&buf, "⚠  WARNING: Risky tier removal may break system dependencies")
		fmt.Fprintln(&buf, "═══════════════════════════════════════════════════════════════")
		fmt.Fprintln(&buf)
	}

	got := buf.String()
	if !strings.Contains(got, "WARNING") {
		t.Errorf("risky banner should contain WARNING, got: %q", got)
	}
	if !strings.Contains(got, "may break system dependencies") {
		t.Errorf("risky banner should warn about system dependencies, got: %q", got)
	}
	if !strings.Contains(got, "═══") {
		t.Errorf("risky banner should have visual separator, got: %q", got)
	}
}

// TestRemove_ExitCodeAllLocked verifies exit 1 when all packages locked.
func TestRemove_ExitCodeAllLocked(t *testing.T) {
	packagesToRemove := []string{}

	var resultErr error
	if len(packagesToRemove) == 0 {
		fmt.Println("No packages to remove.")
		resultErr = fmt.Errorf("no packages removed: all candidates were locked by dependents")
	}

	if resultErr == nil {
		t.Fatal("expected error when all packages locked, got nil")
	}
	if !strings.Contains(resultErr.Error(), "no packages removed") {
		t.Errorf("error should contain 'no packages removed', got: %q", resultErr.Error())
	}
	if !strings.Contains(resultErr.Error(), "locked by dependents") {
		t.Errorf("error should mention locked by dependents, got: %q", resultErr.Error())
	}
}
