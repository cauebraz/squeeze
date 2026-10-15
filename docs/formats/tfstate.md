# tfstate

Input: three Terraform JSON documents that hold state values.

- The state, as `terraform show -json` prints it without a plan file (or with a state file), `format_version` 1.x. Detected when the input is a JSON object that contains `"format_version"` and `"root_module"`, and neither `"planned_values"` nor `"resource_changes"`.
- One resource instance, as `terraform state show -json <address>` prints it (Terraform 1.16 and later): `format_version` 1.x, `resource` and `diagnostics` (`internal/command/views/show.go`, `ShowJSON.DisplayResourceInstanceState`, at v1.16.4). Detected when the input is a JSON object whose first key is `"format_version"` with a string value and whose second key is `"resource"` holding an object, the order Terraform writes.
- The outputs, as `terraform output -json` prints them: an object with one entry per root module output holding `sensitive`, `type` and `value`. Detected when the input is a JSON object whose first entry is an object starting with `"sensitive"` (true or false) followed by `"type"`, the order Terraform writes (`internal/command/views/output.go`).

```
terraform show -json | squeeze
terraform output -json | squeeze
terraform state show -json aws_db_instance.main | squeeze
terraform show -json | squeeze tfstate --max-resources 0
```

All three print every value in plain text, including the outputs and attributes marked `sensitive`. squeeze prints one line per resource address and per output, never an attribute value of the whole state, and never a sensitive value. For `terraform state show -json` it prints the attributes of the one instance with every value Terraform marks sensitive replaced. On the fixtures the text is 16 percent of the state, 25 percent of the outputs and 41 percent of the `state show` resource.

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

```
$ squeeze < internal/formats/tfstate/testdata/state-show-nested/input.json
terraform state show terraform_data.app (tainted): 5 attributes, 2 sensitive
id = "8c1f7a52-3b9e-4d1c-a0f2-6e7d5c4b3a21"
input = {"db":[{"host":"db.internal","password":(sensitive)}],"name":"api","note":"line one\nline two\u001b[31m red","tokens":["public-token",(sensitive)]}
output = (sensitive)
... 2 null attributes (--output json)
```

- The state header counts managed resources, data sources, tainted objects and deposed objects, then the outputs when there are any. A deposed object is counted as deposed, not as a resource. The outputs header counts the outputs and the sensitive ones; `terraform output -json` carries no Terraform version.
- Resources are listed in Terraform's order: the root module, then each child module, each sorted by address. A deposed object follows its address with `(deposed <key>)`, a tainted one with `(tainted)`.
- Outputs are sorted by name, as Terraform sorts them. A value prints as compact JSON on one line: strings keep their quotes and escapes, so a newline or color code in a value prints as `\n` or `\u001b`.
- An output marked `sensitive` prints `(sensitive)`, whatever its type. Terraform marks sensitivity per output, so a value not marked holds no sensitive part.
- An output value is cut at 200 bytes with `...`; `--output json` keeps it whole.
- A missing `value` prints `null`.

For `terraform state show -json`:

- The header names the address, `(tainted)` when the instance is tainted, the attribute count, the attributes that hold a sensitive value, and the error and warning diagnostics. Terraform shows only the current object of an instance, never a deposed one.
- One line per attribute, sorted by name, as compact JSON. Attributes whose value is `null` are left out and counted on a last line; `--output json` holds them. Terraform's own `state show` text leaves them out too.
- Terraform marks sensitivity per value in `sensitive_values`: `true` for a sensitive value, an object or array mirroring a structure that holds one. A value marked `true` prints `(sensitive)`, unquoted, so a string `"(sensitive)"` stays distinct; inside an object or a list only the marked element is replaced. A mark squeeze cannot match to the value (another type, another list length, a value that is neither a boolean, an object nor an array) hides the whole value. A resource with `values` and no `sensitive_values` is an error.
- An attribute value is cut at 200 bytes with `...`; `--output json` keeps it whole, with the sensitive parts as `null`.
- Diagnostics follow the attributes, as in `tfvalidate`. On an error, such as an address with no instance, Terraform exits 1 and prints an empty `resource`; squeeze prints the header and the diagnostics.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-outputs` | `50` | outputs listed, `0` for all |
| `--max-resources` | `200` | state resources listed, `0` for all |

## Conditions for `--fail-on`

A state and its outputs report values, not a verdict, errors or warnings; these conditions come only from `terraform state show -json`.

| Condition | Present when |
|---|---|
| `error` | a diagnostic has severity `error` |
| `fail` | a diagnostic has severity `error`; Terraform then exits 1 (`internal/command/views/show.go` at v1.16.4) |
| `warning` | a diagnostic has severity `warning` |

## JSON data

`data` holds `tool`, `tool_version` (empty for `terraform output -json` and `terraform state show -json`), `source` (`state`, `output` or `resource`), `total`, `sensitive`, `outputs_omitted` and `outputs`. Each output has `name`, `sensitive` and `type` (Terraform's type JSON, such as `["list","string"]`), and, when not sensitive, `value` in full.

For a state, `state` holds `managed`, `data_sources`, `tainted`, `deposed`, `resources_omitted` and `resources`. Each resource has `address`, `mode` (`managed` or `data`) and, when set, `tainted` and `deposed_key`. No attribute value is kept.

For `terraform state show -json`, `source` is `resource`, `outputs` is empty, and `resource` holds `address`, `mode`, `type`, `provider_name`, `tainted` when set, `sensitive` (attributes holding a sensitive value), `null` (attributes whose value is `null`) and `attributes`. Each attribute has `name` and one of: `value` in full; `sensitive: true` and no value; or `value` with each sensitive part as `null` and `sensitive_values`, Terraform's mark for that attribute. `errors`, `warnings` and `diagnostics` (as in `tfvalidate`) are present when there are any; no resource is present when Terraform printed none.

## Limitations

- For the whole state, attribute values are never shown, sensitive or not. Run `terraform state show -json <address> | squeeze` for one resource.
- `terraform state show -json` also carries `identity`, the resource identity, and `depends_on`; neither is shown nor kept.
- `terraform state show` without `-json` goes through `generic`; Terraform already prints `(sensitive value)` there.
- The state's `checks`, the check results of the last apply, are not read.
- `terraform output -json <name>` prints the bare value with no sensitivity mark; squeeze cannot tell it from other JSON, and it goes through `generic`. Pipe the full `terraform output -json` instead.
- An empty state (`{"format_version":"1.0"}`) and an empty outputs object (`{}`) are not detected; `squeeze tfstate` reads them as zero resources or zero outputs.
