// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestPlaceholderNeverTriggers (M8-03 design note 8, R-IM-06): a placeholder person of an import
// is a disabled principal, and authorize refuses it message.create and agent.invoke with
// principal_inactive before any right is read, even in a thread where an active person may write
// and trigger.
func TestPlaceholderNeverTriggers(t *testing.T) {
	w := newWorld(t)
	placeholder := ids.New()
	w.principals.Put(authz.PrincipalFacts{ID: placeholder, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.addSession(placeholder)
	w.versions.Bump()
	thread := w.thread(w.projektA)
	for _, act := range []authorize.Action{authorize.ActionMessageCreate, authorize.ActionAgentInvoke} {
		w.expect(t, w.user(placeholder), act, thread, authorize.OutcomeUnauthenticated, "principal_inactive")
	}
}
