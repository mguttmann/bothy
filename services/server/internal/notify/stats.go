// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"time"
)

// The metric names of the statistics source of notify (docs/observability/admin-statistics.md
// section 2, M6-05).
const (
	// MetricPushRegistrations counts the registrations of a provider active at some time in a
	// bucket: created before its end and not disabled before its start.
	MetricPushRegistrations = "push_registrations"
	// MetricPushDisabled counts the registrations of a provider disabled in a bucket for a
	// provider reason (token_invalid, topic_mismatch, sender_mismatch, repeated_failures); a
	// logout or a revoked device is no failure.
	MetricPushDisabled = "push_disabled"
)

// StatsValue is one aggregate: the value of Metric in the bucket starting at Bucket for the push
// provider Group. It never names a person, a device or a token.
type StatsValue struct {
	Bucket time.Time
	Group  string
	Metric string
	Value  float64
}

// StatsSource is the statistics port of notify (M6-05). push_registrations has no channel, so the
// stats use case calls it only for stats.read on the instance by an owner; it never authorizes.
type StatsSource interface {
	// PushStats returns MetricPushRegistrations and MetricPushDisabled per bucket and provider.
	// starts and ends are parallel: bucket i is the half-open range starts[i], ends[i]. Errors:
	// validation_failed when the lists differ in length, a database error.
	//
	// Example:
	//
	//	vs, err := src.PushStats(ctx, starts, ends)
	PushStats(ctx context.Context, starts, ends []time.Time) ([]StatsValue, error)
}
