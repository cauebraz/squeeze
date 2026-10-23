# trivy

Input: the JSON that `trivy fs`, `config`, `image`, `k8s` and the other targets print with `--format json`.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Run Trivy as `trivy fs --format json . | squeeze` or `trivy k8s --format json | squeeze`.
```

## Example

```
trivy fs --scanners vuln,secret,misconfig --format json . | squeeze
trivy image --format json registry.example.com/shop/api:1.4.0 | squeeze --fail-on critical
trivy k8s --format json --include-namespaces shop | squeeze --fail-on error,critical
```

Trivy's JSON carries, for every vulnerability, its description, references, CVSS vectors, vendor severities and dates.
For every misconfiguration it carries the description, query, references and code.
squeeze keeps one line per finding with severity, ID, package or resource, and fix, plus one line per misconfiguration check with Trivy's resolution.
On the default-flag fixtures with findings, captured from Trivy 0.75.0, the text is 2.0 to 4.5 percent of the input.
On the container-image fixture written from the schema, which holds few findings, it is 15 percent.

Two shapes are read:

- `SchemaVersion` 2, as `pkg/types/report.go` in Trivy defines it, for `fs`, `config`, `image` and the other targets.
- `trivy k8s --format json`, with `--report all` (the default) or `--report summary`, as `pkg/k8s/report/report.go` defines it at v0.75.0 (lines 39-66): no `SchemaVersion`, a `ClusterName`, and the results of each Kubernetes resource under `Resources`, or `Findings` for the summary report.

- Detection: a JSON object that contains `"SchemaVersion"` and `"ArtifactName"` or `"Results"`, or `"ClusterName"` and `"Resources"` or `"Findings"`, anywhere in the document.

## Output

```
$ squeeze < internal/formats/trivy/testdata/image/input.json
trivy 0.75.0: registry.example.com/shop/api:1.4.0 (container_image, debian 12.7): 5 vulnerabilities, 1 misconfiguration, 2 licenses; 1 CRITICAL, 4 HIGH, 1 LOW, 1 UNKNOWN; 3 fixable; 40 misconfiguration checks passed
registry.example.com/shop/api:1.4.0 (debian 12.7) (debian):
  CRITICAL CVE-2024-45491 libexpat1 2.5.0-1 -> 2.5.0-1+deb12u1: libexpat: Integer Overflow or Wraparound
  HIGH CVE-2023-52425 libexpat1 2.5.0-1 (affected): expat: parsing large tokens can trigger a denial of service
  LOW CVE-2011-3374 apt 2.6.1 (will not fix): It was found that apt-key in apt, all versions, do not correctly validate gpg keys
  UNKNOWN CVE-2024-28085 util-linux 2.38.1-5+b1 -> 2.38.1-5+deb12u1: util-linux: CVE-2024-28085: wall: escape sequence injection
app/package-lock.json (npm):
  HIGH CVE-2024-21538 cross-spawn 7.0.3 -> 7.0.5, 6.0.6: cross-spawn: regular expression denial of service
app/deploy/values.yaml (helm):
  HIGH KSV-0014 Deployment/api lines 30-52: Root file system is not read-only
OS Packages (license):
  HIGH GPL-3.0-only libreadline8: restricted
... 1 misconfiguration exception, 1 license below HIGH (--all)
```

- The header names the artifact, its type and, for an image, its OS. It then counts vulnerabilities, misconfigurations, secrets and licenses, and the listed findings by severity, those cut by `--max-findings` included; misconfiguration exceptions and licenses below `HIGH` are not in these severity counts, the same findings the `critical` and `high` conditions leave out. It ends with how many vulnerabilities have a fixed version and how many misconfiguration checks passed.
- Findings are grouped by Trivy's results, in Trivy's order, with the target name and type; two results with the same target name, such as a `requirements.txt` scanned for packages and for secrets, stay apart. Inside a target they are sorted by kind, severity, package or resource, line and ID.
- A vulnerability prints its package, the installed version and `-> fixed versions`, or Trivy's status (`affected`, `will not fix`, ...) when there is no fix.
- A misconfiguration prints its resource and lines from Trivy's cause metadata. Passed checks are counted in the header; checks with status `EXCEPTION` are counted in `... N misconfiguration exceptions (--all)`.
- A secret prints its rule and line. The `Match` and `Code` fields, which hold the line from the file, are never read, so the secret never reaches the text or the JSON, even where Trivy did not mask it.
- Licenses of severity `HIGH` or `CRITICAL` print with their package and category; the others are counted in `... N licenses below HIGH (--all)`.
- A `fixes:` section closes the output with one line per misconfiguration check listed above, `<ID>: <Resolution>`, in the order the checks first appear; findings cut by `--max-findings` add none, and exceptions add theirs with `--all`. Trivy's `Message` is in `--output json` only, as it mostly restates the title.
- Titles and resolutions are cut at 100 bytes with `...`; `--output json` carries them whole.
- Past `--max-findings`, squeeze keeps the most severe findings across all targets and counts the rest by severity: `... 16 more findings, 5 HIGH, 10 MEDIUM, 1 LOW (--max-findings 0)`.
- A line after the JSON document that reads as an error or a warning, such as Trivy's `FATAL` or `WARN` log lines when stderr is piped too, prints after the header as `error: <line>` or `warning: <line>`, without an `Error:` or `Warning:` prefix, an error line sets `error` and a warning line sets `warning`. Other lines after the document are dropped.

## Kubernetes reports

```
$ squeeze < internal/formats/trivy/testdata/k8s-summary/input.json
trivy: cluster kind-squeeze-trivy, 3 resources: 9 vulnerabilities, 7 misconfigurations; 4 CRITICAL, 12 HIGH; 9 fixable; 82 misconfiguration checks passed
shop/Deployment/api:
  Deployment/api (kubernetes):
    HIGH KSV-0118 lines 31-46: Default security context configured
    HIGH KSV-0014 lines 33-41: Root file system is not read-only
    HIGH KSV-0118 lines 33-41: Default security context configured
  alpine:3.18.0 (alpine 3.18.0) (alpine):
    CRITICAL CVE-2022-48174 busybox 1.36.0-r9 -> 1.36.1-r1: busybox: stack overflow vulnerability in ash.c leads to arbitrary code execution
    ...
shop/Role/admin:
  Role/admin (kubernetes):
    CRITICAL KSV-0112 lines 10-15: Manage all resources at the namespace
fixes:
  ...
```

- The header names the cluster and the number of resources, then the same counts as for other targets. The k8s report carries no Trivy version, so the header says `trivy`.
- Findings are grouped by resource, `<namespace>/<kind>/<name>` or `<kind>/<name>` for a cluster-scoped resource, in the order the resource first appears. Trivy's `--report all` lists a workload twice, once for its manifest and once for its images; both entries print under one resource heading, each result as a target indented below it. The limit, the sort and the `fixes:` section work as for other targets, across all resources.
- Misconfiguration lines refer to the manifest Trivy renders from the live object, not to a file in a repository.
- A resource Trivy could not scan, such as an image it could not pull, has an `Error`; squeeze prints it whole, reduced to one line, as `error: <resource>: <error>` after the header and sets the `error` condition.
- Size, measured on the fixtures captured with Trivy 0.75.0 from a kind v0.29.0 cluster (Kubernetes 1.33.1): the `k8s` case is 483,644 bytes of JSON and 6,865 bytes squeezed. Trivy's default `table` output of the same scan (`trivy k8s --report all`) is 80,315 bytes, 11.7 times the squeezed JSON; through `generic` it becomes 18,140 bytes with 14 of its 48 CVE lines. On a scan of `nginx:1.25.0` and `redis:7.0.0` (1,446 vulnerabilities) the JSON is 7,332,166 bytes, the table 1,505,500 bytes and the squeezed JSON 7,250 bytes, the 50 most severe findings and the rest counted by severity. squeeze does not read the table; run `trivy k8s` with `--format json`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also list misconfiguration exceptions and licenses below HIGH |
| `--max-findings` | `50` | findings listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `critical` | a listed finding, or one cut by `--max-findings`, has severity `CRITICAL`; misconfiguration exceptions and licenses below `HIGH` do not count, as for `fail` |
| `error` | Trivy could not scan a Kubernetes resource (its `Error` is set), or an error line follows the JSON |
| `fail` | a vulnerability, a failed misconfiguration, a secret, or a license of severity `HIGH` or `CRITICAL` |
| `fixable` | a vulnerability has a fixed version |
| `high` | the same, with severity `HIGH` or `CRITICAL` |
| `misconfiguration` | a misconfiguration failed |
| `secret` | a secret was found |
| `vulnerability` | a vulnerability was found |
| `warning` | a warning line follows the JSON |

## JSON data

`data` holds `tool`, `tool_version`, `cluster` and `resources` (`name`, `findings`, `error`) for a k8s report, `artifact`, `artifact_type`, `os` when set, `vulnerabilities`, `misconfigurations`, `secrets`, `licenses`, `severe_licenses`, `misconfig_exceptions`, `misconfig_checks_passed`, `fixable`, `by_severity` (listed and cut findings), `targets` (`name`, `resource` for a k8s report, `class`, `type`, `findings`, `misconfig_checks_passed`), `findings`, `findings_omitted`, `omitted_by_severity`, `hidden` (exceptions and licenses below HIGH), `hidden_by_severity` and, when lines after the JSON read as errors or warnings, `diagnostics` (`severity`, `summary`, `count`). Each finding has `kind` (`vulnerability`, `misconfiguration`, `secret`, `license`), `target`, `target_index` (into `targets`), `severity`, `id` and, when set, `title` (in full), `package`, `installed_version`, `fixed_version`, `status`, `resource`, `message`, `resolution`, `start_line`, `end_line` and `url`.

## Limitations

- Trivy exits with 0 on findings unless it runs with `--exit-code`; `fail` reports the findings, not Trivy's exit code.
- A severity outside Trivy's `CRITICAL`, `HIGH`, `MEDIUM`, `LOW` and `UNKNOWN` is reported as `UNKNOWN`.
- The `image` fixture is written from Trivy's schema, since scanning a real image needs the network; the other fixtures are captured runs. The `k8s` fixture is `trivy k8s --format json --report all --list-all-pkgs=false --include-namespaces shop --disable-node-collector` and `k8s-summary` the same with `--report summary --severity CRITICAL,HIGH`, both against a kind cluster with an `alpine:3.18.0` deployment, a deployment whose image does not exist and a Role with `*` verbs and resources.
- `SchemaVersion` other than 2 exits with code 1 and names it; a k8s report has none.
- Trivy's `--report summary` keeps only resources with vulnerabilities, secrets or misconfigurations and merges each workload's entries, keeping the `Error` of the misconfiguration entry (`consolidate`, `vulnerabilitiesOrSecretResource` and `misconfigsResource` in `pkg/k8s/report/report.go` at v0.75.0, lines 82-129 and 282-293). An image Trivy could not scan has no results, so its error is not in the summary report; the `k8s-summary` fixture shows it. `--report all` keeps it. The summary report lists resources in Go map order (`lo.Values`, line 126), which can change between runs of Trivy.
- `--format table` and `--format cyclonedx` of `trivy k8s` are not read; the table goes to `generic`.
- `ExperimentalModifiedFindings`, the findings a VEX statement or an ignore file suppressed, are not read; Trivy itself hides them unless asked.
