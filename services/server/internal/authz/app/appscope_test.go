// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestSnapshot_AppWithoutActiveInstallationIsEmpty (M2-24 design note 11 b): an app whose
// installation is not active keeps its grant G-6, but its snapshot is empty, so its target is
// invisible; with the installation active again the target is back.
func TestSnapshot_AppWithoutActiveInstallationIsEmpty(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.app(), authorize.ActionMessageCreate, w.thread(w.eskalation))
	f, _, _ := w.principals.Principal(context.Background(), w.tracker)
	f.AppInstallationActive = false
	w.principals.Put(f)
	w.versions.Bump()
	w.expect(t, w.app(), authorize.ActionMessageCreate, w.thread(w.eskalation), notFound, "no_grant")
	reader := w.app()
	reader.GrantedScopes, reader.Scopes = []string{domain.ScopeMessagesRead}, []string{domain.ScopeMessagesRead}
	sc, err := w.az.Scope(context.Background(), reader, authorize.ActionMessageRead)
	if err != nil || len(sc.ChannelIDs) != 0 {
		t.Fatalf("scope of a suspended app %+v %v", sc, err)
	}
	f.AppInstallationActive = true
	w.principals.Put(f)
	w.versions.Bump()
	w.allow(t, w.app(), authorize.ActionMessageCreate, w.thread(w.eskalation))
}

// TestAZM32_TokenScopes: the matrix row AZ-M32 needs messages:write in the token too (M2-24
// design note 10 c); a token without it gets scope_missing on the visible thread.
func TestAZM32_TokenScopes(t *testing.T) {
	w := newWorld(t)
	tok := w.app()
	tok.Scopes = nil
	w.expect(t, tok, authorize.ActionMessageCreate, w.thread(w.eskalation), forbidden, "scope_missing")
	noToken := w.app()
	noToken.TokenID, noToken.Scopes = ids.ID{}, nil
	w.allow(t, noToken, authorize.ActionMessageCreate, w.thread(w.eskalation))
}

// TestAppCollection_OwnerOnly (M2-24 design note 13): app.read on the collection of apps (no id)
// answers a non-owner forbidden owner_only, an owner allow; on one app a non-owner gets
// not_found.
func TestAppCollection_OwnerOnly(t *testing.T) {
	w := newWorld(t)
	collection := authorize.Resource{Kind: authorize.ResourceApp}
	w.expect(t, w.user(w.sam), authorize.ActionAppRead, collection, forbidden, "owner_only")
	w.expect(t, w.agent(w.lotse), authorize.ActionAppRead, collection, forbidden, "owner_only")
	w.allow(t, w.user(w.owner), authorize.ActionAppRead, collection)
	one := authorize.Resource{Kind: authorize.ResourceApp, ID: w.tracker}
	w.expect(t, w.user(w.sam), authorize.ActionAppRead, one, notFound, "no_grant")
	w.allow(t, w.user(w.owner), authorize.ActionAppRead, one)
	w.expect(t, actor.Anonymous(), authorize.ActionAppRead, collection, unauthentic, "unauthenticated")
}

// TestScopeTable: the port implementation answers like the domain rules.
func TestScopeTable(t *testing.T) {
	var s authz.AppScopes = app.ScopeTable{}
	if scope, ok := s.EventScope(authorize.EventMessageCreated); !ok || scope != domain.ScopeMessagesRead {
		t.Errorf("message.created: %q %v", scope, ok)
	}
	if _, ok := s.EventScope(authorize.EventAccessChanged); ok {
		t.Error("access.changed is subscribable")
	}
	if !slices.Contains(s.Scopes(), domain.ScopeEventsSubscribe) {
		t.Errorf("scopes %v", s.Scopes())
	}
}

// TestRevokeSubjectGrants (M2-24 design note 9 d): inside a rights change every active grant of
// the app is revoked with the caller and the reason; outside one the call is an internal error.
func TestRevokeSubjectGrants(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	owner := w.user(w.owner)
	ctx = actor.WithActor(ctx, owner)
	_, err := w.admin.RevokeSubjectGrants(ctx, owner, w.tracker, "Uninstalled")
	wantErr(t, err, errs.CodeInternalError, "")
	second := w.grantRow(t, domain.Subject{Kind: domain.SubjectApp, PrincipalID: w.tracker}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
	var n int
	err = w.change(ctx, "app.uninstall", txn.Ref{Kind: "app", ID: w.tracker}, []ids.ID{w.tracker}, func(ctx context.Context) error {
		var err error
		n, err = w.admin.RevokeSubjectGrants(ctx, owner, w.tracker, "Uninstalled")
		return err
	})
	if err != nil || n != 2 {
		t.Fatalf("revoked %d, %v; want G-6 and the second grant", n, err)
	}
	for _, id := range []ids.ID{w.g6, second} {
		g, found, err := w.store.GetGrant(context.Background(), id)
		if err != nil || !found || g.RevokedAt.IsZero() || g.RevokedBy != w.owner || g.RevokeReason != "Uninstalled" {
			t.Errorf("grant %s: %+v %v", id, g, err)
		}
	}
	err = w.change(ctx, "app.uninstall", txn.Ref{Kind: "app", ID: w.tracker}, []ids.ID{w.tracker}, func(ctx context.Context) error {
		var err error
		n, err = w.admin.RevokeSubjectGrants(ctx, owner, w.tracker, "Again")
		return err
	})
	if err != nil || n != 0 {
		t.Fatalf("second revocation %d %v", n, err)
	}
	err = w.change(ctx, "app.uninstall", txn.Ref{Kind: "app", ID: w.tracker}, []ids.ID{w.tracker}, func(ctx context.Context) error {
		_, err := w.admin.RevokeSubjectGrants(ctx, owner, w.tracker, "bad\u202ereason")
		return err
	})
	wantErr(t, err, errs.CodeValidationFailed, "")
}
