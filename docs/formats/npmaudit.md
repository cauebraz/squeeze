# npmaudit

Input: the JSON that `npm audit --json` prints, `auditReportVersion` 2, as npm 10.9.8 and 11.21.0 write it (the same keys in both). npm publishes no schema for it; the shape comes from npm's source at npm 10.9.8: `workspaces/arborist/lib/audit-report.js` (`toJSON`, lines 29-90) writes `auditReportVersion`, `vulnerabilities` keyed by package name and `metadata` with the counts by severity and by dependency type, and `workspaces/arborist/lib/vuln.js` (`toJSON`, lines 111-130) writes each vulnerability's `name`, `severity`, `isDirect`, `via`, `effects`, `range`, `nodes` and `fixAvailable`. Detected when the input, ignoring whitespace, starts with `{"auditReportVersion":` and holds the key `vulnerabilities`.

```
npm audit --json | squeeze
npm audit --json --omit=dev | squeeze --fail-on high
```

npm's JSON carries, for every advisory behind a vulnerable package, its title, URL, CWE list, CVSS vector and vulnerable range, and lists each package's dependents and paths in `node_modules`; its text report repeats the advisories under every dependent. squeeze keeps one line per vulnerable package with severity, range, whether it is a direct dependency, the fix and the advisory IDs. On the fixtures, captured from npm 10.9.8, the text is 7.7 to 7.8 percent of the JSON (2,682 of 34,534 bytes for 25 vulnerabilities), and npm's own text report for the same project is 9,672 bytes.

## Output

```
$ squeeze < internal/formats/npmaudit/testdata/vulnerable/input.json
npm audit: 25 vulnerabilities (3 critical, 13 high, 4 moderate, 5 low) in 301 dependencies; 7 direct; fix: 1 with npm audit fix, 19 with npm audit fix --force (1 breaking), 5 without one
critical form-data <=2.5.5: no fix; GHSA-fjxv-7rqg-78g4, GHSA-hmw2-7cc7-3qxx; required by request
critical minimist 1.0.0 - 1.2.5 direct: fix minimist@1.2.8 (--force); GHSA-xvch-5gv4-984h, GHSA-vh95-rmgr-6w4m
...
high jsonwebtoken <=8.5.1 direct: fix jsonwebtoken@9.0.3 (--force, breaking); GHSA-8cf7-32gw-wr33, GHSA-hjrf-2m68-5959, GHSA-qwph-4952-7xr6
...
high mocha 5.1.0 - 12.0.0-beta-2 direct: fix mocha@8.4.0 (--force); via chokidar, debug, diff, js-yaml, minimatch, serialize-javascript
...
low serve-static <=1.16.0: fix with npm audit fix; GHSA-cm22-4g7w-348p
```

- The header counts the vulnerable packages by severity, most severe first, as npm's `metadata.vulnerabilities` does (npm derives those counts from the same entries, `audit-report.js` lines 77-80), and gives `metadata.dependencies.total`, how many vulnerable packages are direct dependencies (`isDirect`) and how many have each kind of fix.
- Packages are sorted by npm's severity order, `critical`, `high`, `moderate`, `low`, `info` (`vuln.js` lines 20-27), then by name.
- The fix follows `fixAvailable` (`vuln.js` lines 58-61): `true` prints `fix with npm audit fix`; an object prints `fix <name>@<version> (--force)`, the package and version that `npm audit fix --force` installs, with `, breaking` when `isSemVerMajor` is true, which npm's text report calls "a breaking change" (`npm-audit-report` `lib/reporters/detail.js` line 63); `false` prints `no fix`.
- An entry in `via` that is an advisory prints as its ID, the last part of its `url` (`GHSA-...`), or npm's numeric `source` when there is no URL; the most severe advisories come first and a repeated ID prints once. A package whose `via` names only other vulnerable packages, because it depends on them, prints `via <packages>`.
- A transitive package that `npm audit fix --force` cannot fix by updating a named package (`no fix`, or a fix by `npm audit fix`) prints `required by <effects>`, the packages that depend on it, so the dependency to change or replace is named.
- Past `--max-vulnerabilities`, the least severe packages are cut and counted by severity: `... 20 more vulnerabilities, high 11, moderate 4, low 5 (--max-vulnerabilities 0)`.
- Advisory titles, URLs, CWE lists, CVSS scores and vulnerable ranges, `effects` and `nodes` are in `--output json` only.
- When npm prints its error object instead of a report, `{"error":{"code","summary","detail"}}` (for example `ENOLOCK` without a lockfile, `lib/utils/output-error.js` lines 16-24) or, when the registry cannot be reached, a `message` beside an empty `error` (`lib/utils/audit-error.js` lines 24-37, merged by `lib/utils/display.js` lines 103-138), `squeeze npmaudit` exits with code 1 and prints npm's code, summary, detail and message on stderr. Auto-detection does not claim npm's error object, which every npm command prints with `--json`; `generic` reads it.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-vulnerabilities` | `50` | vulnerable packages listed, `0` for all |

## Conditions for `--fail-on`

npm audit exits with 1 when a vulnerability is at or above `--audit-level`, `low` by default (`npm-audit-report` 6.0.0, `lib/exit-code.js` and `lib/index.js` line 21). Each severity condition is that level: `--fail-on high` matches when `npm audit --audit-level=high` exits with 1.

| Condition | Present when |
|---|---|
| `critical` | a vulnerable package has severity `critical` |
| `fail` | `low`: npm audit's default `--audit-level` |
| `fixable` | a vulnerable package has a fix, by `npm audit fix` or `npm audit fix --force` |
| `high` | a vulnerable package has severity `high` or `critical` |
| `info` | a vulnerable package has any severity |
| `low` | a vulnerable package has severity `low` or above |
| `moderate` | a vulnerable package has severity `moderate` or above |

A severity outside npm's five is reported as `unknown`, sorted first and counted as above `critical` for every condition, so an unexpected value cannot hide a failure; npm itself would not count it.

## JSON data

`data` holds `tool` (`npm`), `tool_version` (empty: the report carries none), `audit_report_version`, `dependencies` (`prod`, `dev`, `optional`, `peer`, `peer_optional`, `total`, as npm counts them), `total`, `by_severity` (`name`, `count`, most severe first), `direct`, `fixes` (`auto` by `npm audit fix`, `force` by `--force` and not breaking, `breaking`, `none`), `vulnerabilities`, `vulnerabilities_omitted` and `omitted_by_severity`. Each vulnerability has `name`, `severity`, `range`, `direct`, `fix` (`available` and, for a `--force` fix, `force`, `name`, `version` and `semver_major`), `advisories` (`id`, `source`, `title`, `url`, `severity`, `range`, `cwe`, `cvss_score`), `via` (the vulnerable packages it depends on), `effects` and `nodes`.

## Limitations

- `fail` follows npm's default `--audit-level`. A run with `--audit-level=<level>` fails on that level; use `--fail-on <level>` there.
- `auditReportVersion` 1, the npm 6 report with `advisories` and `actions`, exits with code 1 and names the version.
- `pnpm audit --json` (pnpm 12.10.1) prints the npm 6 shape, `advisories` keyed by ID with `metadata`, and `yarn audit --json` (Yarn 1.22.22) prints one JSON object per line (`auditAdvisory`, `auditSummary`); neither is read, and both go to `generic`. `yarn npm audit --json` (Yarn 2 and later) was not checked.
- npm's text report (`npm audit` without `--json`) is not read and goes to `generic`; run npm audit with `--json`.
- `npm audit --json` counts `metadata.dependencies` over the whole tree with `--omit=dev` too; the header repeats npm's count.
