#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# bothy-setup: host setup of the OpenTofu module generic-vm (brief M7-15, ADR 0012,
# infra/tofu/modules/generic-vm/README.md). cloud-init runs it once on a new host; the SSH
# delivery of the module runs it on an existing host. Both write the same files first:
#
#   <dir>/setup.env         KEY='value' lines rendered from the module inputs (no secrets)
#   <dir>/install-options   the options of install.sh, one per line
#   <dir>/firewall.nft      the nftables table of the host (only the edge port public)
#   <dir>/release.pub       public key of the private signing phase (only then)
#   <dir>/tailscale-auth-key  edge tailscale only: the one-time key, placed by the operator
#
# <dir> is BOTHY_SETUP_DIR, default /etc/bothy. Steps: packages and Docker Engine with the
# Compose plugin from the Docker package repository, the user bothy, the pinned cosign, the
# release files (SHA256SUMS, bundle, install.sh, each with its .sigstore.json), their
# verification before anything is unpacked, the firewall, the setup wizard
# `install.sh --bundle` as the user bothy, and the system service bothy.service. A second run
# changes nothing (16 3.1 rule 1 and every step compares before it writes).
#
# Usage: bothy-setup (as root, no arguments). Exit codes: 0 done, 1 a step failed; every
# message names the step ("bothy-setup: <step>: <reason>").

set -eu
umask 022
LC_ALL=C
export LC_ALL

conf_dir=${BOTHY_SETUP_DIR:-/etc/bothy}
lib_dir=/usr/local/lib/bothy
min_engine=26.0.0
min_compose=2.26.0
# Signing key of the Docker package repository, from https://docs.docker.com/engine/install/ubuntu/
# and .../debian/ (retrieved 2026-10-20).
docker_key_fpr=9DC858229FC7DD38854AE2D88D81803C0EBFCD88
work=

say() {
	printf 'bothy-setup: %s\n' "$*" >&2
}

die() {
	printf 'bothy-setup: %s: %s\n' "$1" "$2" >&2
	exit 1
}

cleanup() {
	if [ -n "$work" ] && [ -d "$work" ]; then
		rm -rf -- "$work"
	fi
}

# cfg prints the value of KEY from setup.env (the module writes KEY='value' without quotes in
# the value). Arguments: $1 key, $2 "optional" when an empty value is allowed.
cfg() {
	cfg_v=$(sed -n "s/^$1='\\(.*\\)'\$/\\1/p" "$conf_dir/setup.env" | tail -n 1)
	if [ -z "$cfg_v" ] && [ "${2:-}" != optional ]; then
		die config "$1 is missing in $conf_dir/setup.env"
	fi
	printf '%s' "$cfg_v"
}

# version_ge reports whether version $1 is at least $2 (numeric per component).
version_ge() {
	vg_a=${1#v}
	vg_b=${2#v}
	vg_a=${vg_a%%[!0-9.]*}
	vg_b=${vg_b%%[!0-9.]*}
	for vg_i in 1 2 3; do
		vg_x=$(printf '%s' "$vg_a" | cut -d. -f"$vg_i")
		vg_y=$(printf '%s' "$vg_b" | cut -d. -f"$vg_i")
		vg_x=${vg_x:-0}
		vg_y=${vg_y:-0}
		[ "$vg_x" -gt "$vg_y" ] && return 0
		[ "$vg_x" -lt "$vg_y" ] && return 1
	done
	return 0
}

# put_file writes standard input to $1 with mode $2 only when the content differs.
# Exit code: 0 written, 1 unchanged.
put_file() {
	pf_tmp=$1.bothy-setup-tmp
	cat >"$pf_tmp"
	if [ -f "$1" ] && cmp -s "$pf_tmp" "$1"; then
		rm -f -- "$pf_tmp"
		return 1
	fi
	chmod "$2" "$pf_tmp"
	mv -f -- "$pf_tmp" "$1"
	return 0
}

apt_get() {
	DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=600 -q -y "$@"
}

# ---- steps -----------------------------------------------------------------------------------

step_preflight() {
	[ "$(id -u)" = 0 ] || die preflight "run as root"
	[ -r "$conf_dir/setup.env" ] || die preflight "$conf_dir/setup.env is missing"
	[ -r "$conf_dir/install-options" ] || die preflight "$conf_dir/install-options is missing"
	[ -r "$conf_dir/firewall.nft" ] || die preflight "$conf_dir/firewall.nft is missing"
	[ -r /etc/os-release ] || die preflight "/etc/os-release is missing"
	# shellcheck disable=SC1091 # the file of the host, read in a subshell
	os_id=$(. /etc/os-release && printf '%s' "$ID")
	# shellcheck disable=SC1091
	os_codename=$(. /etc/os-release && printf '%s' "${VERSION_CODENAME:-}")
	case $os_id in
	ubuntu | debian) ;;
	*) die preflight "the host runs '$os_id'; Ubuntu or Debian with systemd is supported" ;;
	esac
	[ -n "$os_codename" ] || die preflight "VERSION_CODENAME is missing in /etc/os-release"
	command -v systemctl >/dev/null 2>&1 || die preflight "systemd is missing"
	arch=$(dpkg --print-architecture)
	case $arch in
	amd64 | arm64) ;;
	*) die preflight "architecture $arch is not supported (amd64, arm64)" ;;
	esac
	# cloud-init runs the script without HOME; cosign keeps its cache there.
	HOME=${HOME:-/root}
	export HOME
	version=$(cfg SETUP_VERSION)
	stack_dir=$(cfg SETUP_STACK_DIR)
	bundle=bothy-compose-$version.tar.gz
}

step_packages() {
	sp_missing=
	command -v curl >/dev/null 2>&1 || sp_missing="$sp_missing curl"
	command -v gpg >/dev/null 2>&1 || sp_missing="$sp_missing gnupg"
	command -v nft >/dev/null 2>&1 || sp_missing="$sp_missing nftables"
	[ -f /etc/ssl/certs/ca-certificates.crt ] || sp_missing="$sp_missing ca-certificates"
	if [ -z "$sp_missing" ]; then
		return 0
	fi
	say "packages:$sp_missing"
	apt_get update || die packages "apt-get update failed"
	# shellcheck disable=SC2086 # a list of package names
	apt_get install --no-install-recommends $sp_missing || die packages "cannot install$sp_missing"
}

docker_ok() {
	command -v docker >/dev/null 2>&1 || return 1
	do_engine=$(docker version --format '{{.Server.Version}}' 2>/dev/null) || return 1
	do_compose=$(docker compose version --short 2>/dev/null) || return 1
	version_ge "$do_engine" "$min_engine" && version_ge "$do_compose" "$min_compose"
}

step_docker() {
	if docker_ok; then
		say "docker: Engine $do_engine, Compose $do_compose"
		return 0
	fi
	say "docker: installing Docker Engine and the Compose plugin from download.docker.com"
	install -d -m 0755 /etc/apt/keyrings
	sd_key=$(mktemp)
	curl -fsSL --proto '=https' --retry 5 "https://download.docker.com/linux/$os_id/gpg" -o "$sd_key" ||
		{ rm -f -- "$sd_key"; die docker "cannot download the key of the Docker repository"; }
	sd_fpr=$(gpg --show-keys --with-colons "$sd_key" 2>/dev/null | awk -F: '$1 == "fpr" { print $10; exit }')
	if [ "$sd_fpr" != "$docker_key_fpr" ]; then
		rm -f -- "$sd_key"
		die docker "the key of the Docker repository has the fingerprint '$sd_fpr', expected $docker_key_fpr"
	fi
	put_file /etc/apt/keyrings/docker.asc 0644 <"$sd_key" || true
	rm -f -- "$sd_key"
	printf 'deb [arch=%s signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/%s %s stable\n' \
		"$arch" "$os_id" "$os_codename" | put_file /etc/apt/sources.list.d/docker.list 0644 || true
	apt_get update || die docker "apt-get update failed"
	apt_get install docker-ce docker-ce-cli containerd.io docker-compose-plugin ||
		die docker "cannot install the Docker packages"
	systemctl enable --now docker.service >/dev/null 2>&1 || die docker "cannot start docker.service"
	docker_ok || die docker "Docker Engine $min_engine and Compose $min_compose or newer are needed after the installation"
	say "docker: Engine $do_engine, Compose $do_compose"
}

step_user() {
	if ! id bothy >/dev/null 2>&1; then
		useradd --system --user-group --create-home --home-dir /var/lib/bothy --shell /usr/sbin/nologin bothy ||
			die user "cannot create the user bothy"
	fi
	case " $(id -nG bothy) " in
	*" docker "*) ;;
	*) usermod -aG docker bothy || die user "cannot add bothy to the group docker" ;;
	esac
}

step_cosign() {
	sc_version=$(cfg SETUP_COSIGN_VERSION)
	case $arch in
	amd64) sc_sha=$(cfg SETUP_COSIGN_SHA256_AMD64) ;;
	*) sc_sha=$(cfg SETUP_COSIGN_SHA256_ARM64) ;;
	esac
	cosign=$lib_dir/cosign-$sc_version
	if [ -x "$cosign" ] && printf '%s  %s\n' "$sc_sha" "$cosign" | sha256sum --check --status; then
		return 0
	fi
	install -d -m 0755 "$lib_dir"
	sc_tmp=$(mktemp)
	curl -fsSL --proto '=https' --retry 5 -o "$sc_tmp" \
		"https://github.com/sigstore/cosign/releases/download/v$sc_version/cosign-linux-$arch" ||
		{ rm -f -- "$sc_tmp"; die cosign "cannot download cosign $sc_version"; }
	printf '%s  %s\n' "$sc_sha" "$sc_tmp" | sha256sum --check --status ||
		{ rm -f -- "$sc_tmp"; die cosign "cosign $sc_version does not match its pinned SHA-256"; }
	install -m 0755 "$sc_tmp" "$cosign"
	rm -f -- "$sc_tmp"
}

step_fetch() {
	sf_base=$(cfg SETUP_BASE_URL)
	work=$(mktemp -d /var/tmp/bothy-setup.XXXXXXXX)
	for sf_name in SHA256SUMS "$bundle" install.sh; do
		for sf_file in "$sf_name" "$sf_name.sigstore.json"; do
			curl -fsSL --proto '=https,http' --retry 5 -o "$work/$sf_file" "$sf_base$sf_file" ||
				die fetch "cannot download $sf_base$sf_file"
		done
	done
}

# verify_blob checks the signature bundle of a file in the work directory.
verify_blob() {
	case $(cfg SETUP_SIGNING) in
	key)
		[ -r "$conf_dir/release.pub" ] || die verify "$conf_dir/release.pub is missing"
		"$cosign" verify-blob --key "$conf_dir/release.pub" --insecure-ignore-tlog \
			--bundle "$work/$1.sigstore.json" "$work/$1" >/dev/null 2>&1
		;;
	keyless)
		"$cosign" verify-blob --certificate-identity-regexp "$(cfg SETUP_IDENTITY_REGEXP)" \
			--certificate-oidc-issuer "$(cfg SETUP_OIDC_ISSUER)" \
			--bundle "$work/$1.sigstore.json" "$work/$1" >/dev/null 2>&1
		;;
	*) die verify "SETUP_SIGNING must be key or keyless" ;;
	esac
}

# step_verify trusts nothing it downloaded until the signatures say so (docs/spec/18-release.md
# 6): the signature of SHA256SUMS, the checksums of bundle and install.sh from it, then their own
# signatures, then the registry the bundle pins. Only then does install.sh unpack the bundle.
step_verify() {
	verify_blob SHA256SUMS || die verify "the signature of SHA256SUMS does not verify"
	for sv_name in "$bundle" install.sh; do
		sv_line=$(awk -v n="$sv_name" '$2 == n || $2 == "*" n' "$work/SHA256SUMS")
		[ "$(printf '%s\n' "$sv_line" | grep -c .)" = 1 ] ||
			die verify "SHA256SUMS lists $sv_name not exactly once"
		(cd "$work" && printf '%s\n' "$sv_line" | sha256sum --check --strict --status) ||
			die verify "the checksum of $sv_name does not match SHA256SUMS"
	done
	verify_blob "$bundle" || die verify "the signature of $bundle does not verify"
	verify_blob install.sh || die verify "the signature of install.sh does not verify"
	sv_registry=$(tar -xzOf "$work/$bundle" "bothy-$version/infra/compose/.env.example" |
		sed -n 's/^BOTHY_REGISTRY=//p' | tail -n 1)
	[ "$sv_registry" = "$(cfg SETUP_REGISTRY)" ] ||
		die verify "the bundle pins the registry '$sv_registry', the module input image_registry is '$(cfg SETUP_REGISTRY)'"
	say "verify: signatures and checksums of $bundle and install.sh"
}

step_firewall() {
	put_file /etc/bothy/firewall.nft 0644 <"$conf_dir/firewall.nft" || true
	sw_changed=0
	put_file /etc/systemd/system/bothy-firewall.service 0644 <<-'EOF' && sw_changed=1
		[Unit]
		Description=Firewall of the Bothy host (only the edge port public)
		Before=network-pre.target docker.service
		Wants=network-pre.target

		[Service]
		Type=oneshot
		RemainAfterExit=yes
		ExecStart=/usr/sbin/nft -f /etc/bothy/firewall.nft

		[Install]
		WantedBy=multi-user.target
	EOF
	if [ "$sw_changed" = 1 ]; then
		systemctl daemon-reload
	fi
	systemctl enable bothy-firewall.service >/dev/null 2>&1 || die firewall "cannot enable bothy-firewall.service"
	nft -f /etc/bothy/firewall.nft || die firewall "nft rejected /etc/bothy/firewall.nft"
}

step_install() {
	install -d -o bothy -g bothy -m 0750 "$stack_dir"
	si_ts=
	if [ "$(cfg SETUP_EDGE)" = tailscale ] && [ ! -f "$stack_dir/.bothy-install" ]; then
		si_key_file=$conf_dir/tailscale-auth-key
		[ -r "$si_key_file" ] ||
			die install "edge tailscale: place the one-time auth key in $si_key_file (mode 0600, root) and run $lib_dir/bothy-setup again"
		si_ts=$(cat "$si_key_file")
	fi
	# install.sh runs as bothy and reads the verified files; nobody but root can change them.
	chmod 0711 "$work"
	chmod 0644 "$work/$bundle" "$work/install.sh"
	set --
	while IFS= read -r si_opt || [ -n "$si_opt" ]; do
		set -- "$@" "$si_opt"
	done <"$conf_dir/install-options"
	# install.sh prints the enrollment code of the first owner on standard output; cloud-init
	# and the SSH delivery keep that output in a log, so the code is replaced by the command
	# that issues a new one (docs/DECISIONS.md D-M7-15-4).
	(
		set +e
		cd "$stack_dir" || exit 1
		if [ -n "$si_ts" ]; then
			TS_AUTHKEY=$si_ts
			export TS_AUTHKEY
		fi
		runuser -u bothy -- /bin/sh "$work/install.sh" --bundle "$work/$bundle" --dir "$stack_dir" "$@"
		echo "$?" >"$work/install.rc"
	) | sed "s|^Enrollment code (shown once): .*|Enrollment code: withheld from this log; issue one with: cd $stack_dir/infra/compose \\&\\& sudo -u bothy docker compose exec api bothy setup issue-enrollment-code --user $(cfg SETUP_OWNER_HANDLE)|"
	si_ts=
	si_rc=$(cat "$work/install.rc" 2>/dev/null || echo 1)
	[ "$si_rc" = 0 ] || die install "install.sh failed with exit code $si_rc"
	if [ -f "$conf_dir/tailscale-auth-key" ]; then
		rm -f -- "$conf_dir/tailscale-auth-key"
	fi
}

step_service() {
	ss_changed=0
	put_file /etc/systemd/system/bothy.service 0644 <<-EOF && ss_changed=1
		[Unit]
		Description=Bothy chat server (Docker Compose stack in $stack_dir)
		Requires=docker.service
		After=docker.service network-online.target bothy-firewall.service
		Wants=network-online.target

		[Service]
		Type=oneshot
		RemainAfterExit=yes
		User=bothy
		Group=bothy
		WorkingDirectory=$stack_dir/infra/compose
		ExecStart=/usr/bin/docker compose up -d
		ExecStop=/usr/bin/docker compose stop
		TimeoutStartSec=900

		[Install]
		WantedBy=multi-user.target
	EOF
	if [ "$ss_changed" = 1 ]; then
		systemctl daemon-reload
	fi
	systemctl enable bothy.service >/dev/null 2>&1 || die service "cannot enable bothy.service"
	systemctl start bothy.service || die service "cannot start bothy.service"
	say "done: the stack runs in $stack_dir/infra/compose (systemctl status bothy.service)"
}

main() {
	trap cleanup EXIT
	trap 'exit 1' HUP INT TERM
	step_preflight
	step_packages
	step_docker
	step_user
	step_cosign
	step_fetch
	step_verify
	step_firewall
	step_install
	step_service
}

main "$@"
