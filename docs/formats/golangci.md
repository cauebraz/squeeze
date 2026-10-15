# golangci

Input: the JSON that golangci-lint v2 writes with `--output.json.path stdout`, an object with `Issues` (each a `result.Issue`: `FromLinter`, `Text`, `Severity`, `SourceLines`, `Pos`, `SuggestedFixes`) and `Report` (`Warnings`, `Linters`, `Error`), followed by the per-linter statistics that `--show-stats` (on by default) prints to the same stdout. The structs are `JSONResult` in `pkg/printers/json.go`, `Issue` in `pkg/result/issue.go` and `Data` in `pkg/report/data.go` at golangci-lint v2.14.0. golangci-lint v1 writes the same object; its `Replacement` field (v1.62.2 and earlier) is read too. Detected when the input, ignoring whitespace, starts with `{"Issues":[` and holds `"FromLinter"` or `"Report"`.

```
golangci-lint run --output.json.path stdout ./... | squeeze
golangci-lint run --output.json.path stdout ./... | squeeze --fail-on error
```

golangci-lint's JSON carries the source lines of every issue, the byte edits of every suggested fix and the name of every linter it knows (122 in v2.14.0), enabled or not. squeeze keeps one line per issue with its location, linter and fix state. On the fixtures captured from golangci-lint 2.14.0, the text is 2 to 22 percent of the input, and 2 percent with `--max-issues 5` on 100 issues. golangci-lint's own text output of the `issues` fixture is 1,506 bytes against 1,298 for squeeze's; the gain is the counts, the cut and the compiler errors listed one per line.

golangci-lint also writes SARIF with `--output.sarif.path stdout`; the `sarif` format reads it, without the run warnings, the enabled linters and the fix state that only the JSON carries.

## Output

```
$ squeeze < internal/formats/golangci/testdata/issues/input.json
golangci-lint: 12 issues (12 errors) from errcheck 5, staticcheck 3, ineffassign 2, govet 1, unused 1; 6 linters enabled; 1 fixable; 1 run warning
warning: Found unknown linters in //nolint directives: errchek (runner/nolint_filter)
error: Error return value of `f.Write` is not checked (cart/cart.go:15:9, errcheck)
error: Error return value of `f.Close` is not checked (cart/cart.go:16:9, errcheck)
error: ineffectual assignment to total (cart/cart.go:20:2, ineffassign)
...
error: QF1004: could use strings.ReplaceAll instead (cart/cart.go:42:9, staticcheck, fixable)
```

- The header counts issues by severity and by linter, most issues first, and names how many linters `Report.Linters` marks enabled.
- An issue without a severity prints as `error`. golangci-lint leaves `Severity` empty unless the `severity` section of its configuration sets one, exits 1 on any issue whatever its severity (`pkg/commands/run.go:401`), and its checkstyle and SARIF printers write an empty severity as `error` (`pkg/printers/checkstyle.go:17`, `pkg/printers/sarif.go:16`). A configured severity prints as written, lowercased: `info: func unusedHelper is unused (...)`.
- `Report.Error` and each `Report.Warnings` entry print first, as run errors and warnings, with the warning's tag as its address; they are never cut. golangci-lint fills them from what it logs (`pkg/report/log.go:32`), and exits 7 when it logged an error and found no issue (`pkg/commands/run.go:478`).
- A `typecheck` issue is a compile error (`pkg/goanalysis/pkgerrors/parse.go:24`). When its text lists several compiler errors as `file:line:col: message` lines, as `go list` reports them (`: # example.com/shop/broken` followed by one line per error), each error becomes its own issue with its own location, and the package header line is dropped. A line that continues a compiler error joins it. `typecheck` issues print before the others and are never cut by `--max-issues`. Their split count can exceed the one golangci-lint prints.
- Each issue prints with `file:line:column`, its linter, and `fixable` when it carries `SuggestedFixes` or a v1 `Replacement`; `golangci-lint run --fix` applies those.
- Paths print as golangci-lint wrote them. golangci-lint v2 writes them relative to the configuration file by default (`run.relative-path-mode`), so a run with `-c ../cfg/x.yml` prints `../shop/cart/cart.go`.
- Issues are sorted errors first, then warnings, then other severities, and by file, line and column. Identical issues print once with `(xN)`.
- Past `--max-issues`, the notice names the linters of the cut issues with their counts: `... 95 more issues, unused 39, errcheck 28, ineffassign 16, staticcheck 12 (--max-issues 0)`.
- The statistics after the JSON (`12 issues:`, `* errcheck: 5`, `0 issues.`) are dropped. Any other line after the JSON prints as a run warning, or as a run error when it starts with `level=error`, `level=fatal` or `level=panic`.
- `SourceLines`, `Pos.Offset`, `LineRange`, `HunkPos`, the fix edits, `ExpectNoLint` and the linters that are not enabled are dropped.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-issues` | `50` | issues listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | an issue has severity `error` or none, or the run logged an error |
| `fail` | there is an issue or a run error: golangci-lint exits 1 or 7 |
| `fixable` | an issue carries `SuggestedFixes` or a `Replacement` |
| `warning` | an issue has severity `warning`, or the run logged a warning |

## JSON data

`data` holds `tool`, `tool_version` (empty: the JSON carries none), `total`, `errors`, `warnings`, `severities` and `linters` (`name`, `count`, most issues first), `enabled_linters`, `fixable`, `run_errors`, `run_warnings`, `run_diagnostics` (as diagnostics), `issues`, `issues_omitted` and `omitted_by_linter` (`name`, `count` for the issues past `--max-issues`; `no linter` for an issue without one). Each issue has `severity`, `summary`, `linter` and, when set, `address` (the linter again, as every diagnostic carries one), `file`, `line`, `column`, `count` and `fixable`.

## Limitations

- golangci-lint caps the issues itself before it writes them: 50 per linter (`--max-issues-per-linter`) and 3 with the same text (`--max-same-issues`). squeeze cannot count what golangci-lint dropped; run golangci-lint with `--max-issues-per-linter 0 --max-same-issues 0` to see them all.
- When a package does not compile, golangci-lint 2.14.0 reported only the `typecheck` issue for the whole run in the fixtures, not the other linters' issues in packages that compile.
- On a timeout or a linter that cannot run, golangci-lint writes no JSON and exits 3 or 4, with the reason on stderr; squeeze then gets an empty input and prints `no output: the command printed nothing`; golangci-lint's exit code and stderr say why.
- stderr merged with `2>&1` puts golangci-lint's log lines before the JSON, and the input is no longer detected; keep stderr apart. `Report` holds the same warnings and errors.
- The text output (`--output.text.path`, the default) is not read and goes to `generic`.
- `fail` follows golangci-lint's default `issues-exit-code` of 1; a run configured with another exit code still reports `fail` on any issue.
