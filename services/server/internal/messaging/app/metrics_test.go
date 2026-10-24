// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestMessagingMetrics_CreatedAfterCommit: bothy.message.created counts a message with its origin
// and kind once its unit of work committed; a message whose unit rolls back (a vetoing
// subscriber) counts nothing.
func TestMessagingMetrics_CreatedAfterCommit(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "hello")), http.StatusCreated, "")
	w.SetVeto(true)
	r := w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "vetoed"))
	w.SetVeto(false)
	if r.StatusCode < 500 {
		t.Fatalf("the vetoed message answered %d", r.StatusCode)
	}
	m := metricstest.Find(t, w.Metrics, "bothy.message.created")
	if n := metricstest.Sum(m, nil); n != 1 {
		t.Errorf("bothy.message.created = %v, want 1 (the rolled-back message counts nothing)", n)
	}
	if n := metricstest.Sum(m, map[string]string{"bothy.message.origin": "human", "bothy.message.kind": "text"}); n != 1 {
		t.Errorf("human text = %v, want 1; points %+v", n, metricstest.Points(m))
	}
}

// TestMeasure_RollbackNotCounted is the own measurement of messaging (M6-13, 25 3.2) against the
// real unit of work: messages_created counts a committed message with its origin and channel
// after the commit; a message whose unit rolls back (a vetoing subscriber) records nothing; a
// forum thread created through createThread counts once in threads_created with its channel, and
// a forum thread whose unit rolls back records nothing.
func TestMeasure_RollbackNotCounted(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, ch, main := w.TextChannel()
	w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "hello")), http.StatusCreated, "")
	w.SetVeto(true)
	if r := w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "vetoed")); r.StatusCode < 500 {
		t.Fatalf("the vetoed message answered %d", r.StatusCode)
	}
	forum := w.Channel(w.Space("Forum"), "forum", "bugs")
	if r := w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"Vetoed","first_message":`+mw.Body(ids.New(), "first")+`}`); r.StatusCode < 500 {
		t.Fatalf("the vetoed thread answered %d", r.StatusCode)
	}
	w.SetVeto(false)
	w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"Export","first_message":`+mw.Body(ids.New(), "first")+`}`),
		http.StatusCreated, "")
	if inv := w.Measure.Invalid(); len(inv) != 0 {
		t.Fatalf("invalid measure calls %v", inv)
	}
	msgs := w.Measure.Calls(measure.MessagesCreated)
	if len(msgs) != 2 || msgs[0].Key != (measure.Key{ChannelID: ch, Dimension: "human"}) ||
		msgs[1].Key != (measure.Key{ChannelID: forum, Dimension: "human"}) {
		t.Fatalf("messages_created %+v, want the message in the text channel and the first message of the forum thread", msgs)
	}
	threads := w.Measure.Calls(measure.ThreadsCreated)
	if len(threads) != 1 || threads[0].Key != (measure.Key{ChannelID: forum}) {
		t.Fatalf("threads_created %+v, want one in the forum", threads)
	}
}
