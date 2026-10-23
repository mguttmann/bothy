// SPDX-License-Identifier: Apache-2.0

package apns

import "context"

// AppleHosts is the host policy of the adapter, for TestEgressOnly.
func AppleHosts(ctx context.Context, scheme, host string, port uint16) bool {
	return appleHosts(ctx, scheme, host, port)
}

// BaseURLs returns the base URL per environment.
func BaseURLs(a *Adapter) map[string]string {
	out := map[string]string{}
	for k, v := range a.base {
		out[k] = v
	}
	return out
}

// JWTCreations returns the JWT signatures of env so far.
func JWTCreations(a *Adapter, env string) int64 { return a.signs[env].count() }

// SetInflate installs the hook that changes the serialized payload before the size check.
func SetInflate(a *Adapter, f func([]byte) []byte) { a.inflate = f }

// AppleMaxPayloadBytes is the size limit of 13 6.1.
const AppleMaxPayloadBytes = appleMaxPayloadBytes

// PayloadTypes returns a zero payload, for the reflect walk of TestPayloadAllowList.
func PayloadTypes() any { return payload{} }
