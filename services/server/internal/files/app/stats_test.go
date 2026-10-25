// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

type statsReader struct{ calls int }

func (r *statsReader) StorageBytes(context.Context, []ids.ID, []ids.ID) ([]files.StatsValue, error) {
	r.calls++
	return nil, nil
}

func (r *statsReader) ScanStats(context.Context, time.Time, time.Time, string, string) ([]files.StatsValue, error) {
	r.calls++
	return nil, nil
}

// TestStats: the source refuses a nil reader and bad input, reads nothing for an empty channel set
// and passes every valid read to the reader.
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
	ctx, one := context.Background(), []ids.ID{ids.New()}
	if _, err := s.StorageBytes(ctx, one, nil); err == nil {
		t.Error("StorageBytes with lists of different lengths = nil error")
	}
	if got, err := s.StorageBytes(ctx, nil, nil); err != nil || got != nil {
		t.Errorf("StorageBytes(empty) = %v, %v", got, err)
	}
	if _, err := s.ScanStats(ctx, time.Time{}, time.Time{}, "day", "Nowhere/City"); err == nil {
		t.Error("ScanStats with an unknown time zone = nil error")
	}
	if r.calls != 0 {
		t.Fatalf("the reader was called %d times for empty or bad input", r.calls)
	}
	_, _ = s.StorageBytes(ctx, one, one)
	_, _ = s.ScanStats(ctx, time.Time{}, time.Time{}, "day", "UTC")
	if r.calls != 2 {
		t.Errorf("reader calls = %d, want 2", r.calls)
	}
}
