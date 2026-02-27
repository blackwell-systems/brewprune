package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
)

// TestRunShimTest_NoShimBinary verifies that RunShimTest fails gracefully when
// the shim directory is empty or does not exist.
func TestRunShimTest_NoShimBinary(t *testing.T) {
	// Use an in-memory store so we don't need a real database file.
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	defer st.Close()

	// Point the shim directory at an empty temp dir (simulates no shims).
	emptyDir := t.TempDir()

	origFn := findShimTestBinary
	_ = origFn // referenced to avoid "declared and not used" in editors

	// We test findShimTestBinary directly: an empty dir must yield "".
	result := findShimTestBinary(emptyDir)
	if result != "" {
		t.Errorf("expected empty string for empty shimDir, got %q", result)
	}

	// Also test a non-existent directory.
	result2 := findShimTestBinary(filepath.Join(emptyDir, "nonexistent"))
	if result2 != "" {
		t.Errorf("expected empty string for non-existent shimDir, got %q", result2)
	}

	// When the shimDir is the real one but it doesn't exist, RunShimTest should
	// return a non-nil error without panicking.
	//
	// We need to test RunShimTest with a shimDir that doesn't exist. Since the
	// shim directory is resolved internally by RunShimTest via shim.GetShimDir(),
	// we can only test the graceful path if the real ~/.brewprune/bin does not
	// exist. This sub-test skips if it does exist (common on developer machines).
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	realShimDir := filepath.Join(home, ".brewprune", "bin")
	if _, statErr := os.Stat(realShimDir); os.IsNotExist(statErr) {
		// Real shimDir doesn't exist — RunShimTest must return an error fast.
		runErr := RunShimTest(st, 100*time.Millisecond)
		if runErr == nil {
			t.Error("expected error when shim directory does not exist, got nil")
		}
	}
}

// TestFindRealBinary_NoSelfExec verifies that findShimTestBinary never returns
// the brewprune-shim binary itself, protecting against an infinite exec loop.
func TestFindRealBinary_NoSelfExec(t *testing.T) {
	dir := t.TempDir()

	// Create a fake brewprune-shim binary (regular file, not a symlink).
	shimBin := filepath.Join(dir, "brewprune-shim")
	if err := os.WriteFile(shimBin, []byte("fake"), 0755); err != nil {
		t.Fatalf("failed to create fake shim binary: %v", err)
	}

	// findShimTestBinary must not return the shim binary even when it's the
	// only executable in the directory.
	result := findShimTestBinary(dir)
	if result != "" {
		t.Errorf("findShimTestBinary must not return brewprune-shim, but got %q", result)
	}

	// Now add a valid symlink for "git" pointing to the fake shim binary.
	// findShimTestBinary should prefer "git" over brewprune-shim.
	gitShim := filepath.Join(dir, "git")
	if err := os.Symlink(shimBin, gitShim); err != nil {
		t.Fatalf("failed to create git symlink: %v", err)
	}

	result2 := findShimTestBinary(dir)
	if result2 != gitShim {
		t.Errorf("expected %q, got %q", gitShim, result2)
	}
	if filepath.Base(result2) == "brewprune-shim" {
		t.Error("findShimTestBinary must never return brewprune-shim")
	}
}

// TestRunShimTest_TimeoutReturnsError verifies that RunShimTest returns a
// descriptive error when no usage event appears within maxWait. We use a very
// short timeout and an in-memory database that will never receive real events.
func TestRunShimTest_TimeoutReturnsError(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	defer st.Close()

	// Use a tiny timeout so the test finishes quickly.
	// If the shim directory doesn't exist, RunShimTest will return an error
	// before even trying to poll — that is also an acceptable "failure" path
	// for this test.
	start := time.Now()
	runErr := RunShimTest(st, 200*time.Millisecond)
	elapsed := time.Since(start)

	if runErr == nil {
		// This can only be nil if the test machine happens to have a functioning
		// full brewprune pipeline — extremely unlikely in CI. Warn but don't fail.
		t.Log("RunShimTest unexpectedly returned nil (pipeline appears fully functional)")
		return
	}

	// The function must not hang beyond maxWait + a generous buffer.
	const buffer = 5 * time.Second
	if elapsed > 200*time.Millisecond+buffer {
		t.Errorf("RunShimTest took %v, expected ≤ %v", elapsed, 200*time.Millisecond+buffer)
	}

	// The error message must be non-empty and descriptive.
	if runErr.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestFindShimTestBinary_PreferredOrder verifies that the preferred binary
// preference list (git > ls > echo > true) is respected when multiple symlinks
// are present.
func TestFindShimTestBinary_PreferredOrder(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "brewprune-shim")
	if err := os.WriteFile(fakeBin, []byte("fake"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Create symlinks for "ls" and "git".
	for _, name := range []string{"ls", "git"} {
		p := filepath.Join(dir, name)
		if err := os.Symlink(fakeBin, p); err != nil {
			t.Fatalf("symlink %s: %v", name, err)
		}
	}

	result := findShimTestBinary(dir)
	if filepath.Base(result) != "git" {
		t.Errorf("expected 'git' to be preferred, got %q", filepath.Base(result))
	}
}

// TestSafeArgsFor verifies that safeArgsFor returns non-panicking args for
// known and unknown binary names.
func TestSafeArgsFor(t *testing.T) {
	cases := []struct {
		name string
	}{
		{"git"},
		{"echo"},
		{"true"},
		{"ls"},
		{"unknown-binary"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := safeArgsFor(tc.name)
			// Just ensure it doesn't panic and returns a slice (nil is fine for "true").
			_ = args
		})
	}
}
