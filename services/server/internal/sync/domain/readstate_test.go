// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// fieldOf returns the kind, pointer and code of the first field error of err.
func fieldOf(t *testing.T, err error) (errs.Code, string, string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Fatalf("not an errs error: %v", err)
	}
	if len(e.Fields) == 0 {
		return e.Code, "", ""
	}
	return e.Code, e.Fields[0].Pointer, e.Fields[0].Code
}

// TestAdvanceKeepsMaximum: the stored position is GREATEST(old, new) (04 section 14, R-SY-13).
func TestAdvanceKeepsMaximum(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		old, req, want int64
		changed        bool
	}{{0, 0, 0, false}, {0, 10, 10, true}, {10, 5, 10, false}, {10, 10, 10, false}, {10, 11, 11, true}} {
		got, changed := domain.Advance(c.old, c.req)
		if got != c.want || changed != c.changed {
			t.Errorf("Advance(%d, %d) = %d, %v; want %d, %v", c.old, c.req, got, changed, c.want, c.changed)
		}
	}
}

// TestAdvanceRejectsSeqAboveLastMessage: 0 <= last_read_seq <= threads.last_message_seq.
func TestAdvanceRejectsSeqAboveLastMessage(t *testing.T) {
	t.Parallel()
	if err := domain.CheckReadSeq(7, 7); err != nil {
		t.Fatalf("seq at the last message: %v", err)
	}
	if err := domain.CheckReadSeq(0, 0); err != nil {
		t.Fatalf("seq 0 of an empty thread: %v", err)
	}
	for _, seq := range []int64{8, -1} {
		code, ptr, fc := fieldOf(t, domain.CheckReadSeq(seq, 7))
		if code != errs.CodeValidationFailed || ptr != "/last_read_seq" || fc != domain.FieldOutOfRange {
			t.Errorf("seq %d: %s %s %s", seq, code, ptr, fc)
		}
	}
}

// TestMarkUnreadOnlyMovesBack: markThreadUnread stores a value at most the current one; equal is
// no change, larger is out_of_range (advancing is putReadState).
func TestMarkUnreadOnlyMovesBack(t *testing.T) {
	t.Parallel()
	changed, err := domain.MarkUnread(10, 4)
	if err != nil || !changed {
		t.Fatalf("back to 4: %v %v", changed, err)
	}
	changed, err = domain.MarkUnread(10, 10)
	if err != nil || changed {
		t.Fatalf("equal: %v %v", changed, err)
	}
	_, err = domain.MarkUnread(10, 11)
	if code, ptr, fc := fieldOf(t, err); code != errs.CodeValidationFailed || ptr != "/last_read_seq" || fc != domain.FieldOutOfRange {
		t.Fatalf("forward: %s %s %s", code, ptr, fc)
	}
	if _, err := domain.MarkUnread(10, -1); err == nil {
		t.Fatal("negative accepted")
	}
}
