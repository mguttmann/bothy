// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
)

// PersonalData is the participant of runs in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 2, 4.4 and 5.2). It exports the runs the person triggered into
// runs.jsonl and the person's interactions into interactions.jsonl. It erases nothing: runs, run
// steps, follow-ups and components go with their thread or message (20 section 2), and the
// interactions of a deleted message go with it by their foreign key.
type PersonalData struct {
	pool *pg.Pool
}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over pool, the pool of the role bothy_app.
func NewPersonalData(pool *pg.Pool) *PersonalData { return &PersonalData{pool: pool} }

// Module returns "runs".
func (*PersonalData) Module() string { return "runs" }

// Tables returns the tables of runs with personal data of 20 section 2.
func (*PersonalData) Tables() []string {
	return []string{"runs", "run_steps", "run_followups", "message_components", "interactions"}
}

// exportRun is one line of runs.jsonl (docs/api/gdpr-export/v1/lines/runs.schema.json). The
// status line, the last text excerpt, the summary, the plan, the state reason and the run steps
// are left out: the agent writes them from the whole thread, so they can carry contents of other
// people (Art. 15(4) GDPR), and a filter cannot tell which.
type exportRun struct {
	ID             ids.ID      `json:"id"`
	ThreadID       ids.ID      `json:"thread_id"`
	AgentID        ids.ID      `json:"agent_id"`
	TriggerKind    string      `json:"trigger_kind"`
	State          string      `json:"state"`
	QueuedAt       time.Time   `json:"queued_at"`
	StartedAt      time.Time   `json:"started_at,omitzero"`
	FinishedAt     time.Time   `json:"finished_at,omitzero"`
	CostMicros     int64       `json:"cost_micros"`
	Currency       string      `json:"currency"`
	CostIsEstimate bool        `json:"cost_is_estimate"`
	Metrics        *runMetrics `json:"metrics,omitempty"`
}

// runMetrics are the numeric members of run_metrics.v1. last_api_retry is left out, because it
// carries the error text of the model provider.
type runMetrics struct {
	ContextPct         float64 `json:"context_pct,omitempty"`
	TokensIn           int64   `json:"tokens_in,omitempty"`
	TokensOut          int64   `json:"tokens_out,omitempty"`
	TokensCacheRead    int64   `json:"tokens_cache_read,omitempty"`
	TokensCacheWrite   int64   `json:"tokens_cache_write,omitempty"`
	TokensCacheWrite1h int64   `json:"tokens_cache_write_1h,omitempty"`
	Subagents          int64   `json:"subagents,omitempty"`
	BackgroundJobs     int64   `json:"background_jobs,omitempty"`
	APIRetries         int64   `json:"api_retries,omitempty"`
	ProgressEvents     int64   `json:"progress_events,omitempty"`
}

// exportInteraction is one line of interactions.jsonl
// (docs/api/gdpr-export/v1/lines/interactions.schema.json). The response is left out: the app or
// agent wrote it.
type exportInteraction struct {
	ID          ids.ID          `json:"id"`
	ComponentID ids.ID          `json:"component_id"`
	MessageID   ids.ID          `json:"message_id"`
	State       string          `json:"state"`
	CreatedAt   time.Time       `json:"created_at"`
	Values      json.RawMessage `json:"values"`
}

// Export implements personaldata.Participant: one line per run of e.Subject into runs.jsonl and
// one line per own interaction into interactions.jsonl, oldest first, read in one read-only
// snapshot. Errors: those of the storage and of w; a stored metrics object that is no JSON
// object.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	var runs []sqlcgen.PersonalDataRunsRow
	var inters []sqlcgen.PersonalDataInteractionsRow
	err := pg.ReadSnapshot(ctx, p.pool, func(ctx context.Context, q pg.DBTX) error {
		var err error
		if runs, err = sqlcgen.New(q).PersonalDataRuns(ctx, pg.UUID(e.Subject)); err != nil {
			return err
		}
		inters, err = sqlcgen.New(q).PersonalDataInteractions(ctx, pg.UUID(e.Subject))
		return err
	})
	if err != nil {
		return err
	}
	for _, r := range runs {
		line, err := runLine(r)
		if err != nil {
			return err
		}
		if err := w.Line(personaldata.FileRuns, line); err != nil {
			return err
		}
	}
	for _, r := range inters {
		line := exportInteraction{
			ID: pg.IDOf(r.ID), ComponentID: pg.IDOf(r.ComponentID), MessageID: pg.IDOf(r.MessageID),
			State: r.State, CreatedAt: pg.TimeOf(r.CreatedAt), Values: r.Values,
		}
		if err := w.Line(personaldata.FileInteractions, line); err != nil {
			return err
		}
	}
	return nil
}

// runLine converts a stored run into its line. Errors: metrics that are no JSON object.
func runLine(r sqlcgen.PersonalDataRunsRow) (exportRun, error) {
	line := exportRun{
		ID: pg.IDOf(r.ID), ThreadID: pg.IDOf(r.ThreadID), AgentID: pg.IDOf(r.AgentID), TriggerKind: r.TriggerKind,
		State: r.State, QueuedAt: pg.TimeOf(r.QueuedAt), StartedAt: pg.TimeOf(r.StartedAt),
		FinishedAt: pg.TimeOf(r.FinishedAt), CostMicros: r.CostMicros, Currency: r.Currency,
		CostIsEstimate: r.CostIsEstimate,
	}
	var m runMetrics
	if err := json.Unmarshal(r.Metrics, &m); err != nil {
		return exportRun{}, fmt.Errorf("runs: metrics of run %s: %w", line.ID, err)
	}
	if m != (runMetrics{}) {
		line.Metrics = &m
	}
	return line, nil
}

// Erase implements personaldata.Participant and changes nothing in any step: 20 section 2 deletes
// the rows of runs with their thread or message, never with the account; with
// privacy.account_deletion_content = delete, the message deletion of messaging (20 section 4.2)
// deletes the components of each deleted message, and their interactions follow by cascade.
func (*PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
