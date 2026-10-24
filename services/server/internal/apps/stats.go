// SPDX-License-Identifier: Apache-2.0

package apps

import (
	"context"
	"time"
)

// The metric names of the statistics source of apps (docs/observability/admin-statistics.md
// section 2, M6-05).
const (
	// MetricWebhookFinished counts the deliveries that ended (succeeded, failed or dead).
	MetricWebhookFinished = "webhook_finished"
	// MetricWebhookFailed counts the deliveries that ended failed or dead.
	MetricWebhookFailed = "webhook_failed"
)

// StatsValue is one aggregate: the value of Metric in the bucket starting at Bucket for the
// instance (Group ""). It never names an app, a person or carries content.
type StatsValue struct {
	Bucket time.Time
	Group  string
	Metric string
	Value  float64
}

// StatsSource is the statistics port of apps (M6-05). webhook_deliveries has no channel, so the
// stats use case calls it only for stats.read on the instance by an owner; it never authorizes.
type StatsSource interface {
	// WebhookStats returns MetricWebhookFinished and MetricWebhookFailed of the deliveries still
	// within their retention per bucket of created_at in the half-open range from, to, aligned to
	// granularity (hour, day, week or month) in the IANA time zone tz. Errors: validation_failed
	// for an unknown granularity or time zone, a database error.
	//
	// Example:
	//
	//	vs, err := src.WebhookStats(ctx, from, to, "day", "UTC")
	WebhookStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]StatsValue, error)
}
