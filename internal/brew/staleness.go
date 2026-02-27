package brew

import (
	"bytes"
	"os/exec"
	"strings"
)

// CheckStaleness compares dbPackageNames against the current brew list --formula
// output. Returns the count of formulae present in brew list but absent from the DB.
// Returns (0, nil) silently if brew is not on PATH or the command fails — callers
// must not treat this as an error.
func CheckStaleness(dbPackageNames []string) (int, error) {
	cmd := exec.Command("brew", "list", "--formula")
	out, err := cmd.Output()
	if err != nil {
		// brew not found or failed — degrade silently
		return 0, nil
	}

	// Build a set of known DB package names for O(1) lookup
	known := make(map[string]struct{}, len(dbPackageNames))
	for _, name := range dbPackageNames {
		known[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}

	newCount := 0
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		name := strings.ToLower(strings.TrimSpace(string(line)))
		if name == "" {
			continue
		}
		if _, found := known[name]; !found {
			newCount++
		}
	}

	return newCount, nil
}
