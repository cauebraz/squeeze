# squeeze

[![ci](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml/badge.svg)](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml)

squeeze reads a tool's output on stdin and prints the smallest text that still carries every change, failure or decision in it. It is built for coding agents, which pay for every byte they read.

```
$ terraform show -json plan.out | squeeze
terraform 1.9.5 plan: 0 to add, 0 to change, 1 to destroy, 1 to replace
- aws_instance.old (delete_because_no_resource_config)
-/+ aws_db_instance.main (replacement forced by engine_version; replace_because_cannot_update)
  ! engine_version: "15.4" -> "16.1"
  ~ address: (known after apply)
  ~ arn: (known after apply)
  ~ endpoint: (known after apply)
  ~ id: "db-ABCDEFGHIJKLMNOP" -> (known after apply)
  ~ instance_class: "db.t3.micro" -> "db.t3.small"
  ~ password: (sensitive)
```

The plan JSON for this example is 3,774 bytes; the output is 485.

## Why

A coding agent reads tool output as context. That output is written for humans or for other programs, not for a model with a token budget:

- One diagnostics call returned about 580,700 tokens and filled an agent's context window ([anthropics/claude-code#12054](https://github.com/anthropics/claude-code/issues/12054)).
- A `pytest -v` run printed 150 lines when 8 carried the decision ([anthropics/claude-code#49048](https://github.com/anthropics/claude-code/issues/49048), closed as not planned).
- `terraform show -json` repeats the prior state, the configuration and every unchanged attribute to say that three resources change.

Every token spent on noise costs money, pushes useful context out of the window and gives the model more text to misread. squeeze sits in the pipe between the tool and the agent. It keeps what the agent acts on, drops the rest, never prints secrets, and says what it cut and how to get it back. It works with any agent that runs shell commands, since it needs no integration.

## Install

```
go install github.com/cauebraz/squeeze/cmd/squeeze@latest
```

## Usage

```
tool | squeeze [flags]                     detect the format, use its defaults
tool | squeeze <format> [flags]            name the format, unlock its flags
squeeze <format> -h                        list a format's flags
```

## Use from an agent

Add this to the `AGENTS.md` (or `CLAUDE.md`) of a project whose agent reads tool output:

```
- Pipe `terraform show -json <planfile>` through `squeeze` and read its output instead of the JSON.
- When squeeze reports cut lines, rerun with the flag it names, for example `squeeze tfplan --max-attrs 0`.
- Add `--raw-dir .squeeze` to keep the full input (ignore `.squeeze/` in git); read that file only when the squeezed output lacks what you need.
```

## Formats

| Format | Input | Docs |
|---|---|---|
| `tfplan` | `terraform show -json <planfile>` | [docs/formats/tfplan.md](docs/formats/tfplan.md) |

Each format decides what to keep. Every format guarantees:

- Output is plain text with no control characters or color codes, is the same for the same input, and ends with a newline.
- Secrets the tool marks as sensitive never reach stdout.
- Anything cut for size is announced with a count and the flag that shows it.

## Common flags

| Flag | Default | Effect |
|---|---|---|
| `--output` | `text` | `text`, or `json` for the envelope below |
| `--raw-dir` | none | save the unmodified input as `<sha256 prefix>` (mode 0600) and print the path on stderr, so the full output stays one read away |
| `--version` | | print the version; root command only |

`--output json` prints one line:

```
{"schema_version":1,"format":"tfplan","input_bytes":6294,"text_bytes":485,"ratio":0.0771,"data":{...}}
```

`data` is the format's own structure, described on its docs page. `ratio` is `text_bytes / input_bytes`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | output written |
| 1 | read, parse or write error; stdout is empty |
| 2 | the input matches no known format; name one |
| 64 | invalid flags or arguments |

The content of the input never changes the exit code.

## Development

```
just check          # gofmt, go vet, staticcheck, tests
just cover          # coverage profile, fails below 85 percent
just golden         # rewrite golden files after an intended output change
just fuzz tfplan 30 # fuzz one format for 30 seconds
just bench          # benchmarks of every format
```

New formats follow [docs/adding-a-format.md](docs/adding-a-format.md). Coding agents working on this repository follow [AGENTS.md](AGENTS.md).

## Roadmap

- `pytest`, `gotest` and `cargotest` formats
- generic fallback: color-code removal, repeated-line folding, byte cap
- `--fail-on` to set the exit code from the input's content

## License

MIT
