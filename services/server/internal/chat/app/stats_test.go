// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

type statsReader struct{ calls int }

func (r *statsReader) StatsChannels(context.Context, []ids.ID) ([]chat.StatsChannel, error) {
	r.calls++
	return []chat.StatsChannel{{}}, nil
}

func (r *statsReader) StatsThreads(context.Context, []ids.ID, time.Time) ([]chat.StatsThread, error) {
	r.calls++
	return []chat.StatsThread{{}}, nil
}

func (r *statsReader) ThreadStats(context.Context, chat.StatsQuery) ([]chat.StatsValue, error) {
	r.calls++
	return []chat.StatsValue{{}}, nil
}

// TestStats: the source refuses a nil reader and a bad query, reads nothing for an empty channel
// set and passes every valid read to the reader.
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
	if got, err := s.StatsChannels(ctx, nil); err != nil || got != nil {
		t.Errorf("StatsChannels(empty) = %v, %v", got, err)
	}
	if got, err := s.StatsThreads(ctx, nil, time.Time{}); err != nil || got != nil {
		t.Errorf("StatsThreads(empty) = %v, %v", got, err)
	}
	if _, err := s.ThreadStats(ctx, chat.StatsQuery{Granularity: "year", TimeZone: "UTC"}); err == nil {
		t.Error("ThreadStats with an unknown granularity = nil error")
	}
	if got, err := s.ThreadStats(ctx, chat.StatsQuery{Granularity: "day", TimeZone: "UTC"}); err != nil || got != nil {
		t.Errorf("ThreadStats(empty) = %v, %v", got, err)
	}
	if r.calls != 0 {
		t.Fatalf("the reader was called %d times for empty or bad input", r.calls)
	}
	_, _ = s.StatsChannels(ctx, one)
	_, _ = s.StatsThreads(ctx, one, time.Time{})
	_, _ = s.ThreadStats(ctx, chat.StatsQuery{Granularity: "day", TimeZone: "UTC", ChannelIDs: one, GroupIDs: one})
	if r.calls != 3 {
		t.Errorf("reader calls = %d, want 3", r.calls)
	}
}
