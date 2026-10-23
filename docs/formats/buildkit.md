# buildkit

Input: the progress output that `docker buildx build` and `docker build` write to stderr, with `--progress=plain` or `--progress=rawjson`.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Build images as `docker buildx build --progress=plain . 2>&1 | squeeze`.
```

## Example

```
docker buildx build --progress=plain . 2>&1 | squeeze
docker buildx build --progress=rawjson . 2>&1 | squeeze buildkit --all
```

A plain build log repeats each step's name and prints transfer and export status lines.
It prints the last lines of the failing step's log twice, in the step and in the `------` tail, and its command five times.
squeeze keeps the counts, every BuildKit warning, the failed and canceled steps with the tail of their log, and the final error with its Dockerfile location.
On the failing fixtures captured for this format, the text is 24 to 35 percent of the plain log and 3.5 to 10 percent of the rawjson stream.
`generic` printed 1,996 bytes of the 2,777-byte `failed-run` log, against 966.

The two modes:

- `--progress=plain`, the text CI logs show and the default when stderr is not a terminal: `#N <step>` lines, `#N <seconds> <log line>`, `#N DONE`, `#N CACHED`, `#N CANCELED`, `#N ERROR: <message>`, `#N WARN: <message>`, then the `------` log tail, the Dockerfile excerpt and `ERROR: failed to build: ...`.
- `--progress=rawjson`, one `SolveStatus` JSON object per line (`vertexes`, `statuses`, `logs` with base64 data, `warnings`), as defined in `client/graph.go` of moby/buildkit and written by the `rawjson` display in `util/progress/progressui/display.go`. The lines buildx prints after the stream (the Dockerfile excerpt and `ERROR:`) are read as in plain mode.

- Detection: one of the first three non-blank lines is `#0 building with "...`, a `#N [internal] load build definition|.dockerignore|metadata for|remote build context|git source` line (a Compose service name may precede `internal`), or a JSON object that starts with `{"vertexes":[{"`, `{"statuses":[{"`, `{"logs":[{"` or `{"warnings":[{"`.

## Output

```
$ squeeze < internal/formats/buildkit/testdata/canceled/input.log
buildkit: 6 steps, 1 cached, 1 failed, 1 canceled
canceled [assets 2/2] RUN echo "bundling assets" && sleep 5 && echo done
  bundling assets
fail [app 2/2] RUN echo "running tests" && echo "FAIL: TestCheckout (0.02s)" && echo "    cart_test.go:41: total 12, want 10" && exit 2
  running tests
  FAIL: TestCheckout (0.02s)
      cart_test.go:41: total 12, want 10
error: Dockerfile:5: failed to build: failed to solve: process "/bin/sh -c echo \"running tests\" ..." did not complete successfully: exit code: 2
... 4 other steps (--all)
```

- The header counts the steps BuildKit shows, the cached, failed and canceled ones, then the warnings.
  - A step is one `#N` in plain mode and one started vertex in rawjson; a vertex in a progress group counts as its group.
  - Cached is `CACHED` or `"cached":true`; failed is `ERROR:` or a vertex `error`.
  - Canceled is `CANCELED` or an `error` ending in `context canceled`, the rule of BuildKit's own display.
- Each warning prints as `warning: <message>`: the `#N WARN:` lines or the `short` text of a rawjson warning, buildx's `WARNING:` lines, and any entry of the closing `N warnings found` list that no step printed.
- A failed or canceled step prints as `fail <step>` or `canceled <step>` with its log cut to the last `--max-lines` lines, keeping every earlier line with an error or warning word and the line after it; each gap prints as `... N lines cut (--max-lines 0)`. The step's error prints as `error: <message>` unless the final error already ends with it.
- The final `ERROR:` lines print as `error: <message>`, the first one prefixed with the Dockerfile location buildx shows above it (`Dockerfile:5`). A line after the build that squeeze does not know and that holds an error word is printed as an error too.
- Other steps are counted in `... N other steps (--all)`; `--all` prints every step in order with its status (`done`, `cached`, `fail`, `canceled`, `running`) and its log, cut the same way.
- Dropped: status lines (`transferring`, `resolve`, `exporting`), timestamps, the `------` log tail, the Dockerfile excerpt and, in rawjson, `statuses` and each warning's `sourceInfo`, which carries the whole Dockerfile.
- Terminal escape sequences are removed; tabs print as four spaces.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | print every step with its log |
| `--max-lines` | `20` | log lines of each failed step, 0 for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | a step failed or the build printed an `ERROR:` line |
| `fail` | `error`: the build did not complete |
| `warning` | BuildKit or buildx printed a warning |

## JSON data

`data` holds `tool` (`buildkit`), `tool_version` (empty: neither mode prints the BuildKit version), `progress` (`plain` or `rawjson`), `builder` (the instance named on the `#0 building with` line, empty in rawjson), `total_steps`, `cached_steps`, `failed_steps`, `canceled_steps`, `warnings`, `errors` (the final `ERROR:` messages), `location`, `steps` (the printed steps) and `other_steps`. Each warning has `step` (the step that reported it, empty for a buildx warning), `message`, `detail` and `url` (rawjson only). Each step has `name`, `status`, `error`, `output` (the printed lines, cut notices included) and `lines_omitted`.

## Secrets

BuildKit marks no value as sensitive in its progress output. `--secret` values never appear unless a step prints them, which BuildKit does not redact. A `--build-arg` value is expanded into the step name when the step uses the argument as `$NAME` or `${NAME}` (`RUN echo "arg is $APP_TOKEN"` shows as `[2/3] RUN echo "arg is <value>"`), and it appears in that step's log if the step prints it; squeeze prints step names and failed logs as BuildKit wrote them. BuildKit's `SecretsUsedInArgOrEnv` warning flags such arguments, and squeeze always prints it. The rawjson `sourceInfo` of a warning, which holds the Dockerfile with any `ENV` value in it, is never printed.

## Limitations

- In plain mode a log line that BuildKit split across two writes continues on a line without the `#N` prefix; squeeze prints the remainder as its own log line.
- The step log is printed as text. A test runner's output inside a `RUN` step is not read by that runner's format.
- The legacy builder's output (`Step 1/4 : FROM ...`, `DOCKER_BUILDKIT=0`) is not detected and goes through `generic`.
- `--progress=tty` output is redrawn with cursor movements; squeeze reads it only through `generic`. Pass `--progress=plain` or `--progress=rawjson`.
