// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// claimMaxAge is auth.group_claim_max_age of these tests.
const claimMaxAge = 30 * 24 * time.Hour

// provider inserts an OIDC provider named name and returns its id.
func (w *pgWorld) provider(t *testing.T, name string) ids.ID {
	t.Helper()
	id := ids.New()
	w.sql(t, `INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', $2, '{}')`, id.String(), name)
	return id
}

// claimGroup creates a claim group of provider for the claim value ref, as the owner.
func (w *pgWorld) claimGroup(t *testing.T, name string, provider ids.ID, ref string) ids.ID {
	t.Helper()
	g, err := w.admin.CreateGroup(context.Background(), w.user(w.owner), authz.CreateGroupCmd{
		Name: name, Source: "oidc_claim", ProviderID: provider, ExternalRef: ref,
	})
	mustOK(t, err)
	return g.ID
}

// syncClaims runs the claim sync of a login of user at provider the way completeOidcLogin does: a
// unit without rights change first, and on ErrClaimSyncNeedsRightsChange once more declared as a
// rights change of group.member.sync.
func (w *pgWorld) syncClaims(user, provider ids.ID, values ...string) (authz.SyncClaimGroupsResult, error) {
	cmd := authz.SyncClaimGroupsCmd{UserID: user, ProviderID: provider, ClaimValues: values, Now: w.clk.Now(), MaxAge: claimMaxAge}
	var res authz.SyncClaimGroupsResult
	run := func(opts ...txn.Option) error {
		return w.uow.Do(context.Background(), func(ctx context.Context) error {
			var err error
			res, err = w.admin.SyncClaimGroups(ctx, cmd)
			return err
		}, opts...)
	}
	err := run()
	if errors.Is(err, authz.ErrClaimSyncNeedsRightsChange) {
		err = run(txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "group.member.sync", Resource: txn.Ref{Kind: "user"}}))
	}
	return res, err
}

// claimRow is one membership of a person in a group as the claim tests read it.
type claimRow struct {
	expires, removed   time.Time
	addedBy, removedBy string
}

// claimRows returns the memberships of user in group, removed ones included, ascending by id.
func (w *pgWorld) claimRows(t *testing.T, group, user ids.ID) []claimRow {
	t.Helper()
	rows, err := w.db.SuperConn(t).Query(context.Background(), `SELECT expires_at, removed_at, added_by::text, coalesce(removed_by::text, '')
		FROM group_memberships WHERE group_id = $1 AND user_id = $2 ORDER BY id`, group.String(), user.String())
	mustOK(t, err)
	defer rows.Close()
	var out []claimRow
	for rows.Next() {
		var r claimRow
		var exp, rem *time.Time
		mustOK(t, rows.Scan(&exp, &rem, &r.addedBy, &r.removedBy))
		if exp != nil {
			r.expires = exp.UTC()
		}
		if rem != nil {
			r.removed = rem.UTC()
		}
		out = append(out, r)
	}
	mustOK(t, rows.Err())
	return out
}

// TestSyncClaimGroups_AddExtendRemove: a first login adds the membership with the expiry, raises
// permission_version, writes access.changed for the person, group.changed member_added and one
// allow entry with group ids only; a later login extends it without a new row; a claim without
// the value removes it with removed_by the person.
func TestSyncClaimGroups_AddExtendRemove(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	prov := w.provider(t, "Company")
	support := w.claimGroup(t, "Support", prov, "support-staff")
	_, err := w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "group", SubjectID: support, TargetKind: "channel",
		TargetID: w.kampagnen, Level: "read", Reason: "support reads campaigns"})
	mustOK(t, err)
	v0, seq, aseq := w.version(t), w.lastSeq(t), w.auditSeq(t)
	res, err := w.syncClaims(w.robin, prov, "support-staff", "unmapped")
	mustOK(t, err)
	if len(res.Added) != 1 || res.Added[0] != support || len(res.Removed)+len(res.Extended) != 0 {
		t.Fatalf("result %+v", res)
	}
	if v := w.version(t); v != v0+1 {
		t.Fatalf("permission_version %d, want %d", v, v0+1)
	}
	rows := w.claimRows(t, support, w.robin)
	want := w.clk.Now().Add(claimMaxAge)
	if len(rows) != 1 || !rows[0].removed.IsZero() || rows[0].addedBy != w.robin.String() || !rows[0].expires.Equal(want) {
		t.Fatalf("rows %+v, want expires_at %s", rows, want)
	}
	if evs := w.events(t, seq); len(evs) != 1 || evs[0].Audience != w.robin || evs[0].ActorKind != "user" {
		t.Fatalf("access.changed %+v", evs)
	}
	if evs := w.eventsOf(t, "group.changed", seq); len(evs) != 1 || !jsonHas(t, evs[0].Payload, "change", "member_added") {
		t.Fatalf("group.changed %+v", evs)
	}
	allows := w.allowsOf(t, "group.member.sync", aseq)
	if len(allows) != 1 || allows[0].Actor != w.robin || allows[0].ResourceKind != "user" || allows[0].ResourceID != w.robin {
		t.Fatalf("allow entries %+v", allows)
	}
	var details map[string]any
	mustOK(t, json.Unmarshal([]byte(allows[0].Details), &details))
	if details["provider_id"] != prov.String() || len(details["added"].([]any)) != 1 {
		t.Fatalf("details %v", details)
	}
	if strings.Contains(allows[0].Details, "support-staff") || strings.Contains(allows[0].Details, "unmapped") {
		t.Fatalf("a claim value reached the audit details: %s", allows[0].Details)
	}

	// A later login moves the expiry and clears the reminder marker; no second row.
	w.sql(t, `UPDATE group_memberships SET expiry_reminded_at = now() WHERE group_id = $1`, support.String())
	w.clk.Advance(48 * time.Hour)
	res, err = w.syncClaims(w.robin, prov, "support-staff")
	mustOK(t, err)
	if len(res.Extended) != 1 || len(res.Added)+len(res.Removed) != 0 {
		t.Fatalf("result %+v", res)
	}
	rows = w.claimRows(t, support, w.robin)
	if len(rows) != 1 || !rows[0].expires.Equal(w.clk.Now().Add(claimMaxAge)) {
		t.Fatalf("rows after extension %+v", rows)
	}
	if n := w.count(t, `SELECT count(*) FROM group_memberships WHERE group_id = $1 AND expiry_reminded_at IS NOT NULL`, support.String()); n != 0 {
		t.Fatalf("expiry_reminded_at not cleared")
	}

	// The value is gone: the membership is removed by the person.
	seq = w.lastSeq(t)
	res, err = w.syncClaims(w.robin, prov)
	mustOK(t, err)
	if len(res.Removed) != 1 || res.Removed[0] != support {
		t.Fatalf("result %+v", res)
	}
	rows = w.claimRows(t, support, w.robin)
	if len(rows) != 1 || rows[0].removed.IsZero() || rows[0].removedBy != w.robin.String() {
		t.Fatalf("rows after removal %+v", rows)
	}
	if evs := w.eventsOf(t, "group.changed", seq); len(evs) != 1 || !jsonHas(t, evs[0].Payload, "change", "member_removed") {
		t.Fatalf("group.changed %+v", evs)
	}

	// Nothing to change: nothing written, the version stays, and no second attempt is needed.
	before := w.snapshot(t)
	err = w.uow.Do(ctx, func(ctx context.Context) error {
		_, err := w.admin.SyncClaimGroups(ctx, authz.SyncClaimGroupsCmd{UserID: w.robin, ProviderID: prov, Now: w.clk.Now(), MaxAge: claimMaxAge})
		return err
	})
	mustOK(t, err)
	if after := w.snapshot(t); after != before {
		t.Fatalf("a sync without change wrote something:\nbefore %+v\nafter  %+v", before, after)
	}
}

// TestSyncClaimGroups_NeedsRightsChange: outside a unit the sync is an internal error, and in a
// unit without rights change a change writes nothing and asks for the rights change.
func TestSyncClaimGroups_NeedsRightsChange(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	prov := w.provider(t, "Company")
	w.claimGroup(t, "Support", prov, "support")
	cmd := authz.SyncClaimGroupsCmd{UserID: w.sam, ProviderID: prov, ClaimValues: []string{"support"}, Now: w.clk.Now(), MaxAge: claimMaxAge}
	if _, err := w.admin.SyncClaimGroups(context.Background(), cmd); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("outside a unit: %v", err)
	}
	before := w.snapshot(t)
	err := w.uow.Do(context.Background(), func(ctx context.Context) error {
		_, err := w.admin.SyncClaimGroups(ctx, cmd)
		return err
	})
	if !errors.Is(err, authz.ErrClaimSyncNeedsRightsChange) {
		t.Fatalf("error %v", err)
	}
	if after := w.snapshot(t); after != before {
		t.Fatalf("the plan wrote something")
	}
	err = w.change(context.Background(), "group.member.add", ref("group", ids.New()), nil, func(ctx context.Context) error {
		_, err := w.admin.SyncClaimGroups(ctx, cmd)
		return err
	})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("in a rights change of another action: %v", err)
	}
}

// TestGroupClaimSync_OtherProviderUntouched: two providers with groups of the same external_ref;
// a login through provider A changes only A's groups (R-DM-33).
func TestGroupClaimSync_OtherProviderUntouched(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	a, b := w.provider(t, "Company A"), w.provider(t, "Company B")
	ga := w.claimGroup(t, "Support A", a, "support")
	gb := w.claimGroup(t, "Support B", b, "support")
	_, err := w.syncClaims(w.sam, b, "support")
	mustOK(t, err)
	res, err := w.syncClaims(w.sam, a, "support")
	mustOK(t, err)
	if len(res.Added) != 1 || res.Added[0] != ga {
		t.Fatalf("result %+v", res)
	}
	// A claim of A without the value removes A's membership only.
	_, err = w.syncClaims(w.sam, a)
	mustOK(t, err)
	if rows := w.claimRows(t, gb, w.sam); len(rows) != 1 || !rows[0].removed.IsZero() {
		t.Fatalf("B's membership changed: %v", rows)
	}
	if rows := w.claimRows(t, ga, w.sam); len(rows) != 1 || rows[0].removed.IsZero() {
		t.Fatalf("A's membership not removed: %v", rows)
	}
}

// TestGroupClaimSync_ConcurrentLoginsSerialise: parallel syncs of the same person serialise on
// permission_version and end with one membership row.
func TestGroupClaimSync_ConcurrentLoginsSerialise(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	prov := w.provider(t, "Company")
	g := w.claimGroup(t, "Support", prov, "support")
	const n = 6
	errc := make(chan error, n)
	for range n {
		go func() {
			_, err := w.syncClaims(w.alex, prov, "support")
			errc <- err
		}()
	}
	for range n {
		mustOK(t, <-errc)
	}
	if rows := w.claimRows(t, g, w.alex); len(rows) != 1 || !rows[0].removed.IsZero() {
		t.Fatalf("rows %v", rows)
	}
}

// TestGroupClaimSync_ExpiryWithoutJob: with the clock at the expiry of a claim membership and no
// job running, Robin loses read access through the claim group (AZ-M09 with a claim group).
func TestGroupClaimSync_ExpiryWithoutJob(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	prov := w.provider(t, "Company")
	g := w.claimGroup(t, "Marketing Readers", prov, "marketing")
	_, err := w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "group", SubjectID: g, TargetKind: "channel",
		TargetID: w.kampagnen, Level: "read", Reason: "campaigns"})
	mustOK(t, err)
	_, err = w.syncClaims(w.robin, prov, "marketing")
	mustOK(t, err)
	expires := w.clk.Now().Add(claimMaxAge)
	v := w.version(t)
	check := func(at time.Time, allowed bool) {
		t.Helper()
		w.clk.Set(at)
		d, err := w.az.Authorize(ctx, w.user(w.robin), authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.kampagnen})
		mustOK(t, err)
		if allowed != d.Allowed() {
			t.Fatalf("at %v: %+v, want allowed=%v", at, d, allowed)
		}
	}
	check(expires.Add(-time.Microsecond), true)
	check(expires, false)
	if w.version(t) != v {
		t.Fatal("an expiry without job changed the rights version")
	}
}

// count returns the integer result of query as the super user.
func (w *pgWorld) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	mustOK(t, w.db.SuperConn(t).QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

// jsonHas reports whether the JSON object payload has member key with the string value.
func jsonHas(t *testing.T, payload, key, value string) bool {
	t.Helper()
	var m map[string]any
	mustOK(t, json.Unmarshal([]byte(payload), &m))
	return m[key] == value
}
