# squeeze

[![ci](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml/badge.svg)](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml)

squeeze reads a tool's output on stdin and prints the smallest text that still carries every change, failure or decision in it, for coding agents that pay for every byte they read.

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

The plan JSON for this example is 6,294 bytes; the output is 485.

## Install

```
go install github.com/cauebraz/squeeze/cmd/squeeze@latest
```

Needs Go 1.27. There is no release yet, so output can change between commits until 1.0.

## Use from an agent

Add these lines to the project's `AGENTS.md` or `CLAUDE.md`; each format page has a line for its tool.

```
- Pipe long tool output through `squeeze`, using the tool's JSON mode when it has one:
  `terraform show -json plan.out | squeeze`, `go test -json ./... 2>&1 | squeeze`, `npm test 2>&1 | squeeze`.
- squeeze names what it cut and the flag that shows it, such as `--max-attrs 0`; rerun with that flag only when the output lacks what you need.
- `$?` is squeeze's exit code, not the tool's: add `--fail-on error,fail` to exit 3 on a failure, or start the command with `set -o pipefail;`.
```

## Formats

| Format | Input | Docs |
|---|---|---|
| `tfplan` | `terraform show -json <planfile>` | [docs/formats/tfplan.md](docs/formats/tfplan.md) |
| `tfstate` | `terraform show -json` without a plan file, `terraform state show -json <address>`, `terraform output -json` | [docs/formats/tfstate.md](docs/formats/tfstate.md) |
| `tfvalidate` | `terraform validate -json` | [docs/formats/tfvalidate.md](docs/formats/tfvalidate.md) |
| `tflint` | `tflint --format json` | [docs/formats/tflint.md](docs/formats/tflint.md) |
| `ghactions` | `gh run view --log` or `--log-failed` | [docs/formats/ghactions.md](docs/formats/ghactions.md) |
| `buildkit` | `docker buildx build --progress=plain` or `--progress=rawjson` | [docs/formats/buildkit.md](docs/formats/buildkit.md) |
| `jest` | `jest --json`, Vitest `--reporter=json` | [docs/formats/jest.md](docs/formats/jest.md) |
| `junit` | JUnit XML reports from `pytest --junitxml`, `gotestsum`, `go-junit-report`, Surefire, `jest-junit`, `checkov -o junitxml` | [docs/formats/junit.md](docs/formats/junit.md) |
| `kubectl` | `kubectl get <resources> -o json`, a list or one object | [docs/formats/kubectl.md](docs/formats/kubectl.md) |
| `npmaudit` | `npm audit --json` | [docs/formats/npmaudit.md](docs/formats/npmaudit.md) |
| `precommit` | `pre-commit run`, with or without `--verbose` and color | [docs/formats/precommit.md](docs/formats/precommit.md) |
| `checkov` | `checkov -o json` | [docs/formats/checkov.md](docs/formats/checkov.md) |
| `trivy` | `trivy fs`, `config`, `image` or `k8s` with `--format json` | [docs/formats/trivy.md](docs/formats/trivy.md) |
| `pytest` | pytest's terminal output, default, `-v` or `-q` | [docs/formats/pytest.md](docs/formats/pytest.md) |
| `tfjson` | `terraform plan`, `apply`, `destroy`, `init`, `test` or `query` with `-json`, and the same `tofu` commands | [docs/formats/tfjson.md](docs/formats/tfjson.md) |
| `gotest` | `go test -json`, or `go test` text with or without `-v` | [docs/formats/gotest.md](docs/formats/gotest.md) |
| `generic` | any other text, when no format above detects it | [docs/formats/generic.md](docs/formats/generic.md) |

## How it differs

One diagnostics call returned about 580,700 tokens and filled an agent's context window ([anthropics/claude-code#12054](https://github.com/anthropics/claude-code/issues/12054)).
A `pytest -v` run printed an estimated 150 lines when 8 carried the decision ([anthropics/claude-code#49048](https://github.com/anthropics/claude-code/issues/49048)).

- It parses instead of truncating. A byte cap or `tail -n 100` cuts blindly; squeeze keeps every change, failure or decision and names what it left out and the flag that shows it.
- Secrets are never printed. Values the tool marks as sensitive never reach stdout or `--output json`, at any nesting depth.
- It is a pipe, with no integration. No MCP server, plugin or hook: any agent that runs shell commands can run `tool | squeeze`.

## Bench

Across the 13 agent tasks in [bench/README.md](bench/README.md), the median task used 23 percent fewer tokens with squeeze.
Small outputs can cost more: `tfvalidate-errors` used 9 percent more tokens and `checkov-findings` 10 percent more.

![Tokens an agent spends per task with and without squeeze](bench/results.svg)

## Reference

- [docs/cli.md](docs/cli.md): usage, common flags, the JSON envelope, exit codes and the guarantees every format keeps.
- [docs/formats/](docs/formats/): one page per format.
- [CONTRIBUTING.md](CONTRIBUTING.md): branches, commits, pull requests and new formats.

## Development

`just` lists the recipes.
Run `just check` before and after a change.

## License

MIT
