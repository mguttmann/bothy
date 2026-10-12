// SPDX-License-Identifier: Apache-2.0

// Package attr holds the attribute keys the server may put on spans, logs and metrics: every key
// of docs/observability/signals.md "Attributes in spans and logs" and every other non-resource key
// of the collector allowlist (infra/otel/collector.yaml, redaction/allowlist.allowed_keys) except
// the gen_ai.* keys, which live in package genai (signals.md "GenAI conventions").
//
// It is the first line of docs/observability/privacy.md "Two lines": instrumentation sets only
// these keys, the log handlers and the span exporter of platform/otel drop every other key, and
// the collector allowlist is the second line. TestAttrKeysEqualCollectorAllowlist keeps both lists
// equal, so adding a key means adding it to signals.md, to the collector allowlist and here, in
// one change.
//
// Each key has one scope. Span and log keys pass Allowed. Metric-only keys (Signal) are attributes
// of metrics and never appear on spans or logs. Resource keys (ServerRole) belong to the resource
// of the process. IDs of objects and the pseudonym of a person are span and log keys only and
// never a metric attribute (signals.md "Cardinality").
//
// Example:
//
//	logger.LogAttrs(ctx, slog.LevelWarn, "egress request blocked", slog.String(attr.Reason, "host_not_allowed"))
package attr
