package analyzer

import "time"

// ConfidenceScore represents a package's removal confidence score.
type ConfidenceScore struct {
	Package     string
	Score       int       // 0-100
	Tier        string    // "safe", "medium", "risky"
	UsageScore  int       // 0-40 points
	DepsScore   int       // 0-30 points
	AgeScore    int       // 0-20 points
	TypeScore   int       // 0-10 points
	Reason      string    // Human-readable explanation
	IsCritical  bool      // True if package is a core dependency
	IsCask      bool      // True if package is a cask (GUI app)
	SizeBytes   int64     // Package size in bytes (for sorting)
	InstalledAt time.Time // Installation date (for sorting)
	Explanation ScoreExplanation
}

// ScoreExplanation provides detailed breakdown of score components.
type ScoreExplanation struct {
	UsageDetail string // "never observed execution" / "last used 45 days ago"
	DepsDetail  string // "no dependents" / "3 used dependents" / "1 unused dependent"
	AgeDetail   string // "installed 240 days ago"
	TypeDetail  string // "leaf package with binaries" / "library-only (low confidence)" / "core dependency"
}

// UsageStats represents usage statistics for a package.
type UsageStats struct {
	Package   string
	TotalUses int
	LastUsed  *time.Time
	FirstSeen time.Time
	DaysSince int    // Days since last used, -1 if never used
	Frequency string // "daily", "weekly", "monthly", "never"
}

// Recommendation represents a set of packages recommended for removal.
type Recommendation struct {
	Packages        []string
	TotalSize       int64
	Tier            string
	ExpectedSavings int64
}
