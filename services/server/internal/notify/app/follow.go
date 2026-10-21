// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The values of messaging.MessagePosted the following reads.
const (
	originHuman     = "human"
	threadKindReply = "thread"
)

// OnMessagePosted is the automatic following of 13 3.2 (R-PN-04, OQ-05 default; brief M3-19
// design note 17, D-M3-19-14), an in-transaction subscriber of messaging.MessagePosted registered
// after those of runs and sync: a human who writes in a thread (not in the main thread of a
// channel), or creates it with a first message, follows it with level all, unless the person has
// an own setting for that thread. Only an inserted row appends one notification_pref.updated, in
// the unit of work of the message, so a veto later in the unit removes row and event. A
// different answer to OQ-05 would change only this function.
//
// Errors: an internal error before Bind; those of the insert and the appender, which veto the
// message.
func (s *Subscribers) OnMessagePosted(ctx context.Context, ev messaging.MessagePosted) error {
	if ev.Origin != originHuman || ev.ThreadKind != threadKindReply || ev.AuthorID.IsZero() {
		return nil
	}
	events, _, err := s.bound()
	if err != nil {
		return err
	}
	_, _, err = insertPreferenceIfAbsent(ctx, s.store, events, s.clock.Now(), ev.AuthorID,
		notify.Target{Kind: chat.TargetThread, ID: ev.ThreadID}, string(domain.LevelAll))
	return err
}
