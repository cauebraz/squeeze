# squeeze

[![ci](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml/badge.svg)](https://github.com/cauebraz/squeeze/actions/workflows/ci.yml)

squeeze reads a tool's output on stdin and prints the smallest text that still carries every change, failure or decision in it. It is built for coding agents, which pay for every byte they read.

squeeze is experimental. Its purpose is to cut the tokens an agent spends reading tool output; its output shapes, flags, condition names and JSON keys still change between commits, and there is no release yet. Pin a commit if a script depends on them.

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

## Why

A coding agent reads tool output as context. That output is written for humans or for other programs, not for a model with a token budget:

- One diagnostics call returned about 580,700 tokens and filled an agent's context window ([anthropics/claude-code#12054](https://github.com/anthropics/claude-code/issues/12054)).
- A `pytest -v` run printed an estimated 150 lines when 8 carried the decision ([anthropics/claude-code#49048](https://github.com/anthropics/claude-code/issues/49048), closed as not planned); the `pytest` format is squeeze's answer.
- `terraform show -json` repeats the prior state, the configuration and every unchanged attribute to say that three resources change.

Every token spent on noise costs money, pushes useful context out of the window and gives the model more text to misread. squeeze sits in the pipe between the tool and the agent. It keeps what the agent acts on, drops the rest, never prints a value the tool marks as sensitive, and says what it cut and how to get it back. It works with any agent that runs shell commands, since it needs no integration.

### What makes it different

- **It understands the output; it does not truncate it.** An agent's byte cap or `tail -n 100` cuts blindly, and the failing line is often past the cut. squeeze parses the format and keeps every change, failure or decision, then says what it left out and which flag shows it.
- **The output is written for a model.** One line per item, no color codes, no box drawing, the same bytes for the same input. Tools such as `tf-summarize` and `tfplan2md` summarize Terraform plans for people, with colors or Markdown tables; the agent still pays for the layout.
- **Secrets stay out.** Values the tool marks as sensitive never reach stdout or `--output json`, at any nesting depth, and the matched text of a Trivy or Checkov secret finding is never read. A head, tail or byte cap does not know which values those are. The test and lint formats print what the tool printed.
- **The tool's own semantics.** Each format follows its tool: `tfplan` uses Terraform's counts and symbols and shows drift the way Terraform does, `pytest` prints the node IDs pytest reruns, `checkov` takes its counts from Checkov's summary. For the Terraform formats, `just conformance` runs squeeze against Terraform's own test corpus. An agent reading squeeze sees what the operator would see.
- **A pipe, not a platform.** No MCP server, no editor plugin, no hook configuration. Any agent or script that can run `tool | squeeze` can use it, and `--output json` gives scripts the same data with the compression ratio.
- **One binary, more formats.** Each format is a package behind the same interface and contract tests; the agent learns one command.

The name is the action: you squeeze a tool's output to keep the juice and drop the pulp. It is also where the tool sits, in the pipe, `tool | squeeze`, and what the agent is short of, room in its context.

## Install

```
go install github.com/cauebraz/squeeze/cmd/squeeze@latest
go install github.com/cauebraz/squeeze/cmd/squeeze@<commit>   # pin while there is no release
```

## Usage

```
tool | squeeze [flags]                     detect the format, use its defaults; any other text goes through generic
tool | squeeze <format> [flags]            name the format, unlock its flags
squeeze <format> -h                        list a format's flags
```

## Use from an agent

Add this to the `AGENTS.md` (or `CLAUDE.md`) of a project whose agent reads tool output:

```
- Run Terraform with `-json` and pipe it through `squeeze`: `terraform validate -json | squeeze`,
  `terraform plan -json | squeeze`, `terraform test -json | squeeze`, `terraform show -json <planfile> | squeeze`.
  Read squeeze's output instead of Terraform's. Run TFLint as `tflint --format json | squeeze`, Checkov as `checkov -d . -o json | squeeze`, and Trivy as `trivy fs --format json . | squeeze`.
- Run Go tests with `go test -json ./... 2>&1 | squeeze`; rerun one failure with the package and test it names.
- Run Python tests with `pytest 2>&1 | squeeze`; rerun one failure with `pytest <node id>`, using the node ID it prints.
- Pipe other long output through `squeeze` too, for example `npm test 2>&1 | squeeze`; text that no format
  knows goes through `generic`.
- To stop on a condition, add `--fail-on error,fail`; exit code 3 means it matched. Every format accepts
  `error`, `fail` and `warning`, and `squeeze <format> -h` lists the format's own names, such as `destroy`.
- When squeeze reports cut lines, rerun with the flag it names, for example `squeeze tfplan --max-attrs 0`.
- Add `--raw-dir .squeeze` to keep the full input (ignore `.squeeze/` in git) and read that file only when
  the squeezed output lacks what you need; the raw copy is the tool's output as is, secrets included.
```

## Formats

| Format | Input | Docs |
|---|---|---|
| `tfplan` | `terraform show -json <planfile>` | [docs/formats/tfplan.md](docs/formats/tfplan.md) |
| `tfvalidate` | `terraform validate -json` | [docs/formats/tfvalidate.md](docs/formats/tfvalidate.md) |
| `tflint` | `tflint --format json` | [docs/formats/tflint.md](docs/formats/tflint.md) |
| `checkov` | `checkov -o json` | [docs/formats/checkov.md](docs/formats/checkov.md) |
| `trivy` | `trivy fs`, `config` or `image` with `--format json` | [docs/formats/trivy.md](docs/formats/trivy.md) |
| `pytest` | pytest's terminal output, default, `-v` or `-q` | [docs/formats/pytest.md](docs/formats/pytest.md) |
| `tfjson` | `terraform plan`, `apply`, `destroy`, `init` or `test` with `-json` | [docs/formats/tfjson.md](docs/formats/tfjson.md) |
| `gotest` | `go test -json` | [docs/formats/gotest.md](docs/formats/gotest.md) |
| `generic` | any other text, when no format above detects it | [docs/formats/generic.md](docs/formats/generic.md) |

Each format decides what to keep. Every format guarantees:

- Output is plain text with no control characters or color codes, is the same for the same input, and ends with a newline.
- Secrets the tool marks as sensitive never reach stdout.
- Items and lines cut for size are announced with a count and the flag that shows them. Single values (a diagnostic detail, a code line, a title, an attribute value) have fixed limits instead, listed on each format's page, and `--output json` carries them in full where the format's data holds them.
- Errors and warnings are never dropped, whatever their shape. `generic` has no tool to mark them, so it keeps lines with error or warning words first and counts the ones past its line limit in the cut notice.
- Names and addresses from the input are printed on one line without control characters or terminal escape sequences, so the input cannot forge a line of output.

## Common flags

| Flag | Default | Effect |
|---|---|---|
| `--output` | `text` | `text`, or `json` for the envelope below |
| `--raw-dir` | none | save the unmodified input as `<sha256 prefix>` (mode 0600) and print the path on stderr, so the full output stays one read away; the file holds every value the squeezed output hid, secrets included |
| `--fail-on` | none | exit 3 when the input has any of these comma-separated conditions; each format lists its conditions in `squeeze <format> -h` and on its docs page. Three names are shared and always accepted: `error` (a diagnostic, an operation error or, for `generic`, an error word), `fail` (the run's verdict is a failure) and `warning`; a format whose tool has no such channel lists it in `-h` as never present |
| `--version` | | print the version; root command only |

`--output json` prints one line:

```
{"schema_version":1,"format":"tfplan","input_bytes":6294,"text_bytes":485,"ratio":0.0771,"conditions":["change","destroy","replace"],"failed_on":[],"data":{...}}
```

`data` is the format's own structure, described on its docs page. `ratio` is `text_bytes / input_bytes`. `conditions` lists every condition present in the input and `failed_on` the ones `--fail-on` matched.

Until 1.0.0 the text output and the `data` of a format may change between commits; the envelope keys above and the exit codes do not.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | output written |
| 1 | empty input, or a read, parse or write error; stdout is empty |
| 2 | the input matches no known format and is not text; name one |
| 3 | `--fail-on` matched; stdout holds the full output and stderr names the conditions |
| 64 | invalid flags or arguments, including an unknown `--fail-on` condition |

Without `--fail-on`, the changes and failures the input reports never change the exit code; only input squeeze cannot read, detect or parse does.

## Development

```
just check          # gofmt, go vet, staticcheck, exhaustive, go mod tidy, tests
just cover          # coverage across packages, fails below 95 percent
just golden         # rewrite golden files after an intended output change
just fuzz tfplan 30 # fuzz one format for 30 seconds
just fuzz-all 20    # fuzz every format; fails for a format without FuzzSqueeze
just bench          # benchmarks with the output/input ratio (tfplan only)
just conformance    # run the Terraform formats over Terraform's own command testdata at a pinned commit
just demo           # the README example, from the replace-forced fixture
just build          # bin/squeeze with the version from git describe
just measure <model> # tokens of an agent with and without squeeze; results in bench/README.md
```

Go 1.26 or later. Linters run through `go tool`, pinned in `go.mod`; nothing to install besides Go and `just`.

New formats follow [docs/adding-a-format.md](docs/adding-a-format.md). Coding agents working on this repository follow [AGENTS.md](AGENTS.md).


## License

MIT
