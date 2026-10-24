// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

type statsReader struct {
	calls int
	last  runs.StatsQuery
}

func (r *statsReader) RunStats(_ context.Context, q runs.StatsQuery) ([]runs.StatsValue, error) {
	r.calls++
	r.last = q
	return nil, nil
}

func (r *statsReader) ToolStats(_ context.Context, q runs.StatsQuery) ([]runs.StatsValue, error) {
	r.calls++
	r.last = q
	return nil, nil
}

// TestStats: the source refuses a nil reader and a bad query, reads nothing for an empty thread
// set, groups by agent with the thread list as placeholder groups and passes valid reads on.
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
	if _, err := s.RunStats(ctx, runs.StatsQuery{Granularity: "day", TimeZone: "UTC", ThreadIDs: one}); err == nil {
		t.Error("RunStats with lists of different lengths = nil error")
	}
	if _, err := s.ToolStats(ctx, runs.StatsQuery{Granularity: "day", TimeZone: "Local"}); err == nil {
		t.Error("ToolStats with the time zone Local = nil error")
	}
	if got, err := s.RunStats(ctx, runs.StatsQuery{Granularity: "day", TimeZone: "UTC"}); err != nil || got != nil {
		t.Errorf("RunStats(empty) = %v, %v", got, err)
	}
	if got, err := s.ToolStats(ctx, runs.StatsQuery{Granularity: "day", TimeZone: "UTC"}); err != nil || got != nil {
		t.Errorf("ToolStats(empty) = %v, %v", got, err)
	}
	if r.calls != 0 {
		t.Fatalf("the reader was called %d times for empty or bad input", r.calls)
	}
	if _, err := s.RunStats(ctx, runs.StatsQuery{Granularity: "day", TimeZone: "UTC", ThreadIDs: one, ByAgent: true}); err != nil {
		t.Fatal(err)
	}
	if len(r.last.GroupIDs) != 1 || r.last.GroupIDs[0] != one[0] {
		t.Errorf("by agent: groups = %v, want the thread list", r.last.GroupIDs)
	}
	if _, err := s.ToolStats(ctx, runs.StatsQuery{Granularity: "day", TimeZone: "UTC", ThreadIDs: one}); err != nil {
		t.Fatal(err)
	}
	if r.calls != 2 {
		t.Errorf("reader calls = %d, want 2", r.calls)
	}
}
