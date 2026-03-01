# IMPL Audit Round 7

### Agent E — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-e
commit: 933e878f155a8a7be016a9c2ca40c0ca6fccef70
files_changed:
  - internal/app/unused.go
  - internal/app/unused_test.go
  - internal/output/table.go
  - internal/output/table_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDoubleErrorPrefix_Fixed
  - TestReclaimableFooter_AllFlag
  - TestVerboseTipAppearsBeforeOutput
  - TestSortAge_InstalledColumn
  - TestReclaimableFooter_NoAllFlag
verification: PASS (go test ./internal/app ./internal/output — all tests pass)

#### Finding Notes

**Finding 14 (Double "Error: Error:"):** Removed inline "Error: " prefix from the
`fmt.Errorf` call on the `--all`/`--tier` conflict. Cobra prepends "Error: " automatically.

**Finding 13 (Reclaimable footer "(risky, hidden)" when --all):** Investigation showed
`RenderReclaimableFooter` logic was already correct — `showAll=true` suppresses ", hidden".
The call site in `unused.go` passes `unusedAll || unusedTier != ""` which is `true` when
`--all` is used. Added `TestReclaimableFooter_AllFlag` and `TestReclaimableFooter_NoAllFlag`
to confirm the behavior is correct and guarded by tests. Bug did NOT exist in production code.

**Finding 10 (Verbose tip before output):** Moved the pagination tip block before the
verbose table render. Added a TTY check (`os.ModeCharDevice`) so the tip only appears on
interactive terminals. The duplicate post-table tip was removed.

**Finding 9 (--sort age no Installed column):** Added `InstalledAt time.Time` field to
`output.ConfidenceScore`. In `unused.go`, populated `InstalledAt` from
`analyzer.ConfidenceScore.InstalledAt` when `unusedSort == "age"`. Modified
`RenderConfidenceTable` to detect any non-zero `InstalledAt` in the slice — when found,
replaces "Last Used" column header with "Installed" and formats dates as `2006-01-02`.
