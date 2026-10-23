# tflint

Input: the JSON that `tflint --format json` prints, as `formatter/json.go` in TFLint defines it: an `issues` array and an `errors` array.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Run TFLint as `tflint --format json | squeeze`.
```

## Example

```
tflint --format json | squeeze
tflint --recursive --format json | squeeze --fail-on error
```

TFLint's JSON repeats each rule's documentation link, the end of every range and the `fixable` and `fixed` flags of every issue.
squeeze keeps one line per issue with its location, rule and fix state.
On the default-flag fixtures with issues, captured from TFLint 0.64.0 with the bundled `terraform` ruleset, the text is 11 to 35 percent of the input.
A run that reports only one HCL error, or no issue, gains little, since there is little to remove.

- Detection: a JSON object that starts with `"issues"` and has an `"errors"` key anywhere in the document.

## Output

```
$ squeeze < internal/formats/tflint/testdata/issues/input.json
tflint: 6 issues, 4 warnings, 2 notices; 2 fixable
warning: `region` variable has no type (main.tf:5:1, terraform_typed_variables)
warning: variable "unused" is declared but not used (main.tf:9:1, terraform_unused_declarations, fixable)
warning: Missing version constraint for provider "aws" in `required_providers` (main.tf:18:1, terraform_required_providers)
warning: List items should be accessed using square brackets (main.tf:28:31, terraform_deprecated_index, fixable)
notice: `region` variable has no description (main.tf:5:1, terraform_documented_variables)
notice: resource name `LogsBucket` must match the following format: snake_case (main.tf:18:1, terraform_naming_convention)
```

- The header counts the issues by severity. TFLint's JSON calls the lowest level `info`; squeeze prints `notice`, the name TFLint's own text output and `--minimum-failure-severity` use. A severity other than `warning`, `info` or `notice` counts as an error, so an unexpected value cannot hide a failure.
- Each issue prints with `file:line:column` (`file:line` when the column is 0, in the call chain too), the rule name and, when set, `fixable` or `fixed`. The header counts the fixable issues; `tflint --fix` applies them.
- Issues are sorted errors first, then warnings, then notices, and by file, line and column. Identical issues print once with `(xN)`, marked `fixable` or `fixed` when any of them is.
- An issue TFLint raises through a module call is reported, as TFLint does, at the root-module expression that set the value; the module expressions it was evaluated in print under it, outermost first: `evaluated in main.tf:20:19 -> modules/app/main.tf:4:19`.
- Entries of the `errors` array, which TFLint reports when it cannot parse a file or a plugin fails, print first as diagnostics with `tflint` in place of a rule, are never cut by `--max-issues`, and the header says not every check ran. `--no-detail` drops their explanation along with the call chains.
- A line after the JSON document that reads as an error or a warning, such as `Error: Failed to load plugin` when stderr is piped too, prints as a diagnostic with `tflint` in place of a rule, without its `Error:` or `Warning:` prefix. An error line counts as a TFLint error; a warning line sets `warning`. Other lines after the document are dropped.
- The call chain of an issue and the explanation of a tool error are wrapped to 100 bytes and cut after three lines with `...`.
- Past `--max-issues`, the notice names the rules of the cut issues with their counts, so `... 116 more issues, terraform_naming_convention 60, terraform_unused_declarations 56 (--max-issues 0)` tells which rule to fix or disable first. Identical issues print once with `(xN)` and take one line of the limit; the notice and `issues_omitted` count every cut issue, N for an `(xN)` line.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-issues` | `50` | issues listed, `0` for all |
| `--no-detail` | off | print the issue, its location and rule only |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | an issue has severity `error`, TFLint reported an entry in `errors`, or an error line follows the JSON |
| `fail` | any issue or error: TFLint exits with 2 for an issue at any severity unless `--minimum-failure-severity` says otherwise |
| `fixable` | an issue is fixable with `tflint --fix` |
| `notice` | an issue has severity `info` |
| `warning` | an issue or an entry in `errors` has severity `warning`, or a warning line follows the JSON |

## JSON data

`data` holds `tool`, `tool_version` (empty: TFLint's JSON carries none), `errors`, `warnings`, `notices`, `tool_errors`, `fixable`, `fixed`, `rules` (`name`, `count` for every rule with an issue, most issues first), `problems` (the entries of TFLint's `errors` array and the error and warning lines after the JSON, as diagnostics), `issues`, `issues_omitted` and `omitted_by_rule` (`name`, `count` for the issues past `--max-issues`). Each issue has `severity`, `summary`, `rule`, `address` (the rule again, as every diagnostic carries one) and, when set, `detail` (the module chain), `file`, `line`, `column`, `count`, `fixable` and `fixed`.

## Limitations

- The rule documentation links are dropped from the text and the data; the rule name finds them.
- A call chain or tool error explanation longer than three lines of 100 bytes is cut in the text; `--output json` carries it whole in `detail`.
- Detection needs the input to start with `{"issues":` and to hold an `"errors":` key; a JSON document that starts the same way but has no `errors` array goes to `generic`.
- `fail` follows TFLint's default failure severity. A run with `--minimum-failure-severity=warning` that only reports notices exits with 0 from TFLint, while `--fail-on fail` still matches; use `--fail-on error,warning` there.
