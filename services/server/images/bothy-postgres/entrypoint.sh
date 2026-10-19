#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Entry point of bothy-postgres (docs/spec/16-betrieb.md 2 and 8.2, brief M7-01). One image
# serves two Compose services; the command decides the role:
#
#   bothy-backupd ...   repository host `backup`: pgBackRest repository with encryption, TLS
#                       server for archive-push, client of the pgBackRest server in postgres
#   anything else       database host `postgres`: PostgreSQL with archive_command = pgBackRest
#                       archive-push over TLS to the repository host, plus a pgBackRest TLS
#                       server through which the repository host runs its backups
#
# Both roles write /etc/pgbackrest/pgbackrest.conf (mode 0600) before the command starts. TLS
# keys and the cipher pass are read from files under /run/bothy/secrets (R-OP-05); the cipher
# pass goes into the configuration file of backup only, because pgBackRest has no option that
# reads it from a file. The database role holds no cipher pass and no off-site credential
# (R-OP-08).
#
# Environment (docs/operations/configuration.md):
#   BOTHY_PGBACKREST_REPO_HOST  database role: host name of the repository host (required)
#   BOTHY_PG_ARCHIVE_TIMEOUT    database role, command postgres: archive_timeout (required)
#   BOTHY_PGBACKREST_PG_HOST    repository role: host name of the database host (required)
#   BOTHY_PGBACKREST_REPO2      database role: "on" when an off-site target is set; archive-push
#                               then also reaches repo2 through the repository host (brief M7-02).
#                               The repository host gets repo2 from bothy-backupd, never from here.
set -euo pipefail

readonly secrets=/run/bothy/secrets
readonly tls_dir=$secrets/pgbackrest_tls
readonly conf_dir=/etc/pgbackrest
readonly conf=$conf_dir/pgbackrest.conf
# Stanza and TLS port are fixed for an instance; bothy-backupd uses the same stanza.
readonly stanza=bothy
readonly tls_port=8432
readonly pgdata=${PGDATA:?PGDATA is set by the postgres base image}

die() {
	echo "bothy-postgres: $*" >&2
	exit 1
}

# require NAME: the variable must be set and not empty.
require() {
	[ -n "${!1:-}" ] || die "$1 is not set (docs/operations/configuration.md)"
}

# need_file PATH: a secret file must exist and be readable by this user.
need_file() {
	[ -r "$1" ] || die "$1 is missing or not readable; run secrets-init (bothy secrets init)"
}

# host_name VALUE NAME: a plain host name, so it cannot break the configuration file.
host_name() {
	[[ $1 =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]] || die "$2 is not a host name"
}

# write_conf: stdin becomes the configuration file, replaced in one step and readable only by
# the running user.
write_conf() {
	local tmp
	tmp=$(mktemp "$conf_dir/.pgbackrest.conf.XXXXXX")
	chmod 0600 "$tmp"
	cat >"$tmp"
	mv -f "$tmp" "$conf"
}

repository_role() {
	require BOTHY_PGBACKREST_PG_HOST
	host_name "$BOTHY_PGBACKREST_PG_HOST" BOTHY_PGBACKREST_PG_HOST
	local f
	for f in "$tls_dir/ca.crt" "$tls_dir/backup.crt" "$tls_dir/backup.key" "$secrets/pgbackrest_cipher_pass"; do
		need_file "$f"
	done
	local cipher
	cipher=$(<"$secrets/pgbackrest_cipher_pass")
	[[ -n $cipher && $cipher != *[[:space:]]* ]] || die "$secrets/pgbackrest_cipher_pass is empty or holds white space"
	write_conf <<EOF
# Written by the entry point of bothy-postgres on every start; changes are lost.
[global]
repo1-path=/var/lib/pgbackrest
repo1-cipher-type=aes-256-cbc
repo1-cipher-pass=$cipher
log-level-console=info
log-level-file=off
tls-server-address=*
tls-server-port=$tls_port
tls-server-ca-file=$tls_dir/ca.crt
tls-server-cert-file=$tls_dir/backup.crt
tls-server-key-file=$tls_dir/backup.key
tls-server-auth=postgres=$stanza

[$stanza]
pg1-host=$BOTHY_PGBACKREST_PG_HOST
pg1-host-type=tls
pg1-host-port=$tls_port
pg1-host-ca-file=$tls_dir/ca.crt
pg1-host-cert-file=$tls_dir/backup.crt
pg1-host-key-file=$tls_dir/backup.key
pg1-path=$pgdata
EOF
}

database_role() {
	require BOTHY_PGBACKREST_REPO_HOST
	host_name "$BOTHY_PGBACKREST_REPO_HOST" BOTHY_PGBACKREST_REPO_HOST
	case "${BOTHY_PGBACKREST_REPO2:-}" in
	"" | on) ;;
	*) die "BOTHY_PGBACKREST_REPO2 must be empty or on" ;;
	esac
	local f
	for f in "$tls_dir/ca.crt" "$tls_dir/postgres.crt" "$tls_dir/postgres.key"; do
		need_file "$f"
	done
	write_conf <<EOF
# Written by the entry point of bothy-postgres on every start; changes are lost.
[global]
repo1-host=$BOTHY_PGBACKREST_REPO_HOST
repo1-host-type=tls
repo1-host-port=$tls_port
repo1-host-ca-file=$tls_dir/ca.crt
repo1-host-cert-file=$tls_dir/postgres.crt
repo1-host-key-file=$tls_dir/postgres.key
log-level-console=warn
log-level-stderr=warn
log-level-file=off
tls-server-address=*
tls-server-port=$tls_port
tls-server-ca-file=$tls_dir/ca.crt
tls-server-cert-file=$tls_dir/postgres.crt
tls-server-key-file=$tls_dir/postgres.key
tls-server-auth=backup=$stanza
$(repo2_host_lines)

[$stanza]
pg1-path=$pgdata
pg1-socket-path=/var/run/postgresql
EOF
}

# repo2_host_lines prints the repo2 options of the database host: the same repository host and
# certificates as repo1. The repository host knows where repo2 is; the database host only needs
# to know that it exists, so that archive-push writes every segment to both repositories.
repo2_host_lines() {
	case "${BOTHY_PGBACKREST_REPO2:-}" in
	on)
		cat <<EOF
repo2-host=$BOTHY_PGBACKREST_REPO_HOST
repo2-host-type=tls
repo2-host-port=$tls_port
repo2-host-ca-file=$tls_dir/ca.crt
repo2-host-cert-file=$tls_dir/postgres.crt
repo2-host-key-file=$tls_dir/postgres.key
EOF
		;;
	esac
}

[ "$(id -u)" != 0 ] || die "runs as the user postgres of the image, not as root"
check-pgbackrest-version.sh >/dev/null

if [ "${1:-}" = bothy-backupd ]; then
	repository_role
	exec "$@"
fi

database_role
if [ "${1:-}" = postgres ]; then
	require BOTHY_PG_ARCHIVE_TIMEOUT
	[[ $BOTHY_PG_ARCHIVE_TIMEOUT =~ ^[0-9]+(ms|s|min|h)?$ ]] || die "BOTHY_PG_ARCHIVE_TIMEOUT is not a PostgreSQL duration"
	# The pgBackRest server lets the repository host run backups against this database. It runs
	# next to PostgreSQL; when it stops, backups fail and their alerts fire, WAL archiving goes on.
	pgbackrest server &
	shift
	# Settings of 16 7.2 first, so that later -c arguments of the command line still win.
	set -- postgres \
		-c wal_level=replica \
		-c archive_mode=on \
		-c "archive_command=pgbackrest --stanza=$stanza archive-push %p" \
		-c "archive_timeout=$BOTHY_PG_ARCHIVE_TIMEOUT" \
		"$@"
fi
# A restore writes into PGDATA; on a fresh volume the directory does not exist yet.
if [ "${1:-}" = pgbackrest ]; then
	mkdir -p "$pgdata"
	chmod 0700 "$pgdata"
fi
exec docker-entrypoint.sh "$@"
