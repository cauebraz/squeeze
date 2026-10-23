# squeeze command line

## Usage

```
tool | squeeze [flags]              detect the format; its own flags work too, as a cut notice names them
tool | squeeze <format> [flags]     name the format when detection cannot, or to fail early on a wrong flag
squeeze <format> -h                 list a format's flags and conditions
```

Text that no format detects goes through `generic`, and stderr says so.

Without a format name, a flag of the detected format applies to it, such as `--max-checks 0` from a checkov cut notice.
A flag the detected format does not have exits with code 64 and names the format.
A flag value no format accepts, and a `--fail-on` name no format declares, exit with code 64 before the input is read, so empty input does not hide them.

A flag error is one line on stderr that names the flag with `--` and the command to run next:

```
$ printf 'hello\n' | squeeze --max-attrs 0
squeeze: no format detected; used generic
squeeze: --max-attrs is not a flag of the detected format generic (squeeze generic -h)
```

`squeeze help [format]` works as `-h`, and `squeeze version` as `--version`.

## Common flags

| Flag | Default | Effect |
|---|---|---|
| `--output` | `text` | `text`, or `json` for the envelope below |
| `--raw-dir` | none | save the unmodified input as `<dir>/<sha256 prefix>` (mode 0600) and print the path on stderr; the file holds every value the output hid, secrets included |
| `--fail-on` | none | exit 3 when the input has any of these comma-separated conditions, listed in `squeeze <format> -h` and on the format's page |
| `--version` | | print the version; root command only |

`--raw-dir` writes a temporary file in the directory and renames it, so a failed save leaves no partial file.

Three `--fail-on` names are shared and accepted by every format:

- `error`: a diagnostic, an operation error or, for `generic`, an error word.
- `fail`: the run's verdict is a failure.
- `warning`: the tool reported a warning.

A format whose tool has no such channel lists the name in `-h` as never present.

## JSON envelope

`--output json` prints one line:

```
{"schema_version":1,"format":"tfplan","input_bytes":6294,"text_bytes":485,"ratio":0.0771,"conditions":["change","destroy","replace"],"failed_on":[],"data":{...}}
```

- `data` is the format's own structure, described on its page.
- A value from the input keeps its control characters in `data` as JSON escapes (`\n`, `\u007f`, `\u009b`), so the line holds none.
- `ratio` is `text_bytes / input_bytes`.
- `conditions` lists every condition present in the input; `failed_on` lists the ones `--fail-on` matched.

Until 1.0.0 the text output and a format's `data` may change between commits; the envelope keys and the exit codes do not.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | output written; empty or whitespace-only input prints `no output: the command printed nothing` |
| 1 | a read, parse or write error; stdout is empty and stderr names what the format reads, for example `squeeze: tfplan: input is not valid JSON (...); tfplan reads Terraform plan JSON from terraform show -json` |
| 2 | the input matches no format and is not text: its first 64 KiB hold a NUL byte or invalid UTF-8; stderr names which and where |
| 3 | `--fail-on` matched; stdout holds the full output and stderr names the conditions |
| 64 | invalid flags or arguments, including an unknown `--fail-on` condition |

Without `--fail-on`, the changes and failures in the input never change the exit code.

## Guarantees

Each format decides what to keep. Every format guarantees:

- Output is plain text with no control characters or color codes, is the same for the same input, and ends with a newline.
- Secrets the tool marks as sensitive never reach stdout.
- Items and lines cut for size are announced with a count and the flag that shows them.
- A single value (a diagnostic detail, a code line, a title, an attribute value) has a fixed limit instead, listed on the format's page; `--output json` carries it in full where the format's data holds it.
- Errors and warnings are never dropped. `generic` has no tool to mark them, so it keeps lines with error or warning words first and counts the ones past its line limit in the cut notice.
- Strings from the input, such as names, addresses and messages, print on one line without control characters or terminal escape sequences, so the input cannot forge a line of output.

## Edge cases

- Empty input with `--output json` and no format named prints the envelope with `"format":""`: `{"schema_version":1,"format":"","input_bytes":0,"text_bytes":39,"ratio":0,"conditions":[],"failed_on":[],"data":{}}`.
- stdin that is a terminal or a device, such as `squeeze < /dev/null`, exits with code 1; for empty input use `printf '' | squeeze`.
- A reader that closes the pipe early, as `squeeze | head` can, ends squeeze with SIGPIPE; under `set -o pipefail` the pipeline exits with 141 (128 + 13), which is not a squeeze or tool failure.
