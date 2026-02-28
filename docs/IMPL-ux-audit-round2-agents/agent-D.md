# Wave 1 Agent D: remove.go — dual-flag help clarification + doubled error fix

You are Wave 1 Agent D. Fix two UX issues in remove.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/remove.go` — modify
- `internal/app/remove_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

Existing store and analyzer functions — no changes to those.

## 4. What to Implement

Read `internal/app/remove.go` first.

Fix these two findings from `docs/cold-start-audit.md`:

### Finding 1: Dual-flag interface is confusing (--safe/--medium/--risky AND --tier)

The `remove` command has both boolean shortcut flags (`--safe`, `--medium`,
`--risky`) and a `--tier` string flag. The help text doesn't explain the
relationship between them.

**Fix:** Update the `Long:` description on `removeCmd` to explicitly clarify
that `--safe` is a shortcut for `--tier safe` (and similarly for `--medium` and
`--risky`). Replace the existing "If no packages are specified..." section with:

```
If no packages are specified, removes packages based on tier:
  --tier safe     Remove only safe-tier packages (high confidence, no impact)
  --tier medium   Remove safe and medium-tier packages
  --tier medium --risky  Remove all unused packages (requires confirmation)

Tier shortcut flags (equivalent to --tier):
  --safe    same as --tier safe
  --medium  same as --tier medium
  --risky   same as --tier risky
```

Also update the `--tier` flag description in `init()`:
```go
removeCmd.Flags().StringVar(&removeTierFlag, "tier", "", "Remove packages of specified tier: safe, medium, risky (shortcut: --safe, --medium, --risky)")
```

### Finding 2: `remove nonexistent --dry-run` error message is doubled

In `runRemove`, when explicit packages are provided and `st.GetPackage(pkg)`
fails, the error is wrapped:
```go
return fmt.Errorf("package %s not found: %w", pkg, err)
```
The `err` from `store.GetPackage` already contains the message "package X not
found", so this produces "package X not found: package X not found".

**Fix:** Change the error format to NOT wrap the store error (the store error is
redundant):
```go
return fmt.Errorf("package %q not found", pkg)
```

This produces the clean message: `Error: package "nonexistent" not found`

## 5. Tests to Write

Update `internal/app/remove_test.go`:

1. `TestRemoveHelp_ExplainsTierShortcuts` — verify that `removeCmd`'s Long
   description or help output contains both "shortcut" (or "equivalent") and
   "--tier" to confirm the clarification is present.
2. `TestRunRemove_NotFoundError_NotDoubled` — verify that when a nonexistent
   package is specified, the error message contains "not found" exactly once
   (not twice). Read how existing remove tests invoke the command to follow the
   same pattern.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestRemove" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- Do NOT change the tier flag logic or `determineTier()` — only the help text
  and the error formatting.
- Do NOT modify `internal/store/` — if the store error message changes, that
  is out of scope.
- The `--risky` boolean flag in the Long description example is a bit confusing
  since it appears alongside `--tier medium`. Keep the example text accurate to
  the actual behavior: `--risky` removes all tiers. Fix the Long text to be
  accurate without changing the underlying flag behavior.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent D — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
