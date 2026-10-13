// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// PathWebhookID is the path parameter that names the incoming webhook of rl.webhook_in
// (POST /api/v1/hooks/{webhook_id}/{token}).
const PathWebhookID = "webhook_id"

// ipv6Prefix is the IPv6 prefix length one client is counted by (05 "Rate-limit classes": IPv6
// clients usually get a whole /64, so counting addresses would let one client fill any number of
// buckets).
const ipv6Prefix = 64

// Key values. Each kind has its own prefix, so a principal's bucket and the client-IP bucket of an
// anonymous caller never coincide, even within one class.

// KeyForIP returns the key of a client IP: IPv4-mapped IPv6 addresses unmapped, zones dropped,
// IPv6 addresses by their /64 prefix, for example "ip:192.0.2.1" or "ip:2001:db8:1:2::/64". An
// invalid address (no client IP known) gives "ip:invalid", one shared bucket, so a request without
// an address is limited rather than unlimited.
func KeyForIP(ip netip.Addr) string {
	if !ip.IsValid() {
		return "ip:invalid"
	}
	ip = ip.Unmap().WithZone("")
	if ip.Is6() {
		return "ip:" + netip.PrefixFrom(ip, ipv6Prefix).Masked().String()
	}
	return "ip:" + ip.String()
}

// KeyForPrincipal returns the key of a principal, "p:<id>".
func KeyForPrincipal(id ids.ID) string { return "p:" + id.String() }

// KeyForAgent returns the key of an agent (rl.agent_runtime), "a:<id>".
func KeyForAgent(id ids.ID) string { return "a:" + id.String() }

// KeyForHost returns the key of a host after its host key was checked (rl.host), "h:<id>".
func KeyForHost(id ids.ID) string { return "h:" + id.String() }

// KeyForDevice returns the key of a device (rl.gateway_connect), "d:<id>".
func KeyForDevice(id ids.ID) string { return "d:" + id.String() }

// KeyForToken returns the key of an API token (rl.gateway_connect), "t:<id>".
func KeyForToken(id ids.ID) string { return "t:" + id.String() }

// KeyForConnection returns the key of a gateway connection (rl.typing), "c:<id>"; the gateway
// passes its own connection ID.
func KeyForConnection(id string) string { return "c:" + id }

// KeyForWebhook returns the key of an incoming webhook (rl.webhook_in), "w:<id>".
func KeyForWebhook(id string) string { return "w:" + id }

// DeviceOrTokenKey returns the key of rl.gateway_connect for a verified actor: its device when it
// has one (a person's session), else its API token (agents, apps, personal tokens). The gateway
// calls it at identify and resume once the credentials are verified, never at the upgrade (05),
// so nobody can drain the bucket of a foreign device. The error reports an actor with neither.
func DeviceOrTokenKey(a actor.Actor) (string, error) {
	switch {
	case !a.DeviceID.IsZero():
		return KeyForDevice(a.DeviceID), nil
	case !a.TokenID.IsZero():
		return KeyForToken(a.TokenID), nil
	}
	return "", errors.New("ratelimit: rl.gateway_connect needs an actor with a device or an API token")
}

// RequestKey returns the key of a REST request r for class (05 "Rate-limit classes"): the client IP
// of kernel/reqmeta for rl.public_auth; the actor ID for the principal, agent and host classes,
// with the client IP instead for the anonymous actor and for an actor with CredentialInactive
// (unauthenticated calls and dead tokens never share one bucket and never borrow a principal's);
// the path value webhook_id for rl.webhook_in. The error reports a class 05 does not list, a
// gateway class (rl.typing, rl.gateway_connect), which a REST request never carries, and a
// rl.webhook_in request without webhook_id; a webhook_id that is not an ID is errs.CodeNotFound.
func RequestKey(r *http.Request, class Class) (string, error) {
	kind, ok := KindOf(class)
	if !ok {
		return "", fmt.Errorf("ratelimit: unknown class %q", class)
	}
	ctx := r.Context()
	ip, _ := reqmeta.ClientIP(ctx)
	switch kind {
	case KeyClientIP:
		return KeyForIP(ip), nil
	case KeyPrincipal, KeyAgent, KeyHost:
		a, ok := actor.FromContext(ctx)
		// A revoked or expired API token (CredentialInactive, M1-44) counts like an anonymous
		// call: it must not spend the bucket of the principal it once belonged to.
		if !ok || a.IsAnonymous() || a.ID.IsZero() || a.CredentialInactive {
			return KeyForIP(ip), nil
		}
		if kind == KeyAgent {
			return KeyForAgent(a.ID), nil
		}
		if kind == KeyHost {
			return KeyForHost(a.ID), nil
		}
		return KeyForPrincipal(a.ID), nil
	case KeyIncomingWebhook:
		raw := r.PathValue(PathWebhookID)
		if raw == "" {
			return "", fmt.Errorf("ratelimit: class %s needs the path value %s", class, PathWebhookID)
		}
		// webhook_id is an ID (06), but this stage runs before the path is validated: any other
		// value names no webhook and gets 404 without a bucket, so the key length stays fixed
		// and a client cannot fill memory with long keys. The canonical form keeps two
		// spellings of one ID in one bucket.
		id, err := ids.Parse(raw)
		if err != nil {
			return "", errs.New(errs.CodeNotFound, "")
		}
		return KeyForWebhook(id.String()), nil
	case KeyConnection, KeyDeviceOrToken:
	}
	return "", fmt.Errorf("ratelimit: class %s is counted by the gateway, not by a REST operation", class)
}
