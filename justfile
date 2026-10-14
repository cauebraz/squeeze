set shell := ["bash", "-euo", "pipefail", "-c"]

version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
min_coverage := "95"
terraform_repo := "https://github.com/hashicorp/terraform.git"
terraform_ref := "9ec49e06a868cfa161213fac7f74aa03f8937755"

default:
    @just --list

build:
    mkdir -p bin
    go build -trimpath -ldflags "-s -w -X github.com/cauebraz/squeeze/internal/cli.Version={{version}}" -o bin/squeeze ./cmd/squeeze

test:
    go test ./...

fmt:
    gofmt -w .

fmt-check:
    files="$(gofmt -l .)"; if [ -n "$files" ]; then echo "$files"; exit 1; fi

vet:
    go vet ./...

lint:
    go tool staticcheck ./...
    go tool exhaustive ./...

check: fmt-check vet lint tidy-check test

tidy-check:
    go mod tidy -diff

cover:
    go test -coverpkg=./... -coverprofile=coverage.out ./...
    total="$(go tool cover -func=coverage.out | awk '/^total:/ {sub("%", "", $3); print $3}')"; \
    echo "total coverage: ${total}%"; \
    awk -v t="$total" -v m="{{min_coverage}}" 'BEGIN { exit (t + 0 >= m + 0) ? 0 : 1 }' || { echo "coverage ${total}% is below {{min_coverage}}%"; exit 1; }

golden:
    go test ./internal/formats/... -run TestContract -update

fuzz format="tfplan" seconds="30":
    go test ./internal/formats/{{format}} -run '^$' -list '^FuzzSqueeze$' | grep -x FuzzSqueeze >/dev/null || { echo "internal/formats/{{format}} has no FuzzSqueeze"; exit 1; }
    go test ./internal/formats/{{format}} -run '^$' -fuzz '^FuzzSqueeze$' -fuzztime {{seconds}}s

fuzz-all seconds="20":
    for dir in internal/formats/*/; do just fuzz "$(basename "$dir")" {{seconds}}; done

bench:
    go test ./internal/formats/... -run '^$' -bench . -benchmem

demo: build
    ./bin/squeeze < internal/formats/tfplan/testdata/replace-forced/input.json

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

measure model runs="5" *args:
    bench/run.py --model {{model}} --runs {{runs}} {{args}}

measure-self-test:
    bench/run.py --self-test
