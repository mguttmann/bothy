// SPDX-License-Identifier: Apache-2.0

package stats

import "context"

// ClientMetricsBatch is a batch of app metrics (schema ClientMetricsBatch, docs/spec/
// 15-client-architektur.md 17): platform, app version, density and theme of the app and the
// entries. Density and Theme are "" when absent.
type ClientMetricsBatch struct {
	Platform, AppVersion, Density, Theme string
	Metrics                              []ClientMetric
}

// ClientMetric is one entry of a batch (schema ClientMetric). CloseCode and Value are nil when
// absent.
type ClientMetric struct {
	Name, Kind string
	CloseCode  *int
	Value      *float64
	Buckets    []ClientMetricBucket
}

// ClientMetricBucket is one histogram bucket: upper bound and count.
type ClientMetricBucket struct {
	LE    float64
	Count int64
}

// ClientMetrics is the incoming port of submitClientMetrics (client_metrics.submit, M4-31, ADR
// 0057). The caller is the principal in ctx.
type ClientMetrics interface {
	// Submit checks client_metrics.submit on the instance and the caller's
	// privacy.client_metrics (absent means on, 01 section 11), then passes every entry of the
	// closed list of signals.md 4.6a on to the telemetry under bothy.client.* and discards every
	// other entry one by one, counted by reason. It attaches no principal and no device and writes
	// no table. Errors: the decision of authorize (403 for agents and apps, 401 for anonymous),
	// 403 feature_disabled when the setting is off, an error of the settings read.
	//
	// Example:
	//
	//	err := cm.Submit(ctx, stats.ClientMetricsBatch{Platform: "macos", AppVersion: "1.4.0",
	//		Metrics: []stats.ClientMetric{{Name: "client.frames", Kind: "counter", Value: &n}}})
	Submit(ctx context.Context, b ClientMetricsBatch) error
}
