# checkov

Input: the JSON that `checkov -o json` prints, as `common/output/report.py` and `common/runners/runner_registry.py` in Checkov build it: one report object for one framework, a list of report objects for several, or only the summary when there was nothing to scan. `--quiet` and `--compact` output are read too. Detected when the input is a JSON object or array whose first 64 KiB contain `"check_type"` and `"results"`, or a summary object with `"checkov_version"` and `"resource_count"`.

```
checkov -d . -o json | squeeze
checkov -d . --framework terraform dockerfile -o json | squeeze --fail-on high
```

Checkov's JSON carries every passed check as well as every failed one, each with its code block, evaluated keys, entity, benchmarks and guideline link. squeeze keeps the counts and one line per failed check, grouped by resource. On the default-flag fixtures with findings, captured from Checkov 3.3.29, the text is 1.2 to 3.1 percent of the input.

## Output

```
$ squeeze < internal/formats/checkov/testdata/single/input.json
checkov 3.3.29: 18 failed, 14 passed, 1 skipped; 4 resources
aws_s3_bucket.logs (main.tf:5-7): 7 failed checks
  CKV2_AWS_6 Ensure that S3 bucket has a Public Access block
  CKV2_AWS_61 Ensure that an S3 bucket has a lifecycle configuration
  CKV2_AWS_62 Ensure S3 buckets should have event notifications enabled
  CKV_AWS_144 Ensure that S3 bucket has cross-region replication enabled
  CKV_AWS_145 Ensure that S3 buckets are encrypted with KMS by default
  CKV_AWS_18 Ensure the S3 bucket has access logging enabled
  CKV_AWS_21 Ensure all data stored in the S3 bucket have versioning enabled
aws_s3_bucket.assets (main.tf:9-12): 6 failed checks
  CKV2_AWS_6 Ensure that S3 bucket has a Public Access block
  CKV2_AWS_61 Ensure that an S3 bucket has a lifecycle configuration
  CKV2_AWS_62 Ensure S3 buckets should have event notifications enabled
  CKV_AWS_144 Ensure that S3 bucket has cross-region replication enabled
  CKV_AWS_145 Ensure that S3 buckets are encrypted with KMS by default
  CKV_AWS_21 Ensure all data stored in the S3 bucket have versioning enabled
aws_security_group.web (main.tf:14-22): 3 failed checks
  CKV2_AWS_5 Ensure that Security Groups are attached to another resource
  CKV_AWS_23 Ensure every security group and rule has a description
  CKV_AWS_24 Ensure no security groups allow ingress from 0.0.0.0:0 to port 22
module.store.aws_ebs_volume.data (modules/store/main.tf:1-4, via main.tf:24-26): 2 failed checks
  CKV_AWS_189 Ensure EBS Volume is encrypted by KMS using a customer managed Key (CMK)
  CKV_AWS_3 Ensure all data stored in the EBS is securely encrypted
... 1 skipped check (--all)
```

- The header gives Checkov's version and the failed, passed and skipped counts from Checkov's own `summary`, so `--quiet` output, which lists only the failed checks, still reports every count. Parsing errors and the resource count follow. With several frameworks, the header adds the failed count of each.
- Failed checks are grouped by resource. A group line gives the resource, `file:start-end` (with Checkov's leading `/` removed), `via file:start-end` for a resource created through a module call, and the number of failed checks on it. Each check prints its ID and Checkov's check name.
- When Checkov reports a severity, which needs a platform API key, each check starts with it (`CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO`); any other value prints as `UNKNOWN`. Groups are sorted by their worst severity, then by file and line; checks inside a group by severity, then ID.
- Every file Checkov could not parse prints as `parse error: <path>`, with the path as Checkov gives it, which is absolute where check paths are relative to the scan root.
- A finding of the `secrets` framework prints `secret` in place of the resource: Checkov's resource there is a hash of the secret value, which would reveal that two files hold the same value.
- Skipped checks are counted in `... N skipped checks (--all)`; `--all` lists them under `skipped:` with the suppression comment in place of the check name.
- Past `--max-checks`, the notice names the IDs of the cut checks with their counts, so `... 14 more failed checks, CKV_AWS_145 2, CKV_AWS_21 2, ... (--max-checks 0)` tells which check to fix or suppress first.
- The code block, the evaluated keys, the entity and the evaluations are never read, so a value the secrets framework or a check shows in them never reaches the text or the JSON.
- Strings from the input are reduced to one line without control characters or terminal escape sequences before they are printed.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also list skipped checks with their suppression comment |
| `--max-checks` | `50` | failed checks listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `critical` | a failed check has severity `CRITICAL` |
| `error` | Checkov could not parse a file |
| `fail` | a check failed; Checkov exits with 1 in that case unless `--soft-fail` or a threshold says otherwise |
| `high` | a failed check has severity `HIGH` or `CRITICAL` |
| `skip` | a check was skipped |

## JSON data

`data` holds `tool`, `tool_version`, `passed`, `failed`, `skipped`, `parsing_errors`, `resource_count`, `frameworks` (`check_type`, `passed`, `failed`, `skipped`, `parsing_errors`, `resource_count` per report), `failed_by_severity`, `failed_checks`, `failed_checks_omitted`, `omitted_by_check_id` (`name`, `count`), `skipped_checks` and `parsing_error_files`. Each check has `framework`, `check_id`, `name`, `resource`, `file` and, when set, `start_line`, `end_line`, `severity`, `caller`, `guideline` and `suppress_comment`.

## Limitations

- Severities need a Prisma Cloud API key; without one Checkov reports `null`, and `critical` and `high` never match. The `severity` fixture is a captured run with severities filled in as the schema allows.
- SCA reports (`sca_package`, `sca_image`) are read as checks: their vulnerability details, such as the CVE and the fixed version, are not printed.
- Passed checks are counted, not listed.
- A summary-only input with nonzero counts, which Checkov prints when no framework produced a report, says `counts only, the input holds no checks` in the header.
- `checkov -o json -o cli` prints the CLI report before the JSON, so the input is not detected; use `-o json` alone, or `--output-file-path` for the other format.
