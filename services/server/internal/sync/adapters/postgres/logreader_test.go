// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
)

// TestEventBySeq: the log reader of the delivery jobs (sync.LogReader, M2-26) returns a stored
// event by its seq with every field the envelope needs, and found false for a seq outside the
// log.
func TestEventBySeq(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	ctx := context.Background()
	idList := e.mustAppend(t, 2)
	ev, found, err := e.repo.EventBySeq(ctx, 2)
	if err != nil || !found || ev.ID != idList[1] || ev.Seq != 2 || ev.ReceiveAction != "thread.read" ||
		ev.ResourceKind != "thread" || string(ev.Payload) != `{"n":1}` {
		t.Fatalf("EventBySeq(2) = %+v %v %v", ev, found, err)
	}
	if _, found, err := e.repo.EventBySeq(ctx, 3); err != nil || found {
		t.Fatalf("EventBySeq(3) found %v %v", found, err)
	}
	if m, err := e.repo.MinRetainedSeq(ctx); err != nil || m != 1 {
		t.Fatalf("MinRetainedSeq = %d %v", m, err)
	}
}
