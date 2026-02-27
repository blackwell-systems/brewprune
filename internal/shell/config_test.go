package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsurePathEntry_AlreadyOnPath verifies that when dir is already in PATH,
// EnsurePathEntry returns (false, "", nil) without modifying any config file.
func TestEnsurePathEntry_AlreadyOnPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Put tmpDir onto PATH.
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })
	os.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+original)

	added, configFile, err := EnsurePathEntry(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added {
		t.Errorf("expected added=false, got true")
	}
	if configFile != "" {
		t.Errorf("expected configFile=\"\", got %q", configFile)
	}
}

// TestEnsurePathEntry_AppendsToProfile verifies that when the dir is not on
// PATH, EnsurePathEntry appends the export line to the shell config file.
func TestEnsurePathEntry_AppendsToProfile(t *testing.T) {
	tmpDir := t.TempDir()
	shimDir := filepath.Join(tmpDir, "shims")

	// Set HOME so config file resolves inside tmpDir, SHELL to default profile.
	t.Setenv("HOME", tmpDir)
	t.Setenv("SHELL", "/bin/sh")

	// Remove shimDir from PATH to ensure it's not already there.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "/usr/bin:/bin")

	// Pre-create a .profile with existing content.
	profilePath := filepath.Join(tmpDir, ".profile")
	existingContent := "# existing content\n"
	if err := os.WriteFile(profilePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to pre-create .profile: %v", err)
	}

	added, configFile, err := EnsurePathEntry(shimDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Errorf("expected added=true, got false")
	}
	if configFile != profilePath {
		t.Errorf("expected configFile=%q, got %q", profilePath, configFile)
	}

	// Verify the file still contains the original content (not overwritten).
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("failed to read .profile: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, existingContent) {
		t.Errorf("existing content was overwritten; got:\n%s", content)
	}

	// Verify the export line was appended.
	if !strings.Contains(content, "brewprune shims") {
		t.Errorf("expected '# brewprune shims' in .profile; got:\n%s", content)
	}
	if !strings.Contains(content, shimDir) {
		t.Errorf("expected shim dir %q in .profile; got:\n%s", shimDir, content)
	}
	if !strings.Contains(content, "export PATH") {
		t.Errorf("expected 'export PATH' in .profile; got:\n%s", content)
	}
}

// TestEnsurePathEntry_CreatesFileIfMissing verifies that EnsurePathEntry creates
// the config file when it does not already exist.
func TestEnsurePathEntry_CreatesFileIfMissing(t *testing.T) {
	tmpDir := t.TempDir()
	shimDir := filepath.Join(tmpDir, "shims")

	t.Setenv("HOME", tmpDir)
	t.Setenv("SHELL", "/bin/sh")

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "/usr/bin:/bin")

	profilePath := filepath.Join(tmpDir, ".profile")

	// Ensure the file does not exist yet.
	if _, err := os.Stat(profilePath); err == nil {
		t.Fatalf(".profile already exists at %s; test setup error", profilePath)
	}

	added, configFile, err := EnsurePathEntry(shimDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Errorf("expected added=true, got false")
	}
	if configFile != profilePath {
		t.Errorf("expected configFile=%q, got %q", profilePath, configFile)
	}

	// File should now exist.
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Errorf("expected config file %s to be created, but it doesn't exist", profilePath)
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("failed to read created .profile: %v", err)
	}
	if !strings.Contains(string(data), shimDir) {
		t.Errorf("expected shim dir %q in created .profile; got:\n%s", shimDir, string(data))
	}
}

// TestEnsurePathEntry_ZshWritesToZprofile verifies that SHELL=/bin/zsh causes
// EnsurePathEntry to write to ~/.zprofile.
func TestEnsurePathEntry_ZshWritesToZprofile(t *testing.T) {
	tmpDir := t.TempDir()
	shimDir := filepath.Join(tmpDir, "shims")

	t.Setenv("HOME", tmpDir)
	t.Setenv("SHELL", "/bin/zsh")

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "/usr/bin:/bin")

	added, configFile, err := EnsurePathEntry(shimDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Errorf("expected added=true, got false")
	}

	expectedConfig := filepath.Join(tmpDir, ".zprofile")
	if configFile != expectedConfig {
		t.Errorf("expected configFile=%q, got %q", expectedConfig, configFile)
	}

	data, err := os.ReadFile(expectedConfig)
	if err != nil {
		t.Fatalf("failed to read ~/.zprofile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "export PATH") {
		t.Errorf("expected 'export PATH' in .zprofile; got:\n%s", content)
	}
	if !strings.Contains(content, shimDir) {
		t.Errorf("expected shim dir %q in .zprofile; got:\n%s", shimDir, content)
	}
}

// TestEnsurePathEntry_FishUsesFishAddPath verifies that SHELL=/usr/local/bin/fish
// causes EnsurePathEntry to write fish_add_path syntax to the fish config.
func TestEnsurePathEntry_FishUsesFishAddPath(t *testing.T) {
	tmpDir := t.TempDir()
	shimDir := filepath.Join(tmpDir, "shims")

	t.Setenv("HOME", tmpDir)
	t.Setenv("SHELL", "/usr/local/bin/fish")

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "/usr/bin:/bin")

	added, configFile, err := EnsurePathEntry(shimDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Errorf("expected added=true, got false")
	}

	expectedConfig := filepath.Join(tmpDir, ".config", "fish", "conf.d", "brewprune.fish")
	if configFile != expectedConfig {
		t.Errorf("expected configFile=%q, got %q", expectedConfig, configFile)
	}

	data, err := os.ReadFile(expectedConfig)
	if err != nil {
		t.Fatalf("failed to read fish config: %v", err)
	}
	content := string(data)

	// Fish should use fish_add_path, not export PATH.
	if strings.Contains(content, "export PATH") {
		t.Errorf("fish config should not contain 'export PATH'; got:\n%s", content)
	}
	if !strings.Contains(content, "fish_add_path") {
		t.Errorf("expected 'fish_add_path' in fish config; got:\n%s", content)
	}
	if !strings.Contains(content, shimDir) {
		t.Errorf("expected shim dir %q in fish config; got:\n%s", shimDir, content)
	}
}
