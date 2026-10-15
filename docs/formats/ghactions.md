# ghactions

Input: the job logs that `gh run view --log` and `gh run view --log-failed` print, one line per log line as `<job>`, a tab, `<step>`, a tab, then the runner's line: an ISO 8601 UTC timestamp and the text. Detected when the first three non-blank lines, or every line of a shorter input, have that shape. A job log downloaded from the API or the web UI, without the job and step columns, is read too with `squeeze ghactions`.

```
gh run view <run-id> --log-failed | squeeze
gh run view <run-id> --job <job-id> --log | squeeze ghactions --all
```

A job log repeats the runner, image and token details, every step's script and environment, the checkout and setup commands and the post-job cleanup, with a timestamp on every line. squeeze keeps, for each job, the step that failed with its output and the runner's `##[error]` and `##[warning]` annotations. On the fixtures captured from failed runs of this repository, the text is 1.6 and 3.1 percent of the two logs with full jobs and 37 percent of the one that holds only the failing step; `generic` printed 19,522 bytes of the 74,257-byte `matrix` log, against 2,319.

## Output

```
$ squeeze < internal/formats/ghactions/testdata/limits/input.log
github actions 2.337.0: 2 jobs, 2 failed; 2 errors
fail check (macos-latest, 1.27.2): just check
  ... 10 lines cut (--max-lines 0)
  --- FAIL: TestDeepListsStayLinear (4.22s)
      diff_test.go:297: diffAttrs() took 4.22283275s
  FAIL
  ...
  error: Process completed with exit code 1.
fail check (macos-latest, 1.26.9): just check
  ...
... 8 other steps (--all)
```

- The header names the runner version (`Current runner version`), the number of jobs, the failed jobs and the annotations.
- Steps are the runner's `##[group]Run <command>` sections; the command names the step, since `gh` often prints `UNKNOWN STEP` in the step column. The lines before the first step are `set up job`, and the lines from `Post job cleanup.` on are `post job cleanup`. The script, shell and environment lines between `##[group]Run` and `##[endgroup]` are dropped, and so are the other `##[...]` markers.
- A step with an `##[error]` annotation failed: it prints as `fail <job>: <command>` with its output and then each error as `error: <message>`. Every `##[error]` and `##[warning]` annotation prints, whatever step it is in; a step with only warnings prints as `step <job>: <command>` with them.
- A failing step's output is cut to its last `--max-lines` lines, keeping every earlier line with an error or warning word and the line after it, which holds the message of a Go `--- FAIL:` line; each gap prints as `... N lines cut (--max-lines 0)`.
- Steps without annotations are counted in `... N other steps (--all)`; `--all` prints every step, `set up job` and `post job cleanup` included, each cut the same way.
- Timestamps, the byte order mark and terminal escape sequences are removed; tabs print as four spaces.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also print the output of the steps that did not fail |
| `--max-lines` | `30` | output lines of the failing step, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | the log has an `##[error]` annotation |
| `fail` | `error`: the runner writes `##[error]Process completed with exit code N.` when a step fails |
| `warning` | the log has an `##[warning]` annotation |

## JSON data

`data` holds `tool` (`github-actions`), `tool_version` (the runner version, empty when the log does not print it), `errors`, `warnings`, `jobs` and `other_steps`. Each job has `name`, `failed` and `steps`, the printed steps; each step has `name`, `failed`, `errors`, `warnings`, `output` (the printed lines, cut notices included) and `lines_omitted`.

## Limitations

- A step is known from its `##[group]Run` line. A failure before the first step, such as a runner that cannot start, prints under `set up job`.
- The output of a failing step is printed as text. A test runner's output inside it, such as `go test` or `pytest`, is not read by that runner's format; run the command locally and pipe it through `squeeze` for that.
- The job and step names come from `gh`; a job log downloaded without them prints its job as `job`.
