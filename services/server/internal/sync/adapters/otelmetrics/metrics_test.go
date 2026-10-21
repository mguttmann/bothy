// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
)

// TestMetrics_AppendedAndLockWait: the adapter records the event type from the closed catalogue
// set and the lock wait in seconds without attributes.
func TestMetrics_AppendedAndLockWait(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.EventsAppended(ctx, "message.created", 3)
	m.EventSeqLockWait(ctx, 20*time.Millisecond)
	got := metricstest.Collect(t, reader)
	if n := metricstest.Sum(got["bothy.event.appended"], map[string]string{"bothy.event.type": "message.created"}); n != 3 {
		t.Errorf("bothy.event.appended message.created = %v, want 3", n)
	}
	pts := metricstest.Points(got["bothy.db.event_seq.lock_wait"])
	if len(pts) != 1 || pts[0].Count != 1 || pts[0].Sum != 0.02 || len(pts[0].Attrs) != 0 {
		t.Errorf("lock wait points %+v, want one of 0.02 s without attributes", pts)
	}
}
