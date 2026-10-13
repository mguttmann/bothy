// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"maps"
	"slices"
)

// Class is a rate-limit class of 05 "Rate-limit classes", for example "rl.read".
type Class string

// KeyKind says what a class counts by (the column "Key" of 05 "Rate-limit classes").
type KeyKind int

// The key kinds of 05. The zero value is KeyClientIP.
const (
	KeyClientIP        KeyKind = iota // client IP, IPv6 by its /64 prefix
	KeyPrincipal                      // the principal (actor ID); rl.terminal counts the human
	KeyAgent                          // the agent (actor ID)
	KeyDeviceOrToken                  // device of the session, else the API token (gateway)
	KeyConnection                     // the gateway connection
	KeyIncomingWebhook                // the incoming webhook (path value webhook_id)
	KeyHost                           // the host (actor ID after the host key check)
)

// String returns the kind as a short name, for example "client_ip".
func (k KeyKind) String() string {
	switch k {
	case KeyClientIP:
		return "client_ip"
	case KeyPrincipal:
		return "principal"
	case KeyAgent:
		return "agent"
	case KeyDeviceOrToken:
		return "device_or_token"
	case KeyConnection:
		return "connection"
	case KeyIncomingWebhook:
		return "incoming_webhook"
	case KeyHost:
		return "host"
	}
	return "KeyKind(?)"
}

// kinds is the hand-written copy of the columns "Class" and "Key" of 05: actions.yaml
// carries the key only as prose, so no generator can derive it. TestKeyKinds_MatchChapter05
// parses the chapter table and holds this map equal to it.
var kinds = map[Class]KeyKind{
	"rl.public_auth":     KeyClientIP,
	"rl.gateway_connect": KeyDeviceOrToken,
	"rl.read":            KeyPrincipal,
	"rl.write":           KeyPrincipal,
	"rl.message":         KeyPrincipal,
	"rl.trigger":         KeyPrincipal,
	"rl.interaction":     KeyPrincipal,
	"rl.upload":          KeyPrincipal,
	"rl.search":          KeyPrincipal,
	"rl.typing":          KeyConnection,
	"rl.admin":           KeyPrincipal,
	"rl.export":          KeyPrincipal,
	"rl.agent_runtime":   KeyAgent,
	"rl.webhook_in":      KeyIncomingWebhook,
	"rl.import":          KeyPrincipal,
	"rl.terminal":        KeyPrincipal,
	"rl.host":            KeyHost,
}

// KindOf returns the key kind of class and true, or false for a class that 05 does not list.
func KindOf(class Class) (KeyKind, bool) {
	k, ok := kinds[class]
	return k, ok
}

// Classes returns every class of 05 "Rate-limit classes", sorted. Each needs the configuration
// keys ratelimit.<class>.per_minute and ratelimit.<class>.burst.
func Classes() []Class {
	return slices.Sorted(maps.Keys(kinds))
}

// GatewayClass reports whether class is counted by the gateway only (rl.typing per connection,
// rl.gateway_connect at identify and resume); a REST operation must not carry such a class.
func GatewayClass(class Class) bool {
	k, ok := kinds[class]
	return ok && (k == KeyConnection || k == KeyDeviceOrToken)
}
