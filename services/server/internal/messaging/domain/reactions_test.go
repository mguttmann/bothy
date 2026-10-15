// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// TestReactionSummaryOrder (brief M1-59 design note 7): the summary is ordered by the earliest
// reaction of each symbol, then by symbol; the caller's symbols follow the same order.
func TestReactionSummaryOrder(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	groups := []domain.ReactionGroup{
		{Emoji: "b", Count: 1, FirstAt: at.Add(time.Second)},
		{Emoji: "z", Count: 2, FirstAt: at},
		{Emoji: "a", Count: 3, FirstAt: at.Add(time.Second)},
		{Emoji: "c", Count: 1, FirstAt: at.Add(-time.Second)},
	}
	got := domain.OrderSummary(groups)
	want := []string{"c", "z", "a", "b"}
	var order []string
	for _, g := range got {
		order = append(order, g.Emoji)
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	mine := domain.OwnInOrder(got, map[string]bool{"b": true, "c": true, "x": true})
	if !reflect.DeepEqual(mine, []string{"c", "b"}) {
		t.Fatalf("own %v", mine)
	}
	if out := domain.OwnInOrder(nil, nil); out == nil || len(out) != 0 {
		t.Fatalf("own of nothing %v", out)
	}
}

// TestReactionErrors: the 400 of a symbol names the pointer of its origin and invalid_emoji; the
// limits name limit_reached.
func TestReactionErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err           error
		pointer, code string
	}{
		{domain.InvalidEmoji(domain.PointerPathEmoji), "#path/emoji", "invalid_emoji"},
		{domain.InvalidEmoji(domain.PointerQueryEmoji), "#query/emoji", "invalid_emoji"},
		{domain.ReactionLimitReached(), "#path/emoji", "limit_reached"},
		{domain.PinLimitReached(), "", "limit_reached"},
	} {
		e, ok := errs.As(c.err)
		if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != c.pointer || e.Fields[0].Code != c.code {
			t.Errorf("error %+v, want %s %s", c.err, c.pointer, c.code)
		}
	}
}
