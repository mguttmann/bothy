// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The source ports of the components of a card (02 sections 4.1 point 4 and 4.3, M2-15): runs owns
// message_components, messaging owns messages and the request of createMessage, and messaging may
// not import runs. So messaging declares what it needs and runs implements it in
// internal/runs/adapters/messagingsource; internal/wire connects both.

// ComponentInput is one member of MessageCreate.components as received; pointers mean "given".
type ComponentInput struct {
	// CustomID and Type are the members custom_id and type.
	CustomID, Type string
	// Definition is the JSON object definition (component.v1) as the request carried it.
	Definition json.RawMessage
	// SingleUse is single_use; nil when absent.
	SingleUse *bool
	// RequiredCapability and RestrictedTo are "" when absent.
	RequiredCapability string
	RestrictedTo       string
	// ExpiresAt is expires_at; nil when absent.
	ExpiresAt *time.Time
}

// CardInput is the card part of a createMessage request.
type CardInput struct {
	// AuthorID and AuthorKind (user, agent, app) are the principal of the request.
	AuthorID   ids.ID
	AuthorKind string
	// ThreadID is the thread of the new message.
	ThreadID ids.ID
	// RunID is MessageCreate.run_id; zero when absent.
	RunID ids.ID
	// Components are the members of MessageCreate.components in request order.
	Components []ComponentInput
	// Now is the request time, the base of the defaults of expires_at.
	Now time.Time
}

// CheckedCard is a card that passed every check without database access.
type CheckedCard struct {
	// Canonical is the input of the request hash bothy-msg-v2 (M2-15 design note 6 c).
	Canonical []byte
	// Input is the request as checked.
	Input CardInput
	// Components are the components as they will be stored (positions, defaults, normalized
	// definitions), in request order, with State active, Version 1 and empty IDs; Attach assigns
	// the IDs. messaging uses them for the size rule of message.created.
	Components []Component
}

// Component mirrors the schema Component. Zero values are absent members.
type Component struct {
	ID ids.ID
	// CustomID, Type and Definition are the agent's members; Definition is the stored JSON object.
	CustomID, Type string
	Definition     json.RawMessage
	// Position is the order in the card, from 1.
	Position  int
	SingleUse bool
	// RequiredCapability is "" for none; RestrictedTo any, run_starter or owner.
	RequiredCapability, RestrictedTo string
	// RunID is the run the card's question belongs to; zero without run.
	RunID ids.ID
	// State is active, used, expired or disabled.
	State string
	// ExpiresAt, UsedBy and UsedAt are zero when absent.
	ExpiresAt time.Time
	UsedBy    ids.ID
	UsedAt    time.Time
	Version   int64
}

// ComponentWriter is declared by messaging and implemented by runs (02 4.1 point 4): the components
// of a card are content that MessagePosted never carries (M1-56), and a card must be rejected
// before its message exists.
type ComponentWriter interface {
	// Check validates the components of a card request without database access (component.v1,
	// the server fields). Errors: 400 validation_failed with one errors[] entry per finding.
	Check(ctx context.Context, in CardInput) (CheckedCard, error)
	// Attach stores the components of a new card message in the ambient unit of work: it locks
	// the run of RunID with a share lock and checks it (400 invalid on /run_id), checks the
	// author's supports_components (403 feature_disabled), inserts the rows and schedules the
	// expiry job. It returns the stored components in position order.
	Attach(ctx context.Context, messageID ids.ID, card CheckedCard) ([]Component, error)
}

// ComponentReader is declared by messaging and implemented by runs (02 4.3).
type ComponentReader interface {
	// ForMessages returns the components of the given messages ordered by position; a message
	// without components is absent. It reads in the ambient transaction when there is one. The
	// caller names only messages that authorize already checked or that messaging loaded for an
	// event of the same transaction (07 7.4 rule 1).
	ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]Component, error)
}

// CardFacts are the facts of a card message that runs reads under its row lock
// (Messages.LockCard, M2-15 design note 2 b).
type CardFacts struct {
	MessageID, ThreadID, ChannelID, SpaceID, AuthorID ids.ID
	// AuthorKind is user, agent or app; Kind the message kind.
	AuthorKind, Kind string
	// Deleted reports a tombstone.
	Deleted bool
	// Version is messages.version.
	Version int64
}
