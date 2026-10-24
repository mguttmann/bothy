// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
)

type statsReader struct{ calls int }

func (r *statsReader) MessageStats(context.Context, messaging.StatsQuery) ([]messaging.StatsValue, error) {
	r.calls++
	return nil, nil
}

func (r *statsReader) InvocationDenials(context.Context, messaging.StatsQuery) ([]messaging.StatsValue, error) {
	r.calls++
	return nil, nil
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
	bad := messaging.StatsQuery{Granularity: "day", TimeZone: "UTC", ChannelIDs: one}
	empty := messaging.StatsQuery{Granularity: "day", TimeZone: "UTC"}
	for _, read := range []func(context.Context, messaging.StatsQuery) ([]messaging.StatsValue, error){s.MessageStats, s.InvocationDenials} {
		if _, err := read(ctx, bad); err == nil {
			t.Error("lists of different lengths = nil error")
		}
		if got, err := read(ctx, empty); err != nil || got != nil {
			t.Errorf("empty channel set = %v, %v", got, err)
		}
		if _, err := read(ctx, messaging.StatsQuery{Granularity: "day", TimeZone: "UTC", ChannelIDs: one, GroupIDs: one}); err != nil {
			t.Error(err)
		}
	}
	if r.calls != 2 {
		t.Errorf("reader calls = %d, want 2", r.calls)
	}
}
