# junit

Input: a JUnit XML test report, the file that `pytest --junitxml`, `gotestsum --junitfile`, `go-junit-report`, Maven Surefire, `jest-junit`, `checkov -o junitxml` and many CI tools write. JUnit XML has no single official schema; squeeze reads the elements and attributes of the Apache Ant schema (`JUnit.xsd`, https://github.com/windyroad/JUnit-Schema/blob/master/JUnit.xsd), the Surefire test report schema 3.0.2 (https://maven.apache.org/surefire/maven-surefire-plugin/xsd/surefire-test-report.xsd) and pytest's writer (`src/_pytest/junitxml.py` at pytest 8.4.2). Detected when the first element of the input, after an optional byte order mark, XML declaration, comments and doctype, is `testsuites` or `testsuite`; any other XML document is not claimed.

```
pytest --junitxml=report.xml; squeeze < report.xml
gotestsum --junitfile report.xml -- ./...; squeeze < report.xml
squeeze --fail-on fail < target/surefire-reports/TEST-com.example.shop.CartTest.xml
```

A JUnit report holds every test case, passed or not, the run's properties, and for each failure the full stack trace and captured output. squeeze keeps the counts and the failing cases with their message and the first lines of their text. On the fixtures captured from pytest 8.4.2, gotestsum 1.12.3, go-junit-report 2.1.0, checkov 3.3.20 and jest 30.0.5 with jest-junit 16.0.0, the text is 27 to 65 percent of these small reports (between 2,029 and 6,365 bytes); `generic` printed 91 to 111 percent of each.

## Output

```
$ squeeze < internal/formats/junit/testdata/surefire/input.xml
junit: 6 tests in 1 suite: 2 passed, 2 failures, 1 error, 1 skipped, 1 flaky
fail com.example.shop.CartTest totalAppliesQuantity: expected: <4> but was: <2>
  org.opentest4j.AssertionFailedError: expected: <4> but was: <2>
      at org.junit.jupiter.api.AssertionFailureBuilder.build(AssertionFailureBuilder.java:151)
  ...
  ... 2 more lines (--max-lines 0)
  stdout:
    cart has 1 line
    price list loaded from /home/dev/shop/prices.csv
fail com.example.shop.CheckoutTest checkoutTimesOut: execution timed out after 5 s
error com.example.shop.PaymentTest paymentGateway: Connection refused: localhost/127.0.0.1:8089
  java.net.ConnectException: Connection refused: localhost/127.0.0.1:8089
  ...
  stderr:
    WARN retrying payment gateway in 100 ms
```

- The header counts the tests, the suites (`testsuite` elements, nested ones included) and the outcomes. A test is a distinct `classname` and `name` in one suite: pytest writes a second `testcase` element with the same name for an error in teardown after a failure, and counts the test once (`junitxml.py:647-653`). A case with a `failure` element is a failure, one with an `error` element is an error, one with only `skipped` is skipped, and one with only Surefire's `flakyFailure` or `flakyError` (it failed, then passed on a rerun) is flaky and counts as passed.
- The counts come from the elements. When the report's own `tests`, `failures`, `errors` or `skipped` attributes say otherwise, the header ends with `the report's attributes say N <count>`; jest-junit writes a `test.todo` as a `testcase` but leaves it out of `tests`. The attribute is read from the root element, or summed over the top-level suites when the root has none.
- Each failure prints as `fail <classname> <name>: <message>` and each error as `error <classname> <name>: <message>`, failures first, in report order; the class name is left out when it is empty or the name starts with it (jest-junit), and the message when the name already holds it (checkov). A message is cut at 300 characters with `...`. When the element has no `message` attribute (jest-junit), the first line of its text is the message.
- Under each case come the first `--max-lines` lines of the failure text, then the case's `system-out` and `system-err` under `stdout:` and `stderr:`, each cut the same way; a cut prints as `... N more lines (--max-lines 0)`. Blank lines and rule lines such as pytest's `---- Captured Out ----` are dropped. Output of passing cases never prints.
- A suite with a failing case also prints its own `system-out` and `system-err` as `suite <name> stdout:`, where go-junit-report puts a panic that stopped the test binary.
- Cases past `--max-cases` are counted in `... N more failing cases (--max-cases 0)`.
- Tabs print as four spaces; control characters and terminal escape sequences are removed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-cases` | `30` | failing cases listed, `0` for all |
| `--max-lines` | `10` | lines of each failure text, system-out and system-err, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | a case has an `error` element, or the `errors` attribute is above 0 |
| `fail` | a case has a `failure` or `error` element, or the `failures` or `errors` attribute is above 0 |
| `flaky` | a case passed on a rerun after a Surefire `flakyFailure` or `flakyError` |
| `skipped` | a case was skipped, or the `skipped` attribute is above 0 |

## JSON data

`data` holds `tool` (`junit`), `tool_version` (empty: the report names no tool), `suites`, `tests`, `passed`, `failures`, `errors`, `skipped`, `flaky`, `attributes` (`tests`, `failures`, `errors` and `skipped` as the report states them, `null` when absent), `mismatches` (`count`, `counted`, `attribute`), `cases`, `cases_omitted` and `suite_output`. Each case has `kind` (`failure` or `error`), `suite`, `classname`, `name`, `file` and `line` (when the report has them), `type`, `message` (whole), and `text`, `stdout` and `stderr`, each with the printed `lines` and `lines_omitted`. Each `suite_output` entry has `suite`, `stdout` and `stderr`.

## Limitations

- The tool that wrote the report is not named: JUnit XML has no field for it.
- The failure text shows its first lines. A Java or JavaScript stack trace often has the useful frame further down; `--max-lines 0` prints it all.
- Surefire's `rerunFailure` and `rerunError` are not printed; the `failure` or `error` of the same case is.
- The XML must be well formed and in UTF-8 or ASCII. A report with a raw control character, which XML 1.0 does not allow, is an error (exit 1).
