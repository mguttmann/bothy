// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// pushConfigKeys returns the keys of the job notify.push_send (13 "Configuration values of this
// chapter", 03 section 9 for push.max_consecutive_failures; M3-19) and of the APNs adapter
// (push.ttl and push.apns.*, M3-20). The keys push.fcm.* come with M3-21.
func pushConfigKeys() []config.Key {
	return append([]config.Key{
		{Name: "push.max_age", Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: chapter13,
			Doc: "A hint not yet accepted by the provider after this age is dropped."},
		{Name: "push.retry_base", Kind: config.KindDuration, Default: "2s", Min: minDeadline, Chapter: chapter13,
			Doc: "Base of the retry backoff of a push."},
		{Name: "push.retry_cap", Kind: config.KindDuration, Default: "5m", Min: minDeadline, Chapter: chapter13,
			Doc: "Largest retry backoff of a push."},
		{Name: "push.max_concurrent_requests", Kind: config.KindInt, Default: "32", Min: "1", Max: "100000", Chapter: chapter13,
			Doc: "Concurrent provider requests per process."},
		{Name: "push.max_consecutive_failures", Kind: config.KindInt, Default: "5", Min: "1", Max: "100000", Chapter: chapter03,
			Doc: "Consecutive transient failures after which a push registration is deactivated."},
	}, apnsConfigKeys()...)
}

// apnsConfigKeys returns push.ttl and the keys push.apns.* of the APNs adapter (13 5.1, 11 and
// 17; brief M3-20 design note 2). The bounds of push.apns.jwt_refresh_interval are Apple's rule
// for token updates (13 5.1): the loader refuses a value outside them in every role, so the
// server does not start.
func apnsConfigKeys() []config.Key {
	return []config.Key{
		{Name: "push.ttl", Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: chapter13,
			Doc: "Lifetime of a push at the provider (apns-expiration, android.ttl)."},
		{Name: "push.apns.team_id", Kind: config.KindString, Default: "", Chapter: chapter13,
			Doc: "Apple team ID of the APNs key; APNs is enabled when team ID, key ID and topics are set."},
		{Name: "push.apns.key_id", Kind: config.KindString, Default: "", Chapter: chapter13,
			Doc: "Key ID of the APNs .p8 key."},
		{Name: "push.apns.private_key_ref", Kind: config.KindSecretRef, Default: "", Chapter: chapter13,
			Doc: "Reference to the APNs .p8 key in the secret provider; required when APNs is enabled."},
		{Name: "push.apns.topics", Kind: config.KindStringList, Default: "", Chapter: chapter13,
			Doc: "Allowed bundle IDs (apns-topic) of the apps of this instance."},
		{Name: "push.apns.jwt_refresh_interval", Kind: config.KindDuration, Default: "40m", Min: "20m", Max: "60m", Chapter: chapter13,
			Doc: "Renewal interval of the APNs provider JWT; Apple requires 20 to 60 minutes."},
		{Name: "push.apns.retry_5xx_delay", Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: chapter13,
			Doc: "Wait before a push is retried after an APNs 500 or 503."},
		{Name: "push.apns.time_sensitive_enabled", Kind: config.KindBool, Default: "false", Chapter: chapter13,
			Doc: "Send input_needed hints with interruption-level time-sensitive."},
	}
}
