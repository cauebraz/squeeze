# tfplan

Input: the JSON that `terraform show -json <planfile>` prints, `format_version` 1.x. Detected when the input is a JSON object whose first 64 KiB contain `"format_version"` and `"planned_values"` or `"resource_changes"`. A state file (`terraform show -json` without a plan) is rejected.

```
terraform show -json plan.out | squeeze
terraform show -json plan.out | squeeze tfplan --max-attrs 0 --drift
```

The plan JSON carries `prior_state`, `configuration`, `planned_values` and every unchanged attribute. squeeze keeps only the changed attributes. On the compact JSON of the test fixtures the text is 1 to 16 percent of the input; on a generated 2000-resource plan it is 3 percent (`just bench`).

## Output

The header gives the counts, then one line per resource, sorted by action and then by address. Changed attributes are indented below their resource.

```
$ squeeze < internal/formats/tfplan/testdata/sensitive.json
terraform 1.9.5 plan: 0 to add, 2 to change, 0 to destroy
~ aws_ecs_task_definition.api
  ~ environment.API_KEY: (sensitive)
  ~ environment.LOG_LEVEL: "info" -> "debug"
~ aws_secretsmanager_secret_version.db
  ~ secret_string: (sensitive)

$ squeeze < internal/formats/tfplan/testdata/drift.json
terraform 1.9.5 plan: 0 to add, 1 to change, 0 to destroy; 2 drifted
~ aws_s3_bucket.assets
  ~ tags.Owner: "someone" -> null
drift: 2 resources (--drift to list)
```

| Symbol | Meaning |
|---|---|
| `-` | delete, or forget when the line says `removed from state, not destroyed` |
| `-/+` | replace, destroy before create |
| `+/-` | replace, create before destroy |
| `~` | update, or a changed attribute |
| `+` | create |
| `<=` | read a data source |
| `>>` | import without other changes |
| `=` | no change; shown for moves and with `--all` |
| `!` | attribute that forces the replacement |

- A sensitive attribute prints `(sensitive)` at any nesting depth; its value is never printed.
- A value Terraform knows only after apply prints `(known after apply)`.
- Values longer than 48 bytes, maps and lists print the attribute path only.
- Create and delete print no attributes, since the whole object is new or gone.
- Header counts are disjoint: a replacement counts in `to replace`, not in `to add` or `to destroy`. Moves, imports, reads and forgets appear only when present.
- Line notes carry the deposed key, `moved from <address>`, `import`, the attributes that force replacement, and Terraform's `action_reason`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-attrs` | `8` | attributes per resource, `0` for all |
| `--max-resources` | `200` | resources listed, `0` for all |
| `--all` | off | include resources and outputs without changes |
| `--no-values` | off | print attribute paths only |
| `--drift` | off | list drifted resources instead of counting them |

## JSON data

`data` in the `--output json` envelope holds `tool`, `tool_version`, `summary` (every count, plus `errored` and `complete`), `changes`, `omitted`, `drift` and `outputs`. Each change has `address`, `action` and, when set, `previous_address`, `deposed`, `importing`, `reason`, `replaced_by`, `attrs` and `attrs_omitted`. Rendered values in `attrs` keep their JSON form, so strings include their quotes.

## Limitations

- Lists are compared by index. A reordered set shows as changed elements.
- An `actions` combination outside Terraform's published set fails with exit code 1 and names the combination.
