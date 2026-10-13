# tflint

Input: the JSON that `tflint --format json` prints, as `formatter/json.go` in TFLint defines it: an object with an `issues` array and an `errors` array. Detected when the input is a JSON object that starts with `"issues"` and has an `"errors"` key in its first 64 KiB.

```
tflint --format json | squeeze
tflint --recursive --format json | squeeze --fail-on error
```

TFLint's JSON repeats each rule's documentation link, the end of every range and the `fixable` and `fixed` flags of every issue. squeeze keeps one line per issue with its location, rule and fix state. On the fixtures, captured from TFLint 0.64.0 with the bundled `terraform` ruleset, the text is 11 to 35 percent of the input; a run that only reports one HCL error, or no issue at all, gains little, since there is little to remove.

## Output

```
$ squeeze < internal/formats/tflint/testdata/issues/input.json
tflint: 6 issues: 4 warnings, 2 notices; 2 fixable with tflint --fix
warning: `region` variable has no type (main.tf:5:1, terraform_typed_variables)
warning: variable "unused" is declared but not used (main.tf:9:1, terraform_unused_declarations, fixable)
warning: Missing version constraint for provider "aws" in `required_providers` (main.tf:18:1, terraform_required_providers)
warning: List items should be accessed using square brackets (main.tf:28:31, terraform_deprecated_index, fixable)
notice: `region` variable has no description (main.tf:5:1, terraform_documented_variables)
notice: resource name `LogsBucket` must match the following format: snake_case (main.tf:18:1, terraform_naming_convention)
```

- The header counts the issues by severity. TFLint's JSON calls the lowest level `info`; squeeze prints `notice`, the name TFLint's own text output and `--minimum-failure-severity` use. A severity other than `warning`, `info` or `notice` counts as an error, so an unexpected value cannot hide a failure.
- Each issue prints with `file:line:column`, the rule name and, when set, `fixable` or `fixed`. The header counts the fixable issues and names `tflint --fix`.
- Issues are sorted errors first, then warnings, then notices, and by file, line and column. Identical issues print once with `(xN)`.
- An issue inside a module called from elsewhere prints the call chain under it: `called from main.tf:12:19 <- envs/prod/main.tf:3:10`.
- Entries of the `errors` array, which TFLint reports when it cannot parse a file or a plugin fails, print as diagnostics with `tflint` in place of a rule, and the header says TFLint did not finish every check.
- Past `--max-issues`, the notice names the rules of the cut issues with their counts, so `... 116 more issues, terraform_naming_convention 60, terraform_unused_declarations 56 (--max-issues 0)` tells which rule to fix or disable first.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-issues` | `50` | issues listed, `0` for all |
| `--no-detail` | off | print the issue, its location and rule only, without the module call chain |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | an issue has severity `error`, or TFLint reported an entry in `errors` |
| `fail` | any issue or error: TFLint exits with 2 for an issue at any severity unless `--minimum-failure-severity` says otherwise |
| `fixable` | an issue is fixable with `tflint --fix` |
| `notice` | an issue has severity `info` |
| `warning` | an issue has severity `warning` |

## JSON data

`data` holds `tool`, `errors`, `warnings`, `notices`, `tool_errors` (entries of TFLint's `errors` array), `fixable`, `fixed`, `rules` (`rule`, `count` for every rule with an issue, most issues first), `diagnostics`, `diagnostics_omitted` and `omitted_rules` (the same counts for the issues past `--max-issues`). Each diagnostic has `severity` and `summary` and, when set, `detail` (the call chain), `address` (the rule, with `fixable` or `fixed`), `file`, `line`, `column` and `count`.

## Limitations

- The rule documentation links are dropped from the text and the data; the rule name finds them.
- `fail` follows TFLint's default failure severity. A run with `--minimum-failure-severity=warning` that only reports notices exits with 0 from TFLint, while `--fail-on fail` still matches; use `--fail-on error,warning` there.
