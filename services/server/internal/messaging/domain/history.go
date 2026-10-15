// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// Field error codes of the history query (brief M1-57 note 2).
const (
	// FieldMutuallyExclusive marks each of two or three given anchors of listThreadMessages.
	FieldMutuallyExclusive = "mutually_exclusive"
	// FieldOutOfRange marks an anchor below its minimum or at or above 2^53.
	FieldOutOfRange = "out_of_range"
)

// maxAnchor is the largest anchor: thread_seq is a public number and stays below 2^53, the
// largest integer JSON clients read exactly (06 section 2).
const maxAnchor = int64(1)<<53 - 1

// HistoryMode is the window of a history page (06 section 6 item 5). The zero value is the
// newest page.
type HistoryMode int

// The windows of listThreadMessages.
const (
	// HistoryNewest is the page without an anchor: the newest messages.
	HistoryNewest HistoryMode = iota
	// HistoryBefore is before_seq: the highest messages below the anchor.
	HistoryBefore
	// HistoryAfter is after_seq: the lowest messages above the anchor.
	HistoryAfter
	// HistoryAround is around_seq: messages on both sides of the anchor, the anchor included.
	HistoryAround
)

// ValidateHistoryRequest checks the anchors of listThreadMessages without any lookup (brief note
// 2 step 1) and returns the mode and its anchor (0 for the newest page). It needs nothing but the
// query, so it cannot leak. Errors: validation_failed with one field error per given anchor and
// code mutually_exclusive when more than one is given; otherwise with out_of_range when after_seq
// is below 0, before_seq or around_seq below 1, or any anchor at or above 2^53.
//
// Example:
//
//	mode, anchor, err := domain.ValidateHistoryRequest(params.BeforeSeq, params.AfterSeq, params.AroundSeq)
func ValidateHistoryRequest(before, after, around *int64) (HistoryMode, int64, error) {
	given := []struct {
		name string
		v    *int64
		mode HistoryMode
		min  int64
	}{
		{"before_seq", before, HistoryBefore, 1},
		{"after_seq", after, HistoryAfter, 0},
		{"around_seq", around, HistoryAround, 1},
	}
	var fields []errs.FieldError
	n := 0
	for _, g := range given {
		if g.v != nil {
			n++
			fields = append(fields, errs.FieldError{Pointer: "#query/" + g.name, Code: FieldMutuallyExclusive,
				Detail: "Give at most one of before_seq, after_seq and around_seq."})
		}
	}
	if n > 1 {
		return 0, 0, errs.New(errs.CodeValidationFailed, "at most one of before_seq, after_seq and around_seq").WithFields(fields...)
	}
	for _, g := range given {
		if g.v == nil {
			continue
		}
		if *g.v < g.min || *g.v > maxAnchor {
			return 0, 0, fieldError("#query/"+g.name, FieldOutOfRange, "The sequence number is out of range.")
		}
		return g.mode, *g.v, nil
	}
	return HistoryNewest, 0, nil
}

// PickAround splits a page of limit around an anchor (brief note 3): below rows exist below the
// anchor and above rows from the anchor on (each as far as fetched). It returns how many to take
// from each side: at most (limit - 1) / 2 below, the rest from the anchor on, and a short side
// gives its share to the other, so the page holds min(limit, below + above) rows.
func PickAround(below, above, limit int) (fromBelow, fromAbove int) {
	b0 := min(below, (limit-1)/2)
	fromAbove = min(above, limit-b0)
	fromBelow = min(below, limit-fromAbove)
	return fromBelow, fromAbove
}

// Window is the selection of a history page: how many rows of each side form the items and the
// two flags.
type Window struct {
	// Below is the number of rows taken from the descending side below the anchor (for the newest
	// page and before_seq all rows); Above from the ascending side (after_seq, around_seq).
	Below, Above int
	// HasMoreBefore reports a message below the first item; HasMoreAfter one above the last.
	HasMoreBefore, HasMoreAfter bool
}

// PageFlags selects the items of one page and its flags (brief note 4) from what one statement
// returned: below and above rows per side, each fetched with LIMIT limit + 1 so a full side shows
// that more exist, and opposite, the EXISTS of the side the mode does not read (thread_seq >= X for
// before_seq, thread_seq <= X for after_seq; false otherwise). The newest page never has more
// after; an empty thread gives both flags false.
func PageFlags(mode HistoryMode, below, above, limit int, opposite bool) Window {
	switch mode {
	case HistoryBefore:
		return Window{Below: min(below, limit), HasMoreBefore: below > limit, HasMoreAfter: opposite}
	case HistoryAfter:
		return Window{Above: min(above, limit), HasMoreBefore: opposite, HasMoreAfter: above > limit}
	case HistoryAround:
		b, a := PickAround(below, above, limit)
		return Window{Below: b, Above: a, HasMoreBefore: below > b, HasMoreAfter: above > a}
	case HistoryNewest:
	}
	return Window{Below: min(below, limit), HasMoreBefore: below > limit}
}

// CutForList returns content cut to at most maxBytes UTF-8 bytes at a character boundary for a
// list (message.list_content_max_bytes, 06 section 6 item 5) and whether it cut.
func CutForList(content string, maxBytes int) (string, bool) {
	if len(content) <= maxBytes {
		return content, false
	}
	return content[:runeStart(content, max(maxBytes, 0))], true
}
