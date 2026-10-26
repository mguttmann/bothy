#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# The demo instance (brief M7-27, docs/spec/16-operations.md 16, docs/operations/demo.md; make
# demo): starts the server from the real bothy-server image with the unchanged installation file
# compose.yaml, the smoke overlay test/compose.smoke.yaml and the demo overlay demo.yaml (a TLS
# front on 127.0.0.1 with a free port, the demo agent and the host service container), runs
# `bothy setup demo` in the api container on the empty instance, pairs the host service when the
# command asks for it, prints the address, the demo CA and the enrollment code of the owner, and
# keeps the demo running until Ctrl-C; then it removes the stack. Nothing calls a model or an
# external service; nothing is published.
#
# Usage: infra/compose/demo.sh
#
# Inputs (defaults only here):
#   DEMO_SERVER_IMAGE     server image (default: the local tag of `make image`)
#   DEMO_SEED             seed of the warm-up (default 1)
#   DEMO_PACE             wait between two warm-up prompts, a Go duration (default: the data set)
#   DEMO_KEEP=1           leave the stack running at the end and print how to remove it
#   DEMO_WAIT_TIMEOUT_S   bound of every wait of the stack (default 300)
#   DEMO_LOG_TAIL         log lines per service printed on failure (default 200)
#
# Exit codes: 0 the demo ran and was removed, 1 a failure of the stack or of the command, 2 a
# precondition is missing. Compatible with bash 3.2 (the default shell of macOS).

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"

# Minimum versions of smoke.sh: Engine 26.0.0 for volume subpath, Compose 2.26.0 for !reset,
# !override and dockerfile_inline.
readonly min_engine=26.0.0
readonly min_compose=2.26.0
# Where the demo host gets the binaries before bothy-host install, and where install puts them
# (the paths of the terminal suite of M2-48 and of the console E2E test).
readonly host_bin_dir=/opt/termsuite/bin
readonly host_service_bin=/opt/bothy-host/bin/bothy-host
readonly host_config=/etc/bothy-host/config.yaml
# The exchange directory inside api (demo.yaml) and inside the one-off service demo-exchange.
readonly api_exchange=/var/lib/bothy/demo-exchange

phase=preflight
step=inputs
tmp=""
tmp_made=""
project=""
setup_pid=""

say() { printf 'demo: %s\n' "$*" >&2; }
announce() { step="$1"; say "$1"; }
precondition() { say "$*"; exit 2; }

# ---- inputs ----------------------------------------------------------------------------------

# default_image returns the tag `make image` writes (the same function as smoke.sh and e2e.sh).
default_image() {
	local name tag version
	name="$(sed -n 's/^IMAGE_NAME ?= //p' "$root/Makefile")"
	tag="$(sed -n 's/^IMAGE_TAG ?= //p' "$root/Makefile")"
	version="$(tr -d '[:space:]' <"$root/services/server/VERSION")"
	tag="${tag//\$(SERVER_VERSION)/$version}"
	printf '%s:%s' "$name" "$tag"
}

image="${DEMO_SERVER_IMAGE:-$(default_image)}"
seed="${DEMO_SEED:-1}"
pace="${DEMO_PACE:-}"
keep="${DEMO_KEEP:-0}"
wait_timeout="${DEMO_WAIT_TIMEOUT_S:-300}"
log_tail="${DEMO_LOG_TAIL:-200}"
for pair in "DEMO_SEED=$seed" "DEMO_WAIT_TIMEOUT_S=$wait_timeout" "DEMO_LOG_TAIL=$log_tail"; do
	case "${pair#*=}" in
	'' | *[!0-9]*) precondition "${pair%%=*} must be a whole number, got '${pair#*=}'" ;;
	esac
done
[ "$wait_timeout" -gt 0 ] || precondition "DEMO_WAIT_TIMEOUT_S must be greater than 0"
case "$pace" in
'' | [0-9]*) ;;
*) precondition "DEMO_PACE must be a duration such as 20s, got '$pace'" ;;
esac

# ---- sanitize --------------------------------------------------------------------------------
# A developer's shell must not change the stack: every exported BOTHY_*, COMPOSE_*, OTEL_*,
# SMOKE_* and DEMO_* variable goes (the inputs above are read already).
for name in $(compgen -e); do
	case "$name" in
	BOTHY_* | COMPOSE_* | OTEL_* | SMOKE_* | DEMO_*) unset "$name" ;;
	esac
done

# ---- preflight (exit 2) ----------------------------------------------------------------------

# version_ge reports whether version $1 is at least $2 (numeric per component).
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
	precondition "Docker Engine $engine is too old; $min_engine or newer is needed; update Docker"
compose_version="$(docker compose version --short 2>/dev/null)" ||
	precondition "docker compose is missing; install the Compose plugin $min_compose or newer"
version_ge "$compose_version" "$min_compose" ||
	precondition "Docker Compose $compose_version is too old; $min_compose or newer is needed; update Compose"
docker buildx version >/dev/null 2>&1 ||
	precondition "docker buildx is missing; it builds the host image and bothy-host (CONTRIBUTING.md)"
docker image inspect "$image" >/dev/null 2>&1 ||
	precondition "server image '$image' not found locally; build it with 'make image' or set DEMO_SERVER_IMAGE"
for tool in openssl curl uv; do
	command -v "$tool" >/dev/null 2>&1 ||
		precondition "$tool is not on PATH; it is needed for the demo CA, the readiness probe and the demo agent (run make bootstrap for uv)"
done
say "Docker Engine $engine, Compose $compose_version, image $image"

# ---- workspace -------------------------------------------------------------------------------
announce workspace
phase=run
hex() { od -An -N"$1" -tx1 /dev/urandom | tr -d ' \n'; }
umask 077
tmp_made="$(mktemp -d "${TMPDIR:-/tmp}/bothy-demo.XXXXXXXX")"
tmp="$tmp_made"
mkdir -m 0700 "$tmp/secrets-input" "$tmp/out" "$tmp/certs" "$tmp/agent"
# The public certificate of the demo CA, readable by the demo agent's user 65532.
mkdir -m 0755 "$tmp/ca"
run_id="$(hex 4)"
project="bothy-demo-$run_id"
uid_gid="$(id -u):$(id -g)"

# free_port returns a port of 127.0.0.1 the kernel hands out for port 0 (its ephemeral range).
free_port() {
	uv run --project "$root/tools" --locked --quiet python -c \
		'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()'
}
port="$(free_port)"
case "$port" in
'' | *[!0-9]*) say "could not find a free port"; exit 1 ;;
esac
public_url="https://localhost:$port"
export SMOKE_SERVER_IMAGE="$image" SMOKE_RUN_ID="$run_id" SMOKE_SECRETS_INPUT="$tmp/secrets-input" SMOKE_OUT="$tmp/out" SMOKE_TMP="$tmp" SMOKE_CANARY="demo"
export BOTHY_PUBLIC_URL="$public_url" DEMO_PORT="$port" DEMO_UID_GID="$uid_gid" DEMO_CERTS="$tmp/certs" DEMO_CA="$tmp/ca" DEMO_AGENT_CONTEXT="$tmp/agent" DEMO_RUN_ID="$run_id"

compose() {
	docker compose --project-name "$project" --project-directory "$here" \
		--file "$here/compose.yaml" --file "$here/test/compose.smoke.yaml" --file "$here/demo.yaml" \
		--env-file "$here/test/smoke.env" --env-file "$here/test/demo.env" "$@"
}

# exchange runs a shell command in the exchange directory as the server user (demo-exchange).
exchange() {
	compose run --rm --no-deps -T demo-exchange "$1"
}

# host_exec runs a command as root in the demo host.
host_exec() {
	compose exec -T demo-host "$@"
}

# host_booted reports whether systemd of the demo host runs (running or degraded).
host_booted() {
	case "$(host_exec systemctl is-system-running 2>/dev/null)" in
	running* | degraded*) return 0 ;;
	esac
	return 1
}

# diagnose prints the failed step, the containers and the last log lines of the services.
diagnose() {
	say "FAILED at step: $step"
	compose ps --all >&2 || true
	local s
	for s in api worker migrate demo-tls demo-agent; do
		say "last $log_tail log lines of $s:"
		compose logs --no-color --tail "$log_tail" "$s" >&2 || true
	done
	say "the journal of the host service:"
	host_exec journalctl --no-pager -o cat -u 'bothy-host*' 2>/dev/null | tail -n "$log_tail" >&2 || true
	if [ -s "$tmp/out/setup.err" ]; then
		say "stderr of bothy setup demo:"
		cat "$tmp/out/setup.err" >&2
	fi
}

# remove_tmp removes the temp dir, but only the directory mktemp -d created for this run.
remove_tmp() {
	if [ -n "$tmp" ] && [ "$tmp" = "$tmp_made" ] && [ -d "$tmp" ]; then
		case "$tmp" in
		*/bothy-demo.*) rm -rf "$tmp" ;;
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
	trap - EXIT INT TERM
	set +eu
	if [ "$phase" = preflight ]; then
		exit "$rc"
	fi
	if [ -n "$setup_pid" ]; then
		kill "$setup_pid" 2>/dev/null
		wait "$setup_pid" 2>/dev/null
	fi
	if [ "$rc" -ne 0 ] && [ "$phase" != stop ]; then
		diagnose
		rc=1
	fi
	[ "$phase" != stop ] || rc=0
	if [ "$keep" = 1 ]; then
		say "DEMO_KEEP=1: the stack $project and $tmp are kept; remove them with:"
		say "  env SMOKE_SERVER_IMAGE=$image SMOKE_RUN_ID=$run_id SMOKE_SECRETS_INPUT=$tmp/secrets-input SMOKE_OUT=$tmp/out SMOKE_TMP=$tmp SMOKE_CANARY=demo BOTHY_PUBLIC_URL=$public_url DEMO_PORT=$port DEMO_UID_GID=$uid_gid DEMO_CERTS=$tmp/certs DEMO_CA=$tmp/ca DEMO_AGENT_CONTEXT=$tmp/agent DEMO_RUN_ID=$run_id docker compose --project-name $project --project-directory $here -f $here/compose.yaml -f $here/test/compose.smoke.yaml -f $here/demo.yaml --env-file $here/test/smoke.env --env-file $here/test/demo.env down --volumes --remove-orphans --rmi local"
		say "  docker image rm bothy-demo-host:$run_id; rm -rf $tmp"
		exit "$rc"
	fi
	step=cleanup
	say cleanup
	compose down --volumes --remove-orphans --rmi local --timeout "$wait_timeout" >/dev/null 2>&1
	docker image rm "bothy-demo-agent:$run_id" "bothy-demo-host:$run_id" >/dev/null 2>&1
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
	exit "$rc"
}
trap cleanup EXIT
trap 'phase=stop; exit 0' INT TERM

# demo_ca writes a P-256 demo CA and a leaf for localhost and 127.0.0.1 into $tmp/certs and the CA
# certificate into $tmp/ca; the CA key is removed once the leaf is signed. Valid for two days.
demo_ca() {
	local d="$tmp/certs"
	cat >"$d/openssl.cnf" <<-'CNF'
		[req]
		distinguished_name = dn
		prompt = no
		[dn]
		CN = Demo CA
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
	cp "$d/ca.pem" "$tmp/ca/ca.pem"
	chmod 0644 "$tmp/ca/ca.pem"
}

# wait_for runs "$@" until it succeeds, once a second, at most DEMO_WAIT_TIMEOUT_S seconds.
wait_for() {
	local what="$1" deadline
	shift
	deadline=$(($(date +%s) + wait_timeout))
	until "$@" >/dev/null 2>&1; do
		if [ "$(date +%s)" -ge "$deadline" ]; then
			say "timed out after ${wait_timeout}s waiting for $what"
			return 1
		fi
		if [ -n "$setup_pid" ] && ! kill -0 "$setup_pid" 2>/dev/null; then
			say "bothy setup demo ended while waiting for $what"
			return 1
		fi
		sleep 1
	done
}

# exit_code returns the exit code of the one-shot container of service $1.
exit_code() {
	local id
	id="$(compose ps --all --quiet "$1")"
	[ -n "$id" ] || { say "no container of $1"; return 1; }
	docker inspect --format '{{.State.ExitCode}}' "$id"
}

# json_field prints member $2 of the JSON file $1 (a list as one item per line).
json_field() {
	uv run --project "$root/tools" --locked --quiet python -c \
		'import json, sys; v = json.load(open(sys.argv[1]))[sys.argv[2]]; print("\n".join(v) if isinstance(v, list) else v)' "$1" "$2"
}

# ---- the run ---------------------------------------------------------------------------------
announce "demo CA"
demo_ca

announce "requirements of the demo agent"
uv export --project "$root/sdk/python" --frozen --no-dev --no-emit-project --no-header \
	--format requirements.txt >"$tmp/agent/requirements.txt"

announce validate
compose config --quiet

announce "build and pull (demo agent, host image, bothy-host)"
compose build --quiet demo-agent
docker buildx build --quiet --load --tag "bothy-demo-host:$run_id" --file "$here/test/host/Dockerfile" "$here/test/host" >/dev/null
docker buildx build --quiet --file "$root/tools/console-e2e/host-bin.Dockerfile" \
	--output "type=local,dest=$tmp/host-bin" "$root/services/host" >/dev/null
compose pull --quiet postgres otel-collector otel-out-init demo-exchange-init demo-tls

announce "start (secrets init, migrate up --bootstrap, TLS front)"
compose up --detach --wait --wait-timeout "$wait_timeout" api worker otel-collector demo-tls
for s in secrets-init migrate demo-exchange-init; do
	code="$(exit_code "$s")"
	[ "$code" = 0 ] || { say "$s exited with $code"; exit 1; }
done
wait_for "$public_url" curl --silent --fail --cacert "$tmp/ca/ca.pem" "$public_url/api/v1/instance/discovery"

announce "demo agent and host machine"
compose up --detach --no-deps demo-agent demo-host
wait_for "systemd of the demo host" host_booted
host_exec install -d -o root -g root -m 0755 "$host_bin_dir"
compose cp "$tmp/host-bin/bothy-host" "demo-host:$host_bin_dir/bothy-host"
compose cp "$tmp/ca/ca.pem" "demo-host:/usr/local/share/ca-certificates/bothy-demo-ca.crt"
host_exec bash -c "chown root:root $host_bin_dir/bothy-host && chmod 0755 $host_bin_dir/bothy-host && update-ca-certificates >/dev/null"

announce "bothy setup demo (seed $seed)"
: >"$tmp/out/code"
args=(setup demo --exchange "$api_exchange" --seed "$seed")
[ -z "$pace" ] || args+=(--pace "$pace")
compose exec -T api bothy "${args[@]}" >"$tmp/out/code" 2>"$tmp/out/setup.err" &
setup_pid=$!

announce "pair the demo host when the command asks for it"
wait_for "the pairing request" exchange "test -f /exchange/host-pairing.json"
exchange "cat /exchange/host-pairing.json" >"$tmp/out/pairing.json"
users="$(json_field "$tmp/out/pairing.json" session_users)"
first="$(printf '%s\n' "$users" | head -n 1)"
host_exec "$host_bin_dir/bothy-host" install --session-user "$first" >/dev/null
for u in $(printf '%s\n' "$users" | tail -n +2); do
	host_exec "$host_service_bin" session-user add "$u" >/dev/null
done
{
	printf 'host_service:\n  session_users:\n'
	for u in $users; do printf '    - %s\n' "$u"; done
} | host_exec bash -c "cat > $host_config.new && chmod 0644 $host_config.new && mv $host_config.new $host_config"
host_exec "$host_service_bin" restart >/dev/null
# The code goes to bothy-host only as an argument of docker exec and is never printed.
pair_out="$(host_exec "$host_service_bin" pair --server "$public_url" --code "$(json_field "$tmp/out/pairing.json" pairing_code)")"
rm -f "$tmp/out/pairing.json"
fingerprint="$(printf '%s\n' "$pair_out" | sed -n 's/^fingerprint: \([^ ]*\)$/\1/p')"
[ -n "$fingerprint" ] || { say "bothy-host pair printed no fingerprint"; exit 1; }
printf '{"fingerprint":"%s"}' "$fingerprint" |
	exchange "cat > /exchange/.host-paired.json && mv /exchange/.host-paired.json /exchange/host-paired.json"

announce "warm-up and live runs (this takes a few minutes)"
rc=0
wait "$setup_pid" || rc=$?
setup_pid=""
[ "$rc" -eq 0 ] || { say "bothy setup demo exited with $rc"; exit 1; }
code="$(tr -d '[:space:]' <"$tmp/out/code")"
[ -n "$code" ] || { say "bothy setup demo printed no code"; exit 1; }

say "the demo is ready:"
say "  address        $public_url"
say "  CA certificate $tmp/ca/ca.pem (trust it in the app or the browser for this run)"
say "  sign in as Alex with the enrollment code: $code"
phase=ready
[ "$keep" != 1 ] || exit 0
say "press Ctrl-C to stop and remove the demo"
while :; do sleep 5; done
