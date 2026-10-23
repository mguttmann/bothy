// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestCostReadThreadRequirement checks cost.read.thread in the world of 07 section 11.1 (brief
// M2-22 design note 16): Level(read) on the thread, so a reader of the thread and its operator see
// the costs, a principal without read gets not_found like for an unknown thread (05 4.12, R-AZ-01),
// and an app with read is refused because 05 2.6 has no scope for this action.
func TestCostReadThreadRequirement(t *testing.T) {
	w := newWorld(t)
	act := authorize.ActionCostReadThread
	allgemein := w.thread(w.allgemein)
	t.Run("read through G-4", func(t *testing.T) {
		w.allow(t, w.user(w.sam), act, allgemein)
	})
	t.Run("operator agent", func(t *testing.T) {
		w.allow(t, w.agent(w.lotse), act, allgemein)
	})
	t.Run("no grant", func(t *testing.T) {
		// Sam has no grant in Eskalation.
		w.expect(t, w.user(w.sam), act, w.thread(w.eskalation), notFound, "no_grant")
	})
	t.Run("app with G-6 only on Eskalation", func(t *testing.T) {
		w.expect(t, w.app(), act, allgemein, notFound, "no_grant")
	})
	t.Run("app with read", func(t *testing.T) {
		w.grant(t, domain.Subject{Kind: domain.SubjectApp, PrincipalID: w.tracker}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
		w.versions.Bump()
		w.expect(t, w.app(), act, allgemein, forbidden, "subject_kind_not_allowed")
	})
}
