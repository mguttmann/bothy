// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// The keys of the cross-checks of the outgoing webhooks (M2-26).
const (
	keyWebhookBackoffBase = "webhook.retry_backoff_base"
	keyWebhookBackoffCap  = "webhook.retry_backoff_cap"
	keyWebhookRetryMaxAge = "webhook.retry_max_age"
)

// webhookConfigKeys returns the keys the outgoing webhooks read first (M2-26 design note 15): the
// failure limit and the backoff of docs/spec/03-datenmodell.md section 9, the delivery quota, the
// retry deadline, the rotation overlap and the master key reference of
// docs/spec/11-erweiterungen.md section 16. webhook.delivery_timeout is registered with the
// signing keys (M1-32) and webhook.allowed_hosts with the apps (M2-24); webhook.receiver_tolerance
// is a value of the receiver and no server key (D-W-23).
func webhookConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "webhook.max_consecutive_failures", Kind: config.KindInt, Default: "20", Min: "1",
			Doc:     "Failures in a row after which a subscription is disabled.",
			Chapter: chapter03,
		},
		{
			Name: keyWebhookBackoffBase, Kind: config.KindDuration, Default: "10s", Min: minDeadline,
			Doc:     "Base of the backoff between delivery attempts.",
			Chapter: chapter03,
		},
		{
			Name: keyWebhookBackoffCap, Kind: config.KindDuration, Default: "1h", Min: minDeadline,
			Doc:     "Largest backoff between delivery attempts.",
			Chapter: chapter03,
		},
		{
			Name: "webhook.max_pending_per_subscription", Kind: config.KindInt, Default: "10000", Min: "1",
			Doc:     "Open deliveries per subscription; reaching it pauses the subscription.",
			Chapter: chapter11,
		},
		{
			Name: keyWebhookRetryMaxAge, Kind: config.KindDuration, Default: "72h", Min: minDeadline,
			Doc:     "Time after which a failing delivery is dead and its subscription disabled.",
			Chapter: chapter11,
		},
		{
			Name: "webhook.secret_rotation_overlap", Kind: config.KindDuration, Default: "24h", Min: minDeadline,
			Doc:     "How long the server signs with the old and the new HMAC generation after a rotation.",
			Chapter: chapter11,
		},
		{
			Name: "webhook.hmac_master_key_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/webhook_hmac_master_key",
			Doc:     "Secret reference of the master key from which the HMAC secrets of the subscriptions are derived.",
			Chapter: chapter11,
		},
	}
}
