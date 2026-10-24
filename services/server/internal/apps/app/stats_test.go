// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/app"
)

type statsReader struct{ calls int }

func (r *statsReader) WebhookStats(context.Context, time.Time, time.Time, string, string) ([]apps.StatsValue, error) {
	r.calls++
	return nil, nil
}

// TestStats: the source refuses a nil reader and a bad query and passes a valid read on.
func TestStats(t *testing.T) {
	t.Parallel()
	if _, err := app.NewStats(nil); err == nil {
		t.Fatal("NewStats(nil) = nil error")
	}
	r := &statsReader{}
	s, err := app.NewStats(r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := s.WebhookStats(ctx, time.Time{}, time.Time{}, "year", "UTC"); err == nil {
		t.Error("WebhookStats with an unknown granularity = nil error")
	}
	if r.calls != 0 {
		t.Fatal("the reader was called for a bad query")
	}
	if _, err := s.WebhookStats(ctx, time.Time{}, time.Time{}, "week", "Europe/Berlin"); err != nil || r.calls != 1 {
		t.Errorf("WebhookStats = %v, reader calls %d, want nil and 1", err, r.calls)
	}
}
