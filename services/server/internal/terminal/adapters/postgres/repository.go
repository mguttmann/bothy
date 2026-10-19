// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the terminal module (M2-39): the sqlc queries in
// queries/, the generated code in sqlcgen/ and Repository, which implements app.Store over the
// tables hosts, host_session_users and terminal_sessions (docs/spec/03-data-model.md section
// 4.14). Only this package runs SQL on them (03 Grundsatz 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	svc, err := app.New(cfg, app.Deps{Store: repo, ...})
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the terminal application layer.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write or a lock: only the ambient transaction of ctx.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// Host implements app.Store.
func (r *Repository) Host(ctx context.Context, id ids.ID) (domain.Host, bool, error) {
	row, err := r.read(ctx).GetHost(ctx, uuid(id))
	return oneHost(row, err, "reading a host")
}

// LockHost implements app.Store.
func (r *Repository) LockHost(ctx context.Context, id ids.ID) (domain.Host, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Host{}, false, err
	}
	row, err := q.LockHost(ctx, uuid(id))
	return oneHost(row, err, "locking a host")
}

// ListHosts implements app.Store.
func (r *Repository) ListHosts(ctx context.Context, all bool, hostIDs []ids.ID, afterName string, afterID ids.ID, limit int) ([]domain.Host, error) {
	rows, err := r.read(ctx).ListHostsPage(ctx, sqlcgen.ListHostsPageParams{
		AllHosts: all, HostIds: uuids(hostIDs), AfterName: afterName, AfterID: uuid(afterID), MaxRows: maxRows(limit),
	})
	if err != nil {
		return nil, mapError(err, "listing hosts")
	}
	out := make([]domain.Host, 0, len(rows))
	for _, row := range rows {
		out = append(out, toHost(row))
	}
	return out, nil
}

// HostIDs implements app.Store.
func (r *Repository) HostIDs(ctx context.Context) ([]ids.ID, error) {
	rows, err := r.read(ctx).ListHostIDs(ctx)
	return idList(rows, err, "listing the hosts")
}

// AssignedTo implements app.Store.
func (r *Repository) AssignedTo(ctx context.Context, userID ids.ID) ([]ids.ID, error) {
	if userID.IsZero() {
		return nil, nil
	}
	rows, err := r.read(ctx).ListHostIDsAssignedTo(ctx, uuid(userID))
	return idList(rows, err, "listing the hosts of a holder")
}

// InsertHost implements app.Store; a taken live name is 409 conflict without a reason (ruling
// of M2-39, like group names).
func (r *Repository) InsertHost(ctx context.Context, h domain.Host) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertHost(ctx, sqlcgen.InsertHostParams{
		ID: uuid(h.ID), Name: h.Name, State: string(h.State), AssignedUserID: optionalUUID(h.AssignedUserID),
		PairingCodeHash: h.PairingCodeHash, PairingExpiresAt: optionalTime(h.PairingExpiresAt), CreatedBy: uuid(h.CreatedBy),
		CreatedAt: optionalTime(h.CreatedAt),
	})
	if isUnique(err, constraintHostName) {
		return errs.New(errs.CodeConflict, "A host with this name exists.")
	}
	return mapError(err, "inserting a host")
}

// UpdateHost implements app.Store.
func (r *Repository) UpdateHost(ctx context.Context, h domain.Host) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.UpdateHost(ctx, sqlcgen.UpdateHostParams{
		ID: uuid(h.ID), Name: h.Name, AssignedUserID: optionalUUID(h.AssignedUserID), State: string(h.State),
		DisabledAt: optionalTime(h.DisabledAt), DisabledReason: optionalText(h.DisabledReason), RetiredAt: optionalTime(h.RetiredAt),
		Version: h.Version,
	})
	if isUnique(err, constraintHostName) {
		return errs.New(errs.CodeConflict, "A host with this name exists.")
	}
	if err == nil && n != 1 {
		return errs.New(errs.CodeInternalError, "terminal: updating a host changed no row")
	}
	return mapError(err, "updating a host")
}

// LockSessionUsers implements app.Store.
func (r *Repository) LockSessionUsers(ctx context.Context, hostID ids.ID) ([]domain.SessionUserRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockSessionUsersOfHost(ctx, uuid(hostID))
	if err != nil {
		return nil, mapError(err, "locking the session users of a host")
	}
	return sessionUserRows(rows), nil
}

// SessionUsers implements app.Store.
func (r *Repository) SessionUsers(ctx context.Context, hostID ids.ID) ([]domain.SessionUserRow, error) {
	rows, err := r.read(ctx).SessionUsersOfHost(ctx, uuid(hostID))
	if err != nil {
		return nil, mapError(err, "reading the session users of a host")
	}
	return sessionUserRows(rows), nil
}

// sessionUserRows converts rows of host_session_users.
func sessionUserRows(rows []sqlcgen.HostSessionUser) []domain.SessionUserRow {
	out := make([]domain.SessionUserRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.SessionUserRow{
			ID: idOf(row.ID), UserID: idOf(row.UserID), OSUser: row.OsUser, CreatedAt: timeOf(row.CreatedAt), Revoked: row.RevokedAt.Valid,
		})
	}
	return out
}

// ValidSessionUsers implements app.Store.
func (r *Repository) ValidSessionUsers(ctx context.Context, hostIDs []ids.ID) (map[ids.ID][]domain.SessionUser, error) {
	out := map[ids.ID][]domain.SessionUser{}
	if len(hostIDs) == 0 {
		return out, nil
	}
	rows, err := r.read(ctx).ValidSessionUsersOfHosts(ctx, uuids(hostIDs))
	if err != nil {
		return nil, mapError(err, "reading the session users of hosts")
	}
	for _, row := range rows {
		h := idOf(row.HostID)
		out[h] = append(out[h], domain.SessionUser{UserID: idOf(row.UserID), OSUser: row.OsUser, CreatedAt: timeOf(row.CreatedAt)})
	}
	return out, nil
}

// InsertSessionUser implements app.Store; the unique indexes on the name per host and on the
// valid assignment per person are 409 conflict with reason session_user_taken (the last line
// behind domain.PlanAssignments).
func (r *Repository) InsertSessionUser(ctx context.Context, id, hostID ids.ID, a domain.SessionUserAssign, by ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertSessionUser(ctx, sqlcgen.InsertSessionUserParams{
		ID: uuid(id), HostID: uuid(hostID), UserID: uuid(a.UserID), OsUser: a.OSUser, CreatedBy: uuid(by), CreatedAt: optionalTime(at),
	})
	if isUnique(err, constraintSessionUserName) || isUnique(err, constraintSessionUserValid) {
		return errs.New(errs.CodeConflict, "The session user is taken on this host.").WithReason(domain.ReasonSessionUserTaken)
	}
	return mapError(err, "inserting a session user")
}

// RevokeSessionUser implements app.Store.
func (r *Repository) RevokeSessionUser(ctx context.Context, id, by ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.RevokeSessionUser(ctx, sqlcgen.RevokeSessionUserParams{ID: uuid(id), RevokedBy: uuid(by), RevokedAt: optionalTime(at)})
	if err == nil && n != 1 {
		return errs.New(errs.CodeInternalError, "terminal: ending a session user changed no row")
	}
	return mapError(err, "ending a session user")
}

// LockOpenSessions implements app.Store.
func (r *Repository) LockOpenSessions(ctx context.Context, hostID, userID ids.ID) ([]domain.Session, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenSessions(ctx, sqlcgen.LockOpenSessionsParams{HostID: uuid(hostID), UserID: uuid(userID)})
	if err != nil {
		return nil, mapError(err, "locking the open sessions of a host")
	}
	out := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Session{
			ID: idOf(row.ID), HostID: idOf(row.HostID), UserID: idOf(row.UserID), State: row.State, OpenedAt: timeOf(row.OpenedAt),
			BytesIn: row.BytesIn, BytesOut: row.BytesOut,
		})
	}
	return out, nil
}

// CloseSession implements app.Store.
func (r *Repository) CloseSession(ctx context.Context, id ids.ID, reason domain.CloseReason, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.CloseSession(ctx, sqlcgen.CloseSessionParams{ID: uuid(id), CloseReason: optionalText(string(reason)), ClosedAt: optionalTime(at)})
	if err == nil && n != 1 {
		return errs.New(errs.CodeInternalError, "terminal: closing a session changed no row")
	}
	return mapError(err, "closing a session")
}

// oneHost converts the result of a single-row query; no rows is found false.
func oneHost(row sqlcgen.Host, err error, what string) (domain.Host, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Host{}, false, nil
	}
	if err != nil {
		return domain.Host{}, false, mapError(err, what)
	}
	return toHost(row), true, nil
}

// toHost converts a row. The reported columns are decoded through their schemas; a value outside
// them reads as the empty list (the host service is their writer, M2-41 and M2-43).
func toHost(row sqlcgen.Host) domain.Host {
	h := domain.Host{
		ID: idOf(row.ID), Name: row.Name, State: domain.State(row.State), DisabledReason: row.DisabledReason.String,
		DisabledAt: timeOf(row.DisabledAt), RetiredAt: timeOf(row.RetiredAt), Platform: row.Platform.String,
		AssignedUserID: idOf(row.AssignedUserID), LLMCredentials: row.LlmCredentials, CredentialHolderID: idOf(row.CredentialHolderID),
		DeclaredServerHost: row.DeclaredServerHost, PairingCodeHash: row.PairingCodeHash, PairingExpiresAt: timeOf(row.PairingExpiresAt),
		NoisePublicKey: row.NoisePublicKey, AuthJKT: row.AuthJkt.String, KeyFingerprint: row.KeyFingerprint.String,
		KeyGeneration: int64(row.KeyGeneration), FingerprintFailures: int(row.FingerprintFailures),
		PairedAt: timeOf(row.PairedAt), Profiles: domain.DecodeProfiles(row.ReportedProfiles),
		ReportedUsers: domain.DecodeReportedSessionUsers(row.ReportedSessionUsers), Doctor: domain.DecodeDoctor(row.ReportedDoctor),
		OSVersion: row.ReportedOsVersion.String, HostVersion: row.ReportedHostVersion.String, LastSeenAt: timeOf(row.LastSeenAt),
		CreatedBy: idOf(row.CreatedBy), CreatedAt: timeOf(row.CreatedAt), Version: row.Version,
	}
	if row.ReportedIsServerHost.Valid {
		v := row.ReportedIsServerHost.Bool
		h.ReportedIsServerHost = &v
	}
	if row.ReportedRequireLocalConfirm.Valid {
		v := row.ReportedRequireLocalConfirm.Bool
		h.RequireLocalConfirm = &v
	}
	return h
}

// idList converts a list of ids.
func idList(rows []pgtype.UUID, err error, what string) ([]ids.ID, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out, nil
}

// The unique indexes the adapter maps to conflicts.
const (
	constraintHostName         = "hosts_name_live_key"
	constraintAuthJKT          = "hosts_auth_jkt_key"
	constraintSessionUserName  = "host_session_users_host_os_user_key"
	constraintSessionUserValid = "host_session_users_host_user_valid_key"
	sqlstateUniqueViolation    = "23505"
)

// isUnique reports a unique violation of the index name.
func isUnique(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation && pgErr.ConstraintName == name
}

// mapError turns a database error of the operation what into a kernel/errs error: a kernel/errs
// error passes; anything else is internal, never SQL text, a constraint name or a value. The
// use cases check every field before they write, so a check or key violation is a programming
// error here.
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "terminal: "+what+" failed")
}

// maxRows converts a page limit for LIMIT; the use cases cap it at api.page_size_max.
func maxRows(limit int) int32 {
	if limit < 0 {
		return 0
	}
	if limit > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(limit)
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// uuids converts ids for an array parameter; nil becomes the empty array.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(list))
	for _, id := range list {
		out = append(out, uuid(id))
	}
	return out
}

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// optionalText converts an optional text: "" becomes NULL.
func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// optionalTime converts an optional time: the zero time becomes NULL.
func optionalTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// timeOf converts a column; NULL becomes the zero time. Times are UTC (03 Grundsatz 2).
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

// UpdateLlmCredentials implements app.Store.
func (r *Repository) UpdateLlmCredentials(ctx context.Context, id ids.ID, d domain.Declaration, by ids.ID, at time.Time, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	holder := d.HolderID
	if d.Credentials != domain.LLMPersonal {
		holder = ids.ID{}
	}
	n, err := q.UpdateHostLlmCredentials(ctx, sqlcgen.UpdateHostLlmCredentialsParams{
		ID: uuid(id), LlmCredentials: d.Credentials, CredentialHolderID: optionalUUID(holder), DeclaredBy: uuid(by),
		DeclaredAt: optionalTime(at), Version: version,
	})
	if err == nil && n != 1 {
		return errs.New(errs.CodeInternalError, "terminal: declaring the LLM login changed no row")
	}
	return mapError(err, "declaring the LLM login of a host")
}

// PersonalHosts implements app.Store.
func (r *Repository) PersonalHosts(ctx context.Context) ([]domain.Host, error) {
	rows, err := r.read(ctx).ListPersonalHosts(ctx)
	if err != nil {
		return nil, mapError(err, "listing the personal hosts")
	}
	out := make([]domain.Host, 0, len(rows))
	for _, row := range rows {
		out = append(out, toHost(row))
	}
	return out, nil
}

// UpdateReportedSessionUsers implements app.Store; the report is stored in the form of the schema
// host_reported_session_users.v1 (an array of {os_user, workdirs}).
func (r *Repository) UpdateReportedSessionUsers(ctx context.Context, id ids.ID, report []domain.ReportedSessionUser, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	raw, err := encodeReport(report)
	if err != nil {
		return err
	}
	n, err := q.UpdateReportedSessionUsers(ctx, sqlcgen.UpdateReportedSessionUsersParams{ID: uuid(id), ReportedSessionUsers: raw, Version: version})
	if err == nil && n != 1 {
		return errs.New(errs.CodeInternalError, "terminal: writing the reported session users changed no row")
	}
	return mapError(err, "writing the reported session users")
}

// OwnSessionUsers implements app.Store.
func (r *Repository) OwnSessionUsers(ctx context.Context, userID ids.ID) ([]domain.OwnSessionUser, error) {
	if userID.IsZero() {
		return nil, nil
	}
	rows, err := r.read(ctx).ValidSessionUsersOfUser(ctx, uuid(userID))
	if err != nil {
		return nil, mapError(err, "reading the session users of a person")
	}
	out := make([]domain.OwnSessionUser, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.OwnSessionUser{
			ID: idOf(row.ID), HostID: idOf(row.HostID), OSUser: row.OsUser, HostReport: domain.DecodeReportedSessionUsers(row.ReportedSessionUsers),
		})
	}
	return out, nil
}

// HostByPairingHash implements app.Store.
func (r *Repository) HostByPairingHash(ctx context.Context, hash []byte) (domain.Host, bool, error) {
	row, err := r.read(ctx).HostByPairingCodeHash(ctx, hash)
	return oneHost(row, err, "reading the host of a pairing code")
}

// SetPairingCode implements app.Store.
func (r *Repository) SetPairingCode(ctx context.Context, id ids.ID, hash []byte, expires time.Time, dropKeys bool, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.SetHostPairingCode(ctx, sqlcgen.SetHostPairingCodeParams{
		ID: uuid(id), PairingCodeHash: hash, PairingExpiresAt: optionalTime(expires), DropKeys: dropKeys, Version: version,
	})
	return oneRow(n, err, "setting a pairing code")
}

// StorePairing implements app.Store; the unique index on auth_jkt is 409 conflict with reason
// host_key_in_use (the key belongs to another host, also a retired one).
func (r *Repository) StorePairing(ctx context.Context, id ids.ID, p app.PairingWrite, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	profiles := p.Report.Profiles
	if profiles == nil {
		profiles = []domain.Profile{}
	}
	rawProfiles, err := json.Marshal(profiles)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "terminal: encoding the reported profiles")
	}
	rawUsers, err := encodeReport(p.Report.SessionUsers)
	if err != nil {
		return err
	}
	n, err := q.StoreHostPairing(ctx, sqlcgen.StoreHostPairingParams{
		ID: uuid(id), NoisePublicKey: p.Keys.Noise, AuthPublicJwk: p.Keys.JWK, AuthJkt: optionalText(p.Keys.JKT),
		KeyFingerprint: optionalText(p.Fingerprint), PairedAt: optionalTime(p.PairedAt), Platform: optionalText(p.Report.Platform),
		ReportedOsVersion: optionalText(p.Report.OSVersion), ReportedHostVersion: optionalText(p.Report.HostVersion),
		ReportedProfiles: rawProfiles, ReportedSessionUsers: rawUsers, Version: version,
	})
	if isUnique(err, constraintAuthJKT) {
		return errs.New(errs.CodeConflict, "The login key belongs to another host.").WithReason(domain.ReasonHostKeyInUse)
	}
	return oneRow(n, err, "storing a pairing")
}

// SetFingerprintFailures implements app.Store.
func (r *Repository) SetFingerprintFailures(ctx context.Context, id ids.ID, n int) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	rows, err := q.SetHostFingerprintFailures(ctx, sqlcgen.SetHostFingerprintFailuresParams{ID: uuid(id), FingerprintFailures: int32(min(max(n, 0), math.MaxInt32))})
	return oneRow(rows, err, "counting a failed confirmation")
}

// ConfirmPairing implements app.Store.
func (r *Repository) ConfirmPairing(ctx context.Context, id ids.ID, declaredServerHost bool, by ids.ID, at time.Time, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.ConfirmHostPairing(ctx, sqlcgen.ConfirmHostPairingParams{
		ID: uuid(id), DeclaredServerHost: declaredServerHost, ConfirmedBy: uuid(by), ConfirmedAt: optionalTime(at), Version: version,
	})
	return oneRow(n, err, "confirming a host")
}

// oneRow maps the result of a write that must change exactly one row.
func oneRow(n int64, err error, what string) error {
	if err == nil && n != 1 {
		return errs.New(errs.CodeInternalError, "terminal: "+what+" changed no row")
	}
	return mapError(err, what)
}

// encodeReport encodes reported session users in the form of the schema
// host_reported_session_users.v1 (an array of {os_user, workdirs}, the order kept); the one
// encoding of pairing (M2-41) and of the report of M2-40 and M2-43.
func encodeReport(report []domain.ReportedSessionUser) ([]byte, error) {
	list := make([]domain.ReportedSessionUser, 0, len(report))
	for _, u := range report {
		if u.Workdirs == nil {
			u.Workdirs = []string{}
		}
		list = append(list, u)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "terminal: encoding the reported session users")
	}
	return raw, nil
}
