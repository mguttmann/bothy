// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// PersonalData is the participant of agents in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 2, 4.4 and 5.2). It exports the agents whose credentials the
// person holds into the section agents_held of access.json. It erases nothing: agents stay (20
// section 2), the holder ID becomes pseudonymous with the principal in step 6, and agent sessions
// go with their thread.
type PersonalData struct {
	pool *pg.Pool
}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over pool, the pool of the role bothy_app.
func NewPersonalData(pool *pg.Pool) *PersonalData { return &PersonalData{pool: pool} }

// Module returns "agents".
func (*PersonalData) Module() string { return "agents" }

// Tables returns the tables of agents with personal data of 20 section 2. dev_environments is
// not listed: the table does not exist yet (milestone of 21).
func (*PersonalData) Tables() []string { return []string{"agents", "agent_sessions"} }

// heldAgent is one entry of agents_held (docs/api/gdpr-export/v1/sections/access.agents_held.schema.json).
// The table records no time from which the person holds the credentials
// (billing_mode_changed_at changes with the mode, not with the holder), so no time is given.
type heldAgent struct {
	AgentID ids.ID `json:"agent_id"`
}

// Export implements personaldata.Participant: the section agents_held of access.json, an empty
// array when the person holds no agent. Errors: those of the storage and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	var rows []pgtype.UUID
	err := pg.ReadSnapshot(ctx, p.pool, func(ctx context.Context, q pg.DBTX) error {
		var err error
		rows, err = sqlcgen.New(q).PersonalDataAgentsHeld(ctx, pg.UUID(e.Subject))
		return err
	})
	if err != nil {
		return err
	}
	held := make([]heldAgent, 0, len(rows))
	for _, r := range rows {
		held = append(held, heldAgent{AgentID: pg.IDOf(r)})
	}
	return w.Section(personaldata.FileAccess, "agents_held", held)
}

// Erase implements personaldata.Participant and changes nothing in any step: 20 section 2 keeps
// agents (the agent stays usable for the others; the owner chooses a new holder) and deletes
// agent sessions with their thread, never with the account.
func (*PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
