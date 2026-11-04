// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestHeldReason (09 5.2.1): a held run waits with invocation_held, a run without a hold has no
// reason.
func TestHeldReason(t *testing.T) {
	t.Parallel()
	if got := domain.HeldReason(time.Time{}); got != "" {
		t.Fatalf("not held: %q", got)
	}
	if got := domain.HeldReason(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)); got != domain.WaitInvocationHeld {
		t.Fatalf("held: %q", got)
	}
}

// TestWaitChanges (09 5.2.1, F-WARTESCHLANGE-GRUND): a listed queued run gets the reported reason,
// an unlisted run loses a runner reason but keeps invocation_held and an end reason it does not
// own, an unchanged reason is no change, and an unknown reported code is ignored.
func TestWaitChanges(t *testing.T) {
	t.Parallel()
	a, b, c, d, e := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	queued := []domain.QueuedReason{
		{RunID: a, Reason: ""},
		{RunID: b, Reason: domain.WaitSlotsFull},
		{RunID: c, Reason: domain.WaitThreadBusy},
		{RunID: d, Reason: domain.WaitInvocationHeld},
		{RunID: e, Reason: domain.WaitSlotsFull},
	}
	reported := map[ids.ID]string{a: domain.WaitSlotsFull, e: domain.WaitSlotsFull, d: domain.WaitThreadBusy, ids.New(): domain.WaitSlotsFull}
	got := domain.WaitChanges(queued, reported)
	want := map[ids.ID]string{a: domain.WaitSlotsFull, b: "", c: ""}
	if len(got) != len(want) {
		t.Fatalf("changes %+v, want %+v", got, want)
	}
	for _, ch := range got {
		if r, ok := want[ch.RunID]; !ok || r != ch.Reason {
			t.Fatalf("change %+v, want %+v", ch, want)
		}
	}
	if got := domain.WaitChanges([]domain.QueuedReason{{RunID: a}}, map[ids.ID]string{a: "rate_limited"}); len(got) != 0 {
		t.Fatalf("an unknown code changed %+v", got)
	}
}
