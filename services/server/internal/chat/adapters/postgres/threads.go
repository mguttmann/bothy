// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// parentConstraint is the unique index of threads.parent_message_id: a message starts at most one
// thread (03 threads).
const parentConstraint = "threads_parent_message_id_key"

// Thread implements app.Store.
func (r *Repository) Thread(ctx context.Context, id ids.ID) (domain.Thread, bool, error) {
	row, err := r.read(ctx).GetThread(ctx, uuid(id))
	return oneThread(threadRow(row), err, "reading a thread")
}

// LockThread implements app.Store.
func (r *Repository) LockThread(ctx context.Context, id ids.ID) (domain.Thread, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, false, err
	}
	row, err := q.LockThread(ctx, uuid(id))
	return oneThread(threadRow(row), err, "locking a thread")
}

// ShareThread implements app.Store.
func (r *Repository) ShareThread(ctx context.Context, id ids.ID) (domain.Thread, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, false, err
	}
	row, err := q.ShareThread(ctx, uuid(id))
	return oneThread(row, err, "share-locking a thread")
}

// InsertThread implements app.Store: a second thread of the same parent message is 409 conflict
// on /parent_message_id.
func (r *Repository) InsertThread(ctx context.Context, th domain.Thread) (domain.Thread, error) {
	if err := searchLanguageSet(th); err != nil {
		return domain.Thread{}, err
	}
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, err
	}
	row, err := q.InsertThread(ctx, sqlcgen.InsertThreadParams{
		ID: uuid(th.ID), ChannelID: uuid(th.ChannelID), ParentMessageID: optionalUUID(th.ParentMessageID), Title: th.Title,
		TitleSource: string(th.TitleSource), OperatorAgentID: optionalUUID(th.OperatorAgentID), CreatedBy: uuid(th.CreatedBy),
		CreatedAt: pgtype.Timestamptz{Time: th.CreatedAt, Valid: true}, SearchLanguage: th.SearchLanguage,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation && pgErr.ConstraintName == parentConstraint {
		return domain.Thread{}, errs.Wrap(errs.CodeConflict, err, "the message already started a thread").
			WithFields(errs.FieldError{Pointer: "/parent_message_id", Code: "taken", Detail: "already started a thread"})
	}
	if err != nil {
		return domain.Thread{}, mapError(err, "inserting a thread")
	}
	return toThread(threadRow(row)), nil
}

// SaveThread implements app.Store.
func (r *Repository) SaveThread(ctx context.Context, th domain.Thread) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveThread(ctx, sqlcgen.SaveThreadParams{
		ID: uuid(th.ID), Title: th.Title, TitleSource: string(th.TitleSource), State: string(th.State),
		ClosedAt: optionalTime(th.ClosedAt), ClosedBy: optionalUUID(th.ClosedBy), Version: th.Version,
	})
	return mapError(err, "updating a thread")
}

// AppendToThread implements app.Store with the UPDATE ... RETURNING of 03 section 6.2.
func (r *Repository) AppendToThread(ctx context.Context, id ids.ID, reopen bool, at time.Time) (domain.Thread, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, err
	}
	row, err := q.AppendToThread(ctx, sqlcgen.AppendToThreadParams{ID: uuid(id), Reopen: reopen, Now: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		return domain.Thread{}, mapError(err, "appending to a thread")
	}
	return toThread(threadRow(row)), nil
}

// DecrementMessageCount implements app.Store.
func (r *Repository) DecrementMessageCount(ctx context.Context, id ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.DecrementMessageCount(ctx, uuid(id))
	if err != nil {
		return mapError(err, "lowering the message count of a thread")
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "chat: no thread with a message to count down")
	}
	return nil
}

// ListThreads implements app.Store.
func (r *Repository) ListThreads(ctx context.Context, tq app.ThreadQuery) ([]domain.Thread, error) {
	if tq.Limit < 1 || tq.Limit > math.MaxInt32 {
		return nil, errs.New(errs.CodeInternalError, "chat: thread page size outside its range")
	}
	state := optionalText(tq.State)
	tag := optionalUUID(tq.TagID)
	after := pgtype.Timestamptz{Time: tq.AfterAt, Valid: tq.HasAfter}
	var (
		rows []threadRow
		err  error
	)
	if tq.ByCreated {
		var created []sqlcgen.ListThreadsByCreatedRow
		created, err = r.read(ctx).ListThreadsByCreated(ctx, sqlcgen.ListThreadsByCreatedParams{
			ChannelID: uuid(tq.ChannelID), State: state, TagID: tag, HasAfter: tq.HasAfter, AfterAt: after, AfterID: uuid(tq.AfterID),
			MaxRows: int32(tq.Limit),
		})
		for _, row := range created {
			rows = append(rows, threadRow(row))
		}
	} else {
		var active []sqlcgen.ListThreadsByActivityRow
		active, err = r.read(ctx).ListThreadsByActivity(ctx, sqlcgen.ListThreadsByActivityParams{
			ChannelID: uuid(tq.ChannelID), State: state, TagID: tag, HasAfter: tq.HasAfter, AfterAt: after, AfterID: uuid(tq.AfterID),
			MaxRows: int32(tq.Limit),
		})
		for _, row := range active {
			rows = append(rows, threadRow(row))
		}
	}
	if err != nil {
		return nil, mapError(err, "listing threads")
	}
	out := make([]domain.Thread, 0, len(rows))
	for _, row := range rows {
		out = append(out, toThread(row))
	}
	return out, nil
}

// ThreadTags implements app.Store.
func (r *Repository) ThreadTags(ctx context.Context, threads []ids.ID) (map[ids.ID][]ids.ID, error) {
	rows, err := r.read(ctx).ListThreadTags(ctx, uuids(threads))
	if err != nil {
		return nil, mapError(err, "reading the thread tags")
	}
	out := make(map[ids.ID][]ids.ID, len(threads))
	for _, row := range rows {
		th := idOf(row.ThreadID)
		out[th] = append(out[th], idOf(row.TagID))
	}
	return out, nil
}

// ReplaceThreadTags implements app.Store: delete the set, then insert the new one.
func (r *Repository) ReplaceThreadTags(ctx context.Context, thread ids.ID, tags []ids.ID, by ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if err := q.DeleteThreadTags(ctx, uuid(thread)); err != nil {
		return mapError(err, "replacing thread tags")
	}
	for _, t := range tags {
		if err := q.InsertThreadTag(ctx, sqlcgen.InsertThreadTagParams{
			ThreadID: uuid(thread), TagID: uuid(t), AddedBy: uuid(by), AddedAt: pgtype.Timestamptz{Time: at, Valid: true},
		}); err != nil {
			return mapError(err, "replacing thread tags")
		}
	}
	return nil
}

// DeleteTagAssignments implements app.Store.
func (r *Repository) DeleteTagAssignments(ctx context.Context, tag ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DeleteTagAssignments(ctx, uuid(tag)), "deleting the assignments of a tag")
}

// searchLanguageSet refuses a thread without its search language: the use case sets it before
// every insert (14 3.1), and an empty value would otherwise end in a SQL error of the cast.
func searchLanguageSet(th domain.Thread) error {
	if th.SearchLanguage == "" {
		return errs.New(errs.CodeInternalError, "chat: a thread without its search language")
	}
	return nil
}

// threadRow is the column list every query of threads.sql and threads_main.sql returns: the
// columns of threads without search_language and title_search (docs/DECISIONS.md D-M3-13-10).
// The row types sqlc generates per query have exactly these fields, so each converts to it.
type threadRow struct {
	ID              pgtype.UUID
	ChannelID       pgtype.UUID
	Kind            string
	ParentMessageID pgtype.UUID
	Title           string
	TitleSource     string
	State           string
	ClosedAt        pgtype.Timestamptz
	ClosedBy        pgtype.UUID
	OperatorAgentID pgtype.UUID
	LastMessageSeq  int64
	UnreadFloorSeq  int64
	LastActivityAt  pgtype.Timestamptz
	MessageCount    int32
	CreatedBy       pgtype.UUID
	CreatedAt       pgtype.Timestamptz
	MovedAt         pgtype.Timestamptz
	Version         int64
}

// oneThread converts the result of a single-thread query; no rows is found false.
func oneThread(row threadRow, err error, what string) (domain.Thread, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Thread{}, false, nil
	}
	if err != nil {
		return domain.Thread{}, false, mapError(err, what)
	}
	return toThread(row), true, nil
}

// toThread converts a thread row; SearchLanguage stays empty, because no read selects it.
func toThread(row threadRow) domain.Thread {
	return domain.Thread{
		ID: idOf(row.ID), ChannelID: idOf(row.ChannelID), Kind: domain.ThreadKind(row.Kind), ParentMessageID: idOf(row.ParentMessageID),
		Title: row.Title, TitleSource: domain.TitleSource(row.TitleSource), State: domain.ThreadState(row.State),
		ClosedAt: timeOf(row.ClosedAt), ClosedBy: idOf(row.ClosedBy), OperatorAgentID: idOf(row.OperatorAgentID),
		LastMessageSeq: row.LastMessageSeq, UnreadFloorSeq: row.UnreadFloorSeq, MessageCount: int(row.MessageCount),
		CreatedBy: idOf(row.CreatedBy), CreatedAt: timeOf(row.CreatedAt), LastActivityAt: timeOf(row.LastActivityAt),
		MovedAt: timeOf(row.MovedAt), Version: row.Version,
	}
}
