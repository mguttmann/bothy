// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the presence store of the agents application layer (M2-20).
var _ app.PresenceStore = (*Repository)(nil)

// GetPresenceRow implements app.PresenceStore.
func (r *Repository) GetPresenceRow(ctx context.Context, id ids.ID) (domain.PresenceRow, bool, error) {
	row, err := r.read(ctx).GetPresenceRow(ctx, uuid(id))
	return onePresence(sqlcgen.ListLivePresenceRow(row), err, "reading the presence of an agent")
}

// LockPresenceRow implements app.PresenceStore.
func (r *Repository) LockPresenceRow(ctx context.Context, id ids.ID) (domain.PresenceRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.PresenceRow{}, false, err
	}
	row, err := q.LockPresenceRow(ctx, uuid(id))
	return onePresence(sqlcgen.ListLivePresenceRow(row), err, "locking the presence of an agent")
}

// GetPresenceRows implements app.PresenceStore.
func (r *Repository) GetPresenceRows(ctx context.Context, list []ids.ID) ([]domain.PresenceRow, error) {
	if len(list) == 0 {
		return nil, nil
	}
	keys := make([]pgtype.UUID, 0, len(list))
	for _, id := range slices.Compact(slices.SortedFunc(slices.Values(list), func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })) {
		keys = append(keys, uuid(id))
	}
	rows, err := r.read(ctx).GetPresenceRows(ctx, keys)
	if err != nil {
		return nil, mapError(err, "reading the presence of agents")
	}
	out := make([]domain.PresenceRow, 0, len(rows))
	for _, row := range rows {
		p, err := presenceOf(sqlcgen.ListLivePresenceRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// ListLivePresence implements app.PresenceStore.
func (r *Repository) ListLivePresence(ctx context.Context) ([]domain.PresenceRow, error) {
	rows, err := r.read(ctx).ListLivePresence(ctx)
	if err != nil {
		return nil, mapError(err, "listing the live presence")
	}
	out := make([]domain.PresenceRow, 0, len(rows))
	for _, row := range rows {
		p, err := presenceOf(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// WriteReportedChange implements app.PresenceStore.
func (r *Repository) WriteReportedChange(ctx context.Context, w app.PresenceWrite) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.WriteReportedChange(ctx, sqlcgen.WriteReportedChangeParams{
		ReportedBillingMode: optionalText(string(w.Report.BillingMode)), ReportedToolClass: optionalText(string(w.Report.ToolClass)),
		ReportedRuntimeState: string(w.State), ReportedRuntimeVersion: optionalText(w.Report.RuntimeVersion),
		ReportedDetails: w.Report.StoredDetails(), LastHeartbeatAt: optionalTime(w.LastHeartbeatAt), PrincipalID: uuid(w.ID),
	})
	return mapError(err, "storing a change of the reported mode")
}

// PersistHeartbeat implements app.PresenceStore.
func (r *Repository) PersistHeartbeat(ctx context.Context, w app.PresenceWrite) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.PersistHeartbeat(ctx, sqlcgen.PersistHeartbeatParams{
		ReportedRuntimeState: string(w.State), ReportedRuntimeVersion: optionalText(w.Report.RuntimeVersion),
		ReportedDetails: w.Report.StoredDetails(), LastHeartbeatAt: optionalTime(w.LastHeartbeatAt), PrincipalID: uuid(w.ID),
	})
	if err != nil {
		return false, mapError(err, "storing a heartbeat")
	}
	return n > 0, nil
}

// onePresence converts the result of a single-row presence query; no rows is found false.
func onePresence(row sqlcgen.ListLivePresenceRow, err error, what string) (domain.PresenceRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PresenceRow{}, false, nil
	}
	if err != nil {
		return domain.PresenceRow{}, false, mapError(err, what)
	}
	p, err := presenceOf(row)
	return p, err == nil, err
}

// presenceOf converts a presence row; stored details outside their schema are an internal error.
func presenceOf(row sqlcgen.ListLivePresenceRow) (domain.PresenceRow, error) {
	stored, err := domain.ReportFromStore(domain.RuntimeState(row.ReportedRuntimeState), row.ReportedRuntimeVersion.String,
		domain.BillingMode(row.ReportedBillingMode.String), domain.ToolClass(row.ReportedToolClass.String), row.ReportedDetails)
	if err != nil {
		return domain.PresenceRow{}, errs.Wrap(errs.CodeInternalError, err, "agents: stored reported_details outside its schema")
	}
	return domain.PresenceRow{
		ID: idOf(row.PrincipalID), Target: domain.Target{BillingMode: domain.BillingMode(row.BillingMode), ToolClass: domain.ToolClass(row.ToolClass)},
		Stored: stored, LastHeartbeatAt: timeOf(row.LastHeartbeatAt),
	}, nil
}

// reportedOf converts a stored report into the Reported of the record.
func reportedOf(stored domain.Report, last time.Time) domain.Reported {
	rep := stored.Reported()
	rep.LastHeartbeatAt = last
	return rep
}

// optionalText converts an optional text: "" becomes NULL.
func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
