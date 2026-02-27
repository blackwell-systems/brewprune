package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return st
}

func TestNew(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	if w == nil {
		t.Fatal("New() returned nil watcher")
	}

	if w.store != st {
		t.Error("watcher store not set correctly")
	}

	if w.binaryMap == nil {
		t.Error("binaryMap not initialized")
	}

	if w.eventQueue == nil {
		t.Error("eventQueue not initialized")
	}
}

func TestNew_NilStore(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Error("New(nil) expected error, got nil")
	}
}

func TestHandleFileEvent(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create temporary executable
	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "test-exec")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test executable: %v", err)
	}

	// Add to binary map
	w.binaryMap = map[string]string{
		execPath: "test-package",
	}

	// Handle the event
	w.handleFileEvent(execPath)

	// Check that event was queued
	select {
	case event := <-w.eventQueue:
		if event.Package != "test-package" {
			t.Errorf("event.Package = %q, want %q", event.Package, "test-package")
		}
		if event.BinaryPath != execPath {
			t.Errorf("event.BinaryPath = %q, want %q", event.BinaryPath, execPath)
		}
		if event.EventType != "exec" {
			t.Errorf("event.EventType = %q, want %q", event.EventType, "exec")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected event to be queued, but queue is empty")
	}
}

func TestHandleFileEvent_NonExecutable(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create non-executable file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-file")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Add to binary map (shouldn't match because not executable)
	w.binaryMap = map[string]string{
		filePath: "test-package",
	}

	// Handle the event
	w.handleFileEvent(filePath)

	// Should not queue event
	select {
	case <-w.eventQueue:
		t.Error("expected no event for non-executable file")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestHandleFileEvent_UnknownBinary(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create executable but don't add to binary map
	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "unknown-exec")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test executable: %v", err)
	}

	// Handle the event
	w.handleFileEvent(execPath)

	// Should not queue event
	select {
	case <-w.eventQueue:
		t.Error("expected no event for unknown binary")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestGetBrewPrefix(t *testing.T) {
	prefix, err := getBrewPrefix()
	if err != nil {
		// This test might fail in environments without Homebrew
		t.Skipf("brew not available: %v", err)
	}

	if prefix == "" {
		t.Error("getBrewPrefix() returned empty string")
	}

	// Should be a valid directory path
	if _, err := os.Stat(prefix); err != nil {
		t.Errorf("brew prefix %q is not a valid directory: %v", prefix, err)
	}
}

func TestStartStop(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	// Insert a test package
	pkg := &brew.Package{
		Name:        "test",
		Version:     "1.0.0",
		InstallType: "explicit",
		HasBinary:   false,
		BinaryPaths: []string{},
		InstalledAt: time.Now(),
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert test package: %v", err)
	}

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Stop before start should work gracefully
	if err := w.Stop(); err != nil {
		t.Errorf("Stop() before Start() error = %v, want nil", err)
	}
}
