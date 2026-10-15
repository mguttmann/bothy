#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Generation of the Dart REST client and gate G4 for Dart (docs/spec/06-api.md 14 and 15,
# brief M4-02 Design note 2). Called by the package Makefile:
#
#   tool/generate.sh generate   regenerate lib/src/generated/ of this package in place
#   tool/generate.sh check      regenerate into a temporary copy, compare byte for byte with
#                               this package, analyse; never writes into the working tree
#
# The generator runs from the image pinned by digest in GENERATOR_IMAGE with the flags
# GENERATOR_DOCKER_FLAGS (both set by the Makefile: `--network none`), as the calling user, with the repository mounted read-only and only a fresh
# temporary folder writable (docs/DECISIONS.md D-M4-02-1). Temporary folders come from
# `mktemp -d` below $TMPDIR (default /tmp) and are removed on exit; Docker must be able to
# mount that path (Colima mounts only the home folder by default, so set TMPDIR below it there).
#
# Exit codes: 0 success; 1 generation, drift or analysis failure; 2 usage error.

set -euo pipefail

mode=${1:-}
pkg=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
repo=$(cd "$pkg/../.." && pwd)
image=${GENERATOR_IMAGE:?GENERATOR_IMAGE is set by packages/api_client/Makefile}
read -r -a docker_flags <<<"${GENERATOR_DOCKER_FLAGS:?GENERATOR_DOCKER_FLAGS is set by packages/api_client/Makefile}"
# The library entry the generator writes as lib/<pubName>.dart; the copy step moves it below
# lib/src/generated/ so that lib/api_client.dart stays hand-written (D-M4-02-2).
entry=lib/src/generated/generated_api.dart

tmp_root=${TMPDIR:-/tmp}
tmp_root=${tmp_root%/}
cleanup_dirs=()
cleanup() {
  local d
  for d in "${cleanup_dirs[@]+"${cleanup_dirs[@]}"}"; do rm -rf -- "$d"; done
}
trap cleanup EXIT

# new_tmp: create a temporary folder, set $tmp to it and register it for removal on exit. It
# sets a variable instead of printing, because a command substitution would run it in a
# subshell and the registration would never reach this shell.
new_tmp() {
  # The physical path: the analyzer reports resolved paths (/tmp is a link on macOS).
  tmp=$(cd "$(mktemp -d "$tmp_root/api-client-gen.XXXXXX")" && pwd -P)
  cleanup_dirs+=("$tmp")
}

# generate_into <package dir>: run the generator for the spec of $repo and replace
# <package dir>/lib/src/generated/ with its output, then build the serializers and format.
generate_into() {
  local target=$1 out tmp
  new_tmp
  out=$tmp
  docker run "${docker_flags[@]}" --user "$(id -u):$(id -g)" \
    -v "$repo:/repo:ro" -v "$out:/out" "$image" generate \
    -i /repo/docs/api/openapi.yaml -g dart-dio -o /out \
    --ignore-file-override /repo/packages/api_client/.openapi-generator-ignore \
    --additional-properties pubName=api_client,pubLibrary=api_client.generated,serializationLibrary=built_value,enumUnknownDefaultCase=true,legacyDiscriminatorBehavior=false,sourceFolder=src/generated \
    >"$out.log" 2>&1 || { cat "$out.log" >&2; rm -f "$out.log"; echo "api_client: the generator failed" >&2; return 1; }
  rm -f "$out.log"
  # Only the Dart sources leave the temporary folder; anything else under lib/ would be a file
  # the copy step does not know, so it fails instead of dropping it silently.
  local stray
  stray=$(cd "$out" && find lib -type f ! -path 'lib/src/generated/*' ! -path 'lib/api_client.dart')
  if [ -n "$stray" ]; then
    printf 'api_client: the generator wrote unexpected files:\n%s\n' "$stray" >&2
    return 1
  fi
  rm -rf "$target/lib/src/generated"
  mkdir -p "$target/lib/src/generated"
  cp -R "$out/lib/src/generated/." "$target/lib/src/generated/"
  cp "$out/lib/api_client.dart" "$target/$entry"
  (cd "$target" && dart run build_runner build --delete-conflicting-outputs >"$out/build.log" 2>&1) ||
    { cat "$out/build.log" >&2; echo "api_client: build_runner failed" >&2; return 1; }
  (cd "$target" && dart format lib/src/generated >/dev/null)
}

# A workspace with this package as its only member, so that build_runner and the analyzer run
# in the temporary copy with the versions of the root lock file.
make_workspace() {
  local ws=$1
  mkdir -p "$ws/packages"
  awk '/^workspace:/ { print; print "  - packages/api_client"; skip = 1; next }
       skip && /^  - / { next }
       { skip = 0; print }' "$repo/pubspec.yaml" >"$ws/pubspec.yaml"
  cp "$repo/pubspec.lock" "$repo/analysis_options.yaml" "$ws/"
  mkdir -p "$ws/packages/api_client"
  (cd "$pkg" && tar -cf - --exclude ./.dart_tool --exclude ./build --exclude ./coverage .) |
    (cd "$ws/packages/api_client" && tar -xf -)
  (cd "$ws" && flutter pub get >"$ws/pub.log" 2>&1) ||
    { cat "$ws/pub.log" >&2; echo "api_client: flutter pub get failed in the temporary copy" >&2; return 1; }
}

# compare <checked-in dir> <fresh dir>: report missing, extra and changed files by path.
compare() {
  local have=$1 fresh=$2 a b drift=0 f
  a=$(cd "$have" 2>/dev/null && find . -type f | LC_ALL=C sort || true)
  b=$(cd "$fresh" && find . -type f | LC_ALL=C sort)
  while IFS= read -r f; do [ -n "$f" ] && { echo "missing: lib/src/generated/${f#./}"; drift=1; }; done \
    < <(LC_ALL=C comm -13 <(printf '%s\n' "$a") <(printf '%s\n' "$b"))
  while IFS= read -r f; do [ -n "$f" ] && { echo "extra: lib/src/generated/${f#./}"; drift=1; }; done \
    < <(LC_ALL=C comm -23 <(printf '%s\n' "$a") <(printf '%s\n' "$b"))
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    cmp -s "$have/$f" "$fresh/$f" || { echo "changed: lib/src/generated/${f#./}"; drift=1; }
  done < <(LC_ALL=C comm -12 <(printf '%s\n' "$a") <(printf '%s\n' "$b"))
  return $drift
}

# analyse <package dir>: errors fail in generated files (the generator emits warnings there,
# 06 14); any finding fails in hand-written files. The root rules exclude generated code, so the
# copy is analysed with the base set only, which covers both.
analyse() {
  local dir=$1 report
  printf 'include: package:lints/recommended.yaml\n' >"$dir/analysis_options.yaml"
  # The machine format goes to stderr; a non-zero exit is expected whenever there is a finding.
  report=$(cd "$dir" && dart analyze --format=machine . 2>&1 || true)
  awk -F'|' -v root="$dir/" '
    NF >= 8 {
      file = $4; if (index(file, root) == 1) file = substr(file, length(root) + 1)
      generated = (file ~ /^lib\/src\/generated\// || file ~ /\.g\.dart$/)
      if ($1 == "ERROR" || !generated) {
        key = $1 "|" file "|" $5 "|" $3
        if (!(key in seen)) { seen[key] = 1; printf "%s %s:%s:%s %s\n", $1, file, $5, $6, $8; bad = 1 }
      }
    }
    END { exit bad }' <<<"$report"
}

case "$mode" in
  generate)
    generate_into "$pkg"
    echo "api_client: regenerated lib/src/generated/ from docs/api/openapi.yaml"
    ;;
  check)
    new_tmp
    ws=$tmp
    make_workspace "$ws"
    generate_into "$ws/packages/api_client"
    if ! compare "$pkg/lib/src/generated" "$ws/packages/api_client/lib/src/generated"; then
      echo "api_client: generated code differs from docs/api/openapi.yaml; run make -C packages/api_client generate" >&2
      exit 1
    fi
    if ! analyse "$ws/packages/api_client"; then
      echo "api_client: dart analyze found errors in generated code or findings in hand-written code" >&2
      exit 1
    fi
    echo "  api_client: generated code equals a fresh generation and analyses cleanly"
    ;;
  *)
    echo "usage: tool/generate.sh generate|check" >&2
    exit 2
    ;;
esac
