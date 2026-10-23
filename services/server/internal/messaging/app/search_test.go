// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestHitMessages_ListRepresentation (brief M3-14 design note 9): HitMessages renders each message
// exactly as History (listThreadMessages) lists it, content cut at
// message.list_content_max_bytes and reactions included, in the order of the input; an unknown
// id, a deleted message and a message outside the channels are absent, a repeated id appears
// once, and empty inputs give an empty list.
func TestHitMessages_ListRepresentation(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe, func(c *messagingapp.Config) { c.ListContentMaxBytes = 16 })
	space, ch, main := w.TextChannel()
	other := w.Channel(space, "text", "billing")
	otherMain := w.MainThread(other)
	id := func(m map[string]any) ids.ID { return ids.MustParse(m["id"].(string)) }
	short := id(w.Send("owner", main, "the invoice", http.StatusCreated))
	long := id(w.Send("owner", main, "a long invoice text that the list cuts", http.StatusCreated))
	gone := id(w.Send("owner", main, "a deleted invoice", http.StatusCreated))
	elsewhere := id(w.Send("owner", otherMain, "an invoice in billing", http.StatusCreated))
	w.Tombstone(gone)
	w.SQL(`INSERT INTO reactions (message_id, principal_id, emoji) VALUES ($1, $2, 'ok')`, short.String(), w.OwnerID.String())

	ctx := actor.WithActor(context.Background(), w.Actor("owner"))
	listed := map[ids.ID]messaging.Message{}
	for _, thread := range []ids.ID{main, otherMain} {
		page, err := w.Svc.History(ctx, messaging.HistoryQuery{ThreadID: thread})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range page.Items {
			listed[m.ID] = m
		}
	}
	if !listed[long].ContentTruncated || len(listed[short].Reactions) != 1 || len(listed[short].MyReactions) != 1 {
		t.Fatalf("the world does not exercise the list representation: %+v %+v", listed[long], listed[short])
	}

	got, err := w.Svc.HitMessages(ctx, []ids.ID{long, ids.New(), gone, elsewhere, short, long}, []ids.ID{ch})
	if want := []messaging.Message{listed[long], listed[short]}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("HitMessages = %+v, %v\nwant %+v", got, err, want)
	}
	got, err = w.Svc.HitMessages(ctx, []ids.ID{elsewhere, short}, []ids.ID{other, ch})
	if want := []messaging.Message{listed[elsewhere], listed[short]}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("HitMessages over two channels = %+v, %v\nwant %+v", got, err, want)
	}
	for _, c := range []struct{ messages, channels []ids.ID }{{nil, []ids.ID{ch}}, {[]ids.ID{short}, nil}, {[]ids.ID{gone}, []ids.ID{ch}}} {
		if got, err := w.Svc.HitMessages(ctx, c.messages, c.channels); err != nil || got == nil || len(got) != 0 {
			t.Errorf("HitMessages(%v, %v) = %v, %v; want an empty list", c.messages, c.channels, got, err)
		}
	}
}
