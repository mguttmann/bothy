#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Setup wizard of Bothy: installs one instance with Docker Compose in one command. Normative
# description: docs/spec/16-betrieb.md, section 3.1; release artifact of the component bothy,
# docs/spec/18-release.md, section 2.1. It does nothing the manual path
# (docs/operations/install-compose.md) does not also do: check the host, fetch and verify the
# installation bundle of its built-in version, write infra/compose/.env, start the stack, wait for
# /readyz, set up the first owner and print the next steps.
#
# Usage:
#   curl -fsSL <release address>/install.sh | sh
#   curl -fsSL <release address>/install.sh | sh -s -- --yes --public-url https://chat.example.com \
#     --edge direct --scan on --storage s3 --owner-handle alex --owner-name Alex
#   sh install.sh --bundle bothy-compose-<version>.tar.gz [options]
#
# Options (--opt value or --opt=value; an option always wins over a prompt; --help lists them):
#   --dir, --bundle, --ready-timeout, --yes, --public-url, --edge, --scan, --storage,
#   --observability, --offsite, --owner-handle, --owner-name, -h/--help.
# TS_AUTHKEY (edge tailscale) is never an option, because other users see the arguments in ps:
# with a terminal it is a hidden prompt, without one it comes from the environment variable.
#
# Exit codes: 0 success, 1 a step failed, 2 usage error. Every abort prints
# "install.sh: <step>: <reason>" on standard error; the steps are options, preflight, bundle,
# prompts, write, start, owner and finish.
#
# Rules: never sudo, installs nothing, writes only into the target folder (temporary files too).
# Runs with any POSIX sh (dash, the /bin/sh of macOS). Everything runs from main on the last
# line, so a pipe from curl that breaks off mid-download runs nothing.

# Built-in values; make release-build rewrites exactly these three lines (18 section 2.1, rule 3).
BOTHY_INSTALL_VERSION='0.0.0-placeholder'
BOTHY_INSTALL_BUNDLE_SHA256='placeholder'
BOTHY_INSTALL_BASE_URL='https://github.com/OWNER/bothy/releases/download/v0.0.0-placeholder/'

# install.ready_timeout (16 section 14): default of --ready-timeout.
install_ready_timeout=5m
# Minimum versions of 16 section 3, rule 7: Engine for volume subpath, Compose for configs content.
min_engine=26.0.0
min_compose=2.26.0
# The value of BOTHY_TRUSTED_PROXIES in .env.example: loopback only.
default_proxies='127.0.0.1/32,::1/128'
marker_name=.bothy-install

opt_dir=./bothy
opt_bundle=
opt_ready_timeout=$install_ready_timeout
opt_yes=0
opt_public_url=
opt_edge=
opt_scan=
opt_storage=
opt_observability=
opt_offsite=
opt_offsite_set=0
opt_owner_handle=
opt_owner_name=

tty=0
docker_mem=0
ready_seconds=
target=
compose_dir=
env_file=
first_run=1
edge=
scan=
storage=
observability=
offsite=
ts_authkey=
api_service=
code_shown=0
tmp_dir=
env_tmp=
stty_saved=
answer=
# Values for the awk programs of write_env_file.
IW_KEY=
IW_VALUE=
IW_PROFILES=
IW_URL=
IW_DRIVER=
IW_OFFSITE=
IW_TSKEY=
IW_SET_TSKEY=

# usage prints the help text on standard output.
# Arguments: none.
usage() {
	cat <<EOF
Usage: install.sh [options]

Installs Bothy $BOTHY_INSTALL_VERSION with Docker Compose into a folder.

Options (--opt value or --opt=value):
  --dir <path>                target folder (default ./bothy)
  --bundle <file>             use this installation bundle instead of the download
  --ready-timeout <duration>  wait at most this long for /readyz (install.ready_timeout,
                              default $install_ready_timeout; <n>s, <n>m, <n>h or seconds)
  --yes                       take the default of every value that has one, without a prompt
  --public-url <url>          public address of the server (BOTHY_PUBLIC_URL), https://...
  --edge direct|tailscale     edge of the server (default direct)
  --scan on|off               virus scan (default on with at least 4 GiB of Docker memory)
  --storage fs|s3             file storage (default fs)
  --observability on|off      local observability stack (default off)
  --offsite <target>          off-site backup target (BOTHY_BACKUP_OFFSITE_TARGET, default empty)
  --owner-handle <handle>     handle of the first owner (a-z, 0-9, ".", "_", "-")
  --owner-name <name>         display name of the first owner
  -h, --help                  show this help

The edge tailscale needs an auth key: a hidden prompt with a terminal, otherwise the environment
variable TS_AUTHKEY (never an option, because other users can read the arguments).
Without a terminal, --public-url, --owner-handle and --owner-name are required.
EOF
}

# say prints a progress line or a note on standard error.
# Arguments: the message. Effect: one line "install.sh: <message>".
say() {
	printf 'install.sh: %s\n' "$*" >&2
}

# die aborts the run in a step (16 section 3.1, rule 4); the EXIT trap removes temporary files.
# Arguments: $1 step, $2 reason. Exit codes: 2 for the step options (usage error), 1 otherwise.
die() {
	printf 'install.sh: %s: %s\n' "$1" "$2" >&2
	if [ "$1" = options ]; then
		exit 2
	fi
	exit 1
}

# cleanup restores the terminal and removes the temporary files of the run (EXIT trap).
# Arguments: none. Effect: none on the exit status.
cleanup() {
	if [ -n "$stty_saved" ]; then
		stty "$stty_saved" </dev/tty 2>/dev/null
	fi
	if [ -n "$tmp_dir" ]; then
		rm -rf -- "$tmp_dir"
	fi
	if [ -n "$env_tmp" ]; then
		rm -f -- "$env_tmp"
	fi
}

# ---- validation -----------------------------------------------------------------------------

# valid_env_value reports whether a value can stand unquoted in .env: Compose would read
# whitespace, quotes, a backslash or "$" differently from what the operator typed.
# Arguments: $1 value (may be empty). Exit code: 0 valid, 1 not.
valid_env_value() {
	case $1 in
	*[[:space:][:cntrl:]\"\'\\\$\`]*) return 1 ;;
	esac
	return 0
}

# valid_url reports whether a value is a public address (http:// or https://, .env-safe).
# Arguments: $1 value. Exit code: 0 valid, 1 not.
valid_url() {
	valid_env_value "$1" || return 1
	case $1 in
	https://?* | http://?*) return 0 ;;
	esac
	return 1
}

# valid_edge, valid_on_off and valid_storage check the values of a choice.
# Arguments: $1 value. Exit code: 0 valid, 1 not.
valid_edge() {
	[ "$1" = direct ] || [ "$1" = tailscale ]
}
valid_on_off() {
	[ "$1" = on ] || [ "$1" = off ]
}
valid_storage() {
	[ "$1" = fs ] || [ "$1" = s3 ]
}

# valid_handle reports whether a value matches ^[a-z0-9][a-z0-9._-]*$ (LC_ALL=C).
# Arguments: $1 value. Exit code: 0 valid, 1 not.
valid_handle() {
	case $1 in
	[a-z0-9]*) ;;
	*) return 1 ;;
	esac
	case $1 in
	*[!a-z0-9._-]*) return 1 ;;
	esac
	return 0
}

# valid_name reports whether a value is a display name: not empty, no control characters.
# Arguments: $1 value. Exit code: 0 valid, 1 not.
valid_name() {
	case $1 in
	'' | *[[:cntrl:]]*) return 1 ;;
	esac
	return 0
}

# valid_offsite reports whether a value is an off-site target for .env (empty allowed).
# Arguments: $1 value. Exit code: 0 valid, 1 not.
valid_offsite() {
	valid_env_value "$1"
}

# duration_seconds converts a duration (<n>s, <n>m, <n>h or a number of seconds) to seconds.
# Arguments: $1 duration. Prints the seconds. Exit code: 1 when it is not a positive duration.
duration_seconds() {
	ds_num=${1%[smh]}
	ds_unit=${1#"$ds_num"}
	case $ds_num in
	'' | *[!0-9]*) return 1 ;;
	esac
	# A leading zero would make an octal number in $(( )).
	ds_num=$(printf '%s\n' "$ds_num" | sed 's/^0*//')
	[ -n "$ds_num" ] && [ ${#ds_num} -le 9 ] || return 1
	case $ds_unit in
	'' | s) ds_mult=1 ;;
	m) ds_mult=60 ;;
	*) ds_mult=3600 ;;
	esac
	printf '%s\n' "$((ds_num * ds_mult))"
}

# version_at_least reports whether a version is at least a minimum, numerically per component;
# a leading "v" and a suffix after the numbers (-desktop.1, +build) are ignored.
# Arguments: $1 version, $2 minimum (X.Y.Z). Exit code: 0 at least, 1 older or not a version.
version_at_least() {
	va_have=$(printf '%s\n' "$1" | sed -e 's/^v//' -e 's/[^0-9.].*//')
	case $va_have in
	[0-9]*) ;;
	*) return 1 ;;
	esac
	awk -v have="$va_have" -v want="$2" 'BEGIN {
		split(have, h, "."); split(want, w, ".")
		for (i = 1; i <= 3; i++) {
			if (h[i] + 0 > w[i] + 0) exit 0
			if (h[i] + 0 < w[i] + 0) exit 1
		}
		exit 0
	}'
}

# ---- options --------------------------------------------------------------------------------

# set_option checks and stores the value of one option.
# Arguments: $1 option name (--dir ...), $2 value. Exit code: 2 (die options) on a bad value.
set_option() {
	case $1 in
	--dir)
		[ -n "$2" ] || die options "--dir needs a path"
		opt_dir=$2
		;;
	--bundle)
		[ -n "$2" ] || die options "--bundle needs a file"
		opt_bundle=$2
		;;
	--ready-timeout)
		ready_seconds=$(duration_seconds "$2") ||
			die options "--ready-timeout must be a positive duration (<n>s, <n>m, <n>h or seconds), got '$2'"
		opt_ready_timeout=$2
		;;
	--public-url)
		valid_url "$2" ||
			die options "--public-url must start with https:// or http:// and contain no whitespace, quotes, backslash or \$, got '$2'"
		opt_public_url=$2
		;;
	--edge)
		valid_edge "$2" || die options "--edge must be direct or tailscale, got '$2'"
		opt_edge=$2
		;;
	--scan)
		valid_on_off "$2" || die options "--scan must be on or off, got '$2'"
		opt_scan=$2
		;;
	--storage)
		valid_storage "$2" || die options "--storage must be fs or s3, got '$2'"
		opt_storage=$2
		;;
	--observability)
		valid_on_off "$2" || die options "--observability must be on or off, got '$2'"
		opt_observability=$2
		;;
	--offsite)
		valid_offsite "$2" ||
			die options "--offsite must contain no whitespace, quotes, backslash or \$, got '$2'"
		opt_offsite=$2
		opt_offsite_set=1
		;;
	--owner-handle)
		valid_handle "$2" ||
			die options "--owner-handle must start with a-z or 0-9 and contain only a-z, 0-9, '.', '_' and '-', got '$2'"
		opt_owner_handle=$2
		;;
	--owner-name)
		valid_name "$2" || die options "--owner-name must not be empty or contain control characters"
		opt_owner_name=$2
		;;
	--yes | --help)
		die options "$1 takes no value"
		;;
	*)
		die options "unknown option $1 (see --help)"
		;;
	esac
}

# parse_options reads the command line; --help prints the usage and exits 0.
# Arguments: the arguments of the script. Exit code: 2 (die options) on a usage error.
parse_options() {
	while [ $# -gt 0 ]; do
		po_arg=$1
		shift
		case $po_arg in
		-h | --help)
			usage
			exit 0
			;;
		--yes)
			opt_yes=1
			continue
			;;
		--*=*)
			po_name=${po_arg%%=*}
			po_value=${po_arg#*=}
			;;
		--dir | --bundle | --ready-timeout | --public-url | --edge | --scan | --storage | \
			--observability | --offsite | --owner-handle | --owner-name)
			[ $# -gt 0 ] || die options "$po_arg needs a value"
			po_name=$po_arg
			po_value=$1
			shift
			;;
		*)
			die options "unknown option $po_arg (see --help)"
			;;
		esac
		set_option "$po_name" "$po_value"
	done
	if [ -z "$ready_seconds" ]; then
		ready_seconds=$(duration_seconds "$opt_ready_timeout")
	fi
}

# ---- step 1: preflight ----------------------------------------------------------------------

# require_tool aborts the preflight check when a command is not on PATH.
# Arguments: $1 command, $2 how to install it (install.sh installs nothing, rule 2).
require_tool() {
	command -v "$1" >/dev/null 2>&1 || die preflight "$1 is missing; $2"
}

# step_preflight checks host and target folder (16 section 3.1, step 1). It writes nothing and
# creates no folder, so an abort leaves the host as it was.
# Arguments: none. Effect: sets docker_mem. Exit code: 1 (die preflight) on a failed check.
step_preflight() {
	require_tool docker "install Docker Engine $min_engine or newer with the Compose plugin (https://docs.docker.com/engine/install/) or Docker Desktop (https://docs.docker.com/desktop/)"
	pf_engine=$(docker version --format '{{.Server.Version}}' </dev/null 2>/dev/null) ||
		die preflight "the Docker engine does not answer (docker version); start Docker Desktop, Colima or the docker service"
	version_at_least "$pf_engine" "$min_engine" ||
		die preflight "Docker Engine '$pf_engine' is older than $min_engine; update it (https://docs.docker.com/engine/install/)"
	pf_compose=$(docker compose version --short </dev/null 2>/dev/null) ||
		die preflight "Docker Compose v2 is missing (docker compose version); install the Compose plugin (https://docs.docker.com/compose/install/)"
	version_at_least "$pf_compose" "$min_compose" ||
		die preflight "Docker Compose '$pf_compose' is older than $min_compose; update the Compose plugin (https://docs.docker.com/compose/install/)"
	pf_arch=$(docker info --format '{{.Architecture}}' </dev/null 2>/dev/null) ||
		die preflight "docker info failed; check that the Docker engine runs"
	case $pf_arch in
	x86_64 | amd64 | aarch64 | arm64) ;;
	*) die preflight "the architecture '$pf_arch' of the Docker engine is not supported; Bothy runs on linux/amd64 and linux/arm64" ;;
	esac
	pf_mem=$(docker info --format '{{.MemTotal}}' </dev/null 2>/dev/null) ||
		die preflight "docker info failed; check that the Docker engine runs"
	case $pf_mem in
	'' | *[!0-9]*)
		say "note: the memory of the Docker engine is unknown ('$pf_mem'); the virus scan defaults to off"
		docker_mem=0
		;;
	*) docker_mem=$pf_mem ;;
	esac
	require_tool curl "install curl with the package manager of the system (for example: apt-get install curl)"
	require_tool tar "install tar with the package manager of the system (for example: apt-get install tar)"
	require_tool gzip "install gzip with the package manager of the system (for example: apt-get install gzip)"
	if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
		die preflight "sha256sum or shasum is missing; install coreutils (Linux) or perl (shasum)"
	fi
	if [ -n "$opt_bundle" ] && { [ ! -f "$opt_bundle" ] || [ ! -r "$opt_bundle" ]; }; then
		die preflight "the bundle $opt_bundle (--bundle) is not a readable file"
	fi
	if [ -e "$opt_dir" ] || [ -L "$opt_dir" ]; then
		[ -d "$opt_dir" ] || die preflight "$opt_dir exists and is not a folder; choose another --dir"
		if [ ! -f "$opt_dir/$marker_name" ]; then
			pf_list=$(ls -A "$opt_dir") || die preflight "cannot read the folder $opt_dir"
			[ -z "$pf_list" ] ||
				die preflight "$opt_dir is not empty and is not an earlier installation of install.sh (no $marker_name); choose another --dir"
		else
			# A second run repeats an installation; it never upgrades one: a newer bundle over a
			# .env that keeps the old BOTHY_VERSION would mix two releases (docs/operations/upgrade.md).
			pf_version=$(sed -n 's/^version=//p' "$opt_dir/$marker_name")
			[ "$pf_version" = "$BOTHY_INSTALL_VERSION" ] ||
				die preflight "$opt_dir holds an installation of version '$pf_version', this is install.sh $BOTHY_INSTALL_VERSION; it does not upgrade (docs/operations/upgrade.md)"
		fi
	fi
}

# ---- step 2: bundle -------------------------------------------------------------------------

# check_rendered aborts when the built-in values are still those of the source: the release
# build sets them (18 section 2.1, rule 3); the format tells, not a marker word.
# Arguments: none. Exit code: 1 (die bundle).
check_rendered() {
	cr_ok=1
	cr_core=${BOTHY_INSTALL_VERSION%%-*}
	cr_pre=${BOTHY_INSTALL_VERSION#"$cr_core"}
	case $cr_core in
	*[!0-9.]* | *.*.*.* | .* | *. | *..*) cr_ok=0 ;;
	[0-9]*.[0-9]*.[0-9]*) ;;
	*) cr_ok=0 ;;
	esac
	case $cr_pre in
	'') ;;
	-*[!0-9A-Za-z.-]* | -) cr_ok=0 ;;
	esac
	case $BOTHY_INSTALL_BUNDLE_SHA256 in
	*[!0-9a-f]*) cr_ok=0 ;;
	esac
	[ ${#BOTHY_INSTALL_BUNDLE_SHA256} -eq 64 ] || cr_ok=0
	case $BOTHY_INSTALL_BASE_URL in
	https://*/v"$BOTHY_INSTALL_VERSION"/) ;;
	*) cr_ok=0 ;;
	esac
	[ "$cr_ok" = 1 ] || die bundle "this is the unrendered source of install.sh; use the install.sh of a release"
}

# sha256_of prints the SHA-256 of a file in lower-case hex.
# Arguments: $1 file. Exit code: that of the hash tool.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{ print $1 }'
	else
		shasum -a 256 "$1" | awk '{ print $1 }'
	fi
}

# verify_signature checks the keyless cosign signature of the bundle when it can (18 section 6)
# and otherwise names the command. The checksum against the built-in value is the binding check;
# the signature adds the identity of the release workflow.
# Arguments: $1 bundle file, $2 signature file or empty (fetch it from the base address).
# Exit code: 1 (die bundle) when cosign rejects the signature.
verify_signature() {
	vs_bundle=$1
	vs_sig=$2
	vs_identity=
	vs_rest=${BOTHY_INSTALL_BASE_URL#https://github.com/}
	if [ "$vs_rest" != "$BOTHY_INSTALL_BASE_URL" ]; then
		vs_repo_owner=${vs_rest%%/*}
		vs_rest=${vs_rest#*/}
		vs_repo=${vs_rest%%/*}
		case "$vs_repo_owner/$vs_repo" in
		*[!A-Za-z0-9._/-]* | /* | */) ;;
		*)
			if [ "$vs_rest" = "$vs_repo/releases/download/v$BOTHY_INSTALL_VERSION/" ]; then
				vs_identity=$(printf '%s\n' "^https://github.com/$vs_repo_owner/$vs_repo/.github/workflows/release.yml@refs/tags/v" |
					sed 's/\./\\./g')
			fi
			;;
		esac
	fi
	vs_shown_sig=${vs_sig:-$vs_bundle.sigstore.json}
	if [ -n "$vs_identity" ]; then
		vs_command="cosign verify-blob --bundle $vs_shown_sig --certificate-identity-regexp '$vs_identity' --certificate-oidc-issuer https://token.actions.githubusercontent.com $vs_bundle"
	else
		vs_command="cosign verify-blob --bundle $vs_shown_sig --certificate-identity-regexp '<release workflow of the repository>' --certificate-oidc-issuer https://token.actions.githubusercontent.com $vs_bundle"
	fi
	if ! command -v cosign >/dev/null 2>&1; then
		say "note: cosign is not installed, the signature was not checked; to check it: $vs_command"
		return 0
	fi
	if [ -z "$vs_identity" ]; then
		say "note: the download address is not a GitHub release, the signature was not checked; to check it: $vs_command"
		return 0
	fi
	if [ -z "$vs_sig" ]; then
		vs_sig=$vs_bundle.sigstore.json
		if ! curl -fsSL -o "$vs_sig" "$BOTHY_INSTALL_BASE_URL${vs_bundle##*/}.sigstore.json" </dev/null; then
			say "note: no signature at $BOTHY_INSTALL_BASE_URL${vs_bundle##*/}.sigstore.json, the signature was not checked"
			return 0
		fi
	elif [ ! -f "$vs_sig" ]; then
		say "note: no signature $vs_sig next to the bundle, the signature was not checked; to check it: $vs_command"
		return 0
	fi
	# While the repository is private, the release signs with a key pair (ADR 0062): such a
	# signature has no certificate, so the keyless check would reject a genuine bundle.
	if ! grep -e '"certificate"' -e '"x509CertificateChain"' "$vs_sig" >/dev/null 2>&1; then
		say "note: the signature is made with the key pair of the private phase, not with the workflow identity; it was not checked; to check it: cosign verify-blob --key <quality/release/cosign-private-phase.pub of the repository> --bundle $vs_sig $vs_bundle"
		return 0
	fi
	cosign verify-blob --bundle "$vs_sig" --certificate-identity-regexp "$vs_identity" \
		--certificate-oidc-issuer https://token.actions.githubusercontent.com "$vs_bundle" </dev/null >&2 ||
		die bundle "the signature of $vs_bundle does not verify (cosign verify-blob); do not install this bundle"
}

# step_bundle fetches or takes the bundle, verifies it and copies its content into the target
# folder (16 section 3.1, step 2). The folder is created only here, after the preflight check;
# the marker goes in right away, so a run that aborts later can be repeated in the same folder.
# Arguments: none. Effect: sets target, compose_dir, env_file. Exit code: 1 (die bundle).
step_bundle() {
	check_rendered
	if [ ! -d "$opt_dir" ]; then
		mkdir -p -- "$opt_dir" || die bundle "cannot create the folder $opt_dir"
	fi
	target=$(cd -- "$opt_dir" && pwd) || die bundle "cannot enter the folder $opt_dir"
	compose_dir=$target/infra/compose
	env_file=$compose_dir/.env
	if [ ! -f "$target/$marker_name" ]; then
		printf 'version=%s\n' "$BOTHY_INSTALL_VERSION" >"$target/$marker_name" ||
			die bundle "cannot write $target/$marker_name"
	fi
	tmp_dir=$(mktemp -d "$target/.install-tmp.XXXXXX") || die bundle "cannot create a temporary folder in $target"
	sb_name=bothy-compose-$BOTHY_INSTALL_VERSION.tar.gz
	if [ -n "$opt_bundle" ]; then
		sb_bundle=$opt_bundle
		sb_sig=$opt_bundle.sigstore.json
	else
		sb_bundle=$tmp_dir/$sb_name
		sb_sig=
		say "downloading $BOTHY_INSTALL_BASE_URL$sb_name"
		curl -fsSL -o "$sb_bundle" "$BOTHY_INSTALL_BASE_URL$sb_name" </dev/null ||
			die bundle "the download of $BOTHY_INSTALL_BASE_URL$sb_name failed"
	fi
	sb_actual=$(sha256_of "$sb_bundle") || die bundle "cannot compute the SHA-256 of $sb_bundle"
	[ "$sb_actual" = "$BOTHY_INSTALL_BUNDLE_SHA256" ] ||
		die bundle "the SHA-256 of $sb_bundle does not match: expected $BOTHY_INSTALL_BUNDLE_SHA256, actual $sb_actual"
	verify_signature "$sb_bundle" "$sb_sig"
	sb_top=bothy-$BOTHY_INSTALL_VERSION
	tar -tzf "$sb_bundle" >"$tmp_dir/members" </dev/null || die bundle "$sb_bundle is not a readable tar.gz archive"
	while IFS= read -r sb_member; do
		case $sb_member in
		/*) die bundle "the bundle has a member with an absolute path: $sb_member" ;;
		esac
		case /$sb_member/ in
		*/../*) die bundle "the bundle has a member with '..': $sb_member" ;;
		esac
		case $sb_member in
		"$sb_top" | "$sb_top"/*) ;;
		*) die bundle "the bundle has a member outside $sb_top/: $sb_member" ;;
		esac
	done <"$tmp_dir/members"
	if ! mkdir "$tmp_dir/content" || ! tar -xzf "$sb_bundle" -C "$tmp_dir/content" </dev/null; then
		die bundle "cannot unpack $sb_bundle"
	fi
	sb_src=$tmp_dir/content/$sb_top
	[ -f "$sb_src/infra/compose/compose.yaml" ] || die bundle "the bundle has no infra/compose/compose.yaml"
	if [ -e "$sb_src/infra/compose/.env" ] || [ -e "$sb_src/infra/compose/secrets" ]; then
		die bundle "the bundle contains infra/compose/.env or infra/compose/secrets/, which only the operator writes"
	fi
	cp -R "$sb_src/." "$target/" || die bundle "cannot copy the bundle into $target"
	rm -rf -- "$tmp_dir"
	tmp_dir=
}

# ---- step 3: prompts ------------------------------------------------------------------------

# ask_line shows a question on the terminal and reads one answer from it (/dev/tty, so that
# curl ... | sh works).
# Arguments: $1 question, $2 default (shown in brackets; empty: none). Sets answer; an empty
# line gives the default. Exit code: 1 (die prompts) when the terminal input ends.
ask_line() {
	if [ -n "$2" ]; then
		printf '%s [%s]: ' "$1" "$2" >/dev/tty
	else
		printf '%s: ' "$1" >/dev/tty
	fi
	IFS= read -r answer </dev/tty || die prompts "the terminal input ended at the question: $1"
	if [ -z "$answer" ]; then
		answer=$2
	fi
}

# prompt_value asks until the answer passes a check.
# Arguments: $1 question, $2 default, $3 check function, $4 hint for a rejected answer,
# $5 1 when an empty answer is allowed. Sets answer.
prompt_value() {
	while :; do
		ask_line "$1" "$2"
		if { [ -n "$answer" ] || [ "$5" = 1 ]; } && "$3" "$answer"; then
			return 0
		fi
		printf '%s\n' "$4" >/dev/tty
	done
}

# choose gives a value with a default: the option, a prompt (terminal without --yes) or the
# default. Arguments: $1 option value (empty: not given), $2 question, $3 default, $4 check
# function, $5 hint. Sets answer.
choose() {
	if [ -n "$1" ]; then
		answer=$1
	elif [ "$tty" = 1 ] && [ "$opt_yes" = 0 ]; then
		prompt_value "$2" "$3" "$4" "$5" 0
	else
		answer=$3
	fi
}

# require gives a required value: the option or a prompt (also with --yes); without a terminal
# it aborts instead of guessing (16 section 3.1, step 3).
# Arguments: $1 option value, $2 option name, $3 question, $4 check function, $5 hint.
# Sets answer. Exit code: 1 (die prompts) without the option and without a terminal.
require() {
	if [ -n "$1" ]; then
		answer=$1
	elif [ "$tty" = 1 ]; then
		prompt_value "$3" "" "$4" "$5" 0
	else
		die prompts "$2 is missing and there is no terminal to ask for it"
	fi
}

# read_ts_authkey gives TS_AUTHKEY: a hidden prompt with a terminal (empty keeps the value of the
# environment variable), otherwise the environment variable. Never an option (ps shows options).
# Arguments: none. Sets ts_authkey. Exit code: 1 (die prompts) when no valid key is available.
read_ts_authkey() {
	rt_env=${TS_AUTHKEY:-}
	if [ "$tty" = 1 ] && { [ "$opt_yes" = 0 ] || [ -z "$rt_env" ]; }; then
		while :; do
			if [ -n "$rt_env" ]; then
				printf 'Tailscale auth key (TS_AUTHKEY, hidden; empty keeps the environment value): ' >/dev/tty
			else
				printf 'Tailscale auth key (TS_AUTHKEY, hidden): ' >/dev/tty
			fi
			stty_saved=$(stty -g </dev/tty) || die prompts "cannot read the terminal settings"
			stty -echo </dev/tty
			IFS= read -r answer </dev/tty
			rt_rc=$?
			stty "$stty_saved" </dev/tty
			stty_saved=
			printf '\n' >/dev/tty
			[ "$rt_rc" = 0 ] || die prompts "the terminal input ended at the question for TS_AUTHKEY"
			if [ -z "$answer" ]; then
				answer=$rt_env
			fi
			if [ -n "$answer" ] && valid_env_value "$answer"; then
				ts_authkey=$answer
				answer=
				return 0
			fi
			printf 'Enter the auth key from the admin console of Tailscale, without spaces or quotes.\n' >/dev/tty
		done
	fi
	[ -n "$rt_env" ] ||
		die prompts "TS_AUTHKEY is missing: without a terminal, set the environment variable TS_AUTHKEY for the edge tailscale (never an option)"
	valid_env_value "$rt_env" || die prompts "TS_AUTHKEY contains whitespace, quotes, a backslash or \$"
	ts_authkey=$rt_env
}

# note_kept_env says on standard error when options of a second run differ from the existing
# .env, which stays as it is (16 section 3.1, rule 1).
# Arguments: none.
note_kept_env() {
	nk_differs=0
	nk_profiles=,$(env_get COMPOSE_PROFILES),
	if [ -n "$opt_public_url" ] && [ "$opt_public_url" != "$(env_get BOTHY_PUBLIC_URL)" ]; then
		nk_differs=1
	fi
	if [ -n "$opt_edge" ]; then
		case $nk_profiles in *",$opt_edge,"*) ;; *) nk_differs=1 ;; esac
	fi
	for nk_pair in "scan:$opt_scan" "observability:$opt_observability"; do
		nk_profile=${nk_pair%%:*}
		case ${nk_pair#*:}:$nk_profiles in
		on:*",$nk_profile,"* | off:* | :*) ;;
		*) nk_differs=1 ;;
		esac
		case ${nk_pair#*:}:$nk_profiles in
		off:*",$nk_profile,"*) nk_differs=1 ;;
		esac
	done
	if [ -n "$opt_storage" ] && [ "$opt_storage" != "$(env_get BOTHY_STORAGE_DRIVER)" ]; then
		nk_differs=1
	fi
	if [ "$opt_offsite_set" = 1 ] && [ "$opt_offsite" != "$(env_get BOTHY_BACKUP_OFFSITE_TARGET)" ]; then
		nk_differs=1
	fi
	if [ "$nk_differs" = 1 ]; then
		say "note: the existing $env_file is kept; options that differ from it have no effect (edit it by hand)"
	fi
}

# step_prompts collects the answers (16 section 3.1, step 3): options first, then prompts on the
# terminal, then the defaults. A second run takes everything from the existing .env and asks
# only for the first owner, which step owner needs.
# Arguments: none. Effect: sets first_run, edge, scan, storage, observability, offsite,
# ts_authkey, opt_owner_handle, opt_owner_name. Exit code: 1 (die prompts).
step_prompts() {
	if [ -f "$env_file" ]; then
		first_run=0
	fi
	if [ "$first_run" = 1 ]; then
		require "$opt_public_url" --public-url "Public address of the server (BOTHY_PUBLIC_URL, for example https://chat.example.com)" \
			valid_url "The address starts with https:// or http:// and has no spaces or quotes."
		opt_public_url=$answer
		choose "$opt_edge" "Edge: direct (reverse proxy) or tailscale" direct valid_edge "Answer direct or tailscale."
		edge=$answer
		if [ "$edge" = tailscale ]; then
			read_ts_authkey
		fi
		if [ "$docker_mem" -ge 4294967296 ]; then
			sp_scan_default=on
		else
			sp_scan_default=off
		fi
		choose "$opt_scan" "Virus scan (needs at least 3 GiB, better 4 GiB of Docker memory): on or off" \
			"$sp_scan_default" valid_on_off "Answer on or off."
		scan=$answer
		if [ "$scan" = on ] && [ "$docker_mem" -lt 3221225472 ]; then
			say "note: the virus scan is on, but the Docker engine has less than 3 GiB of memory; ClamAV needs at least 3 GiB, better 4 GiB"
		fi
		choose "$opt_storage" "File storage: fs (volume) or s3 (object storage in the stack)" fs valid_storage "Answer fs or s3."
		storage=$answer
		choose "$opt_observability" "Local observability stack: on or off" off valid_on_off "Answer on or off."
		observability=$answer
		if [ "$opt_offsite_set" = 1 ]; then
			offsite=$opt_offsite
		elif [ "$tty" = 1 ] && [ "$opt_yes" = 0 ]; then
			prompt_value "Off-site backup target (empty: local backup only)" "" valid_offsite \
				"The target has no spaces or quotes, for example s3:https://s3.example.com/bothy-backup." 1
			offsite=$answer
		fi
	else
		note_kept_env
	fi
	require "$opt_owner_handle" --owner-handle "Handle of the first owner (a-z, 0-9, '.', '_', '-')" \
		valid_handle "The handle starts with a-z or 0-9 and has only a-z, 0-9, '.', '_' and '-'."
	opt_owner_handle=$answer
	require "$opt_owner_name" --owner-name "Display name of the first owner" valid_name "Enter a name."
	opt_owner_name=$answer
	answer=
}

# ---- step 4: write --------------------------------------------------------------------------

# env_get prints the value of the last line KEY= of .env, without one pair of enclosing quotes.
# Arguments: $1 key.
env_get() {
	awk -v key="$1" 'index($0, key "=") == 1 { v = substr($0, length(key) + 2) }
		END {
			if (v ~ /^".*"$/ || v ~ /^\047.*\047$/) v = substr(v, 2, length(v) - 2)
			print v
		}' "$env_file"
}

# write_env_file replaces .env atomically with the output of awk over a source file: a new
# file with umask 077 and mode 0600 in the compose folder, then mv.
# Arguments: $1 source file, $2 awk program. The values come from the shell variables IW_*, which
# only the subshell of awk exports (an assignment in front of a function call is not portable).
# Exit code: 1 when the file cannot be written; awk exit code 3 means a key line is missing.
write_env_file() {
	env_tmp=$compose_dir/.env.install-tmp
	rm -f -- "$env_tmp"
	if ! (umask 077 && export IW_KEY IW_VALUE IW_PROFILES IW_URL IW_DRIVER IW_OFFSITE IW_TSKEY IW_SET_TSKEY &&
		awk "$2" "$1" >"$env_tmp"); then
		return 1
	fi
	if ! chmod 600 "$env_tmp" || ! mv -f -- "$env_tmp" "$env_file"; then
		return 1
	fi
	env_tmp=
}

# env_set rewrites only the lines KEY= of .env (appends one when there is none); mode 0600.
# Arguments: $1 key, $2 value. Exit code: 1 when .env cannot be written.
env_set() {
	IW_KEY=$1
	IW_VALUE=$2
	# shellcheck disable=SC2016 # awk program, expanded by awk
	write_env_file "$env_file" '
		index($0, ENVIRON["IW_KEY"] "=") == 1 { print ENVIRON["IW_KEY"] "=" ENVIRON["IW_VALUE"]; seen = 1; next }
		{ print }
		END { if (!seen) print ENVIRON["IW_KEY"] "=" ENVIRON["IW_VALUE"] }'
}

# step_write writes .env from the .env.example of the bundle and creates secrets/ (16 section
# 3.1, step 4); only the answers change, BOTHY_VERSION and BOTHY_REGISTRY stay those of the
# bundle. No secret except TS_AUTHKEY goes into .env. A second run leaves both as they are.
# Arguments: none. Exit code: 1 (die write).
step_write() {
	if [ "$first_run" = 0 ]; then
		return 0
	fi
	[ -f "$compose_dir/.env.example" ] || die write "the bundle has no infra/compose/.env.example"
	sw_profiles=$edge
	if [ "$scan" = on ]; then
		sw_profiles=$sw_profiles,scan
	fi
	if [ "$storage" = s3 ]; then
		sw_profiles=$sw_profiles,s3
	fi
	if [ "$observability" = on ]; then
		sw_profiles=$sw_profiles,observability
	fi
	sw_set_key=0
	if [ "$edge" = tailscale ]; then
		sw_set_key=1
	fi
	IW_PROFILES=$sw_profiles
	IW_URL=$opt_public_url
	IW_DRIVER=$storage
	IW_OFFSITE=$offsite
	IW_TSKEY=$ts_authkey
	IW_SET_TSKEY=$sw_set_key
	# shellcheck disable=SC2016 # awk program, expanded by awk
	write_env_file "$compose_dir/.env.example" '
		BEGIN {
			want["COMPOSE_PROFILES"] = ENVIRON["IW_PROFILES"]
			want["BOTHY_PUBLIC_URL"] = ENVIRON["IW_URL"]
			want["BOTHY_STORAGE_DRIVER"] = ENVIRON["IW_DRIVER"]
			want["BOTHY_BACKUP_OFFSITE_TARGET"] = ENVIRON["IW_OFFSITE"]
			if (ENVIRON["IW_SET_TSKEY"] == "1") want["TS_AUTHKEY"] = ENVIRON["IW_TSKEY"]
		}
		{
			eq = index($0, "=")
			if (eq > 1) {
				key = substr($0, 1, eq - 1)
				if (key in want) { print key "=" want[key]; seen[key] = 1; next }
			}
			print
		}
		END {
			for (key in want) if (!(key in seen)) { print "missing line " key "=" > "/dev/stderr"; exit 3 }
		}' || die write "cannot write $env_file from $compose_dir/.env.example"
	ts_authkey=
	IW_TSKEY=
	if [ ! -d "$compose_dir/secrets" ]; then
		(umask 077 && mkdir "$compose_dir/secrets") || die write "cannot create $compose_dir/secrets"
	fi
	chmod 700 "$compose_dir/secrets" || die write "cannot set the mode of $compose_dir/secrets"
	say "wrote $env_file and $compose_dir/secrets/"
}

# ---- step 5: start --------------------------------------------------------------------------

# dc runs docker compose in the compose folder with no project option (the project name comes
# from compose.yaml) and with stdin from /dev/null, so it cannot eat a script piped into sh.
# Arguments: the arguments of docker compose. Exit code: that of docker compose.
dc() {
	(cd "$compose_dir" && docker compose "$@" </dev/null)
}

# wait_until repeats a check until it succeeds or a time is over.
# Arguments: $1 seconds (install.ready_timeout), then the check command.
# Exit code: 0 when the check succeeded in time, 1 otherwise.
wait_until() {
	wu_deadline=$(($(date +%s) + $1))
	shift
	while :; do
		if "$@" >/dev/null 2>&1; then
			return 0
		fi
		[ "$(date +%s)" -lt "$wu_deadline" ] || return 1
		# Poll interval, not a bound: the bound is install.ready_timeout.
		sleep 2
	done
}

# network_name prints networks.default.name of the JSON of docker compose config: a small JSON
# walker, because the key name alone also occurs in services and other networks.
# Arguments: the JSON on stdin.
network_name() {
	awk '
	{ doc = doc $0 "\n" }
	END {
		n = length(doc); i = 1; depth = 0
		while (i <= n) {
			c = substr(doc, i, 1)
			if (c == " " || c == "\t" || c == "\n" || c == "\r") { i++; continue }
			if (c == "{") { depth++; kind[depth] = "o"; wantkey[depth] = 1; key[depth] = ""; i++; continue }
			if (c == "[") { depth++; kind[depth] = "a"; wantkey[depth] = 0; key[depth] = ""; i++; continue }
			if (c == "}" || c == "]") { depth--; i++; continue }
			if (c == ":") { wantkey[depth] = 0; i++; continue }
			if (c == ",") { if (kind[depth] == "o") wantkey[depth] = 1; i++; continue }
			if (c == "\"") {
				s = ""; i++
				while (i <= n) {
					c = substr(doc, i, 1)
					if (c == "\\") { s = s substr(doc, i, 2); i += 2; continue }
					i++
					if (c == "\"") break
					s = s c
				}
				if (kind[depth] == "o" && wantkey[depth]) { key[depth] = s; continue }
				if (depth == 3 && kind[1] == "o" && kind[2] == "o" && kind[3] == "o" &&
				    key[1] == "networks" && key[2] == "default" && key[3] == "name") { print s; exit }
				continue
			}
			while (i <= n && index(",}] \t\r\n", substr(doc, i, 1)) == 0) i++
		}
	}'
}

# set_trusted_proxies writes the gateways of the Compose network default into
# BOTHY_TRUSTED_PROXIES: in the profile direct the server sees the gateway as the peer, not the
# reverse proxy, and with loopback only it rejects every request (16 section 4.3, SEC-TLS-1).
# Arguments: none. Exit code: 1 (die start).
set_trusted_proxies() {
	tp_json=$(dc config --format json) || die start "docker compose config --format json failed"
	tp_net=$(printf '%s\n' "$tp_json" | network_name)
	[ -n "$tp_net" ] || die start "docker compose config --format json has no networks.default.name"
	tp_gateways=$(docker network inspect --format '{{range .IPAM.Config}}{{.Gateway}} {{end}}' "$tp_net" </dev/null) ||
		die start "docker network inspect $tp_net failed"
	tp_v4=
	tp_v6=
	for tp_gw in $tp_gateways; do
		case $tp_gw in
		*[!0-9A-Fa-f:.]*) ;;
		*:*) tp_v6=${tp_v6:+$tp_v6,}$tp_gw/128 ;;
		*.*) tp_v4=${tp_v4:+$tp_v4,}$tp_gw/32 ;;
		esac
	done
	tp_value=$tp_v4
	if [ -n "$tp_v6" ]; then
		tp_value=${tp_value:+$tp_value,}$tp_v6
	fi
	[ -n "$tp_value" ] ||
		die start "the network $tp_net has no gateway; set BOTHY_TRUSTED_PROXIES in $env_file by hand (docs/spec/16-betrieb.md, section 4.3)"
	env_set BOTHY_TRUSTED_PROXIES "$tp_value" || die start "cannot write BOTHY_TRUSTED_PROXIES into $env_file"
	say "set BOTHY_TRUSTED_PROXIES=$tp_value (gateway of the network $tp_net) in $env_file"
}

# api_ready reports whether the API container answers /readyz.
# Arguments: none. Exit code: that of bothy healthcheck.
api_ready() {
	dc exec -T "$api_service" bothy healthcheck --url http://127.0.0.1:8081/readyz
}

# step_start checks and starts the stack (16 section 3.1, step 5): config check, containers
# without start, the gateway for the edge direct, start, readiness within install.ready_timeout
# and for direct the path from outside over the published port.
# Arguments: none. Effect: sets edge (from .env) and api_service. Exit code: 1 (die start).
step_start() {
	ss_profiles=,$(env_get COMPOSE_PROFILES),
	case $ss_profiles in
	*,tailscale,*)
		edge=tailscale
		api_service=api-tailscale
		;;
	*,direct,*)
		edge=direct
		api_service=api
		;;
	*) die start "COMPOSE_PROFILES in $env_file names neither direct nor tailscale" ;;
	esac
	dc config --quiet || die start "docker compose config --quiet failed; check $env_file"
	dc up --no-start || die start "docker compose up --no-start failed"
	if [ "$edge" = direct ]; then
		ss_proxies=$(env_get BOTHY_TRUSTED_PROXIES)
		if [ -z "$ss_proxies" ] || [ "$ss_proxies" = "$default_proxies" ]; then
			set_trusted_proxies
		fi
	fi
	dc up -d || die start "docker compose up -d failed; see docker compose logs in $compose_dir"
	say "waiting for /readyz of $api_service (install.ready_timeout: $opt_ready_timeout)"
	wait_until "$ready_seconds" api_ready ||
		die start "the API did not become ready within install.ready_timeout ($opt_ready_timeout); see: cd $compose_dir && docker compose logs $api_service"
	if [ "$edge" = direct ]; then
		ss_hostport=$(dc port api 8080) || die start "docker compose port api 8080 failed"
		ss_hostport=$(printf '%s\n' "$ss_hostport" | sed -n 1p)
		ss_port=${ss_hostport##*:}
		case $ss_port in
		'' | *[!0-9]*) die start "docker compose port api 8080 gave '$ss_hostport', not <host>:<port>" ;;
		esac
		ss_url=http://127.0.0.1:$ss_port/api/v1/instance/discovery
		ss_status=$(curl -sS -o /dev/null -w '%{http_code}' "$ss_url" </dev/null)
		[ "$ss_status" = 200 ] ||
			die start "GET $ss_url answered '$ss_status', not 200; check BOTHY_TRUSTED_PROXIES in $env_file (gateway of the Compose network, docs/spec/16-betrieb.md, section 4.3)"
	fi
}

# ---- step 6: owner --------------------------------------------------------------------------

# step_owner sets up the first owner (16 section 3.1, step 6). The enrollment code stays in a
# variable: shown once, never in a file or log. Exit code 3 of the command means an owner exists.
# Arguments: none. Effect: sets code_shown. Exit code: 1 (die owner).
step_owner() {
	so_code=$(dc exec -T "$api_service" bothy setup bootstrap-owner --handle "$opt_owner_handle" \
		--display-name "$opt_owner_name")
	so_rc=$?
	case $so_rc in
	0) ;;
	3)
		say "owner already set up; skipped"
		return 0
		;;
	*) die owner "bothy setup bootstrap-owner failed with exit code $so_rc" ;;
	esac
	[ -n "$so_code" ] || die owner "bothy setup bootstrap-owner printed no enrollment code"
	if [ "$tty" = 1 ]; then
		printf 'Enrollment code (shown once): %s\n' "$so_code" >/dev/tty
	else
		printf 'Enrollment code (shown once): %s\n' "$so_code"
	fi
	so_code=
	code_shown=1
}

# ---- step 7: finish -------------------------------------------------------------------------

# tailscale_up reports whether the tailscale sidecar is logged in.
# Arguments: none. Exit code: that of tailscale status.
tailscale_up() {
	dc exec -T tailscale tailscale status
}

# step_finish clears the one-time TS_AUTHKEY after the first login (edge tailscale) and prints
# the next steps (16 section 3.1, step 7).
# Arguments: none. Exit code: 1 (die finish) when .env cannot be written.
step_finish() {
	if [ "$edge" = tailscale ] && [ -n "$(env_get TS_AUTHKEY)" ]; then
		if wait_until "$ready_seconds" tailscale_up; then
			env_set TS_AUTHKEY '' || die finish "cannot clear TS_AUTHKEY in $env_file"
			say "cleared TS_AUTHKEY in $env_file after the first tailscale login"
		else
			say "warning: tailscale reported no login within install.ready_timeout ($opt_ready_timeout); clear the line TS_AUTHKEY= in $env_file by hand once 'docker compose exec tailscale tailscale status' succeeds"
		fi
	fi
	sf_url=$(env_get BOTHY_PUBLIC_URL)
	printf '\nBothy is installed in %s.\n\nNext steps:\n' "$compose_dir"
	printf '  1. Open the Bothy app and enter the server address: %s\n' "$sf_url"
	if [ "$code_shown" = 1 ]; then
		printf '  2. Enter the enrollment code shown above to enroll the first device of the owner.\n'
	else
		printf '  2. The owner was set up before; a new code for an owner: cd %s && docker compose exec %s bothy setup issue-enrollment-code --user %s\n' \
			"$compose_dir" "$api_service" "$opt_owner_handle"
	fi
	printf '  3. Escrow the secrets, apart from the server and the off-site target (docs/spec/16-betrieb.md, section 5.4):\n'
	printf '     cd %s && docker compose run --rm -T secrets-init secrets export --from /run/bothy/secrets/_store --age-recipient <age public key> > bundle.age\n' "$compose_dir"
	printf '  4. See the state: cd %s && docker compose ps\n' "$compose_dir"
	if [ -z "$(env_get BOTHY_BACKUP_OFFSITE_TARGET)" ]; then
		printf '\nWarning: local backup only. BOTHY_BACKUP_OFFSITE_TARGET in %s is empty, so a loss of the host loses the backups too.\n' "$env_file"
	fi
}

# main runs the steps in order.
# Arguments: the arguments of the script. Exit codes: 0, 1 (a step failed), 2 (usage error).
main() {
	LC_ALL=C
	export LC_ALL
	set -u
	set -f
	trap cleanup EXIT
	trap 'exit 1' HUP INT TERM
	parse_options "$@"
	if (: </dev/tty) 2>/dev/null; then
		tty=1
	fi
	step_preflight
	step_bundle
	step_prompts
	step_write
	step_start
	step_owner
	step_finish
}

main "$@"
