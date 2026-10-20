// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestWorld_RevokeGrantSmoke checks the world of M2-46: Alex works on r1, Robin revokes Alex's grant
// and the session ends with access_revoked in the revoking commit.
func TestWorld_RevokeGrantSmoke(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	l := w.start(w.alex, w.r1, w.h1)
	w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", map[string]any{"reason": "terminal test"}, http.StatusOK)
	w.assertEnded(l, domain.CloseAccessRevoked)
	if !w.sameCommit(l.id, "grant.revoke") {
		t.Fatal("the end did not commit with grant.revoke")
	}
}

// TestWorld_IdleSmoke checks the fake clock of the world: without device bytes the relay ends the
// session at the instant idle timeout plus grace, not one tick before.
func TestWorld_IdleSmoke(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	l := w.start(w.alex, w.r1, w.h1)
	w.advance(idleTimeout + idleGrace - tick)
	if got := w.state(l.id.String()); got != "active" {
		t.Fatalf("one tick before the idle deadline: %s", got)
	}
	w.advance(tick)
	w.assertEnded(l, domain.CloseIdleTimeout)
}
