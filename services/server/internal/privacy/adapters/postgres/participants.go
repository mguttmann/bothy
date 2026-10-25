// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/privacy/adapters/postgres/sqlcgen"
)

// tableRequests is the one table of the module with personal data (20 section 2).
const tableRequests = "gdpr_requests"

// Retention is the participant of the job retention.privacy_job (20 section 4.1, brief M7-06):
// it deletes requests in a final state whose completed_at lies before the cutoff
// (retention.privacy_job). A completed export whose package is still stored stays until the job
// privacy.export_expiry removed the package, so no package outlives its request row. It is safe
// for concurrent use.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class implements retention.Participant.
func (*Retention) Class() retention.Class { return retention.ClassPrivacyJob }

// Tables implements retention.Participant.
func (*Retention) Tables() []string { return []string{tableRequests} }

// Purge implements retention.Participant: one batch in its own short statement.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	n, err := sqlcgen.New(pg.Querier(ctx, r.pool)).PurgeRequests(ctx, sqlcgen.PurgeRequestsParams{
		Cutoff: ts(cutoff), RowLimit: int32(min(batch, 1<<20)), // #nosec G115 -- bounded
	})
	if err != nil {
		return 0, false, err
	}
	return int(n), int(n) == batch, nil
}

// PersonalData is the participant of privacy in the export and the deletion (20 sections 4.4 and
// 5.2): it exports the privacy requests of the person into access.json, section
// privacy_requests (kind, state, reason, times; never the package itself). It erases nothing:
// the requests are the evidence of the deletion and go with retention.privacy_job.
type PersonalData struct {
	pool *pg.Pool
}

// Compile-time check.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over pool.
func NewPersonalData(pool *pg.Pool) *PersonalData { return &PersonalData{pool: pool} }

// Module implements personaldata.Participant.
func (*PersonalData) Module() string { return "privacy" }

// Tables implements personaldata.Participant.
func (*PersonalData) Tables() []string { return []string{tableRequests} }

// privacyRequest is one entry of access.json section privacy_requests.
type privacyRequest struct {
	ID          ids.ID    `json:"id"`
	Kind        string    `json:"kind"`
	State       string    `json:"state"`
	Reason      string    `json:"reason"`
	RequestedBy ids.ID    `json:"requested_by"`
	DueAt       time.Time `json:"due_at"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitzero"`
}

// Export implements personaldata.Participant.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	rows, err := many(sqlcgen.New(pg.Querier(ctx, p.pool)).RequestsOfSubject(ctx, uuid(e.Subject)))
	if err != nil {
		return err
	}
	out := make([]privacyRequest, 0, len(rows))
	for _, r := range rows {
		out = append(out, privacyRequest{
			ID: r.ID, Kind: string(r.Kind), State: string(r.State), Reason: r.Reason, RequestedBy: r.RequestedBy, DueAt: r.DueAt,
			CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt,
		})
	}
	return w.Section(personaldata.FileAccess, "privacy_requests", out)
}

// Erase implements personaldata.Participant: nothing in any step (see the type).
func (*PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
