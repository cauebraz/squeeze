# tfjson

Input: Terraform's machine-readable UI, one JSON object per line, from `plan`, `apply`, `destroy`, `init`, `test` or `query` run with `-json`, and the same `tofu` commands.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Run Terraform with `-json` and pipe it through `squeeze`: `terraform plan -json | squeeze`,
  `terraform test -json | squeeze`, `terraform query -json | squeeze`; read squeeze's output instead of Terraform's.
```

## Example

```
terraform plan -json | squeeze
terraform apply -json -auto-approve | squeeze --fail-on apply_errored,error
terraform test -json | squeeze --fail-on test_failed
terraform query -json | squeeze
```

The stream repeats a start, progress and completion message for every resource and every ten seconds of work.
squeeze keeps the planned changes, the final summary, the failures, the diagnostics, the outputs and the failing tests.
On the fixtures run with default flags the text is 4.4 to 25.6 percent of the input.
On Terraform's own `*.jsonlog` test outputs (`just conformance`) it is 4 to 44 percent, since those samples hold only a few lines.

Lines that are not JSON, such as a state lock notice on the same stream, are counted and kept out of the way.
OpenTofu's `version` line carries the key `tofu` instead of `terraform` (`internal/command/views/json_view.go` in OpenTofu); the header and `tool` then name `tofu` with its version.

- Detection: one of the first five non-blank lines is a JSON object with `@level` and `type`.

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

- The header names the operation and uses Terraform's summary words: `plan: 1 to add, ...`, `apply: 2 added, ...`, `destroy: 2 destroyed`, `init: 2 providers installed`, `test: <status>, N passed, ...`, `query: N found in M list blocks`. When an apply stops before its summary, the header says so and repeats the plan's counts.
- Planned changes use the stream's actions with `tfplan`'s symbols: `-` delete, `-/+` replace, `.` remove from state, `~` update, `+` create, `<=` read, `>>` import, `=` move or no change. The stream carries no attributes; run `terraform show -json` on a saved plan and `squeeze tfplan` for those. Each distinct `planned_change` prints its own line, so an address can appear twice: a replacement beside the delete of a deposed object, which Terraform emits as a separate change for the same address. An identical line seen again, as when two runs are piped together, prints once.
- `terraform query` (Terraform 1.14 and later) prints one line per list block, sorted by address, with the total from its `list_complete` message, then the display name of each resource found, indented, in Terraform's order: `list.aws_instance.web: 3 found`. A block without `list_complete`, as in an interrupted run, says `incomplete` and counts the resources seen. `--max-resources` cuts the found resources across all blocks, announced as `... N more found resources (--max-resources 0)`. The identity of each resource is in `--output json` only. The list block's `input_config`, the resource's `resource_object` (`include_resource = true`) and the generated `config` and `import_config` (`-generate-config-out`) are never printed or stored: `input_config` carries values Terraform marks as sensitive (`sensitive_attribute_paths`) and `resource_object` carries the resource's state (`internal/command/views/json/query.go` and `views/hook_json.go` in Terraform).
- Planned action invocations (`planned_action_invocation`, Terraform 1.14 and later) print on one `actions:` line as `tfplan` prints them: `<action> (<event> <resource>)` for an action triggered by a resource's lifecycle, `(invoked)` for one run with `-invoke`. `actions completed: N` counts `action_complete` messages; `action_start` and `action_progress` are counted as other messages (`internal/command/views/json/change.go` and `views/json/hook.go` in Terraform).
- `applied:` counts completed resources and names the three slowest. Data source reads during a plan or apply are counted under `refresh`.
- Every `apply_errored`, `provision_errored`, `action_errored` and `ephemeral_op_errored` message is printed as an `error:` line, followed by the diagnostics.
- A `policy_result` other than `AllowResult` is printed as a `policy:` line with the policy, the result, the target and the enforcement level. squeeze sorts each result the way `terraform query` does (`queryPolicyResultFromEvaluation` in `internal/command/views/query_policy.go` at v1.17.0-rc2): `DenyResult` is a failure (`policy_failed`); `UnknownResult` is neither a failure nor an error, since Terraform could not evaluate the policy at plan time and reports only warnings (`UNKNOWN_EVALUATE_RESULT` in `internal/policy/proto/types.proto`); `PolicyErrorResult`, `SetupErrorResult`, `InvalidResult` and any result squeeze does not know are errors (`error`). `AllowResult` is counted under `policy_result`, and `--all` prints it.
- A `policy_diagnostic` becomes a diagnostic like a `diagnostic` line, with the summary, the detail, the Terraform file and line, the code line, and the policy's target as its address.
- A `policy_info` message (an informational message from a policy) is counted under `policy_info`; `--all` prints it as `policy info: <policy> for <target>: <message>`.
- A `policy_query_summary` from `terraform query -json -policies` prints a `policy:` line for every policy that fails, errors or is unknown for a discovered resource, with Terraform's per-policy result (`fail`, `error`, `unknown`), and a `policy: <result> for <target>` line for a resource whose result is `fail`, `error` or `unknown` with no policy listed. `pass` and `n/a` are not printed. The diagnostics inside the summary become diagnostics with the resource's address.
- Test runs that fail or error are listed with their diagnostics, and so is a passing run that carries a diagnostic, such as a cleanup that failed after it; a test file that errors outside a run is listed with its own diagnostics. Passing runs are counted in `... N passing test runs (--all)`; no-op changes in `... N no-op changes (--all)`; drift in `... N drifted resources (--drift)` unless `--drift` lists it.
- Resources that `terraform test` leaves in state print one line per `test_cleanup` or `test_interrupt` message, after the tests: `left in state after <file> "<run>" (<reason>): <address>, ...`, with `(deposed <key>)` after a deposed object. The reason is `cleanup failed` (Terraform could not destroy them; they need to be cleaned up by hand), `skip_cleanup` (the run sets `skip_cleanup`) or `interrupted`, with `state <name>` for a state other than the main one. Resources Terraform was creating when it was interrupted print as `maybe created by <file> "<run>" (interrupted): ...`. These lines are not cut (`internal/command/views/test.go` and `views/json/test.go` in Terraform).
- Sensitive output values are never printed or stored; other scalar outputs print their value up to 48 bytes.
- Every line at level `error` or `warn` becomes a diagnostic, whatever its type, so a message type squeeze does not know cannot hide a failure. A diagnostic severity other than `warning` counts as an error. Identical diagnostics print once with `(xN)`; `--max-diagnostics` counts printed lines, and `... N more diagnostics` and `diagnostics_omitted` count every cut diagnostic, N for an `(xN)` line.
- A diagnostic's detail is wrapped to 100 bytes and cut after three lines with `...`; its code line is cut at 120 bytes.
- A failed `terraform test` assertion that compares two values (`snippet.test_assertion_expr`) prints them after the code line as `lhs:` and `rhs:`, each as compact JSON on one line cut at 200 bytes, and Terraform's warning about different types as `note:`. Terraform redacts sensitive and ephemeral values in both. Other diagnostics print the expression values (`snippet.values`) as `| <expression> <statement>`, cut at 200 bytes; for an assertion with `lhs` and `rhs` they are not printed, as Terraform does without `-verbose`. `--no-detail` hides all of them.
- Everything else is counted in `... N other messages: <type> <count>, ... (--all)`, with `(` and `)` in a type printed as `[` and `]` so the notice keeps its shape; `--all` prints those messages, each cut at 200 bytes with `...`; `--output json` carries only their counts.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | include no-op resources, passing tests and every other message |
| `--drift` | off | list drifted resources instead of counting them |
| `--max-diagnostics` | `20` | diagnostics listed, `0` for all |
| `--max-resources` | `200` | planned changes and found resources listed, `0` for all |
| `--no-detail` | off | print diagnostic summary, location and code only |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `apply_errored` | an apply, provisioner, action or ephemeral resource operation failed |
| `change` | the summary or a planned change adds, changes, destroys, imports, moves or forgets |
| `deferred` | a change was deferred |
| `destroy` | the summary destroys or a planned change deletes or replaces |
| `drift` | Terraform reported drift |
| `error` | an error diagnostic, a failed operation, or a policy result that is an error (`PolicyErrorResult`, `SetupErrorResult`, `InvalidResult`, `error`) |
| `fail` | `error`, `apply_errored`, `policy_failed` or `test_failed` |
| `forget` | a planned change removes an object from state without destroying it |
| `policy_failed` | a policy denied a resource (`DenyResult`, or `fail` in a query summary) |
| `replace` | a planned change replaces |
| `test_failed` | a test run or file failed or errored |
| `warning` | a warning diagnostic |

## JSON data

`data` holds `tool` (`terraform`, or `tofu` for an OpenTofu stream), `tool_version`, `operation`, `summary` (Terraform's `change_summary` as `operation`, `add`, `change`, `remove`, `import`, `action_invocation`, `action_fail`, or null), `changes`, `resources_omitted`, `noop_hidden`, `lists`, `found`, `found_omitted`, `drift`, `deferred`, `actions`, `applied` (`count`, `slowest`), `actions_completed`, `failures`, `policies`, `errors`, `warnings`, `diagnostics`, `diagnostics_omitted`, `outputs`, `providers_installed`, `tests` (or null) and `other` (message counts by type).

- `changes` and `drift`: `address`, `action` and, when set, `previous_address`, `reason`, `import_id`. `deferred` adds `reason` to `address` and `action`.
- `lists`: `address`, `total`, `complete` and, when set, `resource_type`. `found`: `address` (the list block), `display_name` and, when set, `identity` (the resource identity object as Terraform writes it); `found_omitted` counts the found resources `--max-resources` cut.
- `actions`: `address` and, when set, `event` and `resource` (lifecycle trigger) or `invoked` (`-invoke`), as in `tfplan`.
- `failures`: `type`, `message` and, when set, `address` (the resource, or the action for `action_errored`). `policies`: `policy` (empty for a query result that names no policy), `result` as Terraform writes it, `outcome` (`fail`, `error` or `unknown`) and, when set, `target`, `enforcement`.
- `outputs`: `name` and, when set, `action`, `sensitive`, `value` (scalars only, never for sensitive outputs).
- `tests`: `status`, `passed`, `failed`, `errored`, `skipped`, `runs` (`file`, `run`, `status`, `elapsed_ms` when set, `diagnostics`) `files` (`file`, `status`, `diagnostics`) and `left_in_state` (`file`, `reason` as `cleanup_failed`, `skip_cleanup`, `interrupted` or `planned`, `resources` with `address` and `deposed_key` when set, and, when set, `run` and `state`).
- Each diagnostic, at the top level or in `tests`: `severity`, `summary` and, when set, `detail`, `address`, `file`, `line`, `column`, `code`, `values`, `lhs`, `rhs`, `assertion_warning` (`lhs` and `rhs` in full) and `count`.

## Limitations

- The stream has no attribute diffs; `tfplan` is the format for those.
- A diagnostic's detail is cut after three lines in the text; `--output json` carries the full detail and the full `lhs` and `rhs`. The code line is cut at 120 bytes and each `values` line at 200 bytes in the text only.
- The stream carries no deposed key (`ResourceInstanceChange` in `internal/command/views/json/change.go`), so the delete of a deposed object prints as `- <address>` with nothing to tell it from a delete of the current object; `tfplan` prints the deposed key.
- A policy diagnostic's location in the policy file (`policy_range`, `policy_snippet`) and the values it shows are not printed or kept; the diagnostic points at the Terraform resource instead. `terraform query` output has no operation in the header.
- Detection reads the first five non-blank lines; a stream that starts with more text lines needs `squeeze tfjson`.
