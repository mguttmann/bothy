// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// race runs first and second concurrently so that second starts while first holds the lock of
// permission_version (BR-03, SR8): first begins and waits; second starts and waits at the lock
// (seen in pg_stat_activity); then first continues. It returns both errors.
func (w *pgWorld) race(t *testing.T, first, second func(ctx context.Context) error, declare func(fn func(ctx context.Context) error) error) (error, error) {
	t.Helper()
	locked, release := make(chan struct{}), make(chan struct{})
	errFirst, errSecond := make(chan error, 1), make(chan error, 1)
	go func() {
		errFirst <- declare(func(ctx context.Context) error {
			close(locked)
			<-release
			return first(ctx)
		})
	}()
	<-locked
	go func() { errSecond <- declare(second) }()
	conn := w.db.SuperConn(t)
	deadline := testkit.Deadline()
	for {
		var waiting int
		if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE datname = $1 AND wait_event_type = 'Lock'`,
			w.db.Name()).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the second change never waited at the lock of permission_version")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	return <-errFirst, <-errSecond
}

// TestBR03_ConcurrentChangesThatOnlyTogetherViolateI1: adding Sam to group G and granting G
// trigger on a channel the personal Assistent serves are each fine; run concurrently, exactly one
// commits and the other fails with i1_violation, in both orders (10 BR-03, 03 section 6.3).
func TestBR03_ConcurrentChangesThatOnlyTogetherViolateI1(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"member first", "grant first"} {
		t.Run(order, func(t *testing.T) {
			t.Parallel()
			w := newPGWorld(t)
			ctx := context.Background()
			g, err := w.admin.CreateGroup(ctx, w.user(w.owner), authz.CreateGroupCmd{Name: "G"})
			mustOK(t, err)
			addSam := func(ctx context.Context) error {
				m, err := domain.NewMembership(ids.New(), g.ID, w.sam, w.owner, time.Time{}, w.clk.Now())
				if err != nil {
					return err
				}
				if err := w.hook.AddAffected(ctx, w.sam); err != nil {
					return err
				}
				return w.store.InsertMembership(ctx, m)
			}
			grantG := func(ctx context.Context) error {
				gr, err := domain.NewGrant(ids.New(), domain.Subject{Kind: domain.SubjectGroup, GroupID: g.ID},
					domain.Target{Kind: domain.TargetChannel, ChannelID: w.notizen}, domain.LevelTrigger, 0, time.Time{}, "notes", w.owner, w.clk.Now())
				if err != nil {
					return err
				}
				return w.store.InsertGrant(ctx, gr)
			}
			first, second := addSam, grantG
			if order == "grant first" {
				first, second = grantG, addSam
			}
			declare := func(fn func(ctx context.Context) error) error {
				return w.declared("group.member.add", ref("group", g.ID), nil, fn)
			}
			e1, e2 := w.race(t, first, second, declare)
			if e1 != nil {
				t.Fatalf("the first change failed: %v", e1)
			}
			if e, ok := errs.As(e2); !ok || e.Code != errs.CodeI1Violation {
				t.Fatalf("the second change: %v, want i1_violation", e2)
			}
		})
	}
}

// TestSR8_ConcurrentOwnerRemovals: with two active owners, two concurrent removals end with one
// success and one last_owner, in both orders.
func TestSR8_ConcurrentOwnerRemovals(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"owner first", "robin first"} {
		t.Run(order, func(t *testing.T) {
			t.Parallel()
			w := newPGWorld(t)
			// With two owners no agent may be personal (I1); Assistent switches first.
			w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingOrganization})
			w.makeOwner(t, w.robin)
			remove := func(who ids.ID) func(ctx context.Context) error {
				return func(ctx context.Context) error {
					if err := w.hook.AddAffected(ctx, who); err != nil {
						return err
					}
					return txSQL(ctx, `UPDATE owners SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, who.String())
				}
			}
			first, second := remove(w.owner), remove(w.robin)
			if order == "robin first" {
				first, second = second, first
			}
			declare := func(fn func(ctx context.Context) error) error {
				return w.declared("owner.remove", ref("user", w.robin), nil, fn)
			}
			e1, e2 := w.race(t, first, second, declare)
			if e1 != nil {
				t.Fatalf("the first removal failed: %v", e1)
			}
			if e, ok := errs.As(e2); !ok || e.Code != errs.CodeLastOwner {
				t.Fatalf("the second removal: %v, want last_owner", e2)
			}
			if owners, _ := w.principals.ActiveOwnerIDs(context.Background()); len(owners) != 1 {
				t.Fatalf("owners %v", owners)
			}
		})
	}
}

// validatePayload validates p against AccessChangedPayload of the contract.
func validatePayload(t *testing.T, p string) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	mustOK(t, err)
	model, err := doc.BuildV3Model()
	mustOK(t, err)
	proxy, ok := model.Model.Components.Schemas.Get("AccessChangedPayload")
	if !ok {
		t.Fatal("no schema AccessChangedPayload")
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), p); !valid {
		for _, pr := range problems {
			t.Errorf("schema: %s %s", pr.Message, pr.Reason)
		}
		t.Fatalf("payload does not match AccessChangedPayload: %s", p)
	}
	if testkit.HasJSONNull([]byte(p)) {
		t.Fatalf("null in %s", p)
	}
}

// eventFor returns the one access.changed of principal among evs.
func eventFor(t *testing.T, evs []eventRow, principal ids.ID) eventRow {
	t.Helper()
	var out []eventRow
	for _, e := range evs {
		if e.Audience == principal {
			out = append(out, e)
		}
	}
	if len(out) != 1 {
		t.Fatalf("%d access.changed for %v, want 1 (of %d)", len(out), principal, len(evs))
	}
	return out[0]
}

// sortedTargets returns the expected entries (kind, id) sorted like the payload.
func sortedIDsText(list ...ids.ID) []string {
	var out []string
	for _, id := range domain.SortIDs(slices.Clone(list)) {
		out = append(out, id.String())
	}
	return out
}

// TestAccessChanged_Content (brief M1-29 test 9): revoking G-1 of Alex removes space Support and
// its channels; a level change gives changed with level and previous_level; a capability-only
// change gives changed with the new capabilities; the payload validates, carries no null and the
// version of the change; the envelope is personal (audience and resource the principal, receive
// action self, no scope, the actor of the request).
func TestAccessChanged_Content(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	seq := w.lastSeq(t)
	_, err := w.admin.RevokeGrant(ctx, w.user(w.owner), w.g1, "handover")
	mustOK(t, err)
	evs := w.events(t, seq)
	if len(evs) != 1 {
		t.Fatalf("%d events for the revocation of G-1, want 1 (Alex)", len(evs))
	}
	e := eventFor(t, evs, w.alex)
	if e.Resource != w.alex || e.Receive != "self" || e.ActorKind != "user" || e.Space != "" || e.Channel != "" {
		t.Fatalf("envelope %+v", e)
	}
	validatePayload(t, e.Payload)
	p := decodePayload(t, e.Payload)
	if p.PermissionVersion != w.version(t) || p.PermissionVersion != e.Version {
		t.Fatalf("payload version %d, counter %d, envelope %d", p.PermissionVersion, w.version(t), e.Version)
	}
	var removed []string
	for _, r := range p.Removed {
		if r.Level != "" || r.Capabilities != nil {
			t.Fatalf("removed entry with level or capabilities: %+v", r)
		}
		removed = append(removed, r.Kind+":"+r.ID)
	}
	want := []string{}
	for _, id := range sortedIDsText(w.allgemein, w.eskalation) {
		want = append(want, "channel:"+id)
	}
	want = append(want, "space:"+w.support.String())
	if !slices.Equal(removed, want) || len(p.Added) != 0 || len(p.Changed) != 0 {
		t.Fatalf("payload %+v, want removed %v", p, want)
	}

	// A level change: Sam reads Allgemein through Leser and gets write.
	seq = w.lastSeq(t)
	_, err = w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel",
		TargetID: w.allgemein, Level: "write", Reason: "helps out"})
	mustOK(t, err)
	e = eventFor(t, w.events(t, seq), w.sam)
	validatePayload(t, e.Payload)
	p = decodePayload(t, e.Payload)
	if len(p.Changed) != 1 || p.Changed[0].ID != w.allgemein.String() || p.Changed[0].Level != "write" || p.Changed[0].PreviousLevel != "read" ||
		p.Changed[0].Capabilities == nil || len(*p.Changed[0].Capabilities) != 0 || len(p.Added) != 0 || len(p.Removed) != 0 {
		t.Fatalf("level change payload %s", e.Payload)
	}

	// A capability-only change: Sam gets export on Allgemein with the same level.
	seq = w.lastSeq(t)
	_, err = w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel",
		TargetID: w.allgemein, Level: "read", Capabilities: []string{"export"}, Reason: "exports"})
	mustOK(t, err)
	e = eventFor(t, w.events(t, seq), w.sam)
	validatePayload(t, e.Payload)
	p = decodePayload(t, e.Payload)
	if len(p.Changed) != 1 || p.Changed[0].Level != "write" || p.Changed[0].PreviousLevel != "write" ||
		p.Changed[0].Capabilities == nil || !slices.Equal(*p.Changed[0].Capabilities, []string{"export"}) {
		t.Fatalf("capability change payload %s", e.Payload)
	}

	// The in-transaction event carries the same differences.
	pub := w.published()
	last := pub[len(pub)-1]
	if last.Action != authorize.ActionGrantCreate || last.PermissionVersion != w.version(t) || len(last.Changes) != 1 || last.Changes[0].PrincipalID != w.sam {
		t.Fatalf("AccessChanged %+v", last)
	}
}

// TestAccessChanged_NoEventWithoutDifference: a rights change without visible effect raises the
// version but writes no access.changed (ruling of brief M1-29).
func TestAccessChanged_NoEventWithoutDifference(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	v, seq := w.version(t), w.lastSeq(t)
	mustOK(t, w.declared("agent.billing_mode.set", ref("agent", w.texter), []ids.ID{w.texter, w.alex}, noop))
	if w.version(t) != v+1 || len(w.events(t, seq)) != 0 {
		t.Fatalf("version %d (was %d), events %d", w.version(t), v, len(w.events(t, seq)))
	}
}

// TestAccessChanged_InTransactionDeliveryAndVeto (brief M1-29 test 10): the subscriber runs in
// phase 3.1 inside the transaction, and its veto rolls back the grant, the version and the event.
func TestAccessChanged_InTransactionDeliveryAndVeto(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	veto := errors.New("veto of the test subscriber")
	w.setVeto(veto)
	before := w.snapshot(t)
	_, err := w.admin.CreateGrant(context.Background(), w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.robin,
		TargetKind: "space", TargetID: w.support, Level: "read", Reason: "reads"})
	if !errors.Is(err, veto) {
		t.Fatalf("error %v, want the veto", err)
	}
	if after := w.snapshot(t); after != before {
		t.Fatalf("the veto left something behind:\n%+v\n%+v", before, after)
	}
	if pub := w.published(); len(pub) != 1 || pub[0].Changes[0].PrincipalID != w.robin {
		t.Fatalf("published %+v", pub)
	}
}

// TestAccessChanged_TargetBased (brief M1-29 test 10a): a change that caps Allgemein at read (the
// shape of M1-53's archive) declares only the target; Alex (grant on the space), Sam (group grant
// on the channel), the owner and the operator Lotse each get a changed entry, Robin nothing.
func TestAccessChanged_TargetBased(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	g, err := w.admin.CreateGroup(ctx, w.user(w.owner), authz.CreateGroupCmd{Name: "Schreiber"})
	mustOK(t, err)
	_, err = w.admin.AddGroupMember(ctx, w.user(w.owner), g.ID, authz.AddMemberCmd{UserID: w.sam})
	mustOK(t, err)
	_, err = w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "group", SubjectID: g.ID, TargetKind: "channel",
		TargetID: w.allgemein, Level: "write", Reason: "writers"})
	mustOK(t, err)
	seq := w.lastSeq(t)
	err = w.uow.Do(ctxOf(w.user(w.owner)), func(context.Context) error {
		c, _, _ := w.structure.Channel(ctx, w.allgemein)
		c.Archived = true
		w.structure.PutChannel(c)
		return nil
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "channel.archive", Resource: ref("channel", w.allgemein),
		Targets: []txn.Ref{ref("channel", w.allgemein)}}))
	mustOK(t, err)
	evs := w.events(t, seq)
	for p, prev := range map[ids.ID]string{w.alex: "trigger", w.sam: "write", w.owner: "trigger", w.lotse: "write"} {
		e := eventFor(t, evs, p)
		validatePayload(t, e.Payload)
		pl := decodePayload(t, e.Payload)
		if len(pl.Changed) != 1 || pl.Changed[0].ID != w.allgemein.String() || pl.Changed[0].Level != "read" || pl.Changed[0].PreviousLevel != prev {
			t.Fatalf("payload of %v: %s", p, e.Payload)
		}
	}
	for _, e := range evs {
		if e.Audience == w.robin {
			t.Fatal("Robin without rights on the channel got access.changed")
		}
	}
	if len(evs) != 4 {
		t.Fatalf("%d events, want 4", len(evs))
	}
}

// TestRevocationTakesEffectInANewAuthorizer (brief M1-29 test 11): after the commit of a
// revocation an Authorizer with its own cache answers not_found for the revoked target, and the
// warm cache of the old one follows the new version.
func TestRevocationTakesEffectInANewAuthorizer(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	alex := w.user(w.alex)
	msg := authorize.Resource{Kind: authorize.ResourceChannel, ID: w.allgemein}
	if d, err := w.az.Authorize(ctx, alex, authorize.ActionChannelRead, msg); err != nil || !d.Allowed() {
		t.Fatalf("before: %+v %v", d, err)
	}
	_, err := w.admin.RevokeGrant(ctx, w.user(w.owner), w.g1, "test")
	mustOK(t, err)
	for _, az := range []authorize.Authorizer{w.newAuthorizer(t), w.az} {
		d, err := az.Authorize(ctx, alex, authorize.ActionChannelRead, msg)
		if err != nil || d.Outcome != authorize.OutcomeNotFound {
			t.Fatalf("after: %+v %v", d, err)
		}
	}
}

// TestRightsHook_FailsClosed: a unit without an action, with an action that changes no rights or
// with an unknown action fails with kind internal and leaves nothing; so does every change before
// Bind.
func TestRightsHook_FailsClosed(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	for _, action := range []string{"", "message.read", "made.up"} {
		err := w.declared(action, ref("grant", ids.New()), nil, noop)
		if errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("action %q: %v, want kind internal", action, err)
		}
	}
	if after := w.snapshot(t); after != before {
		t.Fatalf("a refused change left something behind")
	}
	unbound, err := wire.NewRightsHook(w.pool, wire.RightsSources{Structure: w.structure, Policies: w.policies}, w.clk, nil)
	mustOK(t, err)
	if err := unbound.Begin(rightsUnitCtx(context.Background())); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("an unbound hook: %v, want kind internal", err)
	}
	if err := unbound.Bind(app.RightsBindings{}); err == nil {
		t.Fatal("Bind without appender and recorder passed")
	}
}
