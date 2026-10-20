// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// SessionMetrics records the session instruments of docs/observability/signals.md 4.3a that M2-46
// adds (Design note 12): closed counts, codes and states only, never an ID of a person or a host
// name (24 section 10.4). The adapter adapters/otelmetrics implements it; NoSessionMetrics is the
// default until internal/wire binds it.
type SessionMetrics interface {
	// SessionClosed counts bothy.terminal.sessions.closed once with reason, after the commit of the
	// end in the process that committed it.
	SessionClosed(ctx context.Context, reason domain.CloseReason)
	// SessionsActive moves bothy.terminal.sessions.active of state (opening, active or detached) by
	// delta: the sessions the relay of this process holds.
	SessionsActive(ctx context.Context, state string, delta int64)
	// HostsConnected moves bothy.terminal.hosts.connected by delta: the control connections of this
	// process that are usable for sessions.
	HostsConnected(ctx context.Context, delta int64)
}

// NoSessionMetrics records nothing.
type NoSessionMetrics struct{}

// SessionClosed implements SessionMetrics.
func (NoSessionMetrics) SessionClosed(context.Context, domain.CloseReason) {}

// SessionsActive implements SessionMetrics.
func (NoSessionMetrics) SessionsActive(context.Context, string, int64) {}

// HostsConnected implements SessionMetrics.
func (NoSessionMetrics) HostsConnected(context.Context, int64) {}

// sessionMetricsHolder carries the bound SessionMetrics through the atomic pointer.
type sessionMetricsHolder struct{ m SessionMetrics }

// BindSessionMetrics binds the session instruments (M2-46); nil binds NoSessionMetrics.
func (s *Service) BindSessionMetrics(m SessionMetrics) {
	if m == nil {
		m = NoSessionMetrics{}
	}
	s.sessionMetricsP.Store(&sessionMetricsHolder{m: m})
}

// sessionMetrics returns the bound session instruments, NoSessionMetrics before BindSessionMetrics.
func (s *Service) sessionMetrics() SessionMetrics {
	if h := s.sessionMetricsP.Load(); h != nil {
		return h.m
	}
	return NoSessionMetrics{}
}

// Instruments returns the bound session instruments for the relay and the control connections of
// adapters/wsrelay (internal/wire hands them over once).
func (s *Service) Instruments() SessionMetrics { return s.sessionMetrics() }
