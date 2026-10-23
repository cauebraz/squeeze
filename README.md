# squeeze

[![ci](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml/badge.svg)](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml)

Tool output, compacted for coding agents.

squeeze reads a tool's output on stdin and prints only what an agent acts on: every change, failure and decision, plus a note naming what it cut.

## Install

```
go install github.com/cauebraz/squeeze/cmd/squeeze@latest
```

Go 1.27. No release yet: output can change until 1.0.

## Quickstart

**1. Pipe.** squeeze detects the format.

```
$ terraform show -json plan.out | squeeze
terraform 1.9.5 plan: 1 to add, 0 to change, 2 to destroy, 1 to replace
- aws_instance.old (delete_because_no_resource_config)
-/+ aws_db_instance.main (replacement forced by engine_version; replace_because_cannot_update)
  ! engine_version: "15.4" -> "16.1"
  + address: (known after apply)
  + arn: (known after apply)
  ~ endpoint: (known after apply)
  ~ id: "db-ABCDEFGHIJKLMNOP" -> (known after apply)
  ~ instance_class: "db.t3.micro" -> "db.t3.small"
  ~ password: (sensitive)
```

6,294 bytes of plan JSON become 485.

**2. Tell your agent.** Add to `AGENTS.md` or `CLAUDE.md`:

```
- Pipe long tool output through `squeeze`, in the tool's JSON mode when it has one: `go test -json ./... 2>&1 | squeeze`.
- Rerun with the flag squeeze names, such as `--max-attrs 0`, only when the output lacks what you need.
- Add `--fail-on error,fail` to exit 3 on a failure; `$?` is otherwise squeeze's, not the tool's.
```

## How it works

- **Parses, never truncates.** Each format knows its tool's output; every cut is announced with a count and the flag that shows it.
- **Secrets stay out.** Values the tool marks as sensitive never reach stdout or `--output json`.
- **Just a pipe.** No MCP server, plugin or hook: any agent that runs shell commands can use it.

On 13 agent tasks the median task used 23 percent fewer tokens; two small-output tasks used 9 and 10 percent more ([bench](bench/README.md)).

## Formats

| Format | Input |
|---|---|
| `tfplan` | `terraform show -json <planfile>` |
| `tfstate` | `terraform show -json`, `state show -json`, `output -json` |
| `tfvalidate` | `terraform validate -json` |
| `tflint` | `tflint --format json` |
| `ghactions` | `gh run view --log` |
| `buildkit` | `docker buildx build --progress=plain` or `rawjson` |
| `jest` | `jest --json`, Vitest `--reporter=json` |
| `junit` | JUnit XML |
| `kubectl` | `kubectl get -o json` |
| `npmaudit` | `npm audit --json` |
| `precommit` | `pre-commit run` |
| `checkov` | `checkov -o json` |
| `trivy` | `trivy --format json` |
| `pytest` | pytest terminal output |
| `tfjson` | `terraform` or `tofu` with `-json` |
| `gotest` | `go test`, with or without `-json` |
| `generic` | any other text |

## Docs

- [Usage, flags, JSON envelope and exit codes](docs/cli.md)
- [Formats, one page each](docs/formats/)
- [Bench](bench/README.md)
- [Contributing](CONTRIBUTING.md)

## Contributing

```
just check   # format, lint, test
just cover   # coverage, 95 percent minimum
```

MIT license.
