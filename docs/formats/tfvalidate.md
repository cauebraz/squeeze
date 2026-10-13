# tfvalidate

Input: the JSON that `terraform validate -json` prints, `format_version` 1.x. Detected when the input is a JSON object whose first 64 KiB contain `"format_version"`, `"valid"` and `"diagnostics"`, and neither `"planned_values"` nor `"resource_changes"`.

```
terraform validate -json | squeeze
terraform validate -json | squeeze tfvalidate --fail-on error
```

Validation output is mostly the diagnostics themselves, so the saving comes from structure, not volume: one line per diagnostic with `file:line:column`, the detail wrapped to three lines, and the offending code line. On the fixtures the text is 11 to 56 percent of the input; on Terraform's own `validate-*` test outputs (`just conformance`) it is 24 to 41 percent.

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

- The header gives `valid` or `invalid` and Terraform's own error and warning counts.
- A severity other than `warning` counts as an error, so an unexpected value cannot hide a failure.
- Diagnostics are sorted errors first, then by file, line and column.
- Identical diagnostics (same severity, summary, detail and location) are printed once with `(xN)`.
- A diagnostic without a range prints its summary and detail; the resource address, when Terraform gives one, follows the location.
- The detail is wrapped to 100 bytes and cut after three lines with `...`; the code line is cut at 120 bytes.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-diagnostics` | `20` | diagnostics listed, `0` for all |
| `--no-detail` | off | print summary, location and code only |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | at least one error |
| `invalid` | Terraform reports `valid: false` |
| `warning` | at least one warning |

## JSON data

`data` holds `tool`, `valid`, `errors`, `warnings`, `omitted` and `diagnostics`. Each diagnostic has `severity` and `summary` and, when set, `detail` (in full), `address`, `file`, `line`, `column`, `code` and `count`.
