// Command brewprune-shim is a lightweight execution interceptor.
// It is placed at ~/.brewprune/bin/brewprune-shim, with symlinks created for
// each tracked Homebrew binary (e.g. ~/.brewprune/bin/git -> brewprune-shim).
//
// When the user runs a shimmed command, this binary:
//  1. Logs the execution to ~/.brewprune/usage.log (best-effort, non-blocking)
//  2. Execs the real binary at the stable brew prefix path, replacing this process
//
// The shim must NOT import any internal brewprune packages — it is a standalone
// binary compiled and deployed separately from the main CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// shimVersion is the version embedded at build time via ldflags:
//
//	-X main.shimVersion={{.Version}}
//
// In development builds this remains "dev".
const shimVersion = "dev"

func main() {
	// Determine which command was invoked via the symlink name.
	cmdName := filepath.Base(os.Args[0])

	// Warn if this shim binary is stale relative to the installed brewprune version.
	// Best-effort: failures are silently ignored so the user's command always proceeds.
	checkShimVersion()

	// Log execution to ~/.brewprune/usage.log (best-effort: never fail the user's command).
	logExecution(cmdName)

	// Find the real binary at the stable brew prefix.
	realBin := findRealBinary(cmdName)
	if realBin == "" {
		fmt.Fprintf(os.Stderr, "brewprune-shim: cannot find real binary for %q in Homebrew prefix\n", cmdName)
		os.Exit(1)
	}

	// Replace this process with the real binary — no fork, zero overhead.
	if err := syscall.Exec(realBin, os.Args, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "brewprune-shim: exec %s failed: %v\n", realBin, err)
		os.Exit(1)
	}
}

// checkShimVersion compares this binary's embedded shimVersion against the
// expected version stored in ~/.brewprune/shim.version (written by brewprune
// scan). When they differ it emits a one-line warning to stderr, rate-limited
// to once per 24 hours via ~/.brewprune/shim.warn.
//
// All errors are silently swallowed — the version check must never prevent
// the user's command from running.
func checkShimVersion() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Read the expected version written by `brewprune scan`.
	versionPath := filepath.Join(homeDir, ".brewprune", "shim.version")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		// File absent means scan hasn't run with version support yet — no warning.
		return
	}
	expected := strings.TrimSpace(string(data))
	if expected == "" || expected == shimVersion {
		return
	}

	// Versions differ — apply 24-hour rate limit before warning.
	warnPath := filepath.Join(homeDir, ".brewprune", "shim.warn")
	const rateLimitSecs int64 = 86400

	if raw, err := os.ReadFile(warnPath); err == nil {
		if ts, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil {
			if time.Now().Unix()-ts < rateLimitSecs {
				return
			}
		}
	}

	// Emit the warning.
	fmt.Fprintln(os.Stderr, "brewprune upgraded; run 'brewprune scan' to refresh shims (or 'brewprune doctor').")

	// Update the rate-limit timestamp (best-effort).
	_ = os.WriteFile(warnPath, []byte(strconv.FormatInt(time.Now().Unix(), 10)+"\n"), 0600)
}

// logExecution appends a usage record to ~/.brewprune/usage.log.
// Failures are silently ignored so the user's command always proceeds.
func logExecution(cmdName string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	logPath := filepath.Join(homeDir, ".brewprune", "usage.log")

	// O_APPEND ensures atomic single-write semantics on POSIX filesystems.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	// Format: "<unix_nano>,<argv0>\n"
	// argv0 is the shim symlink path, e.g. /Users/alice/.brewprune/bin/git
	fmt.Fprintf(f, "%d,%s\n", time.Now().UnixNano(), os.Args[0])
}

// findRealBinary locates the actual Homebrew binary at the stable opt prefix.
// Tries /opt/homebrew (Apple Silicon) then /usr/local (Intel) as fallbacks.
// Returns "" if the binary would resolve back to the shim itself (infinite exec loop guard).
func findRealBinary(name string) string {
	// Prevent infinite exec loop: brewprune-shim must never exec itself.
	if name == "brewprune-shim" {
		return ""
	}
	prefixes := []string{"/opt/homebrew", "/usr/local"}
	for _, prefix := range prefixes {
		p := filepath.Join(prefix, "bin", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
