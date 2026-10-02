#!/usr/bin/env bash
# Developer-only release-consistency check; never invoked by the executable.
#
#   bash scripts/check-version.sh <1.x.y[-rc.N]>
#
# Verifies that the repository tells one version story: the version baked into a
# release build, the artifact filenames, and the human-facing documentation all
# agree with the version the tag intends. Exits non-zero on the first mismatch,
# naming the file, line, and both values.
set -euo pipefail
export LC_ALL=C TZ=UTC

version=${1:-}
if [[ ! $version =~ ^1\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ ]]; then
  echo "usage: bash scripts/check-version.sh <1.x.y[-rc.N]>" >&2
  exit 2
fi

cd "$(dirname "$0")/.."
failures=0
fail() { echo "FAIL: $*" >&2; failures=$((failures + 1)); }

# --- 1. The built binary must report exactly this version --------------------

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

go build -trimpath -buildvcs=false \
  -ldflags "-X github.com/aman-void/portcheck/internal/cli.version=$version" \
  -o "$work/portcheck" ./cmd/portcheck

reported=$("$work/portcheck" --version)
expected="portcheck version $version"
if [[ $reported != "$expected" ]]; then
  fail "application reports '$reported', want '$expected'"
fi

# A build without an injected version must stay on the development default.
go build -trimpath -buildvcs=false -o "$work/dev" ./cmd/portcheck
dev=$("$work/dev" --version)
if [[ $dev != "portcheck version 1.0.0-dev" ]]; then
  fail "development build reports '$dev', want 'portcheck version 1.0.0-dev'"
fi

# --- 2. Release artifacts must carry the version in their names --------------

# ALLOW_DIRTY because release.sh normally refuses an uncommitted tree, and a
# developer is by definition mid-edit when preparing a release. Only artifact
# names and checksums matter here, never the dirty flag those builds record.
out=$(mktemp -d)/artifacts
if ! ALLOW_DIRTY=1 bash scripts/release.sh "$version" "$out" >/dev/null 2>&1; then
  echo "--- release.sh output ---" >&2
  ALLOW_DIRTY=1 bash scripts/release.sh "$version" "${out}-retry" >&2 || true
  fail "scripts/release.sh failed for $version"
else
  # Exactly six archives, each named portcheck_<version>_<os>_<arch>.<ext>
  shopt -s nullglob
  archives=("$out"/portcheck_*)
  shopt -u nullglob
  if [[ ${#archives[@]} -ne 6 ]]; then
    fail "expected 6 archives, found ${#archives[@]}"
  fi
  for a in "${archives[@]}"; do
    base=$(basename "$a")
    case "$base" in
      "portcheck_${version}_"*) ;;
      *) fail "archive '$base' does not carry version '$version'" ;;
    esac
  done
  for os in linux darwin windows; do
    for arch in amd64 arm64; do
      if [[ $os == windows ]]; then
        want="portcheck_${version}_${os}_${arch}.zip"
      else
        want="portcheck_${version}_${os}_${arch}.tar.gz"
      fi
      [[ -f "$out/$want" ]] || fail "missing expected artifact $want"
    done
  done
  (cd "$out" && sha256sum -c SHA256SUMS >/dev/null) \
    || fail "SHA256SUMS does not verify against the built artifacts"
  [[ -x "$out/install.sh" ]] || fail "install.sh missing or not executable"
fi

# --- 3. Documentation must not claim a different current release -------------

# These are the only places a human reads the current release version. Examples
# and stable-version references (module path prose, generic archive walkthroughs)
# are intentionally not treated as current-release claims.
# Any version-shaped string that looks like a current candidate/stable claim but
# disagrees with $version is reported. Historical plan/report files are excluded:
# they record what was true at the time.
scan_docs() {
  local file=$1
  [[ -f $file ]] || return 0
  # Report candidate-shaped versions that are not the one being released.
  local line_no line
  while IFS= read -r line_no; do
    line=$(sed -n "${line_no}p" "$file")
    if [[ $line =~ 1\.[0-9]+\.[0-9]+-rc\.[0-9]+ ]] && [[ $line != *"$version"* ]]; then
      fail "$file:$line_no references a candidate version other than '$version': $line"
    fi
  done < <(grep -n -E '1\.[0-9]+\.[0-9]+-rc\.[0-9]+' "$file" | cut -d: -f1)
}

for f in README.md docs/releasing.md docs/release-notes.md; do
  scan_docs "$f"
done

# The release notes must carry the substitution placeholder, not a hardcoded
# version, so one file serves both prerelease and final tags.
if ! grep -q '{{VERSION}}' docs/release-notes.md; then
  fail "docs/release-notes.md must use {{VERSION}}, which the workflow substitutes"
fi

# --- 4. Report ---------------------------------------------------------------

if ((failures > 0)); then
  echo "version consistency: $failures problem(s) for $version" >&2
  exit 1
fi
echo "version consistency OK for $version"