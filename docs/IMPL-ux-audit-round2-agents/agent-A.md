# Wave 1 Agent A: root.go — no-arg output, quickstart in help, subcommand suggestion

You are Wave 1 Agent A. Fix three UX issues in the root command.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/root.go` — modify
- `internal/app/root_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

Existing cobra and standard library only. `RootCmd` is already defined.

## 4. What to Implement

Read `internal/app/root.go` first.

Fix these three findings from `docs/cold-start-audit.md`:

### Finding 1: `brewprune` with no args exits 0 showing minimal output

The `RunE` function for the root command currently exits 0 with a 3-line tip
block. The audit found this is too minimal — new users don't know what the tool
does or what commands are available.

**Fix:** Change `RunE` to return `cmd.Help()` instead of the custom tip block.
This prints the full `--help` output (usage, subcommands, flags) and returns
nil (exit 0). Remove the if/else block that checks for the database file.

The new RunE should be simply:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    return cmd.Help()
},
```

### Finding 2: Quick Start section in `--help` omits `brewprune quickstart`

The `Long:` string in `RootCmd` shows a 4-step manual Quick Start but never
mentions `brewprune quickstart`, which automates all four steps.

**Fix:** Add `brewprune quickstart` as the FIRST option in the Quick Start
section, above the manual steps:

```
Quick Start:
  brewprune quickstart         # Recommended: automated setup in one command

  Or manually:
  1. brewprune scan
  2. brewprune watch --daemon  # Keep this running!
  3. Wait 1-2 weeks for usage data
  4. brewprune unused --tier safe
```

### Finding 3 (Edge Case): Unknown subcommand gives no pointer to `--help`

Cobra already has `SuggestionsMinimumDistance = 2` set (which enables "did you
mean?" for close typos). However, for completely unrecognized subcommands with
no close match (like `blorp`), cobra only says:
`Error: unknown command "blorp" for "brewprune"`

**Fix:** Add a `RunE`-adjacent error handler by implementing a
`PersistentPreRunE` on `RootCmd` that does nothing, but set cobra's
`SilenceErrors = false` so cobra's default error handler appends a usage hint.

Actually the better approach: Register a custom template or set
`RootCmd.SetUsageTemplate` — but that's complex.

Simpler: Add a cobra annotation. Actually the simplest fix is to override cobra's
error handling via `RootCmd.SetFlagErrorFunc` or just accept the current behavior
and add a `Use:` that includes a help hint in the `Long` description.

**Actually simplest:** Cobra already suggests similar commands. For `blorp`
with no close match, add explicit help at the end of the `Long:` text:
```
Run 'brewprune --help' for a list of available commands.
```
This appears in the Long description so users see it before trying anything.

But even better: Cobra lets you set a custom error output. The real fix is:
In `Execute()` in `root.go`, wrap the error to append a hint:

```go
func Execute() error {
    err := RootCmd.Execute()
    if err != nil {
        // If the error is "unknown command", cobra already printed it.
        // Add a usage hint to stderr.
        if strings.Contains(err.Error(), "unknown command") {
            fmt.Fprintf(os.Stderr, "Run 'brewprune --help' for a list of available commands.\n")
        }
    }
    return err
}
```

This requires adding `"os"` and `"strings"` to imports if not present. Check
existing imports in root.go first.

## 5. Tests to Write

Update `internal/app/root_test.go`:

1. `TestRootCmd_BareInvocationShowsHelp` — verify that running the root command
   with no args calls `cmd.Help()` (outputs something containing "Usage:" and
   subcommand names, exits 0). The existing test `TestRootCmd_BareInvocationPrintsHint`
   must be updated or replaced to match the new behavior.
2. `TestRootCommandHelp_QuickstartMentioned` — verify that `brewprune --help`
   output contains the string "quickstart".
3. `TestExecute_UnknownCommandHelpHint` — verify that running an unknown
   subcommand causes `Execute()` to append the help hint message to stderr.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestRoot|TestExecute" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- Do NOT change `cmd/brewprune/main.go` — that's out of scope.
- Do not modify any subcommand's `RunE` — only root.go.
- The `RunE: cmd.Help()` approach means the tip block (Tip: Run 'brewprune status'...) is removed. That's intentional — the full help is better.
- Existing tests that check the old "Tip:" output must be updated to reflect
  the new `cmd.Help()` behavior.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent A — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
