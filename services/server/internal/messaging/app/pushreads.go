// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// PushReadStore is the part of Store that answers the reads of the consumer push of notify (M3-19
// design notes 5 and 9 d). Its reads carry no visible channel predicate: the consumer has no
// principal and reads like the log tail, and notify checks every hint with push.deliver (07 7.4
// rule 1). found is false when no row matches; errors are kernel/errs errors.
type PushReadStore interface {
	// LastAnchorBefore returns the newest run anchor (kind run) of thread with thread_seq below
	// before.
	LastAnchorBefore(ctx context.Context, thread ids.ID, before int64) (a messaging.AnchorRef, found bool, err error)
	// AnchorOf returns the anchor of run.
	AnchorOf(ctx context.Context, run ids.ID) (a messaging.AnchorRef, found bool, err error)
	// LastAgentMessageBefore returns the newest message of agent in thread with after <
	// thread_seq < before, origin agent, kind text, voice or card, not deleted.
	LastAgentMessageBefore(ctx context.Context, thread, agent ids.ID, after, before int64) (m messaging.MessageRef, found bool, err error)
	// Newest returns the newest message of thread that is not deleted and neither an anchor nor a
	// system line.
	Newest(ctx context.Context, thread ids.ID) (m messaging.MessageRef, found bool, err error)
}

// LastAnchorBefore implements messaging.Messages (M3-19 design note 5 a) without authorization.
func (s *Service) LastAnchorBefore(ctx context.Context, threadID ids.ID, beforeThreadSeq int64) (messaging.AnchorRef, bool, error) {
	return s.d.Store.LastAnchorBefore(ctx, threadID, beforeThreadSeq)
}

// AnchorOf implements messaging.Messages (M3-19 design note 5 b) without authorization.
func (s *Service) AnchorOf(ctx context.Context, runID ids.ID) (messaging.AnchorRef, bool, error) {
	return s.d.Store.AnchorOf(ctx, runID)
}

// LastAgentMessageBefore implements messaging.Messages (M3-19 design note 5 b) without
// authorization.
func (s *Service) LastAgentMessageBefore(ctx context.Context, threadID, agentID ids.ID, afterThreadSeq, beforeThreadSeq int64) (messaging.MessageRef, bool, error) {
	return s.d.Store.LastAgentMessageBefore(ctx, threadID, agentID, afterThreadSeq, beforeThreadSeq)
}

// Newest implements messaging.Messages (M3-19 design note 9 d) without authorization.
func (s *Service) Newest(ctx context.Context, threadID ids.ID) (messaging.MessageRef, bool, error) {
	return s.d.Store.Newest(ctx, threadID)
}

// ForPreview implements messaging.Messages (M3-22 design note 9) without authorization: the row of
// the message and, unless it is a tombstone, its mentions rows.
func (s *Service) ForPreview(ctx context.Context, messageID ids.ID) (messaging.PreviewMessage, bool, error) {
	m, found, err := s.d.Store.Message(ctx, messageID)
	if err != nil || !found {
		return messaging.PreviewMessage{}, false, err
	}
	out := messaging.PreviewMessage{
		ID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, AuthorID: m.AuthorID, Kind: string(m.Kind),
		ContentMD: m.ContentMD, ContentFormat: int(m.ContentFormat), HasAttachments: m.HasAttachments, Deleted: !m.DeletedAt.IsZero(),
	}
	if out.Deleted {
		out.ContentMD, out.HasAttachments = "", false
		return out, true, nil
	}
	if out.MentionIDs, err = s.d.Store.Mentions(ctx, messageID); err != nil {
		return messaging.PreviewMessage{}, false, err
	}
	return out, true, nil
}
