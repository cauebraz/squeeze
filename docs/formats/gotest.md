# gotest

Input: the JSON stream that `go test -json` prints, one `TestEvent` per line as `go doc cmd/test2json` describes it, with the `BuildEvent` lines of `go help buildjson` mixed in. Detected when one of the first twenty non-blank lines is a JSON object with a known `Action` and a `Package` or `ImportPath`; `go: downloading` and `# package` lines before the stream do not count toward the twenty. Lines that are not JSON, which the go command prints for early errors, are kept.

```
go test -json ./... | squeeze
go test -json -race ./... 2>&1 | squeeze --fail-on fail
```

A `-json` run prints a run line, a framing line and a result event for every test, and every log line of passing tests. squeeze keeps the failures, the build errors, the benchmark results and the counts. On streams captured from real runs the text is 5.8 to 14.5 percent of the input; on the two hand-written fixtures it is 26 and 48 percent, since they hold only a few events.

## Output

```
$ squeeze < internal/formats/gotest/testdata/mixed/input.jsonl
go test: fail; 8 tests: 4 passed, 3 failed, 1 skipped; 4 packages: 1 ok, 1 failed, 1 build failed, 1 without tests
build failed: example.com/shop/broken
  broken/broken.go:3:28: undefined: undefinedName
fail example.com/shop/calc TestAddNegative
  calc_test.go:17: Add(-2, -3) = -5, want -6
      second line of the message
  ... 1 log line (--full)
fail example.com/shop/calc TestDivByZero
  panic: runtime error: integer divide by zero [recovered, repanicked]
  goroutine 21 [running]:
  example.com/shop/calc.Div(...)
      /home/dev/shop/calc/calc.go:5
  example.com/shop/calc.TestDivByZero(0x21448553a248?)
      /home/dev/shop/calc/calc_test.go:43 +0x20
  ... 10 runtime and testing stack lines (--full)
fail example.com/shop/calc TestTable/one
  attr case=one
  calc_test.go:32: Add(1, 0) = 1, want 2
... 1 skipped test, 1 passing package, 1 package without tests (--all)
```

- The header gives `pass` or `fail`, the test counts and the package counts. A package `without tests` is one the go command reports as `[no test files]`.
- A failure line names the package and the test, so `go test <package> -run '^<test>$'` reruns it; `-run` takes a regular expression per `/`-separated level, so a name with `+`, `(`, `[` or `.` needs those escaped, and a subtest is `-run '^Parent$/^Sub$'`. With `-count`, each failed run is listed with `(run N of M)` and the counts cover every run. A parent test that fails only because a subtest failed is not listed or counted; one with output of its own is.
- Under a failure, squeeze prints the lines the test reported with `t.Error` or `t.Fatal` (`OutputType` `error` and `error-continue`), every line from a `panic:` or `fatal error:` on, and every data race report. Other log lines are counted in `... N log lines (--full)`. When the stream carries `OutputType`, only `frame` lines are treated as test framing, so an error line that looks like `--- FAIL: ...` stays. A stream from a Go version without `OutputType` keeps every line of the test and drops lines shaped like framing.
- Stack frames from the Go runtime, the `testing` package and the generated test main are counted in `... N runtime and testing stack lines (--full)`. A frame stays when its package path starts with a domain (a dot in the first element), with the first element of the tested package's path, or with `main.`.
- A test that started but has no result in a package that failed, as after a timeout or a killed binary, is listed with `(did not finish)`. A package without a result is listed the same way, with its own output.
- Build errors are listed per package from the `build-output` events, without the `# package` line. A stream from a Go version before build events reports the package as build failed from its `[build failed]` summary, and the compiler output is in the lines outside the test events.
- Benchmark results print as `bench <package> <result>`, with the spaces collapsed.
- Attributes from `t.Attr` and the directory from `t.ArtifactDir` print under the failure they belong to.
- A `testing: warning:` line, such as `no tests to run`, prints as `warning <package>: ...` and sets the `warning` condition.
- Lines that are not JSON print under `output outside the test events:`.
- An event with an action squeeze does not know is counted in `... N events with an unknown action: <action> <count> (--raw-dir)`; the raw input is the only place to read it.
- Without `--all`, skipped tests, passing packages and packages without tests are counted in one `... N skipped tests, N passing packages, N packages without tests (--all)` line.
- test2json splits a line longer than its 1024-byte output buffer across events; squeeze joins the parts of a test's output before it reads the lines.
- Strings from the stream are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also list skipped tests, passing packages and packages without tests |
| `--full` | off | print every output line of a failure: logs and the full stack |
| `--max-output` | `20` | output lines per failure, build failure or package, `0` for all |
| `--max-tests` | `50` | failed tests listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `build_failed` | a package failed to build |
| `error` | a package failed to build, a test or package did not finish, or a panic, timeout or data race was printed |
| `fail` | a test, a package or a build failed, or a package did not finish |
| `panic` | an output line starts with `panic:`, other than a test timeout |
| `race` | the race detector printed `WARNING: DATA RACE` |
| `skip` | a test was skipped |
| `timeout` | an output line starts with `panic: test timed out` |
| `warning` | the go command printed a `testing: warning:` line |

## JSON data

`data` holds `tool`, `tool_version` (empty: the stream carries none), `status`, `tests` (`passed`, `failed`, `skipped`), `packages` (`ok`, `failed`, `build_failed`, `no_tests`), `build_failures`, `failures`, `failures_omitted`, `package_failures`, `benchmarks` (`package`, `result`), `skipped` (`package`, `test`), `passing_packages` (`package`, `elapsed` when set), `packages_without_tests`, `text`, `warning_messages` (one string per `testing: warning:` line), `panics`, `timeouts`, `races` (counts of matching lines) and `other` (unknown actions by count).

- `failures`: `package`, `test`, `output` and, when set, `run` and `runs` (with `-count`), `elapsed`, `unfinished`, `attrs`, `artifacts`.
- `build_failures`: `package`, `output`. `package_failures`: `package`, `output` and, when set, `unfinished`.
- Every `output` and `text` holds `lines` and, when set, `logs_hidden`, `stack_hidden` and `omitted` (lines past `--max-output`).

## Limitations

- The stack filter decides by function name. A module whose path has no dot and differs from the tested package's first path element has its frames counted as runtime frames; `--full` prints them.
- Failures are sorted by package and test name, not by the order they ran, so parallel runs print the same way every time.
- Detection reads the first twenty non-blank lines after any `go:` and `#` lines; a stream that starts with more text than that needs `squeeze gotest`.
