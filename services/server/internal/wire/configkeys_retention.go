// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// The keys of the retention jobs (docs/spec/20-privacy.md section 13, M7-05).
const (
	keyRetentionContent       = "retention.content"
	keyRetentionContentReason = "retention.content_none_reason"
)

// The defaults of 20 section 13 that other keys share as text.
const (
	retentionSecurityDefault = "90d"
	retentionWebhookDefault  = "30d"
)

// retentionContentMin is the smallest period of retention.content, the minimum of the other
// retention periods: a typo such as 12m (meant as months) must not tombstone every message older
// than twelve minutes.
const retentionContentMin = 24 * time.Hour

// retentionContentNone is the value of retention.content that switches the automatic deletion of
// contents off (20 section 4.1).
const retentionContentNone = "none"

// retentionConfigKeys returns the keys of the jobs retention.<class> (M7-05) from 20 section 13;
// retention.audit and the keys of the IP pseudonyms stay with the audit keys (M1-24).
func retentionConfigKeys() []config.Key {
	return []config.Key{
		{Name: keyRetentionContent, Kind: config.KindString, Default: retentionContentNone, Chapter: chapter20,
			Doc: "Period after which messages are deleted as tombstones, or none for no automatic deletion."},
		{Name: keyRetentionContentReason, Kind: config.KindString, Default: "", Chapter: chapter20,
			Doc: "Reason of the instance operator for retention.content = none; empty makes bothy config check and the start log warn."},
		{Name: "retention.security_after_end", Kind: config.KindDuration, Default: retentionSecurityDefault, Min: "1d", Chapter: chapter20,
			Doc: "How long security data is kept after its expiry, revocation or end."},
		{Name: "retention.cost", Kind: config.KindDuration, Default: "395d", Min: "1d", Chapter: chapter20,
			Doc: "How long cost entries and closed price rows are kept."},
		{Name: "retention.webhook_deliveries", Kind: config.KindDuration, Default: retentionWebhookDefault, Min: "1d", Chapter: chapter20,
			Doc: "How long finished delivery attempts of outgoing webhooks are kept."},
		{Name: "retention.job_interval", Kind: config.KindDuration, Default: "1h", Min: "1m", Chapter: chapter20,
			Doc: "Interval of the jobs retention.<class>; it also bounds the time one run may take."},
		{Name: "retention.batch_size", Kind: config.KindInt, Default: "500", Min: "1", Max: "10000", Chapter: chapter20,
			Doc: "Rows a retention job removes per table and transaction."},
	}
}

// retentionContentCheck is the cross-check retention_content: retention.content is none or a
// duration of at least one day in the form of every duration key.
func retentionContentCheck(v *config.Values) error {
	if _, err := contentPeriod(v.String(keyRetentionContent)); err != nil {
		return err
	}
	return nil
}

// contentPeriod parses retention.content: zero for none, else the duration of at least one day.
func contentPeriod(text string) (time.Duration, error) {
	if text == retentionContentNone {
		return 0, nil
	}
	d, err := config.ParseDuration(text)
	if err != nil || d < retentionContentMin {
		return 0, errors.New("retention.content must be none or a duration of at least 1d")
	}
	return d, nil
}
