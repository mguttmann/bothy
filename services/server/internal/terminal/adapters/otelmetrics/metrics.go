// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the terminal module (M2-44, D-W-105, M2-46): the
// four instruments of docs/observability/signals.md 4.3a through the declarations of the metric
// registry, each with a closed attribute set and never an ID of a person or a host name:
// bothy.terminal.relay.unpaired (bothy.terminal.side), bothy.terminal.sessions.closed
// (bothy.reason), bothy.terminal.sessions.active (bothy.terminal.state) and
// bothy.terminal.hosts.connected (none). It implements app.Metrics and app.SessionMetrics.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
//	svc.BindMetrics(m)
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Metrics are the instruments of the terminal module.
type Metrics struct {
	unpaired, closed *metrics.Counter
	active, hosts    *metrics.UpDownCounter
}

// Compile-time checks: Metrics is the outgoing port of app.
var (
	_ app.Metrics        = (*Metrics)(nil)
	_ app.SessionMetrics = (*Metrics)(nil)
)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. Errors: those of the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	unpaired, err := metrics.NewCounter(mp, metrics.TerminalRelayUnpaired, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	closed, err := metrics.NewCounter(mp, metrics.TerminalSessionsClosed, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	active, err := metrics.NewUpDownCounter(mp, metrics.TerminalSessionsActive, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	hosts, err := metrics.NewUpDownCounter(mp, metrics.TerminalHostsConnected, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{unpaired: unpaired, closed: closed, active: active, hosts: hosts}, nil
}

// SessionClosed implements app.SessionMetrics.
func (m *Metrics) SessionClosed(ctx context.Context, reason domain.CloseReason) {
	m.closed.Add(ctx, 1, string(reason))
}

// SessionsActive implements app.SessionMetrics.
func (m *Metrics) SessionsActive(ctx context.Context, state string, delta int64) {
	m.active.Add(ctx, delta, state)
}

// HostsConnected implements app.SessionMetrics.
func (m *Metrics) HostsConnected(ctx context.Context, delta int64) {
	m.hosts.Add(ctx, delta)
}

// RelayUnpaired implements app.Metrics.
func (m *Metrics) RelayUnpaired(ctx context.Context, side domain.Side) {
	m.unpaired.Add(ctx, 1, string(side))
}
