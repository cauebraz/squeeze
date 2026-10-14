# tfstate

Input: two Terraform JSON documents that hold state values.

- The state, as `terraform show -json` prints it without a plan file (or with a state file), `format_version` 1.x. Detected when the input is a JSON object that contains `"format_version"` and `"root_module"`, and neither `"planned_values"` nor `"resource_changes"`.
- The outputs, as `terraform output -json` prints them: an object with one entry per root module output holding `sensitive`, `type` and `value`. Detected when the input is a JSON object whose first entry is an object starting with `"sensitive"` (true or false) followed by `"type"`, the order Terraform writes (`internal/command/views/output.go`).

```
terraform show -json | squeeze
terraform output -json | squeeze
terraform show -json | squeeze tfstate --max-resources 0
```

Both print every value in plain text, including the outputs and attributes marked `sensitive`. squeeze prints one line per resource address and per output, never an attribute value, and never a sensitive output value. On the fixtures the text is 16 percent of the state and 25 percent of the outputs.

## Output

```
$ squeeze < internal/formats/tfstate/testdata/state/input.json
terraform 1.9.5 state: 7 resources, 1 data source, 1 tainted, 1 deposed; 3 outputs, 1 sensitive
aws_db_instance.main
aws_instance.web[0]
aws_instance.web[1] (tainted)
aws_instance.web[1] (deposed 00000001)
aws_s3_bucket.logs
data.aws_iam_policy_document.logs
module.vpc.aws_subnet.private["a"]
module.vpc.aws_subnet.private["b"]
module.vpc.module.nat.aws_nat_gateway.this
outputs:
bucket_name = "acme-logs-prod"
db_password = (sensitive)
subnet_ids = ["subnet-0a1","subnet-0b2"]
```

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

- The state header counts managed resources, data sources, tainted objects and deposed objects, then the outputs when there are any. A deposed object is counted as deposed, not as a resource. The outputs header counts the outputs and the sensitive ones; `terraform output -json` carries no Terraform version.
- Resources are listed in Terraform's order: the root module, then each child module, each sorted by address. A deposed object follows its address with `(deposed <key>)`, a tainted one with `(tainted)`.
- Outputs are sorted by name, as Terraform sorts them. A value prints as compact JSON on one line: strings keep their quotes and escapes, so a newline or color code in a value prints as `\n` or `\u001b`.
- An output marked `sensitive` prints `(sensitive)`, whatever its type. Terraform marks sensitivity per output, so a value not marked holds no sensitive part.
- An output value is cut at 200 bytes with `...`; `--output json` keeps it whole.
- A missing `value` prints `null`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-outputs` | `50` | outputs listed, `0` for all |
| `--max-resources` | `200` | state resources listed, `0` for all |

## Conditions for `--fail-on`

None. A state and its outputs report values, not a verdict, errors or warnings; `error`, `fail` and `warning` are accepted and never present.

## JSON data

`data` holds `tool`, `tool_version` (empty for `terraform output -json`), `source` (`state` or `output`), `total`, `sensitive`, `outputs_omitted` and `outputs`. Each output has `name`, `sensitive` and `type` (Terraform's type JSON, such as `["list","string"]`), and, when not sensitive, `value` in full.

For a state, `state` holds `managed`, `data_sources`, `tainted`, `deposed`, `resources_omitted` and `resources`. Each resource has `address`, `mode` (`managed` or `data`) and, when set, `tainted` and `deposed_key`. No attribute value is kept.

## Limitations

- Attribute values are never shown, sensitive or not. Run `terraform state show <address>` for one resource.
- The state's `checks`, the check results of the last apply, are not read.
- `terraform output -json <name>` prints the bare value with no sensitivity mark; squeeze cannot tell it from other JSON, and it goes through `generic`. Pipe the full `terraform output -json` instead.
- An empty state (`{"format_version":"1.0"}`) and an empty outputs object (`{}`) are not detected; `squeeze tfstate` reads them as zero resources or zero outputs.
