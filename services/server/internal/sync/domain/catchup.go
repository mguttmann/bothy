// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// Reason is the reason of resync_required (docs/spec/04-sync-protocol.md "resync_required"); the
// empty Reason means catch-up is possible.
type Reason string

// The reasons of resync_required.
const (
	// ReasonNone: catch-up from the cursor is possible.
	ReasonNone Reason = ""
	// ReasonEpochChanged: the stored sync epoch is not the current one (R-SY-18, ADR 0059).
	ReasonEpochChanged Reason = "epoch_changed"
	// ReasonCursorAhead: the cursor is above the head (R-SY-04), the second net of a client
	// without a stored epoch.
	ReasonCursorAhead Reason = "cursor_ahead"
	// ReasonRetention: events after the cursor were removed by sync.event_retention.
	ReasonRetention Reason = "retention"
	// ReasonTooFarBehind: more than sync.catchup_max_events events lie between cursor and head.
	ReasonTooFarBehind Reason = "too_far_behind"
)

// ResyncReason decides whether a catch-up after the cursor after can run (brief M1-63 design
// note 2). Parameters: epoch is the epoch the client stored with its cursor (nil when it sent
// none), head the committed value of event_seq, minRetained the smallest retained seq (head + 1
// for an empty log), current the sync epoch of the instance and maxEvents the value of
// sync.catchup_max_events. The first match wins: a different epoch gives ReasonEpochChanged, a
// cursor above head ReasonCursorAhead, a cursor below minRetained - 1 ReasonRetention, more than
// maxEvents behind ReasonTooFarBehind. Without an epoch there is no epoch check; cursor_ahead
// stays the second net (R-SY-04). It returns ReasonNone when catch-up is possible.
//
// Example:
//
//	if r := domain.ResyncReason(after, epoch, head, minSeq, cur, maxEvents); r != domain.ReasonNone {
//		return resyncRequired(r)
//	}
func ResyncReason(after int64, epoch *ids.ID, head, minRetained int64, current ids.ID, maxEvents int64) Reason {
	switch {
	case epoch != nil && *epoch != current:
		return ReasonEpochChanged
	case after > head:
		return ReasonCursorAhead
	case after < minRetained-1:
		return ReasonRetention
	case head-after > maxEvents:
		return ReasonTooFarBehind
	}
	return ReasonNone
}

// NextCursor returns the cursor after one page (brief M1-63 design note 4): the seq last of the
// last row read when the query returned limit rows (rows == limit), otherwise head. Rows that
// authorize or the audience rule dropped still count as checked, so the cursor is "last checked",
// not "last delivered" (R-SY-03).
func NextCursor(head int64, limit, rows int, last int64) int64 {
	if rows >= limit && rows > 0 {
		return last
	}
	return head
}
