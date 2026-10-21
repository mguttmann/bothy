// SPDX-License-Identifier: Apache-2.0

// Package rundelivery binds the delivery hook of the agent consumers (gateway/app DeliveryHook,
// M2-07 note 7) to the port runs.Delivery (M2-08 design note 9): every invocation-bearing event
// (agent.invocation, agent.followup, interaction.created, command.invoked) passes Prepare
// immediately before it is sent, on the WebSocket and in the REST poll, which share one engine;
// an allowed event gets its fresh invocation token as the delivery field invocation_token, a
// refused one counts as checked and is not sent. Every other event (run.cancel_requested,
// agent.config_changed, ...) passes unchanged and without a token. It replaces passdelivery.
//
// Example:
//
//	engine, err := app.NewConsumerEngine(cfg, app.ConsumerDeps{Hook: rundelivery.Hook{Delivery: runsService}, ...})
package rundelivery

import (
	"context"
	"errors"

	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Hook implements gatewayapp.DeliveryHook over runs.Delivery.
type Hook struct {
	// Delivery prepares the invocation-bearing events.
	Delivery runs.Delivery
}

// Compile-time check: the hook of the consumer engine.
var _ gatewayapp.DeliveryHook = Hook{}

// errNoDelivery reports a hook without its port.
var errNoDelivery = errors.New("gateway: the delivery hook has no runs.Delivery")

// Prepare implements gatewayapp.DeliveryHook. An invocation-bearing envelope goes through
// runs.Delivery.Prepare: Send gives the envelope with the fresh token, no Send gives false (the
// event counts as checked and the cursor covers it), an error is returned unchanged, so the engine
// closes the connection or fails the poll and the event is prepared again on the next delivery
// (ruling D-M2-08-6). Every other envelope is delivered unchanged.
func (h Hook) Prepare(ctx context.Context, agentID ids.ID, ev sync.Envelope) (sync.Envelope, bool, error) {
	if !runs.InvocationBearing(ev.Type) {
		return ev, true, nil
	}
	if h.Delivery == nil {
		return sync.Envelope{}, false, errNoDelivery
	}
	in := runs.DeliveryInput{
		AgentID: agentID, EventID: ev.ID, Type: ev.Type, ResourceKind: ev.Resource.Kind, ResourceID: ev.Resource.ID,
		Payload: ev.Payload, CorrelationID: ev.CorrelationID, Redelivered: ev.Redelivered,
	}
	if ev.Seq != nil {
		in.Seq = *ev.Seq
	}
	out, err := h.Delivery.Prepare(ctx, in)
	if err != nil {
		return sync.Envelope{}, false, err
	}
	if !out.Send {
		return sync.Envelope{}, false, nil
	}
	ev.InvocationToken = out.InvocationToken
	return ev, true, nil
}
