// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Store is the outgoing port of the agents application layer over the table agents (implemented
// by adapters/postgres). Reads run in the ambient transaction of ctx when there is one and on the
// pool otherwise; writes and locks run only inside a unit of work of kernel/txn and fail with kind
// internal outside one. Errors are kernel/errs errors without SQL text or values.
type Store interface {
	// Agent returns the record id; found is false for an unknown id.
	Agent(ctx context.Context, id ids.ID) (a domain.Agent, found bool, err error)
	// LockAgent returns the record id like Agent and locks its row until the end of the unit of
	// work (03 section 6.3).
	LockAgent(ctx context.Context, id ids.ID) (a domain.Agent, found bool, err error)
	// LockAgents locks the records of list in ascending id and returns the known ones in that
	// order; unknown ids are left out.
	LockAgents(ctx context.Context, list []ids.ID) ([]domain.Agent, error)
	// LockPersonalAgents locks every record with billing_mode personal in ascending id, disabled
	// agents included (the emergency switch of 10 section 9.1).
	LockPersonalAgents(ctx context.Context) ([]domain.Agent, error)
	// ListAgents returns up to limit records with an id above after (the zero ID for the first
	// page), ascending by id.
	ListAgents(ctx context.Context, after ids.ID, limit int) ([]domain.Agent, error)
	// PersonalAgents returns every record with billing_mode personal, ascending by id, without
	// regard to the principal status; the policy source filters by status through identity.
	PersonalAgents(ctx context.Context) ([]domain.Agent, error)
	// InsertAgent inserts the record a; its principal must exist (identity, same unit of work).
	InsertAgent(ctx context.Context, a domain.Agent) error
	// UpdateAgent writes description, operator configuration, policy, tool class and version of
	// a. agent.update needs a unit of work; the policy, holder and tool class need a declared
	// rights change, which the caller owns.
	UpdateAgent(ctx context.Context, a domain.Agent) error
}

// Cursors is the outgoing port that encodes and checks the opaque cursors of listAgents
// (implemented by adapters/httpapi with platform/httpserver).
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and unmarshals its key into key. Errors:
	// cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}

// TokenRevoker is the part of identity.Tokens that agent.disable uses: every active API token of
// the agent ends inside the rights change (09 section 2.1).
type TokenRevoker interface {
	// RevokeAllForPrincipal revokes every active token of principalID and returns their number.
	RevokeAllForPrincipal(ctx context.Context, principalID, by ids.ID) (int, error)
}

// ConsumerStore is the outgoing port over agent_consumer_cursors (implemented by
// adapters/postgres). Every method is one statement on one row; it runs in the ambient transaction
// of ctx when there is one and on the pool otherwise, because the save ticker of the gateway has no
// request (docs/DECISIONS.md D-M2-07). Errors are kernel/errs errors without SQL text or values.
type ConsumerStore interface {
	// TakeConsumer creates the row of key at head with holder, or sets holder on the existing row,
	// and returns acked_seq and whether the row was created.
	TakeConsumer(ctx context.Context, key agents.ConsumerKey, holder ids.ID, head int64) (acked int64, created bool, err error)
	// SaveConsumerAck raises acked_seq of key to acked while holder holds it; found is false when
	// no row of key is held by holder.
	SaveConsumerAck(ctx context.Context, key agents.ConsumerKey, holder ids.ID, acked int64) (found bool, err error)
	// RestartConsumer sets acked_seq of key to seq while holder holds it; found as SaveConsumerAck.
	RestartConsumer(ctx context.Context, key agents.ConsumerKey, holder ids.ID, seq int64) (found bool, err error)
	// AckConsumer raises acked_seq of key to seq without a holder condition and returns the stored
	// value; found is false for an unknown consumer.
	AckConsumer(ctx context.Context, key agents.ConsumerKey, seq int64) (acked int64, found bool, err error)
	// GetConsumer reads acked_seq of key; found is false for an unknown consumer.
	GetConsumer(ctx context.Context, key agents.ConsumerKey) (acked int64, found bool, err error)
}

// PresenceStore is the outgoing port of the presence tracker over the reported columns of agents
// (implemented by adapters/postgres, M2-20). Reads run in the ambient transaction of ctx when
// there is one and on the pool otherwise; writes and locks run only inside a unit of work. A
// transaction of the tracker locks at most one agents row and no row of another module (03
// section 6.3). Errors are kernel/errs errors without SQL text or values.
type PresenceStore interface {
	// GetPresenceRow returns the configured values and the stored reported state of id; found is
	// false for an unknown id.
	GetPresenceRow(ctx context.Context, id ids.ID) (row domain.PresenceRow, found bool, err error)
	// LockPresenceRow returns the same row and locks it until the end of the unit of work.
	LockPresenceRow(ctx context.Context, id ids.ID) (row domain.PresenceRow, found bool, err error)
	// GetPresenceRows returns the rows of the known ids of list in ascending id.
	GetPresenceRows(ctx context.Context, list []ids.ID) ([]domain.PresenceRow, error)
	// ListLivePresence returns the rows stored as online or degraded in ascending id.
	ListLivePresence(ctx context.Context) ([]domain.PresenceRow, error)
	// WriteReportedChange stores a change of the reported mode or class with the whole reported
	// state of w; the only write of reported_billing_mode and reported_tool_class.
	WriteReportedChange(ctx context.Context, w PresenceWrite) error
	// PersistHeartbeat stores state, version, details and last heartbeat of w unless the row
	// holds a fresher heartbeat; updated is false then.
	PersistHeartbeat(ctx context.Context, w PresenceWrite) (updated bool, err error)
}

// PresenceWrite is one write of the reported state of an agent.
type PresenceWrite struct {
	// ID is the agent.
	ID ids.ID
	// State is the presence state to store (offline after the deadline).
	State domain.RuntimeState
	// Report is the last report; its mode and class are written only by WriteReportedChange.
	Report domain.Report
	// LastHeartbeatAt is the time of the last heartbeat of the report.
	LastHeartbeatAt time.Time
}
