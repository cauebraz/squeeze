set shell := ["bash", "-euo", "pipefail", "-c"]

version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
min_coverage := "85"

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
    if command -v staticcheck >/dev/null; then staticcheck ./...; else echo "staticcheck not installed; skipped (go install honnef.co/go/tools/cmd/staticcheck@v0.8.2)"; fi

check: fmt-check vet lint test

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
    ./bin/squeeze < internal/formats/tfplan/testdata/replace-forced.json
