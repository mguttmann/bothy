// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	stdsync "sync"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Subscribers holds the in-transaction subscribers of notify (02 section 4.3): OnChannelArchived
// and OnSpaceArchived remove the notification preferences of an archived target and of
// everything below it (R-DM-16, docs/DECISIONS.md D-M3-18-5), and OnMessagePosted is the
// automatic following (M3-19). internal/wire builds it with the
// module objects of the txn registry, before chat and the appender exist, so it opens no
// connection and gets both later through Bind.
type Subscribers struct {
	store Store
	clock clock.Clock

	mu     stdsync.RWMutex
	events eventlog.Appender
	paths  PathSource
}

// NewSubscribers returns the subscribers over the store.
func NewSubscribers(store Store) *Subscribers {
	return &Subscribers{store: store, clock: clock.System()}
}

// WithClock sets the clock of the automatic following (updated_at of an inserted row); tests
// pass a fake clock. It returns s.
func (s *Subscribers) WithClock(c clock.Clock) *Subscribers {
	if c != nil {
		s.clock = c
	}
	return s
}

// Bind sets the appender and the chat paths (start step of chat). Errors: a nil argument.
func (s *Subscribers) Bind(events eventlog.Appender, paths PathSource) error {
	if events == nil || paths == nil {
		return errors.New("notify: Subscribers.Bind needs the appender and the chat paths")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events, s.paths = events, paths
	return nil
}

// bound returns the appender and paths; an internal error before Bind.
func (s *Subscribers) bound() (eventlog.Appender, PathSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.events == nil || s.paths == nil {
		return nil, nil, errs.New(errs.CodeInternalError, "notify: the subscribers of the archive events are not bound")
	}
	return s.events, s.paths, nil
}

// OnChannelArchived runs in the transaction of channel.archive: it deletes every preference on
// the channel and its threads, of every person, and appends one notification_pref.updated with
// deleted per removed row. A veto later in the unit rolls everything back (02 section 8.3).
func (s *Subscribers) OnChannelArchived(ctx context.Context, ev chat.ChannelArchived) error {
	events, paths, err := s.bound()
	if err != nil {
		return err
	}
	channels := []ids.ID{ev.ChannelID}
	threads, err := paths.ThreadIDsOfChannels(ctx, channels)
	if err != nil {
		return err
	}
	return s.clean(ctx, events, ids.ID{}, channels, threads)
}

// OnSpaceArchived runs in the transaction of space.archive: it deletes every preference on the
// space, on every channel of it (archived ones included) and on every thread of those channels,
// with one event per removed row as OnChannelArchived.
func (s *Subscribers) OnSpaceArchived(ctx context.Context, ev chat.SpaceArchived) error {
	events, paths, err := s.bound()
	if err != nil {
		return err
	}
	channels, err := paths.ChannelIDsOfSpace(ctx, ev.SpaceID)
	if err != nil {
		return err
	}
	threads, err := paths.ThreadIDsOfChannels(ctx, channels)
	if err != nil {
		return err
	}
	return s.clean(ctx, events, ev.SpaceID, channels, threads)
}

// clean deletes the rows and appends their events in the order the store returned them
// (ascending user, target kind, target id), each with the last version plus one, audience the
// row's person and the actor of the unit of work.
func (s *Subscribers) clean(ctx context.Context, events eventlog.Appender, space ids.ID, channels, threads []ids.ID) error {
	rows, err := s.store.DeletePreferencesOfTargets(ctx, space, channels, threads)
	if err != nil {
		return err
	}
	list := make([]eventlog.Event, 0, len(rows))
	for _, r := range rows {
		p := r.Preference
		p.Version++
		e, err := prefEvent(ctx, r.UserID, p, true)
		if err != nil {
			return err
		}
		list = append(list, e)
	}
	return events.Append(ctx, list...)
}
