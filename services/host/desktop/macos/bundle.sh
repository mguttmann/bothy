#!/bin/sh
# Assembles "Bothy Computer Helper.app" from the release build of this package
# (docs/decisions.d/CU-02.md D-CU-02-1, D-CU-02-10).
#
# Usage: bundle.sh <out_dir> <version>
#
# The bundle is signed ad hoc with the hardened runtime, so it runs and reports itself as a
# development build (signature: development). The Developer ID signature is a separate step on
# the owner's Mac (make -C services/host helper-sign-macos, D-CU-01-29); this script never
# touches a keychain or a signing identity.
set -eu

if [ "$#" -ne 2 ]; then
  echo "usage: bundle.sh <out_dir> <version>" >&2
  exit 2
fi
out_dir=$1
version=$2
case "$version" in
  "" | *[!0-9A-Za-z.+-]*)
    echo "bundle.sh: version must use only letters, digits, '.', '+' and '-'" >&2
    exit 2
    ;;
esac

here=$(cd "$(dirname "$0")" && pwd)
swift build -c release --package-path "$here" --product bothy-computer-helper
bin_dir=$(swift build -c release --package-path "$here" --show-bin-path)

mkdir -p "$out_dir"
app="$(cd "$out_dir" && pwd)/Bothy Computer Helper.app"
# Replace an earlier bundle completely, so no file of an older build stays inside.
if [ -e "$app" ]; then
  rm -rf -- "$app"
fi
mkdir -p "$app/Contents/MacOS"
cp "$bin_dir/bothy-computer-helper" "$app/Contents/MacOS/bothy-computer-helper"
sed "s/@VERSION@/$version/g" "$here/Resources/Info.plist" > "$app/Contents/Info.plist"
plutil -lint "$app/Contents/Info.plist" > /dev/null
printf 'APPL????' > "$app/Contents/PkgInfo"

codesign --force --sign - --options runtime \
  --entitlements "$here/Resources/helper.entitlements" "$app"
codesign --verify --strict "$app"
echo "$app"
