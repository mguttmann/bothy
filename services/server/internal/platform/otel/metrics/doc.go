// SPDX-License-Identifier: Apache-2.0

// Package metrics is the single Go declaration of every metric instrument the server registers
// (docs/observability/signals.md, docs/spec/17-observability.md rules 8 and 9, R-OB-06): one Spec
// per instrument with name, kind, unit, description, attributes with closed value sets and the
// bucket bounds of signals.md 4.7.
//
// It offers three things:
//
//   - Check, the structural rules every Spec must pass: every attribute has a closed value set,
//     no attribute names a person or an ID, every histogram has bounds;
//   - View, the SDK view of the meter provider: explicit bucket bounds and an allow filter of the
//     catalogue keys for every instrument, library instruments (otelhttp, otelpgx) included, and a
//     drop of everything the catalogue does not name;
//   - typed recording wrappers (Counter, UpDownCounter, Histogram, Gauge, ObservableGauge) that map
//     a value outside the closed set of an attribute to "other", or report it through the strict
//     switch the test helpers turn on (metricstest.Strict).
//
// Recording never blocks: synchronous instruments only add to the SDK aggregation, and the
// attribute sets are cached per value tuple, which the closed sets bound.
//
// Example:
//
//	c, err := metrics.NewCounter(mp, metrics.RateLimitRejected,
//		metrics.Values{metrics.SourceRateClasses: classes})
//	c.Add(ctx, 1, "rl.read")
package metrics
