// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestIsActiveOwner (AR-04, 07 2.14): the owner role of a wake-up's person without a request of
// that person. An active owner is one without a session; a person without the role, a disabled
// owner, an agent and an unknown id are not; no audit entry and no check-log entry are written; a
// source error is an error, never "owner".
func TestIsActiveOwner(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.WithCheckLog(context.Background())
	disabledOwner := ids.New()
	w.principals.Put(authz.PrincipalFacts{ID: disabledOwner, Kind: actor.KindUser, Status: authz.PrincipalDisabled, IsOwner: true})
	for id, want := range map[ids.ID]bool{w.owner: true, w.alex: false, disabledOwner: false, w.lotse: false, ids.New(): false, {}: false} {
		got, err := w.az.IsActiveOwner(ctx, id)
		if err != nil || got != want {
			t.Errorf("IsActiveOwner(%s) = %v, %v; want %v", id, got, err, want)
		}
	}
	if n := len(w.rec.denials) + len(w.rec.records) + len(w.rec.detached); n != 0 {
		t.Errorf("%d audit entries", n)
	}
	if n := len(authorize.Checks(ctx)); n != 0 {
		t.Errorf("%d check-log entries", n)
	}
	w.principals.Fail(true)
	w.versions.Set(3)
	if got, err := w.az.IsActiveOwner(ctx, w.owner); err == nil || got {
		t.Errorf("source error: %v, %v", got, err)
	}
}
