# tfstate

Input: the JSON that `terraform output -json` prints, an object with one entry per root module output holding `sensitive`, `type` and `value`. Detected when the input is a JSON object whose first entry is an object starting with `"sensitive"` (true or false) followed by `"type"`, the order Terraform writes (`internal/command/views/output.go`).

```
terraform output -json | squeeze
terraform output -json | squeeze tfstate --max-outputs 0
```

`terraform output -json` prints every value in plain text, including the outputs marked `sensitive`, and indents each value over several lines. squeeze prints one line per output and never prints a sensitive value. On the fixture the text is 25 percent of the input.

## Output

```
$ squeeze < internal/formats/tfstate/testdata/outputs/input.json
terraform output: 11 outputs, 2 sensitive
api_url = "https://api.example.com/v1"
app = {"name":"api","replicas":3}
banner = "line one\nline two\u001b[31m red"
db = (sensitive)
db_password = (sensitive)
enabled = true
endpoints = ["10.0.1.5",8443]
private_subnet_ids = ["subnet-001a1b2c3d4e5f6","subnet-002a1b2c3d4e5f6","subnet-003a1b2c3d4e5f6","subnet-004a1b2c3d4e5f6","subnet-005a1b2c3d4e5f6","subnet-006a1b2c3d4e5f6","subnet-007a1b2c3d4e5f6","subnet-008a1b2c3d4e5...
replicas = 3
tags = {"env":"prod","team":"platform"}
unset = null
```

- The header counts the outputs and the sensitive ones. `terraform output -json` carries no Terraform version.
- Outputs are sorted by name, as Terraform sorts them.
- A value prints as compact JSON on one line: strings keep their quotes and escapes, so a newline or color code in a value prints as `\n` or `\u001b`.
- An output marked `sensitive` prints `(sensitive)`, whatever its type. Terraform marks sensitivity per output, so a value not marked holds no sensitive part.
- A value is cut at 200 bytes with `...`; `--output json` keeps it whole.
- A missing `value` prints `null`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-outputs` | `50` | outputs listed, `0` for all |

## Conditions for `--fail-on`

None. `terraform output` reports values, not a verdict, errors or warnings; `error`, `fail` and `warning` are accepted and never present.

## JSON data

`data` holds `tool`, `tool_version` (empty), `total`, `sensitive`, `outputs_omitted` and `outputs`. Each output has `name`, `sensitive` and `type` (Terraform's type JSON, such as `["list","string"]`), and, when not sensitive, `value` in full.

## Limitations

- `terraform output -json <name>` prints the bare value with no sensitivity mark; squeeze cannot tell it from other JSON, and it goes through `generic`. Pipe the full `terraform output -json` instead.
- An empty object, `{}`, from a configuration with no outputs is not detected; `squeeze tfstate` reads it as zero outputs.
- The state itself, `terraform show -json` without a plan file, is not read by this format yet.
