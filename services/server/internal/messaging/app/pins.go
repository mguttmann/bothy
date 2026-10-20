// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Pins implements messaging.Pins over the use cases (Service.Pins).
type Pins struct{ *Service }

// Pins returns the incoming port of pinning (M1-59).
func (s *Service) Pins() Pins { return Pins{s} }

// receiveThreadRead is the receive action of pin.added and pin.removed (05 section 6).
const receiveThreadRead = "thread.read"

// Pin implements messaging.Pins (brief design notes 2, 6 and 10): the message with pin.create,
// then in one unit of work the thread row and the message row in the lock order of deleting, 410,
// an existing pin without a write, the bound of pin.max_per_thread and the insert with
// pin.added. The thread lock makes the bound exact across the messages of one thread.
func (s Pins) Pin(ctx context.Context, messageID ids.ID) error {
	a := caller(ctx)
	m, visible, err := s.loadMessage(ctx, a, authorize.ActionPinCreate, messageID)
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.d.Threads.LockForMessageChange(ctx, m.ThreadID); err != nil {
			return err
		}
		row, err := s.lockVisible(ctx, m.ID, visible)
		if err != nil {
			return err
		}
		if !row.DeletedAt.IsZero() {
			return errs.New(errs.CodeMessageDeleted, "the message was deleted")
		}
		return s.insertPin(ctx, row, a.ID, visible)
	})
}

// insertPin pins the locked live message row by the principal by inside the unit of work: an
// existing pin without a write, the bound of pin.max_per_thread, then the insert with pin.added.
func (s Pins) insertPin(ctx context.Context, row domain.Message, by ids.ID, visible []ids.ID) error {
	if _, pinned, err := s.d.Store.Pin(ctx, row.ID, visible); err != nil || pinned {
		return err
	}
	n, err := s.d.Store.CountPins(ctx, row.ThreadID, visible)
	if err != nil {
		return err
	}
	if n >= s.cfg.MaxPinsPerThread {
		return domain.PinLimitReached()
	}
	p := PinRow{MessageID: row.ID, ThreadID: row.ThreadID, PinnedBy: by, PinnedAt: s.now()}
	inserted, err := s.d.Store.InsertPin(ctx, p, visible)
	if err != nil || !inserted {
		return err
	}
	return s.emitPin(ctx, authorize.EventPinAdded, row, p)
}

// Unpin implements messaging.Pins (brief design note 11): the message with pin.delete, then the
// row lock and the delete; a deleted message or no pin is nil without an event.
func (s Pins) Unpin(ctx context.Context, messageID ids.ID) error {
	a := caller(ctx)
	m, visible, err := s.loadMessage(ctx, a, authorize.ActionPinDelete, messageID)
	if err != nil || !m.DeletedAt.IsZero() {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := s.lockVisible(ctx, m.ID, visible)
		if err != nil || !row.DeletedAt.IsZero() {
			return err
		}
		p, found, err := s.d.Store.DeletePin(ctx, row.ID, visible)
		if err != nil || !found {
			return err
		}
		return s.emitPin(ctx, authorize.EventPinRemoved, row, p)
	})
}

// pinJSON is the schema Pin.
type pinJSON struct {
	MessageID string `json:"message_id"`
	ThreadID  string `json:"thread_id"`
	PinnedBy  string `json:"pinned_by"`
	PinnedAt  string `json:"pinned_at"`
}

// emitPin appends typ with the payload PinPayload {pin} of p on the message m, receive action
// thread.read.
func (s *Service) emitPin(ctx context.Context, typ authorize.EventType, m domain.Message, p PinRow) error {
	body, err := encode(struct {
		Pin pinJSON `json:"pin"`
	}{pinJSON{MessageID: p.MessageID.String(), ThreadID: p.ThreadID.String(), PinnedBy: p.PinnedBy.String(), PinnedAt: stamp(p.PinnedAt)}})
	if err != nil {
		return err
	}
	return s.emitOnMessage(ctx, typ, m, receiveThreadRead, body)
}

// List implements messaging.Pins (brief design note 12): thread.read on the thread (a missing and
// an invisible thread take the same path), the pins of its live messages in the visible channels
// of message.read, each checked with message.read in filter mode; a refused item is dropped,
// logged and counted as filter_mismatch.
func (s Pins) List(ctx context.Context, threadID ids.ID) ([]messaging.PinView, error) {
	a := requestPrincipal(ctx)
	info, err := s.readThread(ctx, a, threadID)
	if err != nil {
		return nil, err
	}
	scope, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionMessageRead)
	if err != nil {
		return nil, err
	}
	out := []messaging.PinView{}
	if len(scope.ChannelIDs) == 0 {
		return out, nil
	}
	rows, err := s.d.Store.ThreadPins(ctx, info.ID, scope.ChannelIDs)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	for _, p := range rows {
		r := authorize.Resource{Kind: authorize.ResourceMessage, ID: p.MessageID, ThreadID: p.ThreadID, ChannelID: p.ChannelID, OwnerID: p.AuthorID}
		ok, err := s.allowed(fctx, a, authorize.ActionMessageRead, r)
		if err != nil {
			return nil, err
		}
		if !ok {
			s.filterMismatch(ctx, authorize.ActionMessageRead)
			continue
		}
		out = append(out, messaging.PinView{MessageID: p.MessageID, ThreadID: p.ThreadID, PinnedBy: p.PinnedBy, PinnedAt: p.PinnedAt.UTC().Truncate(time.Microsecond)})
	}
	return out, nil
}
