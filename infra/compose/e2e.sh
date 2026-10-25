#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Desktop E2E stack (brief M4-34 Design note 1; docs/spec/15-client-architecture.md 20): starts the
# server from the real bothy-server image with the unchanged installation file compose.yaml, the
# smoke overlay test/compose.smoke.yaml and the E2E overlay test/compose.e2e.yaml (a TLS front on
# 127.0.0.1 with a free port, the world job and the Fake-Agent), prepares the world through the
# public API and runs the given command against it. `make e2e-desktop` and `make perf-desktop`
# run it with the suite of the app (apps/client/tool/e2e_suite.sh). Documentation:
# infra/compose/README.md and infra/compose/test/README.md.
#
# Usage: infra/compose/e2e.sh [COMMAND [ARG...]]
#   The command runs with these variables (inputs of the suite, never printed by this script):
#   E2E_CONFIG      e2e.json of the world (mode 0600: the address, IDs, enrollment codes and
#                   the password of Alex)
#   E2E_CA          the PEM of the test CA the TLS front's certificate comes from
#   E2E_SERVER_URL  the address the app types (server.public_url, https://localhost:<port>)
#   Without a command the stack is prepared and removed again (with E2E_KEEP=1 it stays).
#
# Inputs (defaults only here):
#   E2E_SERVER_IMAGE      server image (default: the local tag of `make image`)
#   E2E_KEEP=1            keep the stack and the temp dir, print how to remove them
#   E2E_WAIT_TIMEOUT_S    bound of every wait of the stack (default 240)
#   E2E_LOG_TAIL          log lines per service printed on failure (default 200)
#
# Exit codes: 0 pass, 1 a failure of the stack or of the command, 2 precondition missing (P-09:
# nothing is skipped). Compatible with bash 3.2 (the default shell of macOS).

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"

# Minimum versions of smoke.sh: Engine 26.0.0 for volume subpath, Compose 2.26.0 for !reset,
# !override and dockerfile_inline.
readonly min_engine=26.0.0
readonly min_compose=2.26.0

phase=preflight
step=inputs
tmp=""
tmp_made=""
project=""

say() { printf 'e2e: %s\n' "$*" >&2; }
announce() { step="$1"; say "$1"; }
precondition() { say "$*"; exit 2; }

# ---- inputs ----------------------------------------------------------------------------------

# default_image returns the tag `make image` writes: IMAGE_NAME:IMAGE_TAG of the Makefile with
# SERVER_VERSION from services/server/VERSION (the same function as smoke.sh).
default_image() {
	local name tag version
	name="$(sed -n 's/^IMAGE_NAME ?= //p' "$root/Makefile")"
	tag="$(sed -n 's/^IMAGE_TAG ?= //p' "$root/Makefile")"
	version="$(tr -d '[:space:]' <"$root/services/server/VERSION")"
	tag="${tag//\$(SERVER_VERSION)/$version}"
	printf '%s:%s' "$name" "$tag"
}

image="${E2E_SERVER_IMAGE:-$(default_image)}"
keep="${E2E_KEEP:-0}"
wait_timeout="${E2E_WAIT_TIMEOUT_S:-240}"
log_tail="${E2E_LOG_TAIL:-200}"
for pair in "E2E_WAIT_TIMEOUT_S=$wait_timeout" "E2E_LOG_TAIL=$log_tail"; do
	case "${pair#*=}" in
	'' | *[!0-9]*) precondition "${pair%%=*} must be a whole number of seconds or lines, got '${pair#*=}'" ;;
	esac
done
[ "$wait_timeout" -gt 0 ] || precondition "E2E_WAIT_TIMEOUT_S must be greater than 0"

# ---- sanitize --------------------------------------------------------------------------------
# A developer's shell must not change the stack: every exported BOTHY_*, COMPOSE_*, OTEL_*,
# SMOKE_* and E2E_* variable goes (the inputs above are read already); Compose reads its
# variables from test/smoke.env and test/e2e.env and from what this script exports.
for name in $(compgen -e); do
	case "$name" in
	BOTHY_* | COMPOSE_* | OTEL_* | SMOKE_* | E2E_*) unset "$name" ;;
	esac
done

# ---- preflight (exit 2) ----------------------------------------------------------------------

# version_ge reports whether version $1 is at least $2 (numeric per component, a leading v and a
# suffix after the numbers ignored).
version_ge() {
	local a b i x y
	a="${1#v}"
	b="${2#v}"
	a="${a%%[!0-9.]*}"
	b="${b%%[!0-9.]*}"
	for i in 1 2 3; do
		x="$(printf '%s' "$a" | cut -d. -f"$i")"
		y="$(printf '%s' "$b" | cut -d. -f"$i")"
		x="${x:-0}"
		y="${y:-0}"
		if [ "$x" -gt "$y" ]; then return 0; fi
		if [ "$x" -lt "$y" ]; then return 1; fi
	done
	return 0
}

announce preflight
command -v docker >/dev/null 2>&1 ||
	precondition "docker is not on PATH; install Docker Engine $min_engine or newer with Compose $min_compose or newer"
docker info >/dev/null 2>&1 ||
	precondition "no Docker runtime is reachable (docker info failed); start Docker or set DOCKER_HOST"
engine="$(docker version --format '{{.Server.Version}}' 2>/dev/null)" ||
	precondition "cannot read the Docker Engine version; start Docker or set DOCKER_HOST"
version_ge "$engine" "$min_engine" ||
	precondition "Docker Engine $engine is too old; $min_engine or newer is needed (volume subpath); update Docker"
compose_version="$(docker compose version --short 2>/dev/null)" ||
	precondition "docker compose is missing; install the Compose plugin $min_compose or newer"
version_ge "$compose_version" "$min_compose" ||
	precondition "Docker Compose $compose_version is too old; $min_compose or newer is needed (!reset, !override); update Compose"
docker image inspect "$image" >/dev/null 2>&1 ||
	precondition "server image '$image' not found locally; build it with 'make image' or pull it, then set E2E_SERVER_IMAGE"
for tool in openssl curl uv; do
	command -v "$tool" >/dev/null 2>&1 ||
		precondition "$tool is not on PATH; it is needed for the test CA, the readiness probe and the requirements of the Fake-Agent (run make bootstrap for uv)"
done
say "Docker Engine $engine, Compose $compose_version, image $image"

# ---- workspace -------------------------------------------------------------------------------
announce workspace
phase=run
hex() { od -An -N"$1" -tx1 /dev/urandom | tr -d ' \n'; }
umask 077
tmp_made="$(mktemp -d "${TMPDIR:-/tmp}/bothy-e2e.XXXXXXXX")"
tmp="$tmp_made"
mkdir -m 0700 "$tmp/secrets-input" "$tmp/out" "$tmp/certs" "$tmp/agent"
run_id="$(hex 4)"
project="bothy-e2e-$run_id"
uid_gid="$(id -u):$(id -g)"

# free_port returns a port of 127.0.0.1 the kernel hands out for port 0 (its ephemeral range, so
# never a reserved port of the host rules); the stack binds it right after.
free_port() {
	uv run --project "$root/tools" --locked --quiet python -c \
		'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()'
}
port="$(free_port)"
case "$port" in
'' | *[!0-9]*) say "could not find a free port"; exit 1 ;;
esac
public_url="https://localhost:$port"
export SMOKE_SERVER_IMAGE="$image" SMOKE_RUN_ID="$run_id" SMOKE_SECRETS_INPUT="$tmp/secrets-input" SMOKE_OUT="$tmp/out" SMOKE_TMP="$tmp" SMOKE_CANARY="e2e$(hex 12)"
export BOTHY_PUBLIC_URL="$public_url" E2E_PORT="$port" E2E_UID_GID="$uid_gid" E2E_CERTS="$tmp/certs" E2E_AGENT_CONTEXT="$tmp/agent"

compose() {
	docker compose --project-name "$project" --project-directory "$here" \
		--file "$here/compose.yaml" --file "$here/test/compose.smoke.yaml" --file "$here/test/compose.e2e.yaml" \
		--env-file "$here/test/smoke.env" --env-file "$here/test/e2e.env" "$@"
}

# diagnose prints the failed step, the containers and the last log lines of the services.
diagnose() {
	say "FAILED at step: $step"
	compose ps --all >&2 || true
	local s
	for s in api worker migrate e2e-tls e2e-agent; do
		say "last $log_tail log lines of $s:"
		compose logs --no-color --tail "$log_tail" "$s" >&2 || true
	done
}

# remove_tmp removes the temp dir, but only the directory mktemp -d created for this run.
remove_tmp() {
	if [ -n "$tmp" ] && [ "$tmp" = "$tmp_made" ] && [ -d "$tmp" ]; then
		case "$tmp" in
		*/bothy-e2e.*) rm -rf "$tmp" ;;
		esac
	fi
}

# leftovers prints every container, volume and network of the project that still exists.
leftovers() {
	local label="label=com.docker.compose.project=$project"
	docker ps -a --filter "$label" --format '{{.Names}}'
	docker volume ls --filter "$label" --format '{{.Name}}'
	docker network ls --filter "$label" --format '{{.Name}}'
}

cleanup() {
	local rc=$?
	trap - EXIT
	set +eu
	if [ "$phase" = preflight ]; then
		exit "$rc"
	fi
	if [ "$rc" -ne 0 ]; then
		diagnose
		rc=1
	fi
	if [ "$keep" = 1 ]; then
		# The overlay requires its variables (${VAR:?}) for every Compose call, also for down.
		say "E2E_KEEP=1: the stack $project and $tmp are kept; remove them with:"
		say "  env SMOKE_SERVER_IMAGE=$image SMOKE_RUN_ID=$run_id SMOKE_SECRETS_INPUT=$tmp/secrets-input SMOKE_OUT=$tmp/out SMOKE_TMP=$tmp BOTHY_PUBLIC_URL=$public_url E2E_PORT=$port E2E_UID_GID=$uid_gid E2E_CERTS=$tmp/certs E2E_AGENT_CONTEXT=$tmp/agent docker compose --project-name $project --project-directory $here -f $here/compose.yaml -f $here/test/compose.smoke.yaml -f $here/test/compose.e2e.yaml --env-file $here/test/smoke.env --env-file $here/test/e2e.env down --volumes --remove-orphans --rmi local"
		say "  docker image rm bothy-smoke:$run_id; rm -rf $tmp"
		exit "$rc"
	fi
	step=cleanup
	say cleanup
	compose down --volumes --remove-orphans --rmi local --timeout "$wait_timeout" >/dev/null 2>&1
	docker image rm "bothy-smoke:$run_id" "bothy-e2e-agent:$run_id" >/dev/null 2>&1
	remove_tmp
	local left
	left="$(leftovers)"
	if [ -n "$left" ]; then
		say "cleanup left objects of $project behind:"
		printf '%s\n' "$left" >&2
		rc=1
	fi
	if [ -d "$tmp_made" ]; then
		say "cleanup could not remove $tmp_made"
		rc=1
	fi
	if [ "$rc" -eq 0 ]; then say "PASS"; else say "FAIL"; fi
	exit "$rc"
}
trap cleanup EXIT

# test_ca writes a P-256 test CA and a leaf for localhost and 127.0.0.1 into $tmp/certs; the CA
# key is removed once the leaf is signed. Valid for two days: a run takes minutes.
test_ca() {
	local d="$tmp/certs"
	cat >"$d/openssl.cnf" <<-'CNF'
		[req]
		distinguished_name = dn
		prompt = no
		[dn]
		CN = Bothy E2E test CA
		[v3_ca]
		basicConstraints = critical, CA:TRUE
		keyUsage = critical, keyCertSign, cRLSign
		subjectKeyIdentifier = hash
		[v3_leaf]
		basicConstraints = critical, CA:FALSE
		keyUsage = critical, digitalSignature
		extendedKeyUsage = serverAuth
		subjectAltName = DNS:localhost, IP:127.0.0.1
	CNF
	openssl ecparam -name prime256v1 -genkey -noout -out "$d/ca.key" 2>/dev/null
	openssl req -x509 -new -key "$d/ca.key" -sha256 -days 2 -config "$d/openssl.cnf" -extensions v3_ca -out "$d/ca.pem"
	openssl ecparam -name prime256v1 -genkey -noout -out "$d/leaf.key" 2>/dev/null
	openssl req -new -key "$d/leaf.key" -subj "/CN=localhost" -config "$d/openssl.cnf" -out "$d/leaf.csr"
	openssl x509 -req -in "$d/leaf.csr" -CA "$d/ca.pem" -CAkey "$d/ca.key" -set_serial "0x$(hex 16)" -days 2 -sha256 \
		-extfile "$d/openssl.cnf" -extensions v3_leaf -out "$d/leaf.pem" 2>/dev/null
	rm -f "$d/ca.key" "$d/leaf.csr" "$d/openssl.cnf"
}

# wait_for runs "$@" until it succeeds, once a second, at most E2E_WAIT_TIMEOUT_S seconds; exit
# status 2 of the command is a failure that no wait repairs and ends the wait at once.
wait_for() {
	local what="$1" deadline rc
	shift
	deadline=$(($(date +%s) + wait_timeout))
	while :; do
		rc=0
		"$@" >/dev/null 2>&1 || rc=$?
		[ "$rc" -ne 0 ] || return 0
		if [ "$rc" -eq 2 ]; then
			say "$what failed"
			return 1
		fi
		if [ "$(date +%s)" -ge "$deadline" ]; then
			say "timed out after ${wait_timeout}s waiting for $what"
			return 1
		fi
		sleep 1
	done
}

# agent_ready reports whether the Fake-Agent logged both agents ready: 0 ready, 1 not yet, 2 the
# Fake-Agent failed.
agent_ready() {
	local logs
	logs="$(compose logs --no-color e2e-agent 2>&1)"
	case "$logs" in *"e2e-agent: FAIL"*) return 2 ;; esac
	case "$logs" in *"ready support-agent"*) ;; *) return 1 ;; esac
	case "$logs" in *"ready stream-agent"*) ;; *) return 1 ;; esac
}

# exit_code returns the exit code of the one-shot container of service $1.
exit_code() {
	local id
	id="$(compose ps --all --quiet "$1")"
	[ -n "$id" ] || { say "no container of $1"; return 1; }
	docker inspect --format '{{.State.ExitCode}}' "$id"
}

# ---- the run ---------------------------------------------------------------------------------
announce "test CA"
test_ca

announce "requirements of the Fake-Agent"
uv export --project "$root/sdk/python" --frozen --no-dev --no-emit-project --no-header \
	--format requirements.txt >"$tmp/agent/requirements.txt"

announce validate
compose config --quiet

announce "build and pull"
compose build --quiet --build-arg VERSION="$(tr -d '[:space:]' <"$root/services/server/VERSION")" smoke-probe
compose build --quiet e2e-agent
compose pull --quiet postgres otel-collector otel-out-init e2e-tls

announce "start (secrets init, migrate up --bootstrap, TLS front)"
compose up --detach --wait --wait-timeout "$wait_timeout" api worker otel-collector e2e-tls
for s in secrets-init migrate; do
	code="$(exit_code "$s")"
	[ "$code" = 0 ] || { say "$s exited with $code"; exit 1; }
done

announce "bootstrap owner"
: >"$tmp/out/setup.out"
chmod 0600 "$tmp/out/setup.out"
compose exec -T api bothy setup bootstrap-owner --handle alex --display-name "Alex" \
	>"$tmp/out/setup.out" 2>"$tmp/out/setup.err"

announce "world (space, channels, Sam, agents, grants, codes)"
compose run --rm --no-deps --user "$uid_gid" e2e-world

announce "Fake-Agent"
compose up --detach --no-deps e2e-agent
wait_for "the Fake-Agent" agent_ready

announce "TLS front from the host"
wait_for "$public_url" curl --silent --fail --cacert "$tmp/certs/ca.pem" "$public_url/api/v1/instance/discovery"

export E2E_CONFIG="$tmp/out/e2e.json" E2E_CA="$tmp/certs/ca.pem" E2E_SERVER_URL="$public_url"
if [ "$#" -gt 0 ]; then
	announce "run: $1"
	"$@"
fi
