package brew

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Test data: sample brew info --json=v2 --installed output
const mockBrewListJSON = `{
  "formulae": [
    {
      "name": "node",
      "full_name": "node",
      "tap": "homebrew/core",
      "version": "20.10.0",
      "installed": [{"version": "20.10.0", "installed_on_request": 1704067200}],
      "linked_keg": "20.10.0"
    },
    {
      "name": "git",
      "full_name": "git",
      "tap": "homebrew/core",
      "version": "2.43.0",
      "installed": [{"version": "2.43.0", "installed_on_request": 1703462400}]
    },
    {
      "name": "openssl@3",
      "full_name": "openssl@3",
      "tap": "homebrew/core",
      "version": "3.2.0",
      "installed": [{"version": "3.2.0"}]
    }
  ],
  "casks": [
    {
      "token": "visual-studio-code",
      "full_token": "visual-studio-code",
      "tap": "homebrew/cask",
      "version": "1.85.0",
      "installed_time": "2024-01-01T00:00:00Z"
    }
  ]
}`

// Test data: sample brew info --json=v2 output for a single package
const mockBrewInfoJSON = `{
  "formulae": [
    {
      "name": "node",
      "full_name": "node",
      "tap": "homebrew/core",
      "version": "20.10.0",
      "installed": [{"version": "20.10.0", "installed_on_request": 1704067200}],
      "linked_keg": "20.10.0"
    }
  ],
  "casks": []
}`

// Test data: sample brew deps --tree output
const mockBrewDepsTree = `node
├── icu4c
├── libnghttp2
└── openssl@3
    └── ca-certificates`

const mockBrewDepsTreeComplex = `postgresql@14
├── icu4c
├── krb5
│   ├── openssl@3
│   │   └── ca-certificates
│   └── ca-certificates
├── lz4
├── openssl@3
│   └── ca-certificates
└── readline`

const mockBrewDepsTreeNoDeps = `htop`

func TestParseDependencyTree(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string][]string
	}{
		{
			name:  "simple dependency tree",
			input: mockBrewDepsTree,
			expected: map[string][]string{
				"node":            {"icu4c", "libnghttp2", "openssl@3"},
				"icu4c":           {},
				"libnghttp2":      {},
				"openssl@3":       {"ca-certificates"},
				"ca-certificates": {},
			},
		},
		{
			name:  "complex dependency tree with shared deps",
			input: mockBrewDepsTreeComplex,
			expected: map[string][]string{
				"postgresql@14":   {"icu4c", "krb5", "lz4", "openssl@3", "readline"},
				"icu4c":           {},
				"krb5":            {"openssl@3", "ca-certificates"},
				"openssl@3":       {"ca-certificates"},
				"ca-certificates": {},
				"lz4":             {},
				"readline":        {},
			},
		},
		{
			name:  "package with no dependencies",
			input: mockBrewDepsTreeNoDeps,
			expected: map[string][]string{
				"htop": {},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDependencyTree(tt.input)
			if err != nil {
				t.Fatalf("parseDependencyTree() error = %v", err)
			}

			// Check if all expected keys exist
			for key, expectedDeps := range tt.expected {
				actualDeps, exists := result[key]
				if !exists {
					t.Errorf("expected key %s not found in result", key)
					continue
				}

				// Check if dependencies match (order may vary)
				if len(actualDeps) != len(expectedDeps) {
					t.Errorf("key %s: got %d deps, want %d deps", key, len(actualDeps), len(expectedDeps))
					t.Errorf("  got: %v", actualDeps)
					t.Errorf("  want: %v", expectedDeps)
					continue
				}

				// Convert to maps for order-independent comparison
				actualSet := make(map[string]bool)
				for _, dep := range actualDeps {
					actualSet[dep] = true
				}

				for _, expectedDep := range expectedDeps {
					if !actualSet[expectedDep] {
						t.Errorf("key %s: missing expected dependency %s", key, expectedDep)
					}
				}
			}

			// Check for unexpected keys
			for key := range result {
				if _, expected := tt.expected[key]; !expected {
					t.Errorf("unexpected key %s in result", key)
				}
			}
		})
	}
}

func TestParseDependencyTreeEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single line",
			input: `python@3.12`,
		},
		{
			name: "with blank lines",
			input: `node

├── icu4c
└── openssl@3`,
		},
		{
			name: "malformed tree characters",
			input: `node
→ icu4c
→ openssl@3`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDependencyTree(tt.input)
			if err != nil {
				t.Fatalf("parseDependencyTree() error = %v", err)
			}

			// Should at least parse without crashing
			if result == nil {
				t.Error("parseDependencyTree() returned nil")
			}
		})
	}
}

func TestBrewListJSONParsing(t *testing.T) {
	// This tests the JSON parsing logic without actually calling brew
	var listOutput brewListOutput
	err := unmarshalJSON([]byte(mockBrewListJSON), &listOutput)
	if err != nil {
		t.Fatalf("failed to unmarshal mock JSON: %v", err)
	}

	// Verify formulae
	if len(listOutput.Formulae) != 3 {
		t.Errorf("expected 3 formulae, got %d", len(listOutput.Formulae))
	}

	// Check first formula
	if listOutput.Formulae[0].Name != "node" {
		t.Errorf("expected first formula name to be 'node', got '%s'", listOutput.Formulae[0].Name)
	}
	if listOutput.Formulae[0].Version != "20.10.0" {
		t.Errorf("expected node version '20.10.0', got '%s'", listOutput.Formulae[0].Version)
	}
	if listOutput.Formulae[0].Tap != "homebrew/core" {
		t.Errorf("expected tap 'homebrew/core', got '%s'", listOutput.Formulae[0].Tap)
	}

	// Check versioned package
	if listOutput.Formulae[2].Name != "openssl@3" {
		t.Errorf("expected third formula name to be 'openssl@3', got '%s'", listOutput.Formulae[2].Name)
	}

	// Verify casks
	if len(listOutput.Casks) != 1 {
		t.Errorf("expected 1 cask, got %d", len(listOutput.Casks))
	}

	if listOutput.Casks[0].Token != "visual-studio-code" {
		t.Errorf("expected cask token 'visual-studio-code', got '%s'", listOutput.Casks[0].Token)
	}
}

func TestBrewInfoJSONParsing(t *testing.T) {
	var infoOutput brewInfoOutput
	err := unmarshalJSON([]byte(mockBrewInfoJSON), &infoOutput)
	if err != nil {
		t.Fatalf("failed to unmarshal mock JSON: %v", err)
	}

	if len(infoOutput.Formulae) != 1 {
		t.Errorf("expected 1 formula, got %d", len(infoOutput.Formulae))
	}

	if infoOutput.Formulae[0].Name != "node" {
		t.Errorf("expected formula name 'node', got '%s'", infoOutput.Formulae[0].Name)
	}

	if len(infoOutput.Formulae[0].Installed) != 1 {
		t.Errorf("expected 1 installed version, got %d", len(infoOutput.Formulae[0].Installed))
	}

	if infoOutput.Formulae[0].Installed[0].Version != "20.10.0" {
		t.Errorf("expected installed version '20.10.0', got '%s'", infoOutput.Formulae[0].Installed[0].Version)
	}
}

func TestBrewListJSONWithEmptyInstalled(t *testing.T) {
	mockJSON := `{
  "formulae": [
    {
      "name": "test-package",
      "full_name": "test-package",
      "tap": "homebrew/core",
      "version": "1.0.0",
      "installed": []
    }
  ],
  "casks": []
}`

	var listOutput brewListOutput
	err := unmarshalJSON([]byte(mockJSON), &listOutput)
	if err != nil {
		t.Fatalf("failed to unmarshal mock JSON: %v", err)
	}

	if len(listOutput.Formulae) != 1 {
		t.Errorf("expected 1 formula, got %d", len(listOutput.Formulae))
	}

	if len(listOutput.Formulae[0].Installed) != 0 {
		t.Errorf("expected 0 installed versions, got %d", len(listOutput.Formulae[0].Installed))
	}
}

func TestBrewListJSONWithMissingFields(t *testing.T) {
	mockJSON := `{
  "formulae": [
    {
      "name": "minimal-package",
      "version": "1.0.0"
    }
  ],
  "casks": []
}`

	var listOutput brewListOutput
	err := unmarshalJSON([]byte(mockJSON), &listOutput)
	if err != nil {
		t.Fatalf("failed to unmarshal mock JSON: %v", err)
	}

	if len(listOutput.Formulae) != 1 {
		t.Errorf("expected 1 formula, got %d", len(listOutput.Formulae))
	}

	formula := listOutput.Formulae[0]
	if formula.Name != "minimal-package" {
		t.Errorf("expected name 'minimal-package', got '%s'", formula.Name)
	}

	// Missing fields should have zero values
	if formula.Tap != "" {
		t.Errorf("expected empty tap, got '%s'", formula.Tap)
	}
	if formula.FullName != "" {
		t.Errorf("expected empty full_name, got '%s'", formula.FullName)
	}
}

func TestBrewListJSONWithMultipleTaps(t *testing.T) {
	mockJSON := `{
  "formulae": [
    {
      "name": "official",
      "tap": "homebrew/core",
      "version": "1.0.0"
    },
    {
      "name": "custom",
      "tap": "user/custom-tap",
      "version": "2.0.0"
    }
  ],
  "casks": []
}`

	var listOutput brewListOutput
	err := unmarshalJSON([]byte(mockJSON), &listOutput)
	if err != nil {
		t.Fatalf("failed to unmarshal mock JSON: %v", err)
	}

	if len(listOutput.Formulae) != 2 {
		t.Errorf("expected 2 formulae, got %d", len(listOutput.Formulae))
	}

	taps := make(map[string]string)
	for _, formula := range listOutput.Formulae {
		taps[formula.Name] = formula.Tap
	}

	if taps["official"] != "homebrew/core" {
		t.Errorf("expected official tap 'homebrew/core', got '%s'", taps["official"])
	}
	if taps["custom"] != "user/custom-tap" {
		t.Errorf("expected custom tap 'user/custom-tap', got '%s'", taps["custom"])
	}
}

func TestParseDependencyTreeWithDuplicates(t *testing.T) {
	// Tree where same dependency appears multiple times
	input := `app
├── libA
│   └── libCommon
├── libB
│   └── libCommon
└── libCommon`

	result, err := parseDependencyTree(input)
	if err != nil {
		t.Fatalf("parseDependencyTree() error = %v", err)
	}

	// app should have all three direct dependencies
	appDeps := result["app"]
	if len(appDeps) != 3 {
		t.Errorf("expected app to have 3 direct dependencies, got %d", len(appDeps))
	}

	// libCommon should appear in multiple places
	libADeps := result["libA"]
	if len(libADeps) != 1 || libADeps[0] != "libCommon" {
		t.Errorf("expected libA to depend on libCommon, got %v", libADeps)
	}

	libBDeps := result["libB"]
	if len(libBDeps) != 1 || libBDeps[0] != "libCommon" {
		t.Errorf("expected libB to depend on libCommon, got %v", libBDeps)
	}
}

// Helper function to unmarshal JSON (for testing without calling exec.Command)
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func TestParseDependencyTreeDeepNesting(t *testing.T) {
	input := `root
└── level1
    └── level2
        └── level3
            └── level4`

	result, err := parseDependencyTree(input)
	if err != nil {
		t.Fatalf("parseDependencyTree() error = %v", err)
	}

	// Check the chain
	if deps, ok := result["root"]; !ok || len(deps) != 1 || deps[0] != "level1" {
		t.Errorf("root -> level1 chain broken: %v", result["root"])
	}
	if deps, ok := result["level1"]; !ok || len(deps) != 1 || deps[0] != "level2" {
		t.Errorf("level1 -> level2 chain broken: %v", result["level1"])
	}
	if deps, ok := result["level2"]; !ok || len(deps) != 1 || deps[0] != "level3" {
		t.Errorf("level2 -> level3 chain broken: %v", result["level2"])
	}
	if deps, ok := result["level3"]; !ok || len(deps) != 1 || deps[0] != "level4" {
		t.Errorf("level3 -> level4 chain broken: %v", result["level3"])
	}
}

func TestParseDependencyTreeMultipleBranches(t *testing.T) {
	input := `app
├── libA
├── libB
├── libC
│   ├── libD
│   └── libE
└── libF`

	result, err := parseDependencyTree(input)
	if err != nil {
		t.Fatalf("parseDependencyTree() error = %v", err)
	}

	// app should have 4 direct dependencies
	appDeps := result["app"]
	if len(appDeps) != 4 {
		t.Errorf("expected app to have 4 direct dependencies, got %d: %v", len(appDeps), appDeps)
	}

	// libC should have 2 dependencies
	libCDeps := result["libC"]
	if len(libCDeps) != 2 {
		t.Errorf("expected libC to have 2 dependencies, got %d: %v", len(libCDeps), libCDeps)
	}

	// Verify libC's dependencies are libD and libE
	depSet := make(map[string]bool)
	for _, dep := range libCDeps {
		depSet[dep] = true
	}
	if !depSet["libD"] || !depSet["libE"] {
		t.Errorf("libC should depend on libD and libE, got %v", libCDeps)
	}
}

func TestBrewJSONArrayIndexing(t *testing.T) {
	// Test that we handle array indexing correctly
	mockJSON := `{
  "formulae": [
    {
      "name": "first",
      "version": "1.0.0",
      "installed": [
        {"version": "1.0.0", "installed_on_request": 1704067200},
        {"version": "0.9.0"}
      ]
    }
  ],
  "casks": []
}`

	var listOutput brewListOutput
	err := unmarshalJSON([]byte(mockJSON), &listOutput)
	if err != nil {
		t.Fatalf("failed to unmarshal mock JSON: %v", err)
	}

	// Should handle multiple installed versions
	if len(listOutput.Formulae[0].Installed) != 2 {
		t.Errorf("expected 2 installed versions, got %d", len(listOutput.Formulae[0].Installed))
	}

	// First installed version should be the one we use
	if listOutput.Formulae[0].Installed[0].Version != "1.0.0" {
		t.Errorf("expected first installed version '1.0.0', got '%s'", listOutput.Formulae[0].Installed[0].Version)
	}
}

func TestParseDependencyTreeWithSpecialCharacters(t *testing.T) {
	// Test package names with special characters (common in brew)
	input := `node@20
├── icu4c@74
├── libnghttp2
└── openssl@3.2
    └── ca-certificates`

	result, err := parseDependencyTree(input)
	if err != nil {
		t.Fatalf("parseDependencyTree() error = %v", err)
	}

	// Check that @ symbols are preserved in package names
	if deps, ok := result["node@20"]; !ok {
		t.Error("package name 'node@20' not found")
	} else if len(deps) != 3 {
		t.Errorf("expected node@20 to have 3 deps, got %d", len(deps))
	}

	// Check nested versioned package
	if deps, ok := result["openssl@3.2"]; !ok {
		t.Error("package name 'openssl@3.2' not found")
	} else if len(deps) != 1 || deps[0] != "ca-certificates" {
		t.Errorf("expected openssl@3.2 -> ca-certificates, got %v", deps)
	}
}

func TestEmptyDependencyTreeSections(t *testing.T) {
	input := `app


├── lib1
└── lib2`

	result, err := parseDependencyTree(input)
	if err != nil {
		t.Fatalf("parseDependencyTree() error = %v", err)
	}

	// Empty lines should be ignored
	if deps := result["app"]; len(deps) != 2 {
		t.Errorf("expected app to have 2 deps despite empty lines, got %d", len(deps))
	}
}

func TestCompareDepTreeResults(t *testing.T) {
	input := mockBrewDepsTree
	result1, err1 := parseDependencyTree(input)
	result2, err2 := parseDependencyTree(input)

	if err1 != nil || err2 != nil {
		t.Fatalf("parsing errors: %v, %v", err1, err2)
	}

	// Results should be consistent
	if !reflect.DeepEqual(result1, result2) {
		t.Error("parseDependencyTree should produce consistent results")
	}
}
