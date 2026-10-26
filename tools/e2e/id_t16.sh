#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# ID-T16 with the real Share Extension (brief M5-06 Tests; docs/spec/08-identity.md 15): starts
# the Compose smoke stack with the TLS front of the desktop E2E test (infra/compose/e2e.sh, M1-73
# and M4-34) and the overlay infra/compose/test/compose.id-t16.yaml (short access tokens), then
# runs apps/client/tool/id_t16_suite.sh, which trusts the run's test CA in the iOS simulator and
# runs apps/client/e2e_test/share/id_t16_test.dart there through patrol. e2e.sh stops and removes
# everything it started, also on a failure.
#
# Needs macOS with Xcode and an iOS simulator, Docker (the inputs of infra/compose/e2e.sh, the
# server image of `make image`) and patrol_cli (installed by the suite when missing). Exit codes
# are those of e2e.sh: 0 pass, 1 failure, 2 a missing precondition.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
overlay="$root/infra/compose/test/compose.id-t16.yaml"
ttl="$(sed -n 's/^ *BOTHY_AUTH__ACCESS_TOKEN_TTL: "\([0-9]*\)s"$/\1/p' "$overlay")"
leeway="$(sed -n 's/^ *BOTHY_AUTH__CLOCK_LEEWAY: "\([0-9]*\)s"$/\1/p' "$overlay")"
for value in "$ttl" "$leeway"; do
	case "$value" in
	'' | *[!0-9]*) printf 'id_t16: cannot read the token lifetime and the clock leeway from %s\n' "$overlay" >&2; exit 2 ;;
	esac
done
export ID_T16_ACCESS_TOKEN_TTL_S="$ttl" ID_T16_CLOCK_LEEWAY_S="$leeway"
E2E_OVERLAY="$overlay" exec "$root/infra/compose/e2e.sh" "$root/apps/client/tool/id_t16_suite.sh"
