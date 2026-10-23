set shell := ["bash", "-euo", "pipefail", "-c"]

version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
min_coverage := "95"
terraform_repo := "https://github.com/hashicorp/terraform.git"
terraform_ref := "9ec49e06a868cfa161213fac7f74aa03f8937755"

[doc('list the recipes')]
default:
    @just --list

[doc('build bin/squeeze with the version from git describe')]
build:
    mkdir -p bin
    go build -trimpath -ldflags "-s -w -X github.com/cauebraz/squeeze/internal/cli.Version={{version}}" -o bin/squeeze ./cmd/squeeze

[doc('run the tests with the race detector')]
test:
    go test -race ./...

[doc('format the Go code with gofmt')]
fmt:
    gofmt -w .

[doc('fail when a Go file is not gofmt-formatted')]
fmt-check:
    files="$(gofmt -l .)"; if [ -n "$files" ]; then echo "$files"; exit 1; fi

[doc('run go vet')]
vet:
    go vet ./...

[doc('run staticcheck and exhaustive')]
lint:
    go tool staticcheck ./...
    go tool exhaustive ./...

[doc('run gofmt, vet, staticcheck, exhaustive, tidy and tests; before and after a change')]
check: fmt-check vet lint tidy-check test

[doc('fail when go mod tidy would change go.mod or go.sum')]
tidy-check:
    go mod tidy -diff

[doc('check commit messages and the branch name since base; print the size')]
commits base="origin/main":
    scripts/check-commits.sh {{base}}

[doc('run just check on every commit since base, in a temporary worktree')]
check-each base="origin/main":
    #!/usr/bin/env bash
    set -euo pipefail
    work="$(mktemp -d)"
    trap 'git worktree remove --force "$work" 2>/dev/null || true; rm -rf "$work"' EXIT
    for c in $(git rev-list --reverse {{base}}..HEAD); do
        echo "== $(git log -1 --format='%h %s' "$c")"
        git worktree remove --force "$work" 2>/dev/null || true
        git worktree add -q --detach "$work" "$c"
        if ! out="$(cd "$work" && just --quiet check 2>&1)"; then
            echo "$out"
            echo "just check fails at $(git rev-parse --short "$c")"
            exit 1
        fi
    done

[doc('measure coverage across packages; fail below 95 percent')]
cover:
    go test -coverpkg=./... -coverprofile=coverage.out ./...
    total="$(go tool cover -func=coverage.out | awk '/^total:/ {sub("%", "", $3); print $3}')"; \
    echo "total coverage: ${total}%"; \
    awk -v t="$total" -v m="{{min_coverage}}" 'BEGIN { exit (t + 0 >= m + 0) ? 0 : 1 }' || { echo "coverage ${total}% is below {{min_coverage}}%"; exit 1; }

[doc('rewrite golden files after an intended output change')]
golden:
    go test ./internal/formats/... -run TestContract -update

[doc('fuzz one format for the given seconds')]
fuzz format="tfplan" seconds="30":
    go test ./internal/formats/{{format}} -run '^$' -list '^FuzzSqueeze$' | grep -x FuzzSqueeze >/dev/null || { echo "internal/formats/{{format}} has no FuzzSqueeze"; exit 1; }
    go test ./internal/formats/{{format}} -run '^$' -fuzz '^FuzzSqueeze$' -fuzztime {{seconds}}s

[doc('fuzz every format; fail for a format without FuzzSqueeze')]
fuzz-all seconds="20":
    for dir in internal/formats/*/; do just fuzz "$(basename "$dir")" {{seconds}}; done

[doc('run the format benchmarks with the output/input ratio')]
bench:
    go test ./internal/formats/... -run '^$' -bench . -benchmem

[doc('run the README example from the replace-forced fixture')]
demo: build
    ./bin/squeeze < internal/formats/tfplan/testdata/replace-forced/input.json

[doc("run the Terraform formats over Terraform's command testdata at a pinned commit")]
conformance ref=terraform_ref:
    #!/usr/bin/env bash
    set -euo pipefail
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT
    git -C "$work" init -q
    git -C "$work" remote add origin {{terraform_repo}}
    git -C "$work" sparse-checkout set internal/command/testdata
    git -C "$work" fetch -q --depth 1 --filter=blob:none origin {{ref}}
    git -C "$work" checkout -q FETCH_HEAD
    SQUEEZE_REAL_DIR="$work/internal/command/testdata" go test ./internal/formats/tfplan ./internal/formats/tfvalidate ./internal/formats/tfjson -run '^TestRealInputs$' -count=1 -v | grep -E '^(=== RUN|--- |ok|FAIL|PASS)|ratio|leaks|: '

[doc("measure an agent's tokens with and without squeeze (bench/run.py -h)")]
measure model runs="5" *args:
    bench/run.py --model {{model}} --runs {{runs}} {{args}}

[doc('draw bench/results.svg from a bench/results/<timestamp>.jsonl run')]
chart file:
    bench/run.py --chart {{file}}

[doc("check every bench task's setup, verifier and solution without API calls")]
measure-self-test:
    bench/run.py --self-test
