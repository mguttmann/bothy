// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestThreadsCounters (M1-63): no channel answers an empty list without asking the store; the
// store's threads with something unread for somebody come back with their counters.
func TestThreadsCounters(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ch, th := ids.New(), ids.New()
	if _, err := e.store.InsertMainThread(context.Background(), domain.Thread{ID: th, ChannelID: ch, LastMessageSeq: 3, UnreadFloorSeq: 1}); err != nil {
		t.Fatal(err)
	}
	none, err := e.svc.Counters(context.Background(), nil)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("no channels: %v %v", none, err)
	}
	got, err := e.svc.Counters(context.Background(), []ids.ID{ch})
	if err != nil || len(got) != 1 || got[0].ThreadID != th || got[0].LastMessageSeq != 3 || got[0].UnreadFloorSeq != 1 {
		t.Fatalf("counters %+v %v", got, err)
	}
}
