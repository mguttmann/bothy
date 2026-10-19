// SPDX-License-Identifier: Apache-2.0

package eventlog_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestActorFor(t *testing.T) {
	id := ids.New()
	cases := []struct {
		kind   string
		id     ids.ID
		want   eventlog.ActorKind
		wantID ids.ID
	}{
		{"user", id, eventlog.ActorUser, id},
		{"agent", id, eventlog.ActorAgent, id},
		{"app", id, eventlog.ActorApp, id},
		{"host", id, "", ids.ID{}},
		{"anonymous", ids.ID{}, "", ids.ID{}},
		{"system", id, "", ids.ID{}},
		{"", id, "", ids.ID{}},
		{"user", ids.ID{}, "", ids.ID{}},
	}
	for _, c := range cases {
		kind, got := eventlog.ActorFor(c.kind, c.id)
		if kind != c.want || got != c.wantID {
			t.Errorf("ActorFor(%q, %v) = (%q, %v), want (%q, %v)", c.kind, c.id, kind, got, c.want, c.wantID)
		}
	}
}
