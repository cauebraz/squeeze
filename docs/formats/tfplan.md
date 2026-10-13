# tfplan

Input: the JSON that `terraform show -json <planfile>` prints, `format_version` 1.x. Detected when the input is a JSON object whose first 64 KiB contain `"format_version"` and `"planned_values"` or `"resource_changes"`. A state file (`terraform show -json` without a plan) is rejected.

```
terraform show -json plan.out | squeeze
terraform show -json plan.out | squeeze tfplan --max-attrs 0 --drift
```

The plan JSON carries `prior_state`, `configuration`, `planned_values` and every unchanged attribute. squeeze keeps only what changes. On the 23 plans in Terraform's own `show-json` test corpus (`just conformance`) the text is 0.7 to 7.5 percent of the input; on the single-scenario fixtures it is 0.7 to 10.8 percent; on a generated 2000-resource plan it is 3 percent (`just bench`).

## Output

The header uses Terraform's counts, then one line per resource, sorted by action and then by address. Changed attributes are indented below their resource.

```
$ squeeze < internal/formats/tfplan/testdata/sensitive/input.json
terraform 1.9.5 plan: 0 to add, 2 to change, 0 to destroy
~ aws_ecs_task_definition.api
  ~ environment.API_KEY: (sensitive)
  ~ environment.LOG_LEVEL: "info" -> "debug"
~ aws_secretsmanager_secret_version.db
  ~ secret_string: (sensitive)

$ squeeze < internal/formats/tfplan/testdata/deferred/input.json
terraform 1.9.5 plan: 1 to add, 0 to change, 0 to destroy; 2 deferred; incomplete
+ aws_vpc.main
deferred:
+ aws_subnet.private["*"] (instance_count_unknown)
+ kubernetes_namespace.app (provider_config_unknown)
```

Resource symbols follow Terraform's:

| Symbol | Meaning |
|---|---|
| `-` | delete |
| `-/+` | replace, destroy before create |
| `+/-` | replace, create before destroy |
| `.` | forget: removed from state, not destroyed |
| `./+` | forget, then create a replacement |
| `+/.` | create a replacement, then forget |
| `~` | update |
| `+` | create |
| `<=` | read a data source |
| `>>` | import without other changes |
| `=` | no change; shown for moves and with `--all` |

Attribute symbols:

| Symbol | Meaning |
|---|---|
| `~` | changed: `before -> after` |
| `+` | added: the key or element did not exist |
| `-` | removed: the key or element no longer exists |
| `!` | changed and forces the replacement |

- The header counts match `terraform plan`: a replacement counts in `to add` and `to destroy`, forget-and-create counts in `to add`, imports come first. `to replace`, `to forget`, `to move` and `to read` follow when present.
- A sensitive attribute prints `(sensitive)` at any nesting depth; its value is never printed or stored. An attribute whose value stays the same but becomes sensitive, or stops being sensitive, also prints `(sensitive)`.
- A value Terraform knows only after apply prints `(known after apply)`.
- Values longer than 48 bytes, maps and lists print the attribute path only.
- List elements are matched by value, so an element that only moves is not printed. This covers sets, which the plan JSON stores as lists in an order that shifts when an element is added. Elements without a match are paired in order when both are objects or share an index, and their attributes are compared; a pair that changed position shows both indices, as in `~ rule[0->2].priority: 1 -> 4`. The rest print as `-` with the old index or `+` with the new one. A list whose matched elements change order prints `~ <path>: order changed`. Lists nested more than 32 deep are compared by index. This follows Terraform's set rendering (`processSet` in `internal/command/jsonformat/differ/set.go`) and its object pairing for lists (`ProcessSlice` in `internal/command/jsonformat/collections/slice.go`).
- Create and delete print no attributes, since the whole object is new or gone.
- Line notes carry the deposed key, `moved from <address>`, `import`, the attributes that force replacement and Terraform's `action_reason`.
- Drift is shown the way Terraform shows it: only attributes listed in the plan's `relevant_attributes`. A relevant path that goes through a list index makes the whole list relevant, as Terraform's renderer treats every element of a relevant list (`internal/command/jsonformat/differ/list.go`).
- A replacement path that names a set element by its value prints the element as `[*]`, so the value never reaches the text. Drift that affects the plan is counted in `... N drifted resources (--drift)` unless `--drift` lists it; drift that does not affect the plan is counted in a `... N drifted resources not affecting this plan (--all)` line. Resources without changes are counted in `... N resources without changes (--all)`.
- Deferred resources (`deferred_changes`, in incomplete plans) are listed with Terraform's reason. Deferred no-ops are hidden unless moved.
- Action invocations (`action_invocations`) are listed with their trigger event and resource.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | include resources and outputs without changes, and drift that does not affect the plan |
| `--drift` | off | list drifted resources instead of counting them |
| `--max-attrs` | `8` | attributes per resource listed, `0` for all |
| `--max-resources` | `200` | resources listed, `0` for all |
| `--no-values` | off | print attribute paths only |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `change` | the plan creates, updates, destroys, replaces, imports, moves, forgets or reads anything |
| `deferred` | a change was deferred |
| `destroy` | a resource is deleted or replaced by destroying it |
| `drift` | drift that affects the plan |
| `error` | Terraform reports `errored: true` |
| `fail` | the plan errored or is incomplete |
| `forget` | a resource is removed from state without being destroyed |
| `import` | a resource is imported |
| `incomplete` | Terraform reports `complete: false` |
| `move` | a resource moves to a new address |
| `replace` | a resource is replaced |

## JSON data

`data` in the `--output json` envelope holds:

- `tool`, `tool_version`.
- `summary`: `import`, `add`, `change`, `destroy` with Terraform's semantics, then `replace`, `forget`, `move`, `read`, `noop`, `drift`, `drift_ignored`, `deferred`, `actions`, `outputs`, `errored`, `complete`.
- `changes` and `drift`: each with `address`, `action` and, when set, `previous_address`, `deposed`, `importing`, `reason`, `irrelevant`, `replaced_by`, `attrs`, `attrs_omitted`.
- `attrs`: `path`, `kind` (`update`, `add`, `remove`) and, when set, `before`, `after`, `unknown`, `sensitive`, `forces_replacement`, `reordered`. `before` and `after` are the scalar values with their JSON types and are never shortened; the 48-byte limit applies to text only.
- `changes_omitted`, `noop_hidden`, `drift_omitted`, `deferred` (`address`, `action`, `reason`), `actions` (`address`, `event`, `resource`, `invoked`) and `outputs` (`name`, `action` and, when set, `unknown`, `sensitive`).

## Limitations

- The plan JSON carries no provider schema, so squeeze cannot tell a set from a list. Unmatched objects in a set are paired as in a list: Terraform would show the old element removed and the new one added, squeeze shows the attributes that differ between them, with both indices when they differ.
- The plan JSON does not say whether the plan is refresh-only. Terraform shows all drift for those plans; squeeze counts the drift and points to `--all`.
- An `actions` list outside Terraform's published set fails with exit code 1 and names it.
