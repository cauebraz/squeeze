# trivy

Input: the JSON that `trivy --format json` prints for `trivy fs`, `trivy config`, `trivy image` and the other targets, `SchemaVersion` 2, as `pkg/types/report.go` in Trivy defines it. Detected when the input is a JSON object that contains `"SchemaVersion"` and `"ArtifactName"` or `"Results"`, anywhere in the document.

```
trivy fs --scanners vuln,secret,misconfig --format json . | squeeze
trivy image --format json registry.example.com/shop/api:1.4.0 | squeeze --fail-on critical
```

Trivy's JSON carries, for every vulnerability, its description, references, CVSS vectors, vendor severities and dates, and for every misconfiguration its description, query, references and code. squeeze keeps one line per finding with severity, ID, package or resource, and fix. On the default-flag fixtures with findings captured from Trivy 0.75.0 the text is 1.4 to 3.2 percent of the input; on the container-image fixture written from the schema, which holds few findings, it is 15 percent.

## Output

```
$ squeeze < internal/formats/trivy/testdata/image/input.json
trivy 0.75.0: registry.example.com/shop/api:1.4.0 (container_image, debian 12.7): 5 vulnerabilities, 1 misconfiguration, 2 licenses; 1 CRITICAL, 5 HIGH, 2 LOW, 1 UNKNOWN; 3 fixable; 40 misconfiguration checks passed
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

- The header names the artifact, its type and, for an image, its OS; then the count of vulnerabilities, misconfigurations, secrets and licenses, the count by severity (the hidden exceptions and licenses included), how many vulnerabilities have a fixed version, and how many misconfiguration checks passed.
- Findings are grouped by Trivy's results, in Trivy's order, with the target name and type; two results with the same target name, such as a `requirements.txt` scanned for packages and for secrets, stay apart. Inside a target they are sorted by kind, severity, package or resource, line and ID.
- A vulnerability prints its package, the installed version and `-> fixed versions`, or Trivy's status (`affected`, `will not fix`, ...) when there is no fix.
- A misconfiguration prints its resource and lines from Trivy's cause metadata. Passed checks are counted in the header; checks with status `EXCEPTION` are counted in `... N misconfiguration exceptions (--all)`.
- A secret prints its rule and line. The `Match` and `Code` fields, which hold the line from the file, are never read, so the secret never reaches the text or the JSON, even where Trivy did not mask it.
- Licenses of severity `HIGH` or `CRITICAL` print with their package and category; the others are counted in `... N licenses below HIGH (--all)`.
- Titles are cut at 100 bytes with `...`; `--output json` carries them whole.
- Past `--max-findings`, squeeze keeps the most severe findings across all targets and counts the rest by severity: `... 16 more findings, 5 HIGH, 10 MEDIUM, 1 LOW (--max-findings 0)`.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also list misconfiguration exceptions and licenses below HIGH |
| `--max-findings` | `50` | findings listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `critical` | a finding has severity `CRITICAL` |
| `fail` | a vulnerability, a failed misconfiguration, a secret, or a license of severity `HIGH` or `CRITICAL` |
| `fixable` | a vulnerability has a fixed version |
| `high` | a finding has severity `HIGH` or `CRITICAL` |
| `misconfiguration` | a misconfiguration failed |
| `secret` | a secret was found |
| `vulnerability` | a vulnerability was found |

## JSON data

`data` holds `tool`, `tool_version`, `artifact`, `artifact_type`, `os` when set, `vulnerabilities`, `misconfigurations`, `secrets`, `licenses`, `severe_licenses`, `misconfig_exceptions`, `misconfig_checks_passed`, `fixable`, `by_severity`, `targets` (`name`, `class`, `type`, `findings`, `misconfig_checks_passed`), `findings`, `findings_omitted`, `omitted_by_severity` and `hidden` (exceptions and licenses below HIGH). Each finding has `kind` (`vulnerability`, `misconfiguration`, `secret`, `license`), `target`, `target_index` (into `targets`), `severity`, `id` and, when set, `title` (in full), `package`, `installed_version`, `fixed_version`, `status`, `resource`, `message`, `start_line`, `end_line` and `url`.

## Limitations

- Trivy exits with 0 on findings unless it runs with `--exit-code`; `fail` reports the findings, not Trivy's exit code.
- A severity outside Trivy's `CRITICAL`, `HIGH`, `MEDIUM`, `LOW` and `UNKNOWN` is reported as `UNKNOWN`.
- The `image` fixture is written from Trivy's schema, since scanning a real image needs the network; the other fixtures are captured runs.
- `SchemaVersion` other than 2 exits with code 1 and names it.
- `ExperimentalModifiedFindings`, the findings a VEX statement or an ignore file suppressed, are not read; Trivy itself hides them unless asked.
