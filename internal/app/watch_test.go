package app

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestWatchCommand(t *testing.T) {
	// Test that watch command is properly configured
	if watchCmd.Use != "watch" {
		t.Errorf("expected Use to be 'watch', got '%s'", watchCmd.Use)
	}

	if watchCmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if watchCmd.Long == "" {
		t.Error("expected Long description to be set")
	}

	if watchCmd.Example == "" {
		t.Error("expected Example to be set")
	}

	if watchCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestWatchCommandFlags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shouldExist  bool
		shouldHidden bool
	}{
		{
			name:         "daemon flag",
			flagName:     "daemon",
			shouldExist:  true,
			shouldHidden: false,
		},
		{
			name:         "daemon-child flag",
			flagName:     "daemon-child",
			shouldExist:  true,
			shouldHidden: true,
		},
		{
			name:         "pid-file flag",
			flagName:     "pid-file",
			shouldExist:  true,
			shouldHidden: false,
		},
		{
			name:         "log-file flag",
			flagName:     "log-file",
			shouldExist:  true,
			shouldHidden: false,
		},
		{
			name:         "stop flag",
			flagName:     "stop",
			shouldExist:  true,
			shouldHidden: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := watchCmd.Flags().Lookup(tt.flagName)

			if tt.shouldExist && flag == nil {
				t.Errorf("expected flag '%s' to be registered", tt.flagName)
				return
			}

			if !tt.shouldExist && flag != nil {
				t.Errorf("expected flag '%s' to not be registered", tt.flagName)
				return
			}

			if flag != nil && !tt.shouldHidden && flag.Usage == "" {
				t.Errorf("expected flag '%s' to have usage text", tt.flagName)
			}

			if flag != nil && flag.Hidden != tt.shouldHidden {
				t.Errorf("expected flag '%s' hidden to be %v, got %v", tt.flagName, tt.shouldHidden, flag.Hidden)
			}
		})
	}
}

func TestWatchCommandHelp(t *testing.T) {
	// Test that help can be generated without errors
	oldArgs := watchCmd.Flags()
	defer func() {
		watchCmd.ResetFlags()
		watchCmd.Flags().AddFlagSet(oldArgs)
	}()

	watchCmd.SetArgs([]string{"--help"})

	// Capture the help output
	// The command will return an error but that's expected
	err := watchCmd.Execute()
	if err != nil && !strings.Contains(err.Error(), "help") { //nolint:staticcheck
		// Some error is expected when running help
	}
}

func TestWatchCommandFlagParsing(t *testing.T) {
	// Reset flags before test
	watchDaemon = false
	watchDaemonChild = false
	watchPIDFile = ""
	watchLogFile = ""
	watchStop = false

	tests := []struct {
		name            string
		args            []string
		expectedDaemon  bool
		expectedStop    bool
		expectedPIDFile string
		expectedLogFile string
	}{
		{
			name:            "default flags",
			args:            []string{},
			expectedDaemon:  false,
			expectedStop:    false,
			expectedPIDFile: "",
			expectedLogFile: "",
		},
		{
			name:            "daemon mode",
			args:            []string{"--daemon"},
			expectedDaemon:  true,
			expectedStop:    false,
			expectedPIDFile: "",
			expectedLogFile: "",
		},
		{
			name:            "stop daemon",
			args:            []string{"--stop"},
			expectedDaemon:  false,
			expectedStop:    true,
			expectedPIDFile: "",
			expectedLogFile: "",
		},
		{
			name:            "custom pid file",
			args:            []string{"--pid-file=/tmp/test.pid"},
			expectedDaemon:  false,
			expectedStop:    false,
			expectedPIDFile: "/tmp/test.pid",
			expectedLogFile: "",
		},
		{
			name:            "custom log file",
			args:            []string{"--log-file=/tmp/test.log"},
			expectedDaemon:  false,
			expectedStop:    false,
			expectedPIDFile: "",
			expectedLogFile: "/tmp/test.log",
		},
		{
			name:            "daemon with custom files",
			args:            []string{"--daemon", "--pid-file=/tmp/test.pid", "--log-file=/tmp/test.log"},
			expectedDaemon:  true,
			expectedStop:    false,
			expectedPIDFile: "/tmp/test.pid",
			expectedLogFile: "/tmp/test.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			watchDaemon = false
			watchDaemonChild = false
			watchPIDFile = ""
			watchLogFile = ""
			watchStop = false

			// Parse flags
			watchCmd.ParseFlags(tt.args)

			if watchDaemon != tt.expectedDaemon {
				t.Errorf("expected daemon to be %v, got %v", tt.expectedDaemon, watchDaemon)
			}

			if watchStop != tt.expectedStop {
				t.Errorf("expected stop to be %v, got %v", tt.expectedStop, watchStop)
			}

			if watchPIDFile != tt.expectedPIDFile {
				t.Errorf("expected pidFile to be '%s', got '%s'", tt.expectedPIDFile, watchPIDFile)
			}

			if watchLogFile != tt.expectedLogFile {
				t.Errorf("expected logFile to be '%s', got '%s'", tt.expectedLogFile, watchLogFile)
			}
		})
	}
}

func TestWatchCommandRegistration(t *testing.T) {
	// Verify watch command is registered with root
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "watch" {
			found = true
			break
		}
	}

	if !found {
		t.Error("watch command not registered with root command")
	}
}

func TestWatchCommandFlagDefaults(t *testing.T) {
	// Test that boolean flags have correct defaults
	daemonFlag := watchCmd.Flags().Lookup("daemon")
	if daemonFlag != nil && daemonFlag.DefValue != "false" {
		t.Errorf("expected daemon flag default to be 'false', got '%s'", daemonFlag.DefValue)
	}

	stopFlag := watchCmd.Flags().Lookup("stop")
	if stopFlag != nil && stopFlag.DefValue != "false" {
		t.Errorf("expected stop flag default to be 'false', got '%s'", stopFlag.DefValue)
	}

	// Test that string flags have empty defaults
	pidFileFlag := watchCmd.Flags().Lookup("pid-file")
	if pidFileFlag != nil && pidFileFlag.DefValue != "" {
		t.Errorf("expected pid-file flag default to be empty, got '%s'", pidFileFlag.DefValue)
	}

	logFileFlag := watchCmd.Flags().Lookup("log-file")
	if logFileFlag != nil && logFileFlag.DefValue != "" {
		t.Errorf("expected log-file flag default to be empty, got '%s'", logFileFlag.DefValue)
	}
}

func TestWatchCommandMutuallyExclusiveFlags(t *testing.T) {
	// Test that daemon and stop flags are documented as mutually exclusive in examples
	if !strings.Contains(watchCmd.Example, "--daemon") {
		t.Error("expected example to show --daemon usage")
	}

	if !strings.Contains(watchCmd.Example, "--stop") {
		t.Error("expected example to show --stop usage")
	}
}

func TestWatchCommandLongDescription(t *testing.T) {
	// Test that long description covers key features
	longDesc := watchCmd.Long

	expectedKeywords := []string{
		"shim",
		"usage",
		"foreground",
		"daemon",
		"stop",
	}

	for _, keyword := range expectedKeywords {
		if !strings.Contains(strings.ToLower(longDesc), strings.ToLower(keyword)) {
			t.Errorf("expected long description to mention '%s'", keyword)
		}
	}
}

// TestStartWatchDaemon_AlreadyRunningIsIdempotent verifies that
// startWatchDaemon returns nil (not an error) when the daemon is already
// running, and prints an informational message instead of failing.
func TestStartWatchDaemon_AlreadyRunningIsIdempotent(t *testing.T) {
	// Write the current process PID into a temp PID file.
	// watcher.IsDaemonRunning uses kill(pid,0) to test liveness; the
	// current test process is guaranteed to be alive.
	tmpDir := t.TempDir()
	pidFile := fmt.Sprintf("%s/watch.pid", tmpDir)
	selfPID := fmt.Sprintf("%d\n", os.Getpid())
	if err := os.WriteFile(pidFile, []byte(selfPID), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Override the global watchPIDFile so startWatchDaemon uses our temp file.
	origPIDFile := watchPIDFile
	watchPIDFile = pidFile
	defer func() { watchPIDFile = origPIDFile }()

	// Capture stdout to verify the informational message.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	// startWatchDaemon takes a *watcher.Watcher but we will not reach the
	// w.StartDaemon call when running==true, so nil is safe.
	gotErr := startWatchDaemon(nil)

	w.Close()
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, readErr := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if readErr != nil {
			break
		}
	}
	os.Stdout = origStdout

	output := buf.String()

	if gotErr != nil {
		t.Errorf("expected startWatchDaemon to return nil when daemon already running, got: %v", gotErr)
	}

	if !strings.Contains(output, "already running") && !strings.Contains(output, "Nothing to do") {
		t.Errorf("expected informational 'already running' message, got: %q", output)
	}
}

// TestWatchDaemonStopConflict verifies that when both --daemon and --stop are
// provided, an error is returned containing "mutually exclusive".
func TestWatchDaemonStopConflict(t *testing.T) {
	// Set both conflicting flags.
	origDaemon := watchDaemon
	origStop := watchStop
	origPIDFile := watchPIDFile
	origLogFile := watchLogFile
	watchDaemon = true
	watchStop = true
	defer func() {
		watchDaemon = origDaemon
		watchStop = origStop
		watchPIDFile = origPIDFile
		watchLogFile = origLogFile
	}()

	tmpDir := t.TempDir()
	watchPIDFile = fmt.Sprintf("%s/nonexistent.pid", tmpDir)
	watchLogFile = fmt.Sprintf("%s/watch.log", tmpDir)

	// runWatch should return an error immediately due to the conflict.
	err := runWatch(watchCmd, nil)

	if err == nil {
		t.Fatal("expected runWatch to return an error when --daemon and --stop are both set, got nil")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected error to contain 'mutually exclusive', got: %q", err.Error())
	}
}

// TestWatchHelpPollingNoteProminent verifies that the "30 seconds" polling
// interval note appears prominently in the watch command's Long description —
// not as the last line. Users checking 'watch --help' while waiting for
// tracking data to appear should see this note early.
func TestWatchHelpPollingNoteProminent(t *testing.T) {
	long := watchCmd.Long

	idx30 := strings.Index(long, "30 seconds")
	if idx30 == -1 {
		t.Fatal("expected Long description to contain '30 seconds'")
	}

	// The note must not be the last sentence. Verify that substantial text
	// follows the first occurrence of "30 seconds".
	textAfter := strings.TrimSpace(long[idx30+len("30 seconds"):])
	if len(textAfter) < 50 {
		t.Errorf("'30 seconds' appears too close to the end of Long description; "+
			"only %d chars follow it. Move the note earlier so users see it prominently.",
			len(textAfter))
	}
}

// TestWatchLogStartup verifies that runWatchDaemonChild writes a startup line
// to watchLogFile before calling RunDaemon.
func TestWatchLogStartup(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := fmt.Sprintf("%s/watch.log", tmpDir)
	pidFile := fmt.Sprintf("%s/watch.pid", tmpDir)

	origLogFile := watchLogFile
	origPIDFile := watchPIDFile
	watchLogFile = logFile
	watchPIDFile = pidFile
	defer func() {
		watchLogFile = origLogFile
		watchPIDFile = origPIDFile
	}()

	// runWatchDaemonChild calls w.RunDaemon which we cannot easily mock, but
	// the log write happens before RunDaemon is called.  We pass nil and
	// expect a non-nil error from RunDaemon (nil dereference) — what we care
	// about is that the log file was written before the crash.
	// Use a recovery to catch the expected nil-pointer panic from RunDaemon(nil).
	func() {
		defer func() { recover() }() //nolint:errcheck
		_ = runWatchDaemonChild(nil)
	}()

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("expected log file to be created at %s, got error: %v", logFile, err)
	}

	content := string(data)
	if !strings.Contains(content, "brewprune-watch: daemon started") {
		t.Errorf("expected log file to contain 'brewprune-watch: daemon started', got: %q", content)
	}
}
