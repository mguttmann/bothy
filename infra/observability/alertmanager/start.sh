#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Start guard of Alertmanager (brief M7-20 requirement 16, docs/observability/alerting.md
# section 2). Alertmanager v0.34.1 starts with a missing url_file and only fails when it sends,
# so a missing webhook URL would drop every alert in silence. This guard reads every url_file
# of the configuration and refuses to start while one of them is missing, unreadable or holds
# no value; the message names the files, never a value.
#
# Usage (compose.yaml): sh /etc/alertmanager/start.sh /bin/alertmanager [alertmanager flags]
# The configuration is the value of --config.file (default /etc/alertmanager/alertmanager.yaml).
# Exit codes: 1 a secret is missing or the configuration is unreadable, 2 usage; otherwise the
# guard execs Alertmanager.
set -eu

if [ "$#" -lt 1 ]; then
	echo "alertmanager start: usage: start.sh <alertmanager binary> [flags]" >&2
	exit 2
fi
bin=$1
shift
config=/etc/alertmanager/alertmanager.yaml
for arg do
	case $arg in
	--config.file=*) config=${arg#--config.file=} ;;
	esac
done
if [ ! -r "$config" ]; then
	echo "alertmanager start: cannot read the configuration $config" >&2
	exit 1
fi

# The value of every key url_file, without quotes and a trailing comment.
files=$(sed -n 's/^[[:space:]]*-\{0,1\}[[:space:]]*url_file:[[:space:]]*//p' "$config" |
	sed 's/[[:space:]]#.*$//; s/[[:space:]]*$//; s/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'\$/\1/")
missing=
for f in $files; do
	if [ ! -r "$f" ] || ! grep -q '[^[:space:]]' "$f"; then
		missing="$missing ${f##*/}"
	fi
done
if [ -n "$missing" ]; then
	echo "alertmanager start: refusing to start, these secret files are missing, unreadable or empty:$missing." >&2
	echo "alertmanager start: write them with infra/compose/alert-setup.sh, then run 'docker compose run --rm secrets-init' and 'docker compose up -d --force-recreate --no-deps alertmanager' (docs/operations/alerting-setup.md)." >&2
	exit 1
fi
exec "$bin" "$@"
