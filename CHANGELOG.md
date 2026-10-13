# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html); until 1.0.0 the text output and the JSON `data` of a format may change between minor versions, the envelope keys (`schema_version`, `format`, `input_bytes`, `text_bytes`, `ratio`, `data`) and the exit codes do not.

## Unreleased

### Added

- `squeeze` CLI: auto-detects the input format, `squeeze <format>` exposes that format's flags, `--output json` wraps the format's data in a common envelope, `--raw-dir` keeps the full input.
- `tfplan` format for `terraform show -json`: Terraform's counts and symbols, changed attributes only, sensitive values masked, drift filtered by `relevant_attributes`, deferred changes, action invocations, imports, moves and forgets.
- Usage hints: a format flag used without its subcommand, a misspelled format and a file passed as an argument each get the command to run instead.
- A guard for stdin on a terminal, so `squeeze` without input exits with a message instead of waiting.
