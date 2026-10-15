// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// Defaults of the signing key keys that other chapters use too (goconst); the chapter tables hold
// the same text.
const (
	defaultAccessTokenTTL         = "10m"
	defaultWebhookDeliveryTimeout = "15s"
)

// signingKeyConfigKeys returns the keys the server signing keys read (M1-32): the cache and the
// rotation of docs/spec/19-security.md section 16, the scheduled rotation of the invocation key
// and the JWKS cache of 09 section 4.5, and the validity of what each purpose signs, which sets the
// window a retiring key stays published (19 section 10): access tokens (08), invocation tokens
// (04) and webhook deliveries (11). auth.clock_leeway, the margin of every window, belongs to
// identityConfigKeys.
func signingKeyConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "signing_key.cache_ttl", Kind: config.KindDuration, Default: "30s", Min: "1s",
			Doc:     "Longest time a process still accepts a withdrawn signing key or does not know a new one.",
			Chapter: chapter19,
		},
		{
			Name: "signing_key.rotation_check_interval", Kind: config.KindDuration, Default: "1m", Min: "1s",
			Doc:     "Interval of the job instance.key_rotation.",
			Chapter: chapter19,
		},
		{
			Name: "signing_key.access_token_rotation_interval", Kind: config.KindDuration, Default: defaultEventRetention, Min: "1s",
			Doc:     "Scheduled rotation of the signing key of access tokens.",
			Chapter: chapter19,
		},
		{
			Name: "signing_key.invocation_rotation_interval", Kind: config.KindDuration, Default: defaultEventRetention, Min: "1s",
			Doc:     "Scheduled rotation of the signing key of invocation tokens.",
			Chapter: chapter09,
		},
		{
			Name: "agent.jwks_max_age", Kind: config.KindDuration, Default: "1h", Min: "1s",
			Doc:     "Cache lifetime of the JWKS and how long a next key is published before it signs.",
			Chapter: chapter09,
		},
		{
			Name: "agent.invocation_token_ttl", Kind: config.KindDuration, Default: denyAggregateWindowDefault, Min: "1s",
			Doc:     "Validity of an invocation token.",
			Chapter: chapter04,
		},
		{
			Name: "auth.access_token_ttl", Kind: config.KindDuration, Default: defaultAccessTokenTTL, Min: "1s",
			Doc:     "Lifetime of an access token.",
			Chapter: chapter08,
		},
		{
			Name: "webhook.delivery_timeout", Kind: config.KindDuration, Default: defaultWebhookDeliveryTimeout, Min: minDeadline,
			Doc:     "Time limit of one webhook delivery.",
			Chapter: chapter11,
		},
	}
}
