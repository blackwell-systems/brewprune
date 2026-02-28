package app

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestRunStatus_DaemonStoppedSuggestsWatchDaemon verifies that when the
// daemon is not running, the stopped tracking line suggests
// 'brewprune watch --daemon' (not 'brew services start brewprune').
func TestRunStatus_DaemonStoppedSuggestsWatchDaemon(t *testing.T) {
	// Use a temp HOME so getDefaultPIDFile() returns a path with no PID file,
	// making IsDaemonRunning return false (daemon stopped).
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Create a brewprune dir and a fake DB file so runStatus doesn't exit early.
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
	}
	fakeDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	f, err := os.Create(fakeDB)
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	f.Close()

	// Override global dbPath so runStatus uses our temp DB.
	origDBPath := dbPath
	dbPath = fakeDB
	defer func() { dbPath = origDBPath }()

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	if !strings.Contains(output, "watch --daemon") {
		t.Errorf("expected stopped tracking line to suggest 'watch --daemon', got output:\n%s", output)
	}
	if strings.Contains(output, "brew services start") {
		t.Errorf("expected stopped tracking line NOT to suggest 'brew services start', got output:\n%s", output)
	}
}
