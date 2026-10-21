// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
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
