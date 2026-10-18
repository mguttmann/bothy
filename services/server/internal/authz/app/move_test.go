// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestThreadMoveRequirement checks thread.move (brief M1-62, 07 4.2 "Actions with two targets"):
// manage on the thread's path and on the target channel's path, the target in the descriptor
// field TargetChannelID. An invisible target is not_found like an unknown one; read on the target
// without manage is forbidden; owners have manage everywhere except dm channels.
func TestThreadMoveRequirement(t *testing.T) {
	w := newWorld(t)
	thread := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: thread, ChannelID: w.allgemein})
	dmThread := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: dmThread, ChannelID: w.dm})
	move := func(th, target ids.ID) authorize.Resource {
		return authorize.Resource{Kind: authorize.ResourceThread, ID: th, TargetChannelID: target}
	}
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	// Alex reads all of Support (G-1, no manage) and gets manage on Allgemein only; Robin gets
	// manage on Allgemein and sees nothing else of Support; Sam gets manage on all of Support.
	w.grant(t, user(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelTrigger, time.Time{}, domain.CapManage)
	w.grant(t, user(w.robin), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelWrite, time.Time{}, domain.CapManage)
	w.grant(t, user(w.sam), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{}, domain.CapManage)
	w.versions.Bump()

	t.Run("manage on the source only", func(t *testing.T) {
		w.expect(t, w.user(w.alex), authorize.ActionThreadMove, move(thread, w.eskalation), forbidden, "capability_missing")
	})
	t.Run("target invisible", func(t *testing.T) {
		w.expect(t, w.user(w.robin), authorize.ActionThreadMove, move(thread, w.eskalation), notFound, "no_grant")
		w.expect(t, w.user(w.robin), authorize.ActionThreadMove, move(thread, ids.New()), notFound, "no_grant")
		// The nil UUID passes the request validator; it names no channel, so it is an unknown
		// target, not an internal error (red team M1-62).
		w.expect(t, w.user(w.robin), authorize.ActionThreadMove, move(thread, ids.ID{}), notFound, "no_grant")
		w.expect(t, w.user(w.owner), authorize.ActionThreadMove, move(thread, ids.ID{}), notFound, "no_grant")
	})
	t.Run("thread invisible", func(t *testing.T) {
		w.expect(t, w.user(w.sam), authorize.ActionThreadMove, move(w.mainThread[w.kampagnen], w.eskalation), forbidden, "capability_missing")
		w.expect(t, w.user(w.robin), authorize.ActionThreadMove, move(w.mainThread[w.kampagnen], w.allgemein), notFound, "no_grant")
	})
	t.Run("manage on both", func(t *testing.T) {
		w.allow(t, w.user(w.sam), authorize.ActionThreadMove, move(thread, w.eskalation))
	})
	t.Run("owner", func(t *testing.T) {
		w.allow(t, w.user(w.owner), authorize.ActionThreadMove, move(thread, w.kampagnen))
		w.expect(t, w.user(w.owner), authorize.ActionThreadMove, move(thread, w.dm), notFound, "no_grant")
	})
	t.Run("dm participant", func(t *testing.T) {
		w.expect(t, w.user(w.alex), authorize.ActionThreadMove, move(dmThread, w.allgemein), forbidden, "capability_missing")
	})
}
