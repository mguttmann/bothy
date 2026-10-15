// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the messaging module (M1-56, M1-60): the sqlc
// queries in queries/, the generated code in sqlcgen/ and Repository, which implements app.Store
// over the tables messages, mentions and message_templates (docs/spec/03-datenmodell.md section
// 4.6). Only this package runs SQL on them (03 Grundsatz 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	svc, err := app.New(cfg, app.Deps{Store: repo, ...})
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the messaging application layer.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write: only the ambient transaction of ctx.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// Message implements app.Store.
func (r *Repository) Message(ctx context.Context, id ids.ID) (domain.Message, bool, error) {
	row, err := r.read(ctx).GetMessage(ctx, uuid(id))
	return one(messageRow(row), err, "reading a message")
}

// MessageByClientID implements app.Store.
func (r *Repository) MessageByClientID(ctx context.Context, author, clientMsgID ids.ID) (domain.Message, bool, error) {
	row, err := r.read(ctx).GetMessageByClientID(ctx, sqlcgen.GetMessageByClientIDParams{
		AuthorPrincipalID: uuid(author), ClientMsgID: uuid(clientMsgID),
	})
	return one(messageRow(row), err, "reading a message by client_msg_id")
}

// clientIDConstraint is the unique index of 04 section 5.1 point 2.
const clientIDConstraint = "messages_author_client_msg_id_key"

// InsertMessage implements app.Store.
func (r *Repository) InsertMessage(ctx context.Context, m domain.Message) (domain.Message, error) {
	// The use case sets the language before every insert (14 3.1); an empty value would end in
	// a SQL error of the cast.
	if m.SearchLanguage == "" {
		return domain.Message{}, errs.New(errs.CodeInternalError, "messaging: a message without its search language")
	}
	q, err := r.write(ctx)
	if err != nil {
		return domain.Message{}, err
	}
	row, err := q.InsertMessage(ctx, sqlcgen.InsertMessageParams{
		ID: uuid(m.ID), ThreadID: uuid(m.ThreadID), ChannelID: uuid(m.ChannelID), ThreadSeq: m.ThreadSeq,
		AuthorPrincipalID: uuid(m.AuthorID), ClientMsgID: uuid(m.ClientMsgID), ClientPayloadSha256: m.PayloadSHA256,
		Origin: string(m.Origin), Kind: string(m.Kind), SystemCode: pgtype.Text{String: m.SystemCode, Valid: m.SystemCode != ""},
		ContentMd: m.ContentMD, ContentFormat: m.ContentFormat, ReplyToMessageID: optionalUUID(m.ReplyToID),
		Silent: m.Silent, CreatedAt: pgtype.Timestamptz{Time: m.CreatedAt, Valid: true}, SearchLanguage: m.SearchLanguage,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == clientIDConstraint {
		return domain.Message{}, fmt.Errorf("%w", app.ErrClientMsgIDTaken)
	}
	if err != nil {
		return domain.Message{}, mapError(err, "inserting a message")
	}
	return toMessage(messageRow(row)), nil
}

// InsertMentions implements app.Store with one INSERT over unnest (brief M1-56 D9).
func (r *Repository) InsertMentions(ctx context.Context, message, thread ids.ID, principals []ids.ID, at time.Time) error {
	if len(principals) == 0 {
		return nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	list := make([]pgtype.UUID, 0, len(principals))
	for _, p := range principals {
		list = append(list, uuid(p))
	}
	err = q.InsertMentions(ctx, sqlcgen.InsertMentionsParams{
		MessageID: uuid(message), ThreadID: uuid(thread), CreatedAt: pgtype.Timestamptz{Time: at, Valid: true}, PrincipalIds: list,
	})
	return mapError(err, "inserting mentions")
}

// Mentions implements app.Store.
func (r *Repository) Mentions(ctx context.Context, message ids.ID) ([]ids.ID, error) {
	rows, err := r.read(ctx).ListMentions(ctx, uuid(message))
	if err != nil {
		return nil, mapError(err, "reading mentions")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, p := range rows {
		out = append(out, idOf(p))
	}
	return out, nil
}

// messageRow is the column list every query of messages.sql returns: the columns of messages
// without search_language and search_vector (docs/DECISIONS.md D-M3-13-10). The row types sqlc
// generates per query have exactly these fields, so each converts to it.
type messageRow struct {
	ID                  pgtype.UUID
	ThreadID            pgtype.UUID
	ChannelID           pgtype.UUID
	ThreadSeq           int64
	AuthorPrincipalID   pgtype.UUID
	ClientMsgID         pgtype.UUID
	ClientPayloadSha256 []byte
	Origin              string
	Kind                string
	SystemCode          pgtype.Text
	ContentMd           string
	ContentFormat       int16
	ReplyToMessageID    pgtype.UUID
	RunID               pgtype.UUID
	TriggerState        pgtype.Text
	TriggerStateReason  pgtype.Text
	TriggerStateAt      pgtype.Timestamptz
	Silent              bool
	ImportItemID        pgtype.UUID
	Version             int64
	EditedAt            pgtype.Timestamptz
	DeletedAt           pgtype.Timestamptz
	DeletedBy           pgtype.UUID
	CreatedAt           pgtype.Timestamptz
}

// one converts a single-row result: no row is found == false.
func one(row messageRow, err error, what string) (domain.Message, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, false, nil
	}
	if err != nil {
		return domain.Message{}, false, mapError(err, what)
	}
	return toMessage(row), true, nil
}

// toMessage converts a row; SearchLanguage stays empty, because no read selects it.
func toMessage(row messageRow) domain.Message {
	return domain.Message{
		ID: idOf(row.ID), ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), ThreadSeq: row.ThreadSeq,
		AuthorID: idOf(row.AuthorPrincipalID), ClientMsgID: idOf(row.ClientMsgID), PayloadSHA256: row.ClientPayloadSha256,
		Origin: domain.Origin(row.Origin), Kind: domain.Kind(row.Kind), SystemCode: row.SystemCode.String, ContentMD: row.ContentMd,
		ContentFormat: row.ContentFormat, ReplyToID: idOf(row.ReplyToMessageID), Silent: row.Silent, Version: row.Version,
		EditedAt: timeOf(row.EditedAt), DeletedAt: timeOf(row.DeletedAt), DeletedBy: idOf(row.DeletedBy), CreatedAt: timeOf(row.CreatedAt),
	}
}

// mapError turns a database error of the operation what into a kernel/errs error: a kernel/errs
// error passes, anything else is internal (never SQL text or a value).
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "messaging: "+what+" failed")
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// timeOf converts a column; NULL becomes the zero time.
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
