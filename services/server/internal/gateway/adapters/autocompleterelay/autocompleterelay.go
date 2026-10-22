// SPDX-License-Identifier: Apache-2.0

// Package autocompleterelay implements the source port runs.AutocompleteRelay on the hub of the
// gateway (brief M2-17 design note 1), as consumerfeed implements agents.ConsumerFeed (M2-07): it
// adds the correlation of the person's request and hands the request to Hub.AskAutocomplete.
package autocompleterelay

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// Correlator returns correlation_id and trace_context of the request in ctx (the correlator of
// the event log, sync/adapters/telemetry): trace_context only when the request was recorded.
type Correlator interface {
	Correlation(ctx context.Context) (ids.ID, string)
}

// Asker is the hub of the process (app.Hub.AskAutocomplete).
type Asker interface {
	AskAutocomplete(ctx context.Context, r app.AutocompleteRequest) (bool, error)
}

// Relay implements runs.AutocompleteRelay.
type Relay struct {
	hub  Asker
	corr Correlator
}

// Compile-time check: Relay is the source port of runs.
var _ runs.AutocompleteRelay = (*Relay)(nil)

// New returns the relay over hub with the correlator corr (nil adds no correlation).
func New(hub Asker, corr Correlator) *Relay { return &Relay{hub: hub, corr: corr} }

// Ask implements runs.AutocompleteRelay.
func (r *Relay) Ask(ctx context.Context, ownerID ids.ID, ev eventlog.EphemeralEvent, invocationToken string) (bool, error) {
	req := app.AutocompleteRequest{OwnerID: ownerID, Event: ev, InvocationToken: invocationToken}
	if r.corr != nil {
		req.CorrelationID, req.TraceContext = r.corr.Correlation(ctx)
	}
	return r.hub.AskAutocomplete(ctx, req)
}
