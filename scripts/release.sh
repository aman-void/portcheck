#!/usr/bin/env bash
# Developer-only packaging; the Portcheck executable never runs shell commands.
set -euo pipefail
export LC_ALL=C TZ=UTC
umask 022

version=${1:-}
out=${2:-}
if [[ ! $version =~ ^1\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ || -z $out ]]; then
  echo 'usage: bash scripts/release.sh <1.x.y[-rc.N]> <new-output-directory>' >&2
  exit 2
fi

cd "$(dirname "$0")/.."
for tool in go git sh tar gzip zip touch sha256sum; do
  command -v "$tool" >/dev/null || { echo "missing packaging tool: $tool" >&2; exit 1; }
done
# GNU tar supplies deterministic ordering/ownership/timestamps. Packaging runs
# on Linux; the resulting archives are for all six targets.
tar --version | grep -q 'GNU tar' || { echo 'packaging requires GNU tar' >&2; exit 1; }
# Preflight the installer before writing any output, so a packaging failure
# never leaves a directory that looks publishable but has no SHA256SUMS.
install_sh=install.sh
[[ -f $install_sh ]] || { echo "missing installer: $install_sh" >&2; exit 1; }
sh -n "$install_sh" || { echo 'install.sh failed sh -n' >&2; exit 1; }
dirty=false
if [[ -n $(git status --porcelain) ]]; then
  dirty=true
  if [[ ${ALLOW_DIRTY:-0} != 1 ]]; then
    echo 'release requires a clean checkout; ALLOW_DIRTY=1 is for local candidate validation only' >&2
    exit 1
  fi
  echo 'warning: building an uncommitted candidate; do not publish these artifacts' >&2
fi
commit=$(git rev-parse HEAD)
toolchain=$(go env GOVERSION)
# Refuse an existing directory rather than overwrite old artifacts/checksums.
mkdir -p -- "$(dirname -- "$out")"
mkdir -- "$out"
out=$(cd "$out" && pwd)
stage=$(mktemp -d "$out/.stage.XXXXXX")
trap 'rm -rf "$stage"' EXIT

for os in linux darwin windows; do
  for arch in amd64 arm64; do
    name="portcheck_${version}_${os}_${arch}"
    dir="$stage/$name"
    mkdir "$dir"
    binary=portcheck
    [[ $os != windows ]] || binary=portcheck.exe
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X github.com/aman-void/portcheck/internal/cli.version=$version" \
      -o "$dir/$binary" ./cmd/portcheck
    cp LICENSE "$dir/LICENSE"
    printf 'version=%s\ncommit=%s\ndirty=%s\ngo=%s\ntarget=%s/%s\ncgo=0\n' \
      "$version" "$commit" "$dirty" "$toolchain" "$os" "$arch" > "$dir/BUILDINFO.txt"
    if [[ $os == windows ]]; then
      # ZIP's oldest timestamp is 1980; strip host-specific extra fields.
      touch -t 198001010000 "$dir" "$dir"/*
      (cd "$stage" && zip -X -q "$out/$name.zip" "$name/BUILDINFO.txt" "$name/LICENSE" "$name/$binary")
    else
      tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner \
        -C "$stage" -cf - "$name" | gzip -n > "$out/$name.tar.gz"
    fi
  done
done
# Ship the installer as a top-level asset, not inside the per-platform archives.
# One file serves every platform, which is what makes
# releases/latest/download/install.sh a stable URL. POSIX sh only: it runs on
# machines that have curl and tar but no Bash. Preflighted above.
cp "$install_sh" "$out/$install_sh"
chmod 0755 "$out/$install_sh"

(cd "$out" && sha256sum portcheck_*.tar.gz portcheck_*.zip "$install_sh" > SHA256SUMS)
echo "Release artifacts: $out"
