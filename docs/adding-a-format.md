# Adding a format

A format is one package under `internal/formats/<name>/` that implements `core.Format`:

```go
type Format interface {
	Name() string
	Description() string
	Detect(head []byte) bool
	Conditions() []string
	Configure(fs *flag.FlagSet) Squeezer
}
```

- `Name` is the subcommand: lowercase letters, digits and hyphens, starting with a letter.
- `Description` is one line for `squeeze -h`.
- `Detect` gets the first 64 KiB of the input and answers from markers the tool always prints. When no format detects a longer input that starts with `{` or `[`, the registry calls `Detect` again with the whole input. A JSON format then finds its keys past a large preamble, such as a plan's `variables`. `Detect` must reject the input of every other registered format. The registry's fallback, `generic`, is the exception: it detects any text and runs only when no other format detects the input.
- `Conditions` lists, sorted, every condition name the format can report for `--fail-on`: lowercase words joined by `_`.
  - Each name needs a row in the format's docs page and a fixture that produces it; `formattest.Contract` fails for a declared condition no fixture produces.
  - A format whose tool reports a verdict declares `fail`, present when the verdict is a failure; the contract checks it.
  - It declares `error` when the tool reports errors, and `warning` when the tool reports warnings, including log lines the format turns into diagnostics.
  - A format for output with no verdict, errors or warnings, such as `terraform output -json`, declares no condition.
  - The CLI accepts these three names for every format and treats an undeclared one as never present, so `--fail-on error,fail` works on any pipe.
  - The other names are the format's own. `fixable` means a remediation is known: the tool can apply it (tflint) or a fixed version exists (trivy).
- `Configure` registers the format's own flags on `fs` and returns the function that squeezes the input. Auto-detection calls it after detection with the flags the command line gave for it, often none, so the defaults must suit an agent that passes no flags. A flag name means the same kind of value in every format that has it (a boolean in one and a count in another fails `TestFormatFlagNamesAgreeOnKind`).

The returned `core.Result` holds three fields. `Text` is the output for the agent. `Data` is any JSON-serializable value that `--output json` places under `data`. `Conditions` is the sorted names present in this input; `core.Conditions` builds that slice from a map. A format whose tool reports diagnostics with file and line uses `internal/tfdiag` or a package like it, so diagnostics read the same across formats.

## Test layout

Each case is a directory under `internal/formats/<name>/testdata/`:

```
testdata/<case>/input.<ext>   the tool's output; a symlink to another case's input is fine
testdata/<case>/args          optional: format flags for this case, separated by whitespace
testdata/<case>/want.txt      golden text output, written by `just golden`
testdata/<case>/want.json     golden --output json envelope, written by `just golden`
```

A flag variant is a new case with an `args` file and a symlinked input, so every flag combination that matters has a golden file.

## Checklist

1. Create `internal/formats/<name>/` with the format and its unit tests. The package exports only `Format`; its files follow `format.go` (the `core.Format` methods and `squeeze`), `parse.go` (raw input types named `raw<Thing>`), `build.go` (the report that becomes `data`) and `text.go`. A string with a fixed set of values is a named type with constants. Use `github.com/google/go-cmp` for structural comparisons in tests.
2. Add one case directory per scenario, from the tool's real output or its published schema. Never use private or employer material, and never copy test data from a source whose license is not compatible with MIT.
3. Add a contract test; it checks detection, determinism, plain text, the cut-notice shape (`... N <what> (<flag>)`), that every declared condition appears in some case, the JSON envelope (one line of plain text, like the text output) and the golden files of every case:

   ```go
   func TestContract(t *testing.T) {
   	formattest.Contract(t, Format{}, "testdata")
   }
   ```

4. Add `FuzzSqueeze`, seeded with `formattest.Inputs`; `just fuzz-all` fails for a format without it. Assert properties in the fuzz body, not only the absence of a crash. Add `TestRealInputs` with `formattest.Real` so private output under `testdata/real/` is exercised.
5. If the tool marks secrets, add a test that extracts every marked value from the input and asserts none appears in the text or the envelope. Add a test that feeds names and messages holding newlines and escape sequences and asserts the output has no forged line (the guarantee in `docs/cli.md`), and a `TestDetect` that rejects the inputs of the sibling formats.
6. Register the format in `DefaultRegistry` in `internal/cli/cli.go`. Order matters only when two formats could detect the same input; a test fails when a case is detected by any other format. `WithFallback` registers the fallback, which the registry tries last; a registry has at most one. The CLI tests build their format lists from the registry, so nothing else to edit there.
7. Run `just golden`, read every file it wrote, then `just check` and `just cover`.
8. Add `docs/formats/<name>.md` and a row in the README's format table, in registry order.
   - The page reads, in order: an `Input:` line; `## Agent line`, the line a project pastes into its `AGENTS.md`, with the exact command and any caveat; `## Example`, with what the format saves and a `Detection:` bullet; then `## Output`, `## Flags`, `## Conditions for --fail-on`, `## JSON data` and `## Limitations`.
   - A test in `internal/cli` compares the flags table with `squeeze <name> -h` and the conditions table with `Conditions()`.
9. `data` always holds `tool` and `tool_version`, empty when the tool prints none. The noun in `--max-<items>`, in the cut notice and in the `<items>_omitted` key is the same word; a breakdown of the cut items is `omitted_by_<dimension>`. Use the shared helpers instead of local copies:
   - `internal/core`: `CountFlag`, `Plural`, `Limit`, `LimitTail`, `Truncate`, `Ellipsis`, `ToolLabel`, `Seconds`, `DecodeJSON`, `HasJSONPrefix`, `NoticeName`, `Dedent`, `SplitLines`, `TrimRoot`, `Block` (`HeadBlock`, `TailBlock`), `JoinCounts`, `UnknownNotice`, `SortedCounts`, `TopCounts`, `SeverityOrder` and `SharedConditions()`.
   - `internal/sanitize`: `Clean` for a single-line value, `Line` and `NarrowLine` for a line of tool output.
   - `internal/logscan`: `ErrorLine`, `WarningLine`, `FlaggedLines` for error and warning lines after a JSON document, `LeadingLines` for the ones before it, and `TailLines`.
   - `internal/tfdiag` for diagnostics with file and line: `Convert`, `FromLines`, `Normalize`, `Write`, `CountWords`, `CheckFormatVersion`.
   - `internal/jsonl` for line-delimited JSON input. Flags that mean the same thing keep the same name across formats: `--max-<items>`, `--full`, `--all`.

Commit a `testdata/fuzz/` corpus entry only for an input the fuzzer found that crashed or broke an invariant; the seeds come from the fixtures. Real tool output that must stay private goes in `internal/formats/<name>/testdata/real/`, which git ignores. `formattest.Real` runs over every file there that the format detects, or over the directory named by `SQUEEZE_REAL_DIR`. An empty `testdata/real/` skips the test; a `SQUEEZE_REAL_DIR` with no input the format detects fails it, so `just conformance` cannot pass on a corpus that moved.
