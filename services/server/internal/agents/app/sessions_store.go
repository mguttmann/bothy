// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ErrSessionRefTaken is returned by SessionStore.UpdateSession when the new reference is current
// in another thread: the partial unique index (agent_id, external_session_ref) WHERE is_current
// refused the row (03 agent_sessions). The use case answers 409 conflict with reason
// session_bound_elsewhere, never a 500.
var ErrSessionRefTaken = errors.New("agents: the external session is current in another thread")

// NewSession is the input of SessionStore.InsertSession: one row of agent_sessions with version 1
// and cumulative_cost_micros 0. A row with IsCurrent false carries EndedAt and EndReason (a row
// created ended, M2-12 design note 12); a current row carries neither. Currency "" means the
// column default.
type NewSession struct {
	ID, ThreadID, AgentID ids.ID
	ExternalSessionRef    string
	// Model is "" for NULL.
	Model     string
	Started   bool
	IsCurrent bool
	// CostBaselineMicros and Currency are the cost baseline (22 section 9 step 7).
	CostBaselineMicros int64
	Currency           string
	CreatedAt          time.Time
	// EndedAt and EndReason are set exactly when IsCurrent is false.
	EndedAt   time.Time
	EndReason string
	// Version is the first version of the row; 0 means 1 (the column default). A row that
	// replaces another after bind_failed starts above the replaced row's version, so a concurrent
	// duplicate report with the old If-Match fails its compare-and-swap (M2-12 design note 12).
	Version int64
}

// SessionEnd is the input of SessionStore.EndSession: the current row ID at Version ends at At
// with Reason (ended_at, is_current = false, end_reason, version + 1).
type SessionEnd struct {
	ID      ids.ID
	Version int64
	Reason  string
	At      time.Time
}

// SessionChange is the input of SessionStore.UpdateSession: the new reference, model and started
// of the row ID at Version (version + 1). Every field is written as given; the use case passes the
// stored value for a field the report does not change.
type SessionChange struct {
	ID                 ids.ID
	Version            int64
	ExternalSessionRef string
	// Model is "" for NULL.
	Model   string
	Started bool
}

// SessionCost is the input of SessionStore.SetSessionCost: the new cost sum and currency of the
// row ID at Version (version + 1), as AddCost decided them (M2-22 design note 5 g).
type SessionCost struct {
	ID                   ids.ID
	Version              int64
	CumulativeCostMicros int64
	Currency             string
}

// SessionStore is the outgoing port of agent_sessions; adapters/postgres implements it. No other
// module reads or writes the table (03 principle 12). Reads join the ambient transaction of ctx;
// the Lock methods and the writes need a unit of work. Rows come back with every column of
// agents.Session. Errors are kernel/errs errors.
type SessionStore interface {
	// ThreadSessions reads every row of the thread, newest first (created_at DESC, id DESC).
	ThreadSessions(ctx context.Context, threadID ids.ID) ([]agents.Session, error)
	// CurrentSession reads the current row of the thread without a lock; found is false for none.
	CurrentSession(ctx context.Context, threadID ids.ID) (agents.Session, bool, error)
	// LockCurrentSession reads the current row of the thread with its row lock (SELECT ... FOR
	// UPDATE); found is false for none.
	LockCurrentSession(ctx context.Context, threadID ids.ID) (agents.Session, bool, error)
	// LockSessions reads the rows ids with their row locks taken in ascending id (03 6.3); unknown
	// ids are absent from the result, which is ordered by id.
	LockSessions(ctx context.Context, ids []ids.ID) ([]agents.Session, error)
	// CurrentByRef reads the row in which (agentID, ref) is current, in any thread, without a lock;
	// found is false when the reference is current nowhere.
	CurrentByRef(ctx context.Context, agentID ids.ID, ref string) (agents.Session, bool, error)
	// LatestByRef reads the most recent row (created_at DESC, id DESC) of (agentID, ref) in any
	// thread, current or ended; found is false for none (the cost baseline of a bind, M2-12 design
	// note 14).
	LatestByRef(ctx context.Context, agentID ids.ID, ref string) (agents.Session, bool, error)
	// LatestInThreadByRef reads the most recent row of (threadID, agentID, ref), current or ended;
	// found is false for none (AttachOnStart, M2-12 design note 10).
	LatestInThreadByRef(ctx context.Context, threadID, agentID ids.ID, ref string) (agents.Session, bool, error)
	// InsertSession inserts s with ON CONFLICT DO NOTHING on the two partial unique indexes;
	// inserted is false when the thread already has a current row or (AgentID, ref) is current in
	// another thread (the caller re-reads to tell which).
	InsertSession(ctx context.Context, s NewSession) (row agents.Session, inserted bool, err error)
	// EndSession ends the current row e.ID if its version is e.Version; changed is false
	// otherwise (the caller holds the row lock, so false is a programming error or a concurrent
	// end it re-reads).
	EndSession(ctx context.Context, e SessionEnd) (row agents.Session, changed bool, err error)
	// UpdateSession writes c if the row's version is c.Version and raises the version; changed is
	// false otherwise. Errors: ErrSessionRefTaken when the new reference is current elsewhere.
	UpdateSession(ctx context.Context, c SessionChange) (row agents.Session, changed bool, err error)
	// SetSessionCost writes c if the row's version is c.Version and raises the version; changed is
	// false otherwise (the caller holds the row lock, so false is a programming error).
	SetSessionCost(ctx context.Context, c SessionCost) (row agents.Session, changed bool, err error)
}
