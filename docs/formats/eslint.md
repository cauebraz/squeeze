# eslint

Input: the JSON that `eslint -f json` prints, an array with one `LintResult` per file, or the object that `eslint -f json-with-metadata` prints, with that array under `results` and the working directory under `metadata.cwd` (https://eslint.org/docs/latest/use/formatters/, https://eslint.org/docs/latest/integrate/nodejs-api). Detected when the input, ignoring whitespace, starts with `[{"filePath":` or `{"results":[{"filePath":` and holds the keys `messages` and `errorCount`.

```
npx eslint -f json . | squeeze
npx eslint -f json-with-metadata . | squeeze --fail-on error
```

ESLint's JSON is one line that carries the whole source of every file with a problem, the end position of every problem and the edits of every fix and suggestion. squeeze keeps one line per problem with its location, rule and fix state. On the fixtures, captured from ESLint 10.12.0, the text is 9 to 23 percent of the input, and 1 percent with `--max-problems 3`.

## Output

```
$ squeeze < internal/formats/eslint/testdata/problems/input.json
eslint: 12 problems (7 errors, 5 warnings) in 3 of 5 files; 1 file not parsed; 6 fixable with --fix; 1 suppressed
error: Parsing error: Unexpected keyword 'return' (src/broken/parse.js:2:3)
error: 'count' is assigned a value but never used. (src/cart.js:3:7, no-unused-vars)
error: Expected '===' and instead saw '=='. (src/cart.js:5:20, eqeqeq)
error: Expected { after 'if' condition. (src/cart.js:5:29, curly, fixable)
...
warning: Unused eslint-disable directive (no problems were reported from 'eqeqeq'). (src/lib/format.js:6:3, fixable)
```

- The header counts problems as ESLint does, from each file's `errorCount`, `warningCount`, `fatalErrorCount`, `fixableErrorCount` and `fixableWarningCount`, and counts the messages that an `eslint-disable` comment suppressed (`suppressedMessages`).
- Severity `1` is a warning and `2` an error; any other value counts as an error, so an unexpected value cannot hide a failure.
- A file ESLint could not parse (`fatal: true`, no rule) prints first and is never cut by `--max-problems`.
- Each problem prints with `file:line:column`, its rule and `fixable` when the message carries a `fix`. A message without a rule, such as an unused `eslint-disable` directive, prints without one.
- Paths are relative to `metadata.cwd` with `json-with-metadata`, and otherwise to the deepest directory that holds every file in the results; `data.root` names it.
- Problems are sorted errors first, then warnings, and by file, line and column. Identical problems print once with `(xN)`.
- Past `--max-problems`, the notice names the rules of the cut problems with their counts: `... 117 more problems, no-var 40, eqeqeq 39, no-unused-vars 38 (--max-problems 0)`. An `(xN)` line takes one line of the limit; the notice and `problems_omitted` count every cut problem.
- The `source` and `output` of each file, the end positions, the fix edits, the suggestions and `usedDeprecatedRules` are dropped.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-problems` | `50` | problems listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | a problem has severity 2, including a file that could not be parsed |
| `fail` | `error`: ESLint exits with 1 when a problem is an error |
| `fixable` | a problem can be fixed with `eslint --fix` |
| `warning` | a problem has severity 1 |

## JSON data

`data` holds `tool`, `tool_version` (empty: ESLint's JSON carries none), `root`, `files`, `files_with_problems`, `errors`, `warnings`, `fatal`, `fixable`, `suppressed`, `rules` (`name`, `count` for every rule with a problem, most problems first), `parse_errors` (as diagnostics), `problems`, `problems_omitted` and `omitted_by_rule` (`name`, `count` for the problems past `--max-problems`; `no rule` for a problem without one). Each problem has `severity`, `summary`, `rule` and, when set, `address` (the rule again, as every diagnostic carries one), `file`, `line`, `column`, `count` and `fixable`.

## Limitations

- `fail` follows ESLint's default. A run with `--max-warnings` fails on warnings too; use `--fail-on error,warning` there.
- ESLint's default `stylish` text is not read and goes to `generic`; run ESLint with `-f json`.
- Paths are cut on `/`; Windows paths print as ESLint wrote them.
