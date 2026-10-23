# jest

Input: the JSON report that `jest --json` prints on stdout, or the report of Vitest's `json` reporter.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Run Jest as `npx jest --json 2>/dev/null | squeeze` and Vitest as
  `npx vitest run --reporter=json --outputFile=/dev/stdout | squeeze`; rerun a failing file with the path it prints.
```

## Example

```
npx jest --json 2>/dev/null | squeeze
npx vitest run --reporter=json --outputFile=/dev/stdout | squeeze
```

The report carries every test with its full name and a failure message per failed assertion with the whole stack.
Jest repeats the same failures in each file's `message` with code frames.
squeeze keeps the failing tests grouped by file with their messages, the files that failed to run and the counts.
The fixtures were captured from Jest 30.5.2 and Vitest 5.0.2 on the same scratch project.
The text is 9.5 percent of the Jest report (2,480 of 26,217 bytes), 19 percent of the Vitest report (1,490 of 7,799) and 8.0 percent of a run with 57 failing tests (12,979 of 162,848).
Jest's own text report of the same run is 12,479 bytes on stderr, and 12,231 with `AI_AGENT` set.

`jest --json` sends its usual report and the tests' console output to stderr.
`2>/dev/null` keeps them out of the pipe, since squeeze reads the JSON only when the input starts with it.
Vitest 5 writes the report to `.vitest/json/output.json` unless `--outputFile` names a file, and prints `JSON report written to <file>` on stdout.
With `--outputFile=/dev/stdout` that line follows the JSON and squeeze drops it.
`squeeze < .vitest/json/output.json` reads the file.

Jest's report is `FormattedTestResults` in `@jest/test-result` (https://jestjs.io/docs/cli#--json).
Vitest documents its report as compatible with Jest's (https://vitest.dev/guide/reporters#json-reporter).
The report names no version; `data.tool` is `jest` when it carries Jest's `numRuntimeErrorTestSuites` and `vitest` otherwise.

- Detection: the input, ignoring whitespace, starts with `{"num` and holds the keys `numTotalTests`, `numFailedTestSuites` and `testResults`.

## Output

```
$ squeeze < internal/formats/jest/testdata/run/input.json
jest: fail; 14 tests: 8 passed, 4 failed, 1 skipped, 1 todo; 6 files: 1 passed, 2 failed, 3 failed to run; 1 snapshot: 1 failed
test/broken.test.js: failed to run
    SyntaxError: test/broken.test.js: Unexpected token, expected "," (4:26)
    > 4 |   expect(total([]).toBe(0);
        |                           ^
    ... 81 node_modules, node internal and code frame lines (--full)
test/cart.test.js
  fail cart > applyDiscount > takes ten off with TEN
    Error: expect(received).toBe(expected) // Object.is equality
    Expected: 80
    Received: 90
        at Object.toBe (test/cart.test.js:17:41)
    ... 18 node_modules, node internal and code frame lines (--full)
...
... 1 skipped test, 1 todo test, 1 passing file (--all)
```

- The header counts tests as the tool does, from `numTotalTests`, `numPassedTests`, `numFailedTests`, `numPendingTests` (skipped) and `numTodoTests`. Files are counted from `testResults`, one entry per test file: Vitest's `numTotalTestSuites` also counts `describe` blocks, so it is not used. Snapshots print when one failed (`unmatched`), is obsolete (`unchecked`), was written (`added`) or updated, or a snapshot file was removed.
- The status is `fail` when the report's `success` is false or a test or file failed. Jest sets `success` to false for a snapshot failure too, which includes obsolete snapshots unless `-u` was given.
- A file failed to run when its `message` holds an error that is not a failed test: a syntax error, a module that cannot be found, an exception in a `beforeAll` or `afterAll` hook. For Jest, the `message` blocks headed `Test suite failed to run`, or by a name that is not a failed test, are kept, and the blocks that repeat a failed test are dropped. For Vitest, the whole `message` is the file's error. These files print first in their group and are never cut by `--max-tests`.
- Failing tests print under their file, in file name order, as `fail <describe> > <test>`, followed by their failure messages. The `>` separator replaces Jest's `›`.
- Message lines are cut to `--max-output`. Stack frames in `node_modules`, in `node:` internals or `<anonymous>`, and the context lines of a code frame (all but the `>` line and the caret) are dropped and counted in `... N node_modules, node internal and code frame lines (--full)`; the assertion text, the expected and received values and the diff are kept.
- Paths are relative to the project root: the deepest directory above the test files that holds the `node_modules` named in the stacks, or else the deepest directory that holds every test file; `data.root` names it. `file://` URLs in Vitest stacks print as relative paths.
- Open handles that `--detectOpenHandles` found print as `open handle: <message>` with their stack. A `runExecError` prints as `run error`. A run that Jest reports as interrupted (`wasInterrupted`) says `interrupted` in the header.
- Skipped, todo and passing items are counted in `... N skipped tests, N todo tests, N passing files (--all)`; `--all` lists them, each test with the tool's status (`pending`, `skipped`, `disabled`, `todo`).
- A test status other than `passed`, `failed`, `pending`, `skipped`, `disabled`, `focused` and `todo` is counted in `... N tests with an unknown status: <status> <count> (--raw-dir)`.
- Text after the JSON document, other than Vitest's `JSON report written to` line, prints under `output after the JSON:`.
- Terminal escape sequences, which Jest writes into the messages with `--colors`, are removed; tabs print as four spaces.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also list skipped and todo tests and passing files |
| `--full` | off | keep node_modules and node internal stack frames and code frame context |
| `--max-output` | `20` | message lines per failed test, file error or open handle, 0 for all |
| `--max-tests` | `50` | failed tests listed, 0 for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | a test file failed to run, or the report has a `runExecError` |
| `fail` | the status is `fail`: `success` is false, or a test or file failed |
| `skip` | a test was skipped (`numPendingTests`) |
| `snapshot` | a snapshot did not match, or Jest reports a snapshot failure |
| `todo` | the run has `test.todo` tests |
| `warning` | Jest found open handles (`--detectOpenHandles`) |

## JSON data

`data` holds `tool` (`jest` or `vitest`), `tool_version` (empty: neither report carries one), `status`, `root`, `interrupted`, `tests` (`total`, `passed`, `failed`, `skipped`, `todo`), `files` (`total`, `passed`, `failed`, `failed_to_run`, `skipped`), `snapshots` (`total`, `failed`, `obsolete`, `written`, `updated`, `files_removed`, `failure`), `run_error`, `file_errors` (`file`, `output`), `failures` (`file`, `test`, `line` and `column` of the test when Jest ran with `--testLocationInResults`, `output`), `failures_omitted`, `open_handles` (`name`, `message`, `output`), `not_run` (`file`, `test`, `status`), `passing_files`, `unknown_status` and `trailing`. Each `output` has `lines`, the printed lines, `omitted`, the lines past `--max-output`, and `hidden`, the frames and code frame lines dropped without `--full`.

## Limitations

- Vitest's JSON carries no assertion diff: a failed `toEqual` or snapshot prints `expected { ... } to deeply equal { ... }` or `Snapshot ... mismatched` without the lines that differ. Vitest's own text report shows them.
- Console output from tests is not in either report.
- A failure before the run, such as a `globalSetup` that throws, makes Jest print no JSON: the input is empty and squeeze prints `no output: the command printed nothing`. Read Jest's stderr for the failure.
- With `2>&1`, Jest's stderr text comes before the JSON and the input is not detected; use `2>/dev/null` or `--outputFile`.
- Vitest 5.0.2 leaves `snapshot.failure` false for a mismatch; the mismatch fails its test, so the status is `fail` and `snapshot` is present from `unmatched`.
- Coverage (`coverageMap`, per-file `coverage`) is dropped.
