// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// UnreadStore is the outgoing port of the badge read of notify (M3-19 design note 16 a),
// implemented by adapters/postgres. It is a port of its own beside Store, so the in-memory stores
// of the tests that do not need it stay unchanged; Deps.Unread falls back to Store when Store
// implements it.
type UnreadStore interface {
	// UnreadCandidates returns the threads of channels with last_message_seq above
	// unread_floor_seq, with their channel, its kind and both counters, ascending by thread id.
	// Errors: kernel/errs errors without SQL text or values.
	UnreadCandidates(ctx context.Context, channels []ids.ID) ([]chat.ThreadPosition, error)
}

// UnreadCandidates implements chat.Threads (M3-19 design note 16 a): the threads of channelIDs
// that can be unread for somebody, without authorization. The caller (the badge of notify) passes
// the channels of the recipient's authz Scope and confirms every thread that counts with
// push.deliver, so this read applies no visibility of its own. No channel returns an empty list
// without a query. Errors: internal when the service has no UnreadStore (an empty answer would
// silently hide every unread thread), else the store's error.
func (s *Service) UnreadCandidates(ctx context.Context, channelIDs []ids.ID) ([]chat.ThreadPosition, error) {
	if len(channelIDs) == 0 {
		return []chat.ThreadPosition{}, nil
	}
	if s.d.Unread == nil {
		return nil, errs.New(errs.CodeInternalError, "chat: no unread store bound")
	}
	return s.d.Unread.UnreadCandidates(ctx, channelIDs)
}
