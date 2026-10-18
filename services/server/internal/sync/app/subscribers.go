// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	stdsync "sync"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// ThreadMoveRows is the outgoing port the subscriber of chat.ThreadMoved uses (M1-62); the
// personal repository of adapters/postgres implements it over read_states and drafts.
type ThreadMoveRows interface {
	// MoveThreadReadStates sets channel_id = channel on the read states of every person for
	// thread, without a version change. It needs a unit of work.
	MoveThreadReadStates(ctx context.Context, thread, channel ids.ID) error
	// MoveThreadDrafts sets channel_id = channel on the drafts of every person for thread,
	// without a version change. It needs a unit of work.
	MoveThreadDrafts(ctx context.Context, thread, channel ids.ID) error
}

// Subscribers holds the in-transaction subscribers of sync (02 section 4.3): OnMessagePosted
// (M1-61) and OnThreadMoved (M1-62). internal/wire builds it with the module objects of the txn
// registry, before any appender exists, so it opens no connection and gets the appender later
// through Bind.
type Subscribers struct {
	rows  DraftRows
	moves ThreadMoveRows

	mu     stdsync.RWMutex
	events eventlog.Appender
}

// NewSubscribers returns the subscribers over the draft rows and the copies a thread move changes.
func NewSubscribers(rows DraftRows, moves ThreadMoveRows) *Subscribers {
	return &Subscribers{rows: rows, moves: moves}
}

// Bind sets the appender of draft.deleted (start step of the sync module). Errors: a nil
// appender.
func (s *Subscribers) Bind(events eventlog.Appender) error {
	if events == nil {
		return errors.New("sync: Subscribers.Bind needs the appender")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = events
	return nil
}

// appender returns the bound appender; nil before Bind.
func (s *Subscribers) appender() eventlog.Appender {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events
}

// OnMessagePosted runs in the transaction of a new message (brief M1-61 notes 5 and 12). It
// only clears drafts: with ClearDraft it deletes the author's channel draft of the forum for the
// first message of a new thread (CreatedThread; only a forum has channel drafts, so in a text
// channel nothing is found) and the author's draft of the thread otherwise, and appends
// draft.deleted when a row was deleted. A rollback of the message keeps the draft. It writes no
// read state and no event for mentions: clients count new mentions from message.created.
func (s *Subscribers) OnMessagePosted(ctx context.Context, ev messaging.MessagePosted) error {
	if !ev.ClearDraft || ev.AuthorKind != authorKindUser {
		return nil
	}
	events := s.appender()
	if events == nil {
		return errs.New(errs.CodeInternalError, "sync: the subscriber of MessagePosted is not bound")
	}
	thread, channel := ev.ThreadID, ids.ID{}
	if ev.CreatedThread {
		thread, channel = ids.ID{}, ev.ChannelID
	}
	d, found, err := s.rows.DeleteDraft(ctx, ev.AuthorID, thread, channel)
	if err != nil || !found {
		return err
	}
	e, err := draftDeletedEvent(ev.AuthorID, ev.SpaceID, d)
	if err != nil {
		return err
	}
	return events.Append(ctx, e)
}

// OnThreadMoved runs in the transaction of thread.move (brief M1-62 note 1f) and changes the
// copies read_states.channel_id and drafts.channel_id of the thread to the target channel, so
// the personal rows follow the thread (03 threads). Copies only: no version change and no event;
// clients learn the new channel from thread.moved. It needs no appender. Errors: internal without
// the move rows.
func (s *Subscribers) OnThreadMoved(ctx context.Context, ev chat.ThreadMoved) error {
	if s.moves == nil {
		return errs.New(errs.CodeInternalError, "sync: the subscriber of ThreadMoved has no rows")
	}
	if err := s.moves.MoveThreadReadStates(ctx, ev.ThreadID, ev.ToChannelID); err != nil {
		return err
	}
	return s.moves.MoveThreadDrafts(ctx, ev.ThreadID, ev.ToChannelID)
}

// authorKindUser is MessagePosted.AuthorKind of a person: only people have drafts.
const authorKindUser = "user"

// draftDeletedJSON is the payload DraftDeletedPayload.
type draftDeletedJSON struct {
	DraftID   string `json:"draft_id"`
	ThreadID  string `json:"thread_id,omitempty"`
	ChannelID string `json:"channel_id"`
}

// draftDeletedEvent returns draft.deleted of the deleted draft d of person in space: scope
// space, channel and, for a thread draft, thread; resource the draft without a version, because
// the catalogue (actions.yaml, resource_version false) records none for this type.
func draftDeletedEvent(person, space ids.ID, d sync.Draft) (eventlog.Event, error) {
	p, err := marshal(draftDeletedJSON{DraftID: d.ID.String(), ThreadID: optionalID(d.ThreadID), ChannelID: d.ChannelID.String()})
	if err != nil {
		return eventlog.Event{}, err
	}
	e := eventlog.Event{
		Type: string(authorize.EventDraftDeleted), SchemaVersion: payloadSchemaOne, ActorKind: eventlog.ActorUser, ActorPrincipalID: person,
		AudiencePrincipalID: person, ReceiveAction: receiveSelf, Payload: p,
		SpaceID: space, ChannelID: d.ChannelID, ThreadID: d.ThreadID, ResourceKind: resourceDraft, ResourceID: d.ID,
	}
	return e, nil
}
