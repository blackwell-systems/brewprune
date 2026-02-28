package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAliases_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadAliases(dir)
	if err != nil {
		t.Fatalf("LoadAliases() returned error for missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadAliases() returned nil config")
	}
	if len(cfg.Aliases) != 0 {
		t.Errorf("expected empty Aliases map, got %v", cfg.Aliases)
	}
}

func TestLoadAliases_CommentsAndBlankLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	content := `# this is a comment
# another comment


# inline comment line
ll=eza
`
	if err := os.WriteFile(filepath.Join(dir, "aliases"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadAliases(dir)
	if err != nil {
		t.Fatalf("LoadAliases() error: %v", err)
	}
	if len(cfg.Aliases) != 1 {
		t.Errorf("expected 1 alias, got %d: %v", len(cfg.Aliases), cfg.Aliases)
	}
	if got := cfg.Aliases["ll"]; got != "eza" {
		t.Errorf("Aliases[\"ll\"] = %q, want %q", got, "eza")
	}
}

func TestLoadAliases_ValidLines(t *testing.T) {
	dir := t.TempDir()
	content := "rg=ripgrep\ncat=bat\n"
	if err := os.WriteFile(filepath.Join(dir, "aliases"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadAliases(dir)
	if err != nil {
		t.Fatalf("LoadAliases() error: %v", err)
	}

	tests := []struct {
		alias string
		pkg   string
	}{
		{"rg", "ripgrep"},
		{"cat", "bat"},
	}
	for _, tt := range tests {
		if got := cfg.Aliases[tt.alias]; got != tt.pkg {
			t.Errorf("Aliases[%q] = %q, want %q", tt.alias, got, tt.pkg)
		}
	}
}

func TestLoadAliases_InvalidLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	// Mix of valid and invalid lines.
	content := `noequalssign
=missingalias
validalias=validpkg
 =
another=good
`
	if err := os.WriteFile(filepath.Join(dir, "aliases"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadAliases(dir)
	if err != nil {
		t.Fatalf("LoadAliases() error: %v", err)
	}
	if len(cfg.Aliases) != 2 {
		t.Errorf("expected 2 aliases (only valid lines), got %d: %v", len(cfg.Aliases), cfg.Aliases)
	}
	if got := cfg.Aliases["validalias"]; got != "validpkg" {
		t.Errorf("Aliases[\"validalias\"] = %q, want %q", got, "validpkg")
	}
	if got := cfg.Aliases["another"]; got != "good" {
		t.Errorf("Aliases[\"another\"] = %q, want %q", got, "good")
	}
}

func TestLoadAliases_MultipleAliases(t *testing.T) {
	dir := t.TempDir()
	content := `# brewprune alias config
# Format: alias=package
ll=eza
rg=ripgrep
cat=bat
vi=vim
`
	if err := os.WriteFile(filepath.Join(dir, "aliases"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadAliases(dir)
	if err != nil {
		t.Fatalf("LoadAliases() error: %v", err)
	}
	if len(cfg.Aliases) != 4 {
		t.Errorf("expected 4 aliases, got %d: %v", len(cfg.Aliases), cfg.Aliases)
	}

	expected := map[string]string{
		"ll":  "eza",
		"rg":  "ripgrep",
		"cat": "bat",
		"vi":  "vim",
	}
	for alias, pkg := range expected {
		if got := cfg.Aliases[alias]; got != pkg {
			t.Errorf("Aliases[%q] = %q, want %q", alias, got, pkg)
		}
	}
}
