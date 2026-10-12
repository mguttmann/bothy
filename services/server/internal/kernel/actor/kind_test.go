// SPDX-License-Identifier: Apache-2.0

package actor_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

func TestKindValues(t *testing.T) {
	t.Parallel()
	want := map[actor.Kind]string{
		actor.KindUser: "user", actor.KindAgent: "agent", actor.KindApp: "app",
		actor.KindAnonymous: "anonymous", actor.KindHost: "host",
	}
	for k, s := range want {
		if string(k) != s {
			t.Errorf("kind %q, want %q", k, s)
		}
		if !k.Valid() {
			t.Errorf("%q.Valid() = false", k)
		}
	}
	for _, k := range []actor.Kind{"", "User", "bot"} {
		if k.Valid() {
			t.Errorf("%q.Valid() = true", k)
		}
	}
}

func TestKindIsPrincipal(t *testing.T) {
	t.Parallel()
	for k, want := range map[actor.Kind]bool{
		actor.KindUser: true, actor.KindAgent: true, actor.KindApp: true,
		actor.KindAnonymous: false, actor.KindHost: false, "x": false,
	} {
		if got := k.IsPrincipal(); got != want {
			t.Errorf("%q.IsPrincipal() = %v, want %v", k, got, want)
		}
	}
}
