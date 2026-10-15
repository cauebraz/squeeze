# cargo

Input: what `cargo build`, `cargo check`, `cargo clippy` or `cargo test` print, best with stderr folded into stdout (`2>&1`):

- the JSON messages of `--message-format=json`, one object per line on stdout (https://doc.rust-lang.org/cargo/reference/external-tools.html#json-messages). squeeze reads `compiler-message`, whose `message` is rustc's JSON diagnostic (https://doc.rust-lang.org/rustc/json.html#diagnostics), and `build-finished`; `compiler-artifact` and `build-script-executed` are skipped;
- the text of the libtest harness that `cargo test` runs: `running N tests`, one `test <name> ... ok|FAILED|ignored` line per test, the `---- <name> stdout ----` block of each failure and one `test result:` line per test binary;
- cargo's own status lines on stderr: `Running <target> (<binary>)`, `Doc-tests <crate>`, `Finished`, `error: could not compile`, `error: test failed, to rerun pass ...` and any other `error:` or `warning:` line, with its `Caused by:` block;
- rustc's human diagnostics, when `cargo test` or `cargo build` runs without `--message-format=json`: the `error[E0308]: ...` or `warning: ...` header, its `-->` location, the first source line and the first `^^^` label.

Detected when a line starts with `{"reason":"compiler-message"`, `{"reason":"compiler-artifact"`, `{"reason":"build-script-executed"` or `{"reason":"build-finished"`; when a line is `error: could not compile` or cargo's `Finished ... target(s) in` line; or when the input has both a `running N tests` line and a `test ... ok|FAILED|ignored` or `test result:` line.

```
cargo clippy --message-format=json 2>&1 | squeeze
cargo test --no-fail-fast 2>&1 | squeeze --fail-on fail
cargo test --no-fail-fast --message-format=json 2>&1 | squeeze
```

`--message-format=json` repeats each diagnostic three ways: the structured fields, the `rendered` text with the source excerpt, and for an error code the full `explanation` from `rustc --explain`. squeeze keeps one line per diagnostic built from the structured fields and never prints `rendered` or `explanation`. On the fixtures, captured from cargo 1.99.0, rustc 1.99.0 and clippy 0.1.99, the text is 2 to 15 percent of the JSON input, and 35 to 45 percent of `cargo test` text.

## Output

```
$ squeeze < internal/formats/cargo/testdata/test/input.txt
cargo: fail; build ok; 0 errors, 1 warning; 11 tests: 5 passed, 5 failed, 1 ignored in 5 targets, 3 failed
warning: unused variable: `unused` (shop/src/lib.rs:34:13, unused_variables)
  help: if this is intentional, prefix it with an underscore: `_unused`
  > let unused = 1;
failed target unittests src/lib.rs (shop): 2 passed, 3 failed, 1 ignored; rerun: cargo test -p shop --lib
  fail tests::discount_ten_percent (shop/src/lib.rs:40:9)
    assertion `left == right` failed
      left: 181
     right: 180
...
failed target doc-tests shop: 1 passed, 1 failed, 0 ignored; rerun: cargo test -p shop --doc
  fail shop/src/lib.rs - discount (line 17)
    assertion `left == right` failed
      left: 91
     right: 90
    Test executable failed (exit status: 101).
```

- The header gives the verdict, the build state, the diagnostic counts and, when tests ran, the counts summed from every `test result:` line and the number of test binaries ("targets").
- The build is `failed` when `build-finished` has `success: false` or cargo prints `error: could not compile`; the header names each unit that did not compile, such as `shop (lib)`. It is `ok` on `success: true`, a `Finished` line or test results, and left out when the input shows neither.
- Each diagnostic prints with `file:line:column` of its primary span, its code (`E0308`) or lint name (`unused_variables`, `clippy::eq_op`) and `fixable` when one of its suggestions is `MachineApplicable`, the applicability that `cargo fix` and `cargo clippy --fix` apply. The detail line holds the primary span's label, the labels of the other spans with their line, and the `help:` and `note:` children; a suggested replacement of one line up to 60 bytes is quoted after its help, except a `HasPlaceholders` one. The `#[warn(...)] on by default` notes and clippy's "for further information visit" link are dropped. The `>` line is the first source line of the primary span.
- `note`, `help` and `failure-note` messages at the top level, such as "Some errors have detailed explanations", are not diagnostics. A level rustc adds later counts as a warning, so it is not dropped.
- Diagnostics are sorted errors first, then warnings, and by file, line and column. Identical diagnostics print once with `(xN)`; the header counts each.
- Any other `error:` or `warning:` line from cargo becomes a diagnostic, with the lines of its `Caused by:` block as the detail. For a build script that panics, the location is the panic's `file:line:column` and the binary path is dropped from "process didn't exit successfully".
- cargo's `warning: build failed, waiting for other jobs to finish...`, `warning: <crate> generated N warnings` and `error: N targets failed:` summaries are not diagnostics; the build state and the targets carry them.
- A failed target prints with its counts and, from cargo's `error: test failed, to rerun pass <args>` line, the command that reruns it. A test binary that stops without a `test result:` line, such as one killed by a signal, prints `did not finish` with the lines of cargo's `Caused by:` block.
- Each failed test prints with the location of its panic and the panic message, which holds the `left:` and `right:` lines of `assert_eq!`, followed by what the test printed. The `note: run with RUST_BACKTRACE=1` line and a doc test's temporary file location are dropped. Failures are sorted by name within a target.
- Targets are named from cargo's `Running` line: `unittests src/lib.rs (shop)` with the crate from the test binary's name, or the path of an integration test, and `doc-tests <crate>`. Without cargo's stderr they are `test binary N`.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

Fixed limits, kept whole in `--output json`: a source line is cut at 120 bytes, the detail at 3 lines of 100 characters, and a failure or cause line at 200 bytes.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-diagnostics` | `50` | compiler diagnostics listed, `0` for all |
| `--max-failures` | `20` | failed tests listed, `0` for all |
| `--max-output` | `10` | panic and output lines per failed test, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `build_failed` | `build-finished` has `success: false`, or cargo prints `error: could not compile` |
| `error` | a diagnostic is an error |
| `fail` | the build failed, a diagnostic is an error, a test failed or a test binary did not finish |
| `fixable` | a diagnostic has a `MachineApplicable` suggestion |
| `ignored` | a `test result:` line counts an ignored test |
| `warning` | a diagnostic is a warning |

## JSON data

`data` holds `tool` (`cargo`), `tool_version` (empty: cargo's messages carry none), `status` (`ok` or `fail`), `build` (`ok`, `failed` or empty), `failed_units`, `errors`, `warnings`, `fixable`, `codes` (`name`, `count` for every code or lint name, most first), `diagnostics`, `diagnostics_omitted`, `omitted_by_code` (`no code` for a diagnostic without one), `tests` (`passed`, `failed`, `ignored`, `measured`, `filtered_out`), `targets`, `failures` and `failures_omitted`. Each diagnostic has `severity`, `summary`, `id` (the code or lint name), and when set `detail`, `address` (the id again), `file`, `line`, `column`, `code` (the source line), `count` and `fixable`. Each target has `name`, `result` (`ok`, `FAILED`, or empty when it did not finish), the five counts, and when set `rerun` and `cause`. Each failure has `target`, `test`, `file`, `line`, `column`, `message` and `output`.

## Limitations

- A line of a test's output that starts with `... ` prints with a `| ` prefix, so it cannot be read as one of squeeze's cut notices.
- libtest's JSON output (`cargo test -- -Z unstable-options --format json`) is not read: on stable Rust 1.99.0 it fails with "the option `Z` is only accepted on the nightly compiler".
- The libtest text is not a documented format; the parser follows the output of Rust 1.99.0 and also reads the `panicked at '<message>', <file>` line of Rust before 1.73.
- From rustc's human output, squeeze reads no suggestion, so no diagnostic is `fixable`, and the lint name only from the `#[warn(...)]` note, which rustc prints only with the first warning of each lint. Use `--message-format=json` for both.
- `cargo nextest` output is not read, and output that tests print with `--nocapture` while others run can split a `test ... ok` line; such a test is counted from the `test result:` line only.
- An integration test target is named by its path only, so two packages with the same `tests/<name>.rs` print the same name.
