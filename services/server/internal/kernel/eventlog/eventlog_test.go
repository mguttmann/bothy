// SPDX-License-Identifier: Apache-2.0

package eventlog_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
)

// TestActorKinds holds the values of event_log.actor_kind (03, R-DM-29) and of the OpenAPI
// enum ActorKind.
func TestActorKinds(t *testing.T) {
	t.Parallel()
	got := []eventlog.ActorKind{eventlog.ActorUser, eventlog.ActorAgent, eventlog.ActorApp, eventlog.ActorSystem}
	want := []string{"user", "agent", "app", "system"}
	for i, k := range got {
		if string(k) != want[i] {
			t.Errorf("actor kind %d = %q, want %q", i, k, want[i])
		}
	}
}

// appenderFunc proves that a function type can implement Appender, the way tests of other
// modules fake it.
type appenderFunc func(ctx context.Context, events ...eventlog.Event) error

func (f appenderFunc) Append(ctx context.Context, events ...eventlog.Event) error {
	return f(ctx, events...)
}

func TestAppenderIsAnInterface(t *testing.T) {
	t.Parallel()
	var got []eventlog.Event
	var a eventlog.Appender = appenderFunc(func(_ context.Context, events ...eventlog.Event) error {
		got = append(got, events...)
		return nil
	})
	if err := a.Append(context.Background(), eventlog.Event{Type: "thread.closed"}); err != nil || len(got) != 1 {
		t.Fatalf("Append = %v, %d events", err, len(got))
	}
}
