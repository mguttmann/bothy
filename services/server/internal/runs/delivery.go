// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Delivery prepares the hand-over of one invocation-bearing event to one agent (02 4.3 "delivery
// preparation with I2, guard rules and token", M2-08). Every transport calls Prepare immediately
// before it sends such an event (04 15.3 step 2): the agent consumers of the gateway on the
// WebSocket and in the REST poll (one engine serves both, M2-07). The port uses only kernel types
// and the standard library, because sync is above runs (02 4.4).
//
// Prepare returns an error only for failures (database, signing, an unknown run, a run of another
// agent, a type that bears no invocation); every policy result is an outcome. A denial sends
// nothing (fail closed): the queued run fails with a reason, the triggering message gets
// trigger_state failed and one audit entry is written. An allow carries a fresh invocation token
// for exactly this delivery.
//
// Example:
//
//	out, err := delivery.Prepare(ctx, runs.DeliveryInput{AgentID: a, EventID: ev.ID, Seq: seq, Type: "agent.invocation",
//		ResourceKind: "run", ResourceID: runID, Payload: payload})
//	if err == nil && out.Send {
//		env.InvocationToken = out.InvocationToken
//	}
type Delivery interface {
	Prepare(ctx context.Context, in DeliveryInput) (DeliveryOutcome, error)
}

// DeliveryInput is one event about to be delivered to the agent AgentID.
type DeliveryInput struct {
	// AgentID is audience_principal_id of the event, the receiving agent.
	AgentID ids.ID
	// EventID and Seq name the event.
	EventID ids.ID
	Seq     int64
	// Type is agent.invocation, agent.followup, interaction.created or command.invoked.
	Type string
	// ResourceKind and ResourceID are the resource of the event (run for the first two).
	ResourceKind string
	ResourceID   ids.ID
	// Payload is the stored payload.
	Payload json.RawMessage
	// CorrelationID is event_log.correlation_id; zero when absent.
	CorrelationID ids.ID
	// Redelivered reports a delivery after agent.ack_timeout or after a reconnect.
	Redelivered bool
}

// DeliveryOutcome is the result of Prepare.
type DeliveryOutcome struct {
	// Send is false when the event counts as checked and not admitted: the transport sends
	// nothing and its cursor covers the event.
	Send bool
	// InvocationToken is the compact JWS of the fresh invocation token; set exactly when Send is
	// true. It is a delivery field only and never stored or logged (R-SY-17).
	InvocationToken string
}

// InvocationBearing reports whether an event of eventType carries an invocation and therefore
// passes Delivery.Prepare before every delivery to an agent: agent.invocation, agent.followup,
// interaction.created and command.invoked (04 15.3, 05 section 6).
func InvocationBearing(eventType string) bool {
	switch eventType {
	case "agent.invocation", "agent.followup", "interaction.created", "command.invoked":
		return true
	}
	return false
}
