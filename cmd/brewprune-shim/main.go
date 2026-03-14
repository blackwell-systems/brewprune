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
	"strings"
	"syscall"
	"time"
)

// shimVersion is set at build time via -ldflags "-X main.shimVersion=x.y.z"
var shimVersion = "dev"

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
// to once per calendar day via ~/.brewprune/shim.version.warned.
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
		// File absent means scan hasn't run yet — no warning before first scan.
		return
	}
	expected := strings.TrimSpace(string(data))
	if expected == "" || expected == shimVersion {
		return
	}

	// Versions differ — apply date-based rate limit before warning.
	today := time.Now().Format("2006-01-02")
	warnPath := filepath.Join(homeDir, ".brewprune", "shim.version.warned")

	if raw, err := os.ReadFile(warnPath); err == nil {
		if strings.TrimSpace(string(raw)) == today {
			return
		}
	}

	// Emit the warning.
	fmt.Fprintln(os.Stderr, "brewprune upgraded; run 'brewprune scan' to refresh shims (or 'brewprune doctor').")

	// Update the rate-limit file with today's date (best-effort).
	_ = os.WriteFile(warnPath, []byte(today+"\n"), 0600)
}

// isPromptGitCall detects if git is being called by a shell prompt daemon
// (gitstatusd from powerlevel10k, starship, etc.) to avoid inflating usage stats.
//
// Strategy: Filter git commands that are overwhelmingly used by prompts and
// rarely invoked directly by users. Be conservative - it's better to track
// a few extra prompt calls than miss real user git usage.
func isPromptGitCall() bool {
	// Check for GITSTATUS_DAEMON environment variable (powerlevel10k)
	if os.Getenv("GITSTATUS_DAEMON") != "" {
		return true
	}

	if len(os.Args) < 2 {
		// No arguments - track it (might be 'git' with no command)
		return false
	}

	// Build the full command line for pattern matching
	cmdLine := strings.Join(os.Args[1:], " ")

	// Filter patterns that indicate automated prompt queries
	// These combinations are virtually never typed by humans
	promptPatterns := []string{
		"status --porcelain",              // gitstatusd
		"status --porcelain=v2",           // gitstatusd v2
		"rev-parse --git-dir",             // check if in git repo
		"rev-parse --is-inside-work-tree", // check if in repo
		"rev-parse --show-toplevel",       // get repo root
		"symbolic-ref --short HEAD",       // get current branch
		"symbolic-ref -q HEAD",            // get current branch (quiet)
		"rev-parse --abbrev-ref HEAD",     // get current branch
		"rev-parse --short HEAD",          // get short commit hash
		"describe --tags --exact-match",   // get current tag
		"describe --contains --all",       // get branch/tag containing commit
		"status -uno",                     // status without untracked
		"diff --quiet --exit-code",        // check for changes (silent)
		"diff-index --quiet HEAD",         // check for uncommitted changes
	}

	for _, pattern := range promptPatterns {
		if strings.Contains(cmdLine, pattern) {
			return true
		}
	}

	return false
}

// logExecution appends a usage record to ~/.brewprune/usage.log.
// Failures are silently ignored so the user's command always proceeds.
func logExecution(cmdName string) {
	// Skip tracking git calls from shell prompt daemons (gitstatusd, etc.)
	// to avoid inflating git usage with thousands of prompt-driven git status checks.
	if cmdName == "git" && isPromptGitCall() {
		return
	}

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
// Tries /opt/homebrew (Apple Silicon), /usr/local (Intel), and
// /home/linuxbrew/.linuxbrew (Linux) as primary candidates, then falls back
// to any executable on PATH outside the shim directory (e.g. /bin/cat).
// Returns "" if the binary would resolve back to the shim itself (infinite exec loop guard).
func findRealBinary(name string) string {
	// Prevent infinite exec loop: brewprune-shim must never exec itself.
	if name == "brewprune-shim" {
		return ""
	}

	// Primary: stable Homebrew prefix paths.
	prefixes := []string{"/opt/homebrew", "/usr/local", "/home/linuxbrew/.linuxbrew"}
	for _, prefix := range prefixes {
		p := filepath.Join(prefix, "bin", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fallback: search PATH entries, skipping the shim directory to prevent loops.
	homeDir, _ := os.UserHomeDir()
	shimDir := filepath.Join(homeDir, ".brewprune", "bin")
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == shimDir {
			continue // skip — would resolve back to this shim
		}
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}

	return ""
}
