# tfjson

Input: Terraform's machine-readable UI, one JSON object per line, from `plan`, `apply`, `destroy`, `init` or `test` run with `-json`. Detected when one of the first five non-blank lines is a JSON object with `@level` and `type`. Lines that are not JSON, such as a state lock notice on the same stream, are counted and kept out of the way.

```
terraform plan -json | squeeze
terraform apply -json -auto-approve | squeeze --fail-on apply_errored,error
terraform test -json | squeeze --fail-on test_failed
```

The stream repeats a start, progress and completion message for every resource and every ten seconds of work. squeeze keeps the planned changes, the final summary, the failures, the diagnostics, the outputs and the failing tests. On the fixtures the text is 4.7 to 18.4 percent of the input; on Terraform's own `*.jsonlog` test outputs (`just conformance`) it is 4 to 44 percent, since those samples hold only a few lines.

## Output

```
$ squeeze < internal/formats/tfjson/testdata/apply-errored/input.jsonl
terraform 1.9.5 apply: no apply summary; plan was 2 to add, 0 to change, 0 to destroy; 1 error
+ aws_instance.web
+ aws_s3_bucket.logs
applied: 1 resource (slowest: aws_s3_bucket.logs 1s)
error: aws_instance.web: Creation errored after 4s
error: creating EC2 Instance (main.tf:10:1, aws_instance.web)
  operation error EC2: RunInstances, https response error StatusCode: 400, api error
  InvalidAMIID.NotFound: The image id '[ami-0abcdef1234567890]' does not exist
  > resource "aws_instance" "web" {
... 2 other messages: apply_start 2 (--all)

$ squeeze < internal/formats/tfjson/testdata/test-fail/input.jsonl
terraform 1.9.5 test: error, 1 passed, 1 failed, 1 errored, 0 skipped; 2 errors
fail tests/bucket.tftest.hcl "versioning" 2.1s
  error: Test assertion failed (tests/bucket.tftest.hcl:14:5)
    bucket versioning must be enabled
    > condition     = aws_s3_bucket_versioning.this.versioning_configuration[0].status == "Enabled"
error tests/network.tftest.hcl
  error: Invalid value for variable (tests/network.tftest.hcl:3:3)
    cidr_block must be a valid IPv4 CIDR.
    > cidr_block = "10.0.0.0/33"
... 1 passing test run (--all)
... 1 other message: test_abstract 1 (--all)
```

- The header names the operation and uses Terraform's summary words: `plan: 1 to add, ...`, `apply: 2 added, ...`, `destroy: 2 destroyed`, `init: 2 providers installed`, `test: <status>, N passed, ...`. When an apply stops before its summary, the header says so and repeats the plan's counts.
- Planned changes use the stream's actions with `tfplan`'s symbols: `-` delete, `-/+` replace, `.` remove from state, `~` update, `+` create, `<=` read, `>>` import, `=` move or no change. The stream carries no attributes; run `terraform show -json` on a saved plan and `squeeze tfplan` for those.
- `applied:` counts completed resources and names the three slowest. Data source reads during a plan or apply are counted under `refresh`.
- Every `apply_errored`, `provision_errored`, `action_errored` and `ephemeral_op_errored` message is printed as an `error:` line, followed by the diagnostics.
- A `policy_result` other than `AllowResult` is printed as a `policy:` line with the policy, the target and the enforcement level.
- Test runs that fail or error are listed with their diagnostics; a test file that errors outside a run is listed with its own diagnostics. Passing runs are counted in `... N passing test runs (--all)`; no-op changes in `... N no-op changes (--all)`; drift in `... N drifted resources (--drift)` unless `--drift` lists it.
- Sensitive output values are never printed or stored; other scalar outputs print their value up to 48 bytes.
- Every line at level `error` or `warn` becomes a diagnostic, whatever its type, so a message type squeeze does not know cannot hide a failure. A diagnostic severity other than `warning` counts as an error.
- Strings from the stream are reduced to one line without control characters or terminal escape sequences before they are printed, so an address or a name cannot forge a line of output.
- Everything else is counted in `... N other messages: <type> <count>, ... (--all)`; `--all` prints those messages, each cut at 200 bytes with `...`; `--output json` carries only their counts.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | include no-op resources, passing tests and every other message |
| `--drift` | off | list drifted resources instead of counting them |
| `--max-diagnostics` | `20` | diagnostics listed, `0` for all |
| `--max-resources` | `200` | planned changes listed, `0` for all |
| `--no-detail` | off | print diagnostic summary, location and code only |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `apply_errored` | an apply, provisioner, action or ephemeral resource operation failed |
| `change` | the summary or a planned change adds, changes, destroys, imports, moves or forgets |
| `deferred` | a change was deferred |
| `destroy` | the summary destroys or a planned change deletes or replaces |
| `drift` | Terraform reported drift |
| `error` | an error diagnostic or a failed operation |
| `fail` | `error`, `apply_errored`, `policy_failed` or `test_failed` |
| `forget` | a planned change removes an object from state without destroying it |
| `policy_failed` | a policy result other than `AllowResult` |
| `replace` | a planned change replaces |
| `test_failed` | a test run or file failed or errored |
| `warning` | a warning diagnostic |

## JSON data

`data` holds `tool`, `tool_version`, `operation`, `summary` (Terraform's `change_summary` as `operation`, `add`, `change`, `remove`, `import`, `action_invocation`, `action_fail`, or null), `changes`, `changes_omitted`, `noop_hidden`, `drift`, `deferred`, `applied` (`count`, `slowest`), `failures`, `policies`, `errors`, `warnings`, `diagnostics`, `diagnostics_omitted`, `outputs`, `providers_installed`, `tests` (or null) and `other` (message counts by type).

- `changes` and `drift`: `address`, `action` and, when set, `previous_address`, `reason`, `import_id`. `deferred` adds `reason` to `address` and `action`.
- `failures`: `type`, `message` and, when set, `address`. `policies`: `policy`, `result` and, when set, `target`, `enforcement`.
- `outputs`: `name` and, when set, `action`, `sensitive`, `value` (scalars only, never for sensitive outputs).
- `tests`: `status`, `passed`, `failed`, `errored`, `skipped`, `runs` (`file`, `run`, `status`, `elapsed_ms` when set, `diagnostics`) and `files` (`file`, `status`, `diagnostics`).

## Limitations

- The stream has no attribute diffs; `tfplan` is the format for those.
- Detection reads the first five non-blank lines; a stream that starts with more text lines needs `squeeze tfjson`.
