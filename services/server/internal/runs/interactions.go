// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Interactions is the incoming port of the components and interactions of runs (02 4.3, M2-15):
// createInteraction, respondToInteraction and updateComponent. The principal comes from the
// context (kernel/actor), as for the other operations of the port; errors are kernel/errs errors.
//
// Example:
//
//	res, err := interactions.Create(ctx, runs.CreateInteraction{ComponentID: c, ClientInteractionID: ids.New()})
//	if err == nil && res.Replay {
//		// 200 with Idempotent-Replay
//	}
type Interactions interface {
	// Create operates a component for the person of the request (interaction.create): the
	// rights, the component's state and the single-use rule under row locks, the values against
	// the definition, idempotent by client_interaction_id; interaction.created for the card's
	// agent and, without a waiting run, a new run with trigger_kind interaction.
	Create(ctx context.Context, cmd CreateInteraction) (InteractionResult, error)
	// Respond stores the final state and the hint of the agent that owns the component
	// (interaction.respond): only from received or dispatched; the same final state again is no
	// change; every other transition is 409 invalid_state_transition.
	Respond(ctx context.Context, cmd RespondInteraction) (Interaction, error)
	// UpdateComponent changes the state or the definition of a component of the author of the
	// card (component.update) with If-Match on the component version; a change raises the
	// component and message versions and appends message.updated.
	UpdateComponent(ctx context.Context, cmd UpdateComponent) (Component, error)
	// DisableAuthoredBy disables every active component of the cards of authorID in the ambient
	// unit of work and returns how many it disabled (M2-25: app.suspend and app.uninstall, 11
	// section 4.3). It calls no authorize: the caller authorized its owner action. Per card one
	// message.updated; the lock order of M2-15 holds (messages, then components, ascending).
	// Errors: internal outside a unit of work; the errors of the database and of messaging.
	DisableAuthoredBy(ctx context.Context, authorID ids.ID) (int, error)
}

// CreateInteraction is the request of createInteraction (schema InteractionCreate).
type CreateInteraction struct {
	ComponentID, ClientInteractionID ids.ID
	// Values is the JSON object values as received; nil means {}.
	Values json.RawMessage
}

// InteractionResult is the result of Interactions.Create: the interaction and whether it is the
// original of an earlier identical request (200 with Idempotent-Replay).
type InteractionResult struct {
	Interaction Interaction
	Replay      bool
}

// RespondInteraction is the request of respondToInteraction (schema InteractionRespond).
type RespondInteraction struct {
	InteractionID ids.ID
	// State is acknowledged, failed or rejected.
	State string
	// ResponseText is response.text; nil when absent. HasResponse reports a response member.
	ResponseText *string
	HasResponse  bool
}

// UpdateComponent is the request of updateComponent (schema ComponentUpdate).
type UpdateComponent struct {
	ComponentID ids.ID
	// IfMatch is the version of the required If-Match header; 0 never matches.
	IfMatch int64
	// State is active or disabled; nil keeps the state.
	State *string
	// Definition is the new component.v1 object; nil keeps the definition.
	Definition json.RawMessage
}

// Interaction mirrors the schema Interaction. Zero values are absent members.
type Interaction struct {
	ID, ComponentID, MessageID ids.ID
	// CustomID is the custom_id of the component.
	CustomID string
	// ThreadID is the thread of the card; RunID the component's run or the run the interaction
	// opened (zero when neither exists).
	ThreadID, RunID     ids.ID
	PrincipalID         ids.ID
	ClientInteractionID ids.ID
	Values              json.RawMessage
	State               string
	HasResponse         bool
	ResponseText        *string
	CreatedAt           time.Time
}

// Component mirrors the schema Component. Zero values are absent members.
type Component struct {
	ID, MessageID  ids.ID
	CustomID, Type string
	// Definition is the stored component.v1 object.
	Definition json.RawMessage
	Position   int
	SingleUse  bool
	// RequiredCapability is "" for none; RestrictedTo any, run_starter or owner.
	RequiredCapability, RestrictedTo string
	RunID                            ids.ID
	State                            string
	ExpiresAt                        time.Time
	UsedBy                           ids.ID
	UsedAt                           time.Time
	Version                          int64
}
