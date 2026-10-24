// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

type statsReader struct{ calls int }

func (r *statsReader) PushStats(context.Context, []time.Time, []time.Time) ([]notify.StatsValue, error) {
	r.calls++
	return nil, nil
}

// TestStats: the source refuses a nil reader and bucket lists of different lengths, reads nothing
// for no bucket and passes a valid read on.
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
	ctx, at := context.Background(), []time.Time{time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := s.PushStats(ctx, at, nil); err == nil {
		t.Error("PushStats with lists of different lengths = nil error")
	}
	if got, err := s.PushStats(ctx, nil, nil); err != nil || got != nil {
		t.Errorf("PushStats(empty) = %v, %v", got, err)
	}
	if r.calls != 0 {
		t.Fatal("the reader was called for empty or bad input")
	}
	if _, err := s.PushStats(ctx, at, at); err != nil || r.calls != 1 {
		t.Errorf("PushStats = %v, reader calls %d, want nil and 1", err, r.calls)
	}
}
