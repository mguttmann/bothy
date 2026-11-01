// SPDX-License-Identifier: Apache-2.0

package rest

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// OptionalID converts an optional id for a response: the zero ID is an absent member.
func OptionalID(id ids.ID) *Uuid {
	if id.IsZero() {
		return nil
	}
	u := Uuid(id)
	return &u
}

// OptionalTime converts an optional time for a response in UTC with millisecond precision
// (06 section 2): the zero time is an absent member.
func OptionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Truncate(time.Millisecond)
	return &v
}

// IDs converts a list of ids for a response; the result is never nil, so an empty list is [].
func IDs(list []ids.ID) []Uuid {
	out := make([]Uuid, 0, len(list))
	for _, id := range list {
		out = append(out, Uuid(id))
	}
	return out
}

// ThreadFields are the values of a Thread document. The layout equals chat.Thread and
// sync.SnapshotThread field for field, so both convert to it and share ThreadOf (one conversion
// for getThread and the active threads of the snapshot, M2-52). Zero values are absent members.
type ThreadFields struct {
	ID, ChannelID                      ids.ID
	Kind                               string
	ParentMessageID                    ids.ID
	Title, TitleSource                 string
	State                              string
	ClosedAt                           time.Time
	ClosedBy                           ids.ID
	OperatorAgentID                    ids.ID
	BillingMode                        string
	LastMessageSeq, UnreadFloorSeq     int64
	MessageCount                       int
	CreatedBy                          ids.ID
	CreatedAt, LastActivityAt, MovedAt time.Time
	Version                            int64
	TagIDs                             []ids.ID
	DeletedAt                          time.Time
}

// ThreadOf converts a thread (schema Thread): empty values become absent members, never null
// (ADR 0053); unread_floor_seq only when not 0, billing_mode only with an operator; tag_ids always
// present.
func ThreadOf(th ThreadFields) Thread {
	out := Thread{
		Id: Uuid(th.ID), ChannelId: Uuid(th.ChannelID), Kind: ThreadKind(th.Kind),
		ParentMessageId: OptionalID(th.ParentMessageID), Title: th.Title, TitleSource: ThreadTitleSource(th.TitleSource),
		State: ThreadState(th.State), ClosedAt: OptionalTime(th.ClosedAt), ClosedBy: OptionalID(th.ClosedBy),
		OperatorAgentId: OptionalID(th.OperatorAgentID), LastMessageSeq: th.LastMessageSeq,
		LastActivityAt: th.LastActivityAt.UTC().Truncate(time.Millisecond), MessageCount: th.MessageCount,
		CreatedBy: Uuid(th.CreatedBy), CreatedAt: th.CreatedAt.UTC().Truncate(time.Millisecond), MovedAt: OptionalTime(th.MovedAt),
		Version: th.Version, TagIds: IDs(th.TagIDs), DeletedAt: OptionalTime(th.DeletedAt),
	}
	if th.UnreadFloorSeq != 0 {
		v := th.UnreadFloorSeq
		out.UnreadFloorSeq = &v
	}
	if th.BillingMode != "" && !th.OperatorAgentID.IsZero() {
		m := BillingMode(th.BillingMode)
		out.BillingMode = &m
	}
	return out
}
