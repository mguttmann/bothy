#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Tests of the setup wizard infra/compose/install.sh (docs/spec/16-operations.md, section 3.1) with
# stubs of docker, curl, cosign and sudo (stubs/): no container starts and nothing goes to the
# network. The cases are in cases.sh; README.md next to this file says what they cover.
# `make install-test` runs this file in a pinned Debian container, INSTALL_TEST_NATIVE=1 on the host.
#
# Usage: sh infra/compose/test/install/run.sh [test name ...]   (default: every test_* of cases.sh)
#
# It renders install.sh like the release build (the three built-in lines) with a test version and
# the SHA-256 of a fixture bundle, then runs every case in a fresh folder with PATH set to the stubs
# and a copy of the system tools without docker, so a real docker CLI is never reachable. It
# writes only into a work folder under TMPDIR and removes it at the end, so the repository can be
# mounted read-only.
#
# Exit codes: 0 every test passed or was skipped, 1 a test failed, 2 a precondition is missing.

set -u

HERE=$(cd "$(dirname "$0")" && pwd) || exit 2
ROOT=$(cd "$HERE/../../../.." && pwd) || exit 2
SOURCE_SCRIPT=$ROOT/infra/compose/install.sh

# Values of the fixture release. The server version and the registry differ from the script's
# version on purpose: install.sh must take them from the bundle (18-release.md, section 2.1).
TEST_VERSION=1.2.3
TEST_BASE_URL=https://github.com/example/bothy/releases/download/v$TEST_VERSION/
TEST_BUNDLE_NAME=bothy-compose-$TEST_VERSION.tar.gz
SERVER_VERSION=9.8.7
REGISTRY=ghcr.io/example
# Used by cases.sh, which ShellCheck checks on its own.
# shellcheck disable=SC2034
ENROLL_CODE=K7QX-4M2P-9D3R
# shellcheck disable=SC2034
PUBLIC_URL=https://chat.example.com

# precondition aborts the runner before any test (exit 2).
# Arguments: the reason.
precondition() {
	printf 'install-test: %s\n' "$*" >&2
	exit 2
}

for tool in awk sed grep find sort tar gzip mktemp perl cksum; do
	command -v "$tool" >/dev/null 2>&1 || precondition "$tool is missing"
done
PERL=$(command -v perl)

# sha256_of prints the SHA-256 of a file in lower-case hex.
# Arguments: $1 file. Exit code: that of the hash tool.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{ print $1 }'
	else
		shasum -a 256 "$1" | awk '{ print $1 }'
	fi
}
command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 ||
	precondition "sha256sum or shasum is missing"

WORK=$(mktemp -d "${TMPDIR:-/tmp}/bothy-install-test.XXXXXX") || precondition "mktemp -d failed"
trap 'rm -rf "$WORK"' EXIT
trap 'exit 1' HUP INT TERM

# ---- tools on PATH --------------------------------------------------------------------------
# stubs/ first, then links to every system tool except the ones a stub replaces. A copy of /usr/bin
# and /bin without docker makes "docker missing" real on a host that has docker there.
mkdir "$WORK/stubs" "$WORK/sysbin" "$WORK/logs" "$WORK/cases" || precondition "mkdir failed"
if ! cp "$HERE/stubs/"* "$WORK/stubs/" || ! chmod 755 "$WORK/stubs/"*; then
	precondition "cannot copy the stubs"
fi
for dir in /usr/bin /bin; do
	for file in "$dir"/*; do
		name=${file##*/}
		case $name in docker | curl | cosign | sudo) continue ;; esac
		[ -e "$WORK/sysbin/$name" ] || ln -s "$file" "$WORK/sysbin/$name"
	done
done
TEST_PATH=$WORK/stubs:$WORK/sysbin
if ! mkdir "$WORK/nodocker" || ! cp "$WORK/stubs/curl" "$WORK/stubs/cosign" "$WORK/stubs/sudo" "$WORK/nodocker/"; then
	precondition "cannot prepare the stubs without docker"
fi
# shellcheck disable=SC2034
NODOCKER_PATH=$WORK/nodocker:$WORK/sysbin
if ! mkdir "$WORK/nocosign" || ! cp "$WORK/stubs/docker" "$WORK/stubs/curl" "$WORK/stubs/sudo" "$WORK/nocosign/"; then
	precondition "cannot prepare the stubs without cosign"
fi
# shellcheck disable=SC2034
NOCOSIGN_PATH=$WORK/nocosign:$WORK/sysbin

# ---- fixture bundles ------------------------------------------------------------------------

# make_tree writes the content of a bundle: compose.yaml, the real .env.example of the repository
# with the server version and registry of the fixture release, and the top-level files.
# Arguments: $1 folder of the top directory (created).
make_tree() {
	mkdir -p "$1/infra/compose" || return 1
	printf 'name: bothy\nservices: {}\n' >"$1/infra/compose/compose.yaml" || return 1
	sed -e "s|^BOTHY_VERSION=.*|BOTHY_VERSION=$SERVER_VERSION|" \
		-e "s|^BOTHY_REGISTRY=.*|BOTHY_REGISTRY=$REGISTRY|" \
		"$ROOT/infra/compose/.env.example" >"$1/infra/compose/.env.example" || return 1
	printf '%s\n' "$TEST_VERSION" >"$1/VERSION" && printf 'Apache-2.0\n' >"$1/LICENSE"
}

TOP=bothy-$TEST_VERSION
make_tree "$WORK/src/good/$TOP" || precondition "cannot build the fixture tree"
if ! mkdir "$WORK/dl" || ! tar -czf "$WORK/dl/$TEST_BUNDLE_NAME" -C "$WORK/src/good" "$TOP"; then
	precondition "cannot build the fixture bundle"
fi
# A keyless signature carries the certificate of the workflow identity (public phase); a signature
# of the private phase (key pair, ADR 0062) carries only a public key hint.
printf '{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json","verificationMaterial":{"certificate":{"rawBytes":"MIIB"}}}\n' \
	>"$WORK/dl/$TEST_BUNDLE_NAME.sigstore.json"
if ! mkdir "$WORK/dl-key" || ! cp "$WORK/dl/$TEST_BUNDLE_NAME" "$WORK/dl-key/"; then
	precondition "cannot build the key-signed download folder"
fi
printf '{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json","verificationMaterial":{"publicKey":{"hint":"k"}}}\n' \
	>"$WORK/dl-key/$TEST_BUNDLE_NAME.sigstore.json"
GOOD_SHA=$(sha256_of "$WORK/dl/$TEST_BUNDLE_NAME")

# A bundle with other content under the same name: its checksum does not match.
if ! make_tree "$WORK/src/bad/$TOP" || ! printf 'changed\n' >"$WORK/src/bad/$TOP/CHANGELOG.md" ||
	! mkdir "$WORK/dl-bad" || ! tar -czf "$WORK/dl-bad/$TEST_BUNDLE_NAME" -C "$WORK/src/bad" "$TOP"; then
	precondition "cannot build the changed bundle"
fi

# A bundle with a member outside the top directory; the script carries its checksum.
if ! make_tree "$WORK/src/evil/$TOP" || ! mkdir "$WORK/src/evil/other" || ! printf 'x\n' >"$WORK/src/evil/other/file" ||
	! mkdir "$WORK/dl-evil" || ! tar -czf "$WORK/dl-evil/$TEST_BUNDLE_NAME" -C "$WORK/src/evil" "$TOP" other; then
	precondition "cannot build the bundle with a foreign member"
fi

# render writes install.sh as the release build does: only the three built-in lines change.
# Arguments: $1 SHA-256 to build in, $2 output file. Exit code: 1 when the source is missing.
render() {
	[ -f "$SOURCE_SCRIPT" ] || return 1
	sed -e "s|^BOTHY_INSTALL_VERSION='[^']*'\$|BOTHY_INSTALL_VERSION='$TEST_VERSION'|" \
		-e "s|^BOTHY_INSTALL_BUNDLE_SHA256='[^']*'\$|BOTHY_INSTALL_BUNDLE_SHA256='$1'|" \
		-e "s|^BOTHY_INSTALL_BASE_URL='[^']*'\$|BOTHY_INSTALL_BASE_URL='$TEST_BASE_URL'|" \
		"$SOURCE_SCRIPT" >"$2"
}
INSTALL=$WORK/install.sh
INSTALL_EVIL=$WORK/install-evil.sh
# As the trial release renders it: the download address is a static server on 127.0.0.1.
INSTALL_TRIAL=$WORK/install-trial.sh
TRIAL_BASE_URL=http://127.0.0.1:41234/
render "$GOOD_SHA" "$INSTALL" || printf 'install-test: %s is missing; every test will fail\n' "$SOURCE_SCRIPT" >&2
render "$(sha256_of "$WORK/dl-evil/$TEST_BUNDLE_NAME")" "$INSTALL_EVIL"
render "$GOOD_SHA" "$INSTALL_TRIAL" && sed "s|^BOTHY_INSTALL_BASE_URL='[^']*'\$|BOTHY_INSTALL_BASE_URL='$TRIAL_BASE_URL'|" \
	"$INSTALL_TRIAL" >"$INSTALL_TRIAL.tmp" && mv "$INSTALL_TRIAL.tmp" "$INSTALL_TRIAL"

# ---- helpers for the cases ------------------------------------------------------------------

# fail ends the running test as failed. Arguments: the reason.
fail() {
	printf 'FAIL: %s\n' "$*"
	exit 1
}

# inst runs a command without a controlling terminal (a new session, so /dev/tty cannot be opened)
# and with a clean environment in the case folder: PATH with the stubs, STUB_LOG and STUB_DL_DIR.
# Arguments: [NAME=value ...] command [arguments], usually: sh "$INSTALL" <options>.
# Effect: sets RC; standard output in $OUT, standard error in $ERR.
inst() {
	# shellcheck disable=SC2016 # Perl code, expanded by perl
	"$PERL" -MPOSIX -e 'my $p = fork; die "fork: $!" unless defined $p;
		if ($p) { waitpid($p, 0); exit($? & 127 ? 128 + ($? & 127) : $? >> 8) }
		POSIX::setsid(); exec @ARGV or die "exec: $!"' \
		env -i PATH="$TEST_PATH" HOME="$CASE" STUB_LOG="$STUB_LOG" STUB_DL_DIR="$WORK/dl" "$@" \
		>"$OUT" 2>"$ERR" </dev/null
	RC=$?
}

# show prints the output of the last run, for the log of a failed test.
show() {
	printf -- '--- rc %s\n--- stdout\n' "$RC"
	cat "$OUT"
	printf -- '--- stderr\n'
	cat "$ERR"
	printf -- '--- stub log\n'
	cat "$STUB_LOG" 2>/dev/null
}

# expect_rc fails the test unless the last run exited with $1.
expect_rc() {
	[ "$RC" = "$1" ] || {
		show
		fail "exit code $RC, expected $1"
	}
}

# expect_in fails the test unless file $1 contains the fixed string $2.
expect_in() {
	grep -F -e "$2" "$1" >/dev/null || {
		show
		fail "$1 does not contain: $2"
	}
}

# expect_not_in fails the test if file $1 contains the fixed string $2.
expect_not_in() {
	if grep -F -e "$2" "$1" >/dev/null; then
		show
		fail "$1 contains: $2"
	fi
}

# env_value prints the value of the last line KEY= of the .env of the default target folder.
# Arguments: $1 key, [$2 .env file].
env_value() {
	sed -n "s/^$1=//p" "${2:-$CASE/bothy/infra/compose/.env}" | tail -n 1
}

# expect_env fails the test unless the .env line of key $1 has the value $2.
expect_env() {
	ev_value=$(env_value "$1" "${3:-$CASE/bothy/infra/compose/.env}")
	[ "$ev_value" = "$2" ] || {
		show
		fail "$1=$ev_value in .env, expected $1=$2"
	}
}

# mode_of prints the permission string of ls -ld (for example -rw-------).
mode_of() {
	# shellcheck disable=SC2012 # the mode string of ls is the same on GNU and BSD, stat is not
	ls -ld "$1" | awk '{ print substr($1, 1, 10) }'
}

# snapshot lists every path of the work folder except the logs, to prove that a run wrote nothing.
snapshot() {
	find "$WORK" -path "$WORK/logs" -prune -o -print | sort
}

# shellcheck source=/dev/null
. "$HERE/cases.sh"

# ---- run ------------------------------------------------------------------------------------
if [ $# -gt 0 ]; then
	tests=$*
else
	tests=$(sed -n 's/^\(test_[a-z0-9_]*\)() {$/\1/p' "$HERE/cases.sh")
fi
passed=0
failed=0
skipped=0
for name in $tests; do
	CASE=$WORK/cases/$name
	OUT=$WORK/logs/$name.out
	ERR=$WORK/logs/$name.err
	STUB_LOG=$WORK/logs/$name.stub
	RC=
	if ! mkdir "$CASE" || ! : >"$STUB_LOG"; then
		precondition "cannot prepare $name"
	fi
	(cd "$CASE" && "$name") >"$WORK/logs/$name.log" 2>&1
	status=$?
	if [ "$status" != 77 ] && { [ -e "$STUB_LOG.sudo" ] || grep '^sudo' "$STUB_LOG" >/dev/null; }; then
		printf 'FAIL: install.sh called sudo\n' >>"$WORK/logs/$name.log"
		status=1
	fi
	case $status in
	0)
		printf 'ok - %s\n' "$name"
		passed=$((passed + 1))
		;;
	77)
		printf 'ok - %s # SKIP %s\n' "$name" "$(tail -n 1 "$WORK/logs/$name.log")"
		skipped=$((skipped + 1))
		;;
	*)
		printf 'not ok - %s\n' "$name"
		sed 's/^/    /' "$WORK/logs/$name.log"
		failed=$((failed + 1))
		;;
	esac
done
printf '%s passed, %s failed, %s skipped\n' "$passed" "$failed" "$skipped"
[ "$failed" = 0 ]
