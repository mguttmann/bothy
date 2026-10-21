// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ComponentRow is one row of message_components as this module reads it (03 section 4.8, M2-15).
// Zero values are NULL.
type ComponentRow struct {
	ID, MessageID ids.ID
	// CustomID is the agent's identifier; Type button, select or form; Definition the normalized
	// component.v1 object (runs/domain).
	CustomID, Type string
	Definition     []byte
	// Position is the order in the card, from 1.
	Position  int
	SingleUse bool
	// RequiredCapability is "" for none; RestrictedTo any, run_starter or owner.
	RequiredCapability, RestrictedTo string
	// RunID is the run of the card's question; zero without run.
	RunID ids.ID
	// State is active, used, expired or disabled.
	State string
	// ExpiresAt is zero for none (never written by this module, which always sets one).
	ExpiresAt time.Time
	// UsedBy and UsedAt are set once the component is used.
	UsedBy  ids.ID
	UsedAt  time.Time
	Version int64
}

// ComponentWrite is the input of ComponentRepository.SetComponent: the whole new content of the
// mutable columns of one locked component; version rises by one.
type ComponentWrite struct {
	ID         ids.ID
	State      string
	Definition []byte
	UsedBy     ids.ID
	UsedAt     time.Time
}

// InteractionRow is one row of interactions (03 section 4.8, M2-15).
type InteractionRow struct {
	ID, ComponentID, MessageID, PrincipalID, ClientInteractionID ids.ID
	// Values is the stored JSON object ("{}" at least).
	Values []byte
	// State is received, dispatched, acknowledged, failed or rejected.
	State string
	// Response is the stored JSON object of the agent's answer; nil for NULL.
	Response             []byte
	CreatedAt, UpdatedAt time.Time
}

// NewInteraction is the input of ComponentRepository.InsertInteraction: an interaction in
// received at At.
type NewInteraction struct {
	ID, ComponentID, MessageID, PrincipalID, ClientInteractionID ids.ID
	Values                                                       []byte
	At                                                           time.Time
}

// ErrInteractionTaken is the error of ComponentRepository.InsertInteraction when the principal
// already has an interaction with the client_interaction_id (04 section 5.1 point 4): a concurrent
// twin committed first although the locks of the card should have ordered both (Design note 10 d).
var ErrInteractionTaken = errors.New("runs: the principal already used this client_interaction_id")

// ComponentRepository is the outgoing port of message_components and interactions (M2-15);
// adapters/postgres implements it next to Repository. Reads join the ambient transaction of ctx;
// writes and locks need one. The reads follow 07 7.4 rule 1: they name the components of
// messages that authorize checked or that messaging loaded for an event of the same transaction,
// or that a job or the end of a run names (tools/lint/sql_visibility.yaml). Errors are kernel/errs
// errors except ErrInteractionTaken.
type ComponentRepository interface {
	// InsertComponents inserts the components of one new card in state active with version 1.
	InsertComponents(ctx context.Context, rows []ComponentRow) error
	// ComponentsOfMessages reads the components of messageIDs in one statement, ordered by
	// message and position.
	ComponentsOfMessages(ctx context.Context, messageIDs []ids.ID) ([]ComponentRow, error)
	// Component reads one component; found is false for an unknown id.
	Component(ctx context.Context, id ids.ID) (row ComponentRow, found bool, err error)
	// LockComponent reads one component with its row lock.
	LockComponent(ctx context.Context, id ids.ID) (row ComponentRow, found bool, err error)
	// LockSingleUseGroup is the statement of 03 "Einmal-Regel je Nachricht": the active
	// single_use components of messageID in ascending id, locked.
	LockSingleUseGroup(ctx context.Context, messageID ids.ID) ([]ComponentRow, error)
	// LockSingleUseSet locks every single_use component of messageID in ascending id.
	LockSingleUseSet(ctx context.Context, messageID ids.ID) ([]ComponentRow, error)
	// LockDueComponents locks the active components of messageID with expires_at at or before
	// now, in ascending id.
	LockDueComponents(ctx context.Context, messageID ids.ID, now time.Time) ([]ComponentRow, error)
	// NextComponentExpiry returns the earliest expires_at of the active components of messageID;
	// found is false when none is left.
	NextComponentExpiry(ctx context.Context, messageID ids.ID) (at time.Time, found bool, err error)
	// SetComponent writes w to the locked component and raises its version by one.
	SetComponent(ctx context.Context, w ComponentWrite) (ComponentRow, error)
	// ActiveComponentMessagesOfRun returns the card messages with active components of runID,
	// ascending.
	ActiveComponentMessagesOfRun(ctx context.Context, runID ids.ID) ([]ids.ID, error)
	// LockActiveRunComponents locks the active components of runID on messageID in ascending id.
	LockActiveRunComponents(ctx context.Context, runID, messageID ids.ID) ([]ComponentRow, error)
	// DeleteComponentsOfMessage deletes the components of messageID (their interactions follow by
	// cascade) and returns how many it deleted.
	DeleteComponentsOfMessage(ctx context.Context, messageID ids.ID) (int64, error)
	// InsertInteraction inserts n in received. Errors: ErrInteractionTaken (wrapped) for a second
	// (principal, client_interaction_id).
	InsertInteraction(ctx context.Context, n NewInteraction) (InteractionRow, error)
	// InteractionByClientID reads the interaction of principal with clientID.
	InteractionByClientID(ctx context.Context, principal, clientID ids.ID) (row InteractionRow, found bool, err error)
	// Interaction reads one interaction.
	Interaction(ctx context.Context, id ids.ID) (row InteractionRow, found bool, err error)
	// LockInteraction reads one interaction with its row lock.
	LockInteraction(ctx context.Context, id ids.ID) (row InteractionRow, found bool, err error)
	// SetInteractionState writes the state of the locked interaction id at at and, when response
	// is not nil, its response.
	SetInteractionState(ctx context.Context, id ids.ID, state string, response []byte, at time.Time) (InteractionRow, error)
	// RunOfInteraction returns the run whose triggering_interaction_id is id; found is false when
	// the interaction opened none.
	RunOfInteraction(ctx context.Context, id ids.ID) (run ids.ID, found bool, err error)
	// LockRunShared reads the run id with a share lock (FOR SHARE) for the rest of the unit of
	// work: the lock of a new card on its run (Design note 6 b).
	LockRunShared(ctx context.Context, id ids.ID) (row RunRow, found bool, err error)
}
