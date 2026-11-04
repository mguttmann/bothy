// SPDX-License-Identifier: Apache-2.0

// Package assistrelay implements the source port search.AssistRelay on the hub of the gateway
// (docs/spec/14-search.md 4.10), as autocompleterelay implements runs.AutocompleteRelay: it adds
// the correlation of the person's request and hands the event to Hub.AskAssistant.
package assistrelay

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/search"
)

// Correlator returns correlation_id and trace_context of the request in ctx (the correlator of
// the event log, sync/adapters/telemetry): trace_context only when the request was recorded.
type Correlator interface {
	Correlation(ctx context.Context) (ids.ID, string)
}

// Asker is the hub of the process (app.Hub.AskAssistant).
type Asker interface {
	AskAssistant(ctx context.Context, r app.AutocompleteRequest) (bool, error)
}

// Relay implements search.AssistRelay.
type Relay struct {
	hub  Asker
	corr Correlator
}

// Compile-time check: Relay is the source port of search.
var _ search.AssistRelay = (*Relay)(nil)

// New returns the relay over hub with the correlator corr (nil adds no correlation).
func New(hub Asker, corr Correlator) *Relay { return &Relay{hub: hub, corr: corr} }

// AskAssistant implements search.AssistRelay.
func (r *Relay) AskAssistant(ctx context.Context, agentID ids.ID, ev eventlog.EphemeralEvent) (bool, error) {
	req := app.AutocompleteRequest{OwnerID: agentID, Event: ev}
	if r.corr != nil {
		req.CorrelationID, req.TraceContext = r.corr.Correlation(ctx)
	}
	return r.hub.AskAssistant(ctx, req)
}
