# sarif

Input: a SARIF 2.1.0 log (OASIS Static Analysis Results Interchange Format, https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html), the format that `trivy --format sarif`, `tflint --format sarif`, `checkov -o sarif`, `semgrep --sarif` and `golangci-lint` with a SARIF output write. Detected when the input is a JSON object with a `runs` key and either `"version": "2.1.0"` or a `sarif-schema-2.1.0` schema URI in the first 64 KiB.

```
trivy fs --format sarif . | squeeze
tflint --format sarif | squeeze --fail-on error
checkov -d . -o sarif --output-file-path out && squeeze < out/results_sarif.sarif
```

A SARIF log repeats each rule's full description, help text and links, and often the code around each result. squeeze keeps one line per result with its message, location and rule. On the fixtures captured from Trivy 0.75.0, TFLint 0.64.0 and Checkov 3.3.20, the text is 7.5 to 11.6 percent of the input; `generic` kept the rule of 3 of the 11 results of each Trivy log.

## Output

```
$ squeeze < internal/formats/sarif/testdata/spec/input.sarif
sarif: examplelint 1.4.0, 6 results (3 errors, 2 warnings, 1 note); 1 suppressed; 2 not failures: pass 1, review 1; 2 tool errors, not every check ran
error: execution was not successful (examplelint)
error: could not read src/vendor/big.js: file too large (src/vendor/big.js, examplelint)
warning: rule EX009 is unknown and was ignored (examplelint)
error: Hard-coded credential in source (/home/dev/shop/src/config files/db.js:4:7, EX001)
  The value is read from an environment variable in production.
error: Hard-coded credential under review (src/legacy.js:9, EX001)
error: Function is too long (240 lines) (src/order.js, EX002)
warning: no message (EX006)
warning: Function is too long (84 lines) (src/cart.js:10, EX002)
note: Prefer template literals (src/cart.js:12:3, EX003)
```

- The header names each run's tool (`tool.driver.name` and `version`, or `semanticVersion`) with its result counts by level.
- A result's level is its `level`, else the `defaultConfiguration.level` of its rule (found by `ruleIndex`, else by `ruleId`), else `warning` (SARIF 3.27.10). `error`, `warning` and `note` print as such; any other value counts as an error, so an unexpected value cannot hide a failure.
- A result whose `kind` is not `fail` (`pass`, `open`, `review`, `informational`, `notApplicable`) is not a failure: it is counted by kind in the header and not listed. A result with a suppression whose `status` is `accepted` or absent is counted as suppressed and not listed; a `rejected` suppression leaves it listed.
- Each result prints the first line of its message. When the message has more lines, as Trivy's do, the rule's `shortDescription` is the first line and the whole message the detail, wrapped to 100 bytes and cut after three lines. The location is the first `physicalLocation`: its URI without `file://` and percent-decoded, and the region's start line and column.
- `invocations` that report `executionSuccessful: false`, and tool execution or configuration notifications at level `error` or `warning` (the default), print first, are never cut by `--max-results`, and the header says not every check ran.
- Results are sorted errors first, then warnings, then notes, and by file, line and column. Identical results print once with `(xN)`. Past `--max-results`, the notice names the rules of the cut results with their counts, as `... 8 more results, AWS-0089 1, ... (--max-results 0)`.
- Code snippets (`region.snippet`, `contextRegion`), rule descriptions, help text, fixes, code flows and properties are dropped, so a snippet that holds a secret, as a secret scanner's may, is never printed. The message text is printed as the tool wrote it.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-results` | `50` | results listed, `0` for all |
| `--no-detail` | off | print the first line of each message, its location and rule only |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | a result has level `error`, or a tool reported an error notification or an unsuccessful execution |
| `fail` | `error` or `warning`: SARIF defines no exit code, so a result at level `error` or `warning` counts as a failure and a `note` does not |
| `note` | a result has level `note` |
| `warning` | a result has level `warning` |

## JSON data

`data` holds `tool` (the driver names of the runs, joined by `, `), `tool_version` (the driver version when the log has one run, else empty), `runs` (`tool`, `version`, `results`, `errors`, `warnings`, `notes` per run), `errors`, `warnings`, `notes`, `suppressed`, `other_kinds` (`name`, `count` of the results whose kind is not `fail`), `tool_errors`, `notifications` (as diagnostics), `rules` (`name`, `count` for every rule with a result, most results first), `results`, `results_omitted` and `omitted_by_rule` (`name`, `count` for the results past `--max-results`; `no rule` for a result without one). Each result has `severity`, `summary`, `rule`, `tool` and, when set, `detail` (the full message), `address` (the rule again, as every diagnostic carries one), `file`, `line`, `column` and `count`.

## Limitations

- Only the first location of a result is printed; related locations and code flows are dropped.
- A relative URI is printed as the log wrote it; `uriBaseId` and `originalUriBaseIds` are not resolved.
- Trivy, TFLint and Checkov also have their own JSON output, read by the `trivy`, `tflint` and `checkov` formats with more of each tool's data (fixed versions, fix state, severities); prefer it when the tool can write it.
