#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Compose smoke test (brief M1-73, plan P-06, exit criterion 8 of M1): starts the server core from
# the real bothy-server image with the unchanged installation file compose.yaml plus the test
# overlay test/compose.smoke.yaml, and proves the path from first setup to live delivery over
# WebSocket, then scans the collector export and the container logs for the run's canaries.
# Documentation: infra/compose/test/README.md. `make portability` runs it.
#
# Inputs (defaults only here):
#   SMOKE_SERVER_IMAGE        server image (default: the local tag of `make image`)
#   SMOKE_KEEP=1              keep the stack and the temp dir, print how to remove them
#   SMOKE_WAIT_TIMEOUT_S      bound of every wait of the stack (default 240)
#   SMOKE_TELEMETRY_SETTLE_S  wait between the stop of api and worker and the collector stop
#   SMOKE_LOG_TAIL            log lines per service printed on failure (default 200)
#
# Exit codes: 0 pass, 1 smoke failure, 2 precondition missing (P-09: nothing is skipped).
# Compatible with bash 3.2 (the default shell of macOS).

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"

# Minimum versions: Engine 26.0.0 for volume subpath, Compose 2.26.0 for !reset, !override and
# configs (infra/compose/README.md).
readonly min_engine=26.0.0
readonly min_compose=2.26.0

phase=preflight
step=inputs
tmp=""
tmp_made=""
project=""

say() { printf 'smoke: %s\n' "$*" >&2; }
announce() { step="$1"; say "$1"; }
precondition() { say "$*"; exit 2; }

# ---- inputs ----------------------------------------------------------------------------------

# default_image returns the tag `make image` writes: IMAGE_NAME:IMAGE_TAG of the Makefile with
# SERVER_VERSION from services/server/VERSION.
default_image() {
	local name tag version
	name="$(sed -n 's/^IMAGE_NAME ?= //p' "$root/Makefile")"
	tag="$(sed -n 's/^IMAGE_TAG ?= //p' "$root/Makefile")"
	version="$(tr -d '[:space:]' <"$root/services/server/VERSION")"
	tag="${tag//\$(SERVER_VERSION)/$version}"
	printf '%s:%s' "$name" "$tag"
}

image="${SMOKE_SERVER_IMAGE:-$(default_image)}"
keep="${SMOKE_KEEP:-0}"
wait_timeout="${SMOKE_WAIT_TIMEOUT_S:-240}"
# At least decision_wait + batch.timeout + flush_interval of test/otel/file.yaml (3 s), with room
# for the last metric export interval (smoke.env: 1000 ms).
settle="${SMOKE_TELEMETRY_SETTLE_S:-5}"
log_tail="${SMOKE_LOG_TAIL:-200}"
for pair in "SMOKE_WAIT_TIMEOUT_S=$wait_timeout" "SMOKE_TELEMETRY_SETTLE_S=$settle" "SMOKE_LOG_TAIL=$log_tail"; do
	case "${pair#*=}" in
	'' | *[!0-9]*) precondition "${pair%%=*} must be a whole number of seconds or lines, got '${pair#*=}'" ;;
	esac
done
[ "$wait_timeout" -gt 0 ] || precondition "SMOKE_WAIT_TIMEOUT_S must be greater than 0"

# ---- sanitize --------------------------------------------------------------------------------
# A developer's shell must not change the stack: every exported BOTHY_*, COMPOSE_* and OTEL_*
# variable goes; Compose reads its variables from test/smoke.env only.
for name in $(compgen -e); do
	case "$name" in
	BOTHY_* | COMPOSE_* | OTEL_*) unset "$name" ;;
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
	precondition "server image '$image' not found locally; build it with 'make image' or pull it, then set SMOKE_SERVER_IMAGE"
say "Docker Engine $engine, Compose $compose_version, image $image"

# ---- workspace -------------------------------------------------------------------------------
announce workspace
phase=run
hex() { od -An -N"$1" -tx1 /dev/urandom | tr -d ' \n'; }
umask 077
tmp_made="$(mktemp -d "${TMPDIR:-/tmp}/bothy-smoke.XXXXXXXX")"
tmp="$tmp_made"
mkdir -m 0700 "$tmp/secrets-input" "$tmp/out" "$tmp/otel" "$tmp/logs"
run_id="$(hex 4)"
project="bothy-smoke-$run_id"
canary="smk$(hex 12)"
export SMOKE_RUN_ID="$run_id" SMOKE_SECRETS_INPUT="$tmp/secrets-input" SMOKE_OUT="$tmp/out" SMOKE_TMP="$tmp" SMOKE_CANARY="$canary"
uid_gid="$(id -u):$(id -g)"

compose() {
	docker compose --project-name "$project" --project-directory "$here" \
		--file "$here/compose.yaml" --file "$here/test/compose.smoke.yaml" \
		--env-file "$here/test/smoke.env" "$@"
}

# diagnose prints the failed step, the containers and the last log lines of the server services.
diagnose() {
	say "FAILED at step: $step"
	compose ps --all >&2 || true
	local s
	for s in api worker migrate otel-collector; do
		say "last $log_tail log lines of $s:"
		compose logs --no-color --tail "$log_tail" "$s" >&2 || true
	done
}

# remove_tmp removes the temp dir, but only the directory mktemp -d created for this run.
remove_tmp() {
	if [ -n "$tmp" ] && [ "$tmp" = "$tmp_made" ] && [ -d "$tmp" ]; then
		case "$tmp" in
		*/bothy-smoke.*) rm -rf "$tmp" ;;
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
	set +e
	if [ "$phase" = preflight ]; then
		exit "$rc"
	fi
	if [ "$rc" -ne 0 ]; then
		diagnose
		rc=1
	fi
	if [ "$keep" = 1 ]; then
		say "SMOKE_KEEP=1: the stack $project and $tmp are kept; remove them with:"
		say "  docker compose --project-name $project --project-directory $here -f $here/compose.yaml -f $here/test/compose.smoke.yaml --env-file $here/test/smoke.env down --volumes --remove-orphans --rmi local"
		say "  docker image rm bothy-smoke:$run_id; rm -rf $tmp"
		exit "$rc"
	fi
	step=cleanup
	say cleanup
	compose down --volumes --remove-orphans --rmi local --timeout "$wait_timeout" >/dev/null 2>&1
	docker image rm "bothy-smoke:$run_id" >/dev/null 2>&1
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

# exit_code returns the exit code of the one-shot container of service $1.
exit_code() {
	local id
	id="$(compose ps --all --quiet "$1")"
	[ -n "$id" ] || { say "no container of $1"; return 1; }
	docker inspect --format '{{.State.ExitCode}}' "$id"
}

# ---- the run ---------------------------------------------------------------------------------
announce validate
compose config --quiet

announce "build and pull"
compose build --quiet --build-arg VERSION="$(tr -d '[:space:]' <"$root/services/server/VERSION")" smoke-probe
compose pull --quiet postgres otel-collector otel-out-init

announce "start (secrets init, migrate up --bootstrap)"
compose up --detach --wait --wait-timeout "$wait_timeout" api worker otel-collector
for s in secrets-init migrate; do
	code="$(exit_code "$s")"
	[ "$code" = 0 ] || { say "$s exited with $code"; exit 1; }
done

announce "idempotence (secrets init and migrate a second time)"
compose run --rm --no-deps secrets-init >>"$tmp/logs/rerun.log" 2>&1
compose run --rm --no-deps migrate >>"$tmp/logs/rerun.log" 2>&1

announce "precheck (503 setup_required)"
compose run --rm --no-deps --user "$uid_gid" smoke-probe precheck

announce "bootstrap owner"
: >"$tmp/out/setup.out"
chmod 0600 "$tmp/out/setup.out"
compose exec -T api bothy setup bootstrap-owner --handle alex --display-name "Alex $canary" \
	>"$tmp/out/setup.out" 2>"$tmp/logs/setup.err"
if compose exec -T api bothy setup bootstrap-owner --handle alex --display-name "Alex $canary" \
	>"$tmp/out/setup-second.out" 2>>"$tmp/logs/setup.err"; then
	say "a second bootstrap-owner succeeded; it must abort without change (ID-T07)"
	exit 1
fi

announce "probe (enrollment, live delivery, catch-up, step-up, revocation)"
compose run --rm --no-deps --user "$uid_gid" smoke-probe run

announce "flush telemetry"
compose stop --timeout "$wait_timeout" api worker
sleep "$settle"
compose stop --timeout "$wait_timeout" otel-collector

announce collect
compose logs --no-color --timestamps secrets-init postgres migrate otel-out-init api worker otel-collector >"$tmp/logs/compose.log"
compose cp otel-collector:/var/lib/otelcol-smoke/. "$tmp/otel/"

announce "scan (canaries, positive control, telemetry presence)"
compose run --rm --no-deps --user "$uid_gid" smoke-scan scan
