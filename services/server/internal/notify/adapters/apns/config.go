// SPDX-License-Identifier: Apache-2.0

package apns

import (
	"errors"
	"time"
)

// The configuration keys of the adapter (13 17); error texts name them, never a value.
const (
	keyTeamID        = "push.apns.team_id"
	keyKeyID         = "push.apns.key_id"
	keyPrivateKeyRef = "push.apns.private_key_ref"
	keyTopics        = "push.apns.topics"
)

// Config holds the keys push.ttl and push.apns.* (13 17); internal/wire binds it from the
// loaded configuration.
type Config struct {
	// TeamID is push.apns.team_id, the claim iss of the JWT; KeyID is push.apns.key_id, the
	// header kid; PrivateKeyRef is push.apns.private_key_ref, the reference of the .p8 key in
	// the secret provider.
	TeamID, KeyID, PrivateKeyRef string
	// Topics is push.apns.topics, the bundle IDs an apns-topic may name.
	Topics []string
	// JWTRefreshInterval is push.apns.jwt_refresh_interval (the loader keeps it within 20 to 60
	// minutes); Retry5xxDelay is push.apns.retry_5xx_delay; TTL is push.ttl.
	JWTRefreshInterval, Retry5xxDelay, TTL time.Duration
	// TimeSensitiveEnabled is push.apns.time_sensitive_enabled.
	TimeSensitiveEnabled bool
}

// Enabled reports whether APNs is configured: team ID, key ID and topics are all set. Without
// them internal/wire builds no adapter and reads no secret.
func (c Config) Enabled() bool {
	return c.TeamID != "" && c.KeyID != "" && len(c.Topics) > 0
}

// errIncomplete is the error of a partial APNs configuration; it names the keys only.
var errIncomplete = errors.New("push.apns.team_id, push.apns.key_id and push.apns.topics must be set together (APNs is enabled with all three)")

// Validate is the cross-check push_apns_complete (02 13 item 4): team ID, key ID and topics are
// all set or all empty; when they are set, push.apns.private_key_ref is set too; no topic is
// empty or repeated. The errors name keys, never values.
func (c Config) Validate() error {
	set := 0
	for _, ok := range []bool{c.TeamID != "", c.KeyID != "", len(c.Topics) > 0} {
		if ok {
			set++
		}
	}
	if set != 0 && set != 3 {
		return errIncomplete
	}
	seen := map[string]bool{}
	for _, t := range c.Topics {
		if t == "" {
			return errors.New("push.apns.topics must not contain an empty entry")
		}
		if seen[t] {
			return errors.New("push.apns.topics must not repeat an entry")
		}
		seen[t] = true
	}
	if set == 3 && c.PrivateKeyRef == "" {
		return errors.New("push.apns.private_key_ref must be set when APNs is enabled (push.apns.team_id, push.apns.key_id and push.apns.topics)")
	}
	return nil
}
