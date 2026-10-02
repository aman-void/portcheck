---
description: Run the full Portcheck verification gate
---

Run the release gate and report pass or fail for each step. Fix nothing; report only.

```sh
set -u
echo "=== gofmt ==="
gofmt -l . && echo "PASS: no formatting differences" || echo "FAIL: files listed above"

echo "=== vet ==="
go vet ./... && echo "PASS: go vet clean" || echo "FAIL: go vet"

echo "=== tests ==="
go test ./... -count=1 || echo "FAIL: tests"

echo "=== race ==="
go test -race ./... -count=1 || echo "FAIL: race"

echo "=== build ==="
go build ./... && echo "PASS: all packages build" || echo "FAIL: build"
go build -o bin/portcheck ./cmd/portcheck && echo "PASS: binary built" || echo "FAIL: binary"

echo "=== dependency contract ==="
deps=$(go list -m all)
if [ "$deps" = "github.com/aman-void/portcheck" ]; then
  echo "PASS: zero external dependencies"
else
  echo "FAIL: unexpected dependencies:"; echo "$deps"
fi

echo "=== version metadata ==="
got=$(./bin/portcheck --version)
echo "reported: $got"
case "$got" in
  *"0.0.0-"*|*"+dirty"*)
    echo "FAIL: VCS pseudo-version leaked; must report 1.0.0-dev for local builds" ;;
  *"1.0.0-dev"*|*"1."*) echo "PASS: version looks correct" ;;
  *) echo "FAIL: unexpected version string" ;;
esac

echo "=== cross-compile (includes test files) ==="
fail=0
for os in linux darwin windows; do
  for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o /dev/null ./... || { echo "FAIL: build $os/$arch"; fail=1; }
  done
done
for os in windows darwin; do
  for p in . ./internal/cli ./internal/process ./internal/checker ./cmd/portcheck; do
    GOOS=$os go test -c -o /dev/null "$p" || { echo "FAIL: test compile $os $p"; fail=1; }
  done
done
[ $fail -eq 0 ] && echo "PASS: all six targets build, test files typecheck on windows and darwin"

echo "=== working tree ==="
git status --short
git diff --quiet && git diff --cached --quiet \
  && echo "clean" || echo "NOTE: uncommitted changes present"
```

Then summarize as a table: step, result, and the failing output for anything that
failed. If everything passed, say so in one line and state the commit SHA under
test. Do not speculate about causes you did not observe.
