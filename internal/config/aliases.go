// Package config provides configuration file parsing for brewprune.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Dir returns the brewprune config directory, respecting XDG_CONFIG_HOME.
// Defaults to ~/.config/brewprune if XDG_CONFIG_HOME is not set.
func Dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "brewprune"), nil
}

// AliasConfig holds the alias-to-package mappings declared by the user.
// Each key is the alias name (as invoked on the command line) and the value
// is the canonical Homebrew package name whose usage counter should be updated.
type AliasConfig struct {
	Aliases map[string]string
}

// LoadAliases reads the aliases file at {dir}/aliases and returns the parsed
// config. If the file does not exist, an empty config is returned without an
// error. Invalid or malformed lines are silently skipped.
func LoadAliases(dir string) (*AliasConfig, error) {
	cfg := &AliasConfig{
		Aliases: make(map[string]string),
	}

	path := filepath.Join(dir, "aliases")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Expect exactly one "=" separating alias from package.
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue // no "=" or "=" is first character — invalid, skip
		}

		alias := strings.TrimSpace(line[:idx])
		pkg := strings.TrimSpace(line[idx+1:])

		if alias == "" || pkg == "" {
			continue // either side is blank — invalid, skip
		}

		cfg.Aliases[alias] = pkg
	}

	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	return cfg, nil
}
