// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// pushConfigKeys returns the keys of the job notify.push_send (13 "Konfigurationswerte dieses
// Kapitels", 03 section 9 for push.max_consecutive_failures; M3-19). The provider keys push.ttl,
// push.apns.* and push.fcm.* come with M3-20 and M3-21.
func pushConfigKeys() []config.Key {
	return []config.Key{
		{Name: "push.max_age", Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: chapter13,
			Doc: "A hint not yet accepted by the provider after this age is dropped."},
		{Name: "push.retry_base", Kind: config.KindDuration, Default: "2s", Min: "1ms", Chapter: chapter13,
			Doc: "Base of the retry backoff of a push."},
		{Name: "push.retry_cap", Kind: config.KindDuration, Default: "5m", Min: "1ms", Chapter: chapter13,
			Doc: "Largest retry backoff of a push."},
		{Name: "push.max_concurrent_requests", Kind: config.KindInt, Default: "32", Min: "1", Max: "100000", Chapter: chapter13,
			Doc: "Concurrent provider requests per process."},
		{Name: "push.max_consecutive_failures", Kind: config.KindInt, Default: "5", Min: "1", Max: "100000", Chapter: chapter03,
			Doc: "Consecutive transient failures after which a push registration is deactivated."},
	}
}
