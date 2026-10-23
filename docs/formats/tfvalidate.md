# tfvalidate

Input: the JSON that `terraform validate -json` prints, `format_version` 1.x.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Run `terraform validate -json | squeeze` and read squeeze's output instead of Terraform's.
```

## Example

```
terraform validate -json | squeeze
terraform validate -json | squeeze tfvalidate --fail-on error
```

Validation output is mostly the diagnostics themselves, so the saving comes from structure, not volume.
squeeze prints one line per diagnostic with `file:line:column`, the detail wrapped to three lines, and the offending code line.
On the fixtures the text is 11 to 56 percent of the input.
On Terraform's own `validate-*` test outputs (`just conformance`) it is 24 to 41 percent.

- Detection: a JSON object that contains `"format_version"`, `"valid"` and `"diagnostics"`, and neither `"planned_values"` nor `"resource_changes"`.

## Output

```
$ squeeze < internal/formats/tfvalidate/testdata/errors-and-warnings/input.json
terraform validate: invalid, 2 errors, 2 warnings
error: Missing required argument (main.tf:22:34)
  The argument "ami" is required, but no definition was found.
  > resource "aws_instance" "worker" {
error: Reference to undeclared input variable (modules/vpc/main.tf:14:15)
  An input variable with the name "subnet_ids" has not been declared. This variable can be declared
  with a variable "subnet_ids" {} block.
  > subnet_ids = var.subnet_ids
warning: Redundant ignore_changes element (main.tf:30:7)
  Adding an attribute name to ignore_changes tells Terraform to ignore future changes to the argument
  in configuration after the object has been created, retaining the value originally configured. The
  attribute id is decided by the provider alone and therefore there can never be any configuration...
  > ignore_changes = [id]
warning: Argument is deprecated (storage.tf:7:3)
  Use the aws_s3_bucket_versioning resource instead.
  > versioning {
```

- The header gives `valid` or `invalid` and the error and warning counts: Terraform's own, or the diagnostics counted when those are higher, so a missing count cannot hide an error from `--fail-on`.
- A severity other than `warning` counts as an error, so an unexpected value cannot hide a failure.
- Diagnostics are sorted errors first, then by file, line and column.
- Identical diagnostics (same severity, summary, detail, location and values) are printed once with `(xN)`. `--max-diagnostics` counts printed lines, so an `(xN)` line takes one; `... N more diagnostics` and `diagnostics_omitted` count every cut diagnostic, N for an `(xN)` line.
- A line after the JSON document that reads as an error or a warning, such as `Error: Failed to load plugin schemas` when stderr is piped too, becomes a diagnostic without a location, its `Error:` or `Warning:` prefix removed, and counts in the header. An error line makes the run `invalid`, since validation did not complete. Other lines after the document are dropped.
- A diagnostic without a range prints its summary and detail; the resource address, when Terraform gives one, follows the location.
- The detail is wrapped to 100 bytes and cut after three lines with `...`; the code line is cut at 120 bytes.
- The expression values Terraform gives with a diagnostic (`snippet.values`, such as `var.bucket_names is list of string with 2 elements`) follow the code line, one per line as `| <expression> <statement>`, each cut at 200 bytes. `--no-detail` hides them.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-diagnostics` | `20` | diagnostics listed, `0` for all |
| `--no-detail` | off | print diagnostic summary, location and code only |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | at least one error |
| `fail` | Terraform reports `valid: false`, or an error line follows the JSON |
| `invalid` | Terraform reports `valid: false`, or an error line follows the JSON |
| `warning` | at least one warning |

## JSON data

`data` holds `tool`, `tool_version` (empty: the validate JSON carries none), `valid` (false when an error line follows the JSON), `errors`, `warnings`, `diagnostics_omitted` and `diagnostics`. Each diagnostic has `severity` and `summary` and, when set, `detail` (in full), `address`, `file`, `line`, `column`, `code`, `values` and `count`.

## Limitations

- The detail is cut after three lines in the text; `--output json` carries the full detail. The code line is cut at 120 bytes in both.
