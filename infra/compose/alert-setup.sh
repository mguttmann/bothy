#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
#
# Guided setup of the alert routes (brief M7-20 Design note 1, docs/observability/alerting.md
# section 2, guide docs/operations/alerting-setup.md). Reads one webhook URL from standard
# input and writes it into the owner's secret source infra/compose/secrets/ (16 5.3), where
# secrets-init picks it up for Alertmanager. It never prints the URL: the output is one line
# "<status> <name> sha256:<12 hex>" with the status created, updated or kept and the
# fingerprint of secrets-init (16 5.3 item 5), so the two outputs can be compared.
#
# Usage:
#   sh alert-setup.sh [--secrets-dir DIR] [--internal-base URL] channel|external|heartbeat < url
#
#   channel    the URL createIncomingWebhook returned for the channel "Operations"; the path
#              /api/v1/hooks/<id>/<token> is kept and the origin replaced by --internal-base
#              (default http://api:8081, server.internal_listen_addr in the Compose network),
#              because Alertmanager reaches the server only through the internal listener
#              -> alertmanager_bothy_webhook_url
#   external   the URL of the external on-call channel, unchanged
#              -> alertmanager_external_webhook_url
#   heartbeat  the URL of the external heartbeat service, unchanged
#              -> alertmanager_heartbeat_url
#
# The file gets mode 0600 in a directory of mode 0700. A second run with the same URL changes
# nothing. On a terminal the input is not echoed.
# Exit codes: 0 success, 1 runtime error (cannot write), 2 usage or an input that is no URL of
# the expected form.
set -eu

die() {
	echo "alert-setup: $2" >&2
	exit "$1"
}

secrets_dir=$(cd "$(dirname "$0")" && pwd)/secrets
internal_base=http://api:8081
kind=
while [ "$#" -gt 0 ]; do
	case $1 in
	--secrets-dir)
		[ "$#" -ge 2 ] || die 2 "--secrets-dir needs a directory"
		secrets_dir=$2
		shift 2
		;;
	--internal-base)
		[ "$#" -ge 2 ] || die 2 "--internal-base needs a URL"
		internal_base=$2
		shift 2
		;;
	-h | --help)
		sed -n '/^# Usage:/,/^# Exit codes/p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	-*) die 2 "unknown option $1" ;;
	*)
		[ -z "$kind" ] || die 2 "only one of channel, external or heartbeat"
		kind=$1
		shift
		;;
	esac
done
case $kind in
channel) name=alertmanager_bothy_webhook_url ;;
external) name=alertmanager_external_webhook_url ;;
heartbeat) name=alertmanager_heartbeat_url ;;
'') die 2 "name the route: channel, external or heartbeat (see --help)" ;;
*) die 2 "unknown route $kind; use channel, external or heartbeat" ;;
esac
printf '%s\n' "$internal_base" | grep -Eq '^http://[A-Za-z0-9.-]+(:[0-9]+)?$' ||
	die 2 "--internal-base must be http://<host>[:<port>] without a path"

# Read the URL without echoing it on a terminal.
url=
if [ -t 0 ]; then
	# Restore the terminal on every way out, also on Ctrl-C at the prompt.
	old_tty=$(stty -g)
	trap 'stty "$old_tty"' EXIT
	trap 'exit 130' INT TERM
	stty -echo
	printf 'Paste the URL for %s (input hidden): ' "$kind" >&2
	IFS= read -r url || true
	stty "$old_tty"
	trap - EXIT INT TERM
	echo >&2
else
	IFS= read -r url || true
fi
url=$(printf '%s' "$url" | tr -d '\r')

case $kind in
channel)
	# https://<host>/api/v1/hooks/<webhook id>/<token>, nothing after the token.
	path=$(printf '%s\n' "$url" | sed -n 's|^https\{0,1\}://[^/?#[:space:]]\{1,\}\(/api/v1/hooks/[A-Za-z0-9_-]\{1,\}/[A-Za-z0-9_-]\{1,\}\)$|\1|p')
	[ -n "$path" ] || die 2 "the input is not the URL of an incoming webhook (https://<host>/api/v1/hooks/<id>/<token>); the value is not shown"
	value=$internal_base$path
	;;
*)
	printf '%s\n' "$url" | grep -Eq '^https?://[^[:space:]/?#]+([/?#][^[:space:]]*)?$' ||
		die 2 "the input is not an http or https URL; the value is not shown"
	value=$url
	;;
esac

sha() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{ print substr($1, 1, 12) }'
	else
		shasum -a 256 "$1" | awk '{ print substr($1, 1, 12) }'
	fi
}

umask 077
if [ ! -d "$secrets_dir" ]; then
	mkdir -p "$secrets_dir" || die 1 "cannot create $secrets_dir"
	chmod 700 "$secrets_dir" || die 1 "cannot set the mode of $secrets_dir"
fi
target=$secrets_dir/$name
tmp=$(mktemp "$secrets_dir/.alert-setup.XXXXXX" 2>/dev/null) || die 1 "cannot write into $secrets_dir"
trap 'rm -f "$tmp"' EXIT
printf '%s' "$value" >"$tmp" || die 1 "cannot write into $secrets_dir"
if [ -f "$target" ] && cmp -s "$tmp" "$target"; then
	status=kept
	chmod 600 "$target" || die 1 "cannot set the mode of $target"
else
	status=created
	[ ! -e "$target" ] || status=updated
	chmod 600 "$tmp" || die 1 "cannot set the mode of the new file"
	mv -f "$tmp" "$target" || die 1 "cannot replace $target"
fi
echo "$status $name sha256:$(sha "$target")"
