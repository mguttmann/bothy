// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestWebhookInPostRequirement (M2-27 design note 3a): webhook_in.post is All(AppKind,
// Level(write)) on the channel of the webhook. The webhook actor is the app with its installation,
// the installation's scopes cut to messages:write and no token; the action needs no scope. An app
// with write gets allow, with read only forbidden level_insufficient, without a grant not_found;
// anonymous gets unauthenticated and a person with write forbidden subject_kind_not_allowed.
func TestWebhookInPostRequirement(t *testing.T) {
	w := newWorld(t)
	w.grant(t, domain.Subject{Kind: domain.SubjectApp, PrincipalID: w.tracker},
		domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
	w.versions.Bump()
	hook := actor.Actor{ID: w.tracker, Kind: actor.KindApp, InstallationID: w.install,
		GrantedScopes: []string{domain.ScopeMessagesWrite}}
	post := authorize.ActionWebhookInPost

	w.allow(t, hook, post, w.channel(w.eskalation))
	w.expect(t, hook, post, w.channel(w.allgemein), forbidden, "level_insufficient")
	w.expect(t, hook, post, w.channel(w.kampagnen), notFound, "no_grant")
	w.expect(t, actor.Anonymous(), post, w.channel(w.eskalation), unauthentic, "unauthenticated")
	// Alex holds trigger on the space Support, so write on Eskalation.
	w.allow(t, w.user(w.alex), authorize.ActionMessageCreate, w.thread(w.eskalation))
	w.expect(t, w.user(w.alex), post, w.channel(w.eskalation), forbidden, "subject_kind_not_allowed")

	// The scopes of the installation do not matter for this action.
	noScope := hook
	noScope.GrantedScopes = nil
	w.allow(t, noScope, post, w.channel(w.eskalation))
}
