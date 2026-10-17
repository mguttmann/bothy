// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// grantIDs returns the ids of a page.
func grantIDs(p authz.GrantPage) []ids.ID {
	out := make([]ids.ID, 0, len(p.Items))
	for _, g := range p.Items {
		out = append(out, g.ID)
	}
	return out
}

// TestAdminReads_Groups (M1-48): the owner lists every live group with member_count, a member
// only his groups, a member filter on someone else gives none; GetGroup and ListGroupMembers are
// not found for a stranger; pages follow the cursor, a foreign cursor is cursor_invalid.
func TestAdminReads_Groups(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	owner := w.user(w.owner)
	var made []ids.ID
	for _, n := range []string{"A", "B", "C"} {
		g, err := w.admin.CreateGroup(ctx, owner, authz.CreateGroupCmd{Name: n})
		mustOK(t, err)
		made = append(made, g.ID)
	}
	p, err := w.admin.ListGroups(ctx, owner, authz.ListGroupsQuery{})
	mustOK(t, err)
	if len(p.Items) != 2 || p.Items[0].ID != w.leser || p.Items[0].MemberCount != 1 || p.NextCursor == "" {
		t.Fatalf("first page %+v", p)
	}
	next, err := w.admin.ListGroups(ctx, owner, authz.ListGroupsQuery{PageQuery: authz.PageQuery{Cursor: p.NextCursor, Limit: 10}})
	mustOK(t, err)
	if got := []ids.ID{next.Items[0].ID, next.Items[1].ID}; next.NextCursor != "" || !slices.Equal(got, made[1:]) {
		t.Fatalf("second page %+v", next)
	}
	_, err = w.admin.ListGroups(ctx, owner, authz.ListGroupsQuery{PageQuery: authz.PageQuery{Cursor: p.NextCursor}, MemberID: w.sam})
	wantErr(t, err, errs.CodeCursorInvalid, "")
	_, err = w.admin.ListGroups(ctx, owner, authz.ListGroupsQuery{PageQuery: authz.PageQuery{Cursor: "x"}})
	wantErr(t, err, errs.CodeCursorInvalid, "")
	sam, err := w.admin.ListGroups(ctx, w.user(w.sam), authz.ListGroupsQuery{})
	mustOK(t, err)
	if len(sam.Items) != 1 || sam.Items[0].ID != w.leser {
		t.Fatalf("Sam lists %+v", sam)
	}
	if none, err := w.admin.ListGroups(ctx, w.user(w.sam), authz.ListGroupsQuery{MemberID: w.alex}); err != nil || len(none.Items) != 0 {
		t.Fatalf("Sam with the filter on Alex: %+v, %v", none, err)
	}
	if g, err := w.admin.GetGroup(ctx, w.user(w.sam), w.leser); err != nil || g.MemberCount != 1 {
		t.Fatalf("Sam reads Leser: %+v, %v", g, err)
	}
	_, err = w.admin.GetGroup(ctx, w.user(w.robin), w.leser)
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.ListGroupMembers(ctx, w.user(w.robin), w.leser, authz.PageQuery{})
	wantErr(t, err, errs.CodeNotFound, "")
	m, err := w.admin.ListGroupMembers(ctx, owner, w.leser, authz.PageQuery{})
	mustOK(t, err)
	if len(m.Items) != 1 || m.Items[0].UserID != w.sam || m.Items[0].User.ID != w.sam || m.Items[0].User.Handle == "" {
		t.Fatalf("members %+v", m)
	}
	// At the second of the expiry the membership is over.
	w.clk.Advance(48 * time.Hour)
	m, err = w.admin.ListGroupMembers(ctx, owner, w.leser, authz.PageQuery{})
	mustOK(t, err)
	g, err := w.admin.GetGroup(ctx, owner, w.leser)
	mustOK(t, err)
	if len(m.Items) != 0 || g.MemberCount != 0 {
		t.Fatalf("after the expiry: %+v, count %d", m, g.MemberCount)
	}
}

// TestAdminReads_Grants (M1-48): the owner sees every grant and filters; Alex with manage on
// Support sees only grants there; half pairs and unknown kinds are validation_failed; GetGrant
// outside the scope is not found.
func TestAdminReads_Grants(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	owner := w.user(w.owner)
	all, err := w.admin.ListGrants(ctx, owner, authz.ListGrantsQuery{PageQuery: authz.PageQuery{Limit: 100}})
	mustOK(t, err)
	if got := grantIDs(all); !slices.Equal(got, []ids.ID{w.g1, w.g2, w.g3}) || all.NextCursor == "" {
		t.Fatalf("owner first page %v (limit capped at 3)", got)
	}
	q := authz.ListGrantsQuery{SubjectKind: "group", SubjectID: w.leser}
	if p, err := w.admin.ListGrants(ctx, owner, q); err != nil || !slices.Equal(grantIDs(p), []ids.ID{w.g4}) {
		t.Fatalf("group filter %v, %v", grantIDs(p), err)
	}
	for _, c := range []struct {
		q       authz.ListGrantsQuery
		pointer string
	}{
		{authz.ListGrantsQuery{TargetID: w.support}, "#query/target_kind"},
		{authz.ListGrantsQuery{TargetKind: "space"}, "#query/target_id"},
		{authz.ListGrantsQuery{TargetKind: "instance", TargetID: w.support}, "#query/target_id"},
		{authz.ListGrantsQuery{SubjectKind: "user"}, "#query/subject_id"},
		{authz.ListGrantsQuery{SubjectID: w.sam}, "#query/subject_kind"},
		{authz.ListGrantsQuery{TargetKind: "planet", TargetID: w.support}, "#query/target_kind"},
		{authz.ListGrantsQuery{SubjectKind: "robot", SubjectID: w.sam}, "#query/subject_kind"},
	} {
		_, err := w.admin.ListGrants(ctx, owner, c.q)
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed || e.Fields[0].Pointer != c.pointer {
			t.Errorf("%+v: %v", c.q, err)
		}
	}
	w.withManager(t, "trigger")
	alex := w.user(w.alex)
	p, err := w.admin.ListGrants(ctx, alex, authz.ListGrantsQuery{PageQuery: authz.PageQuery{Limit: 3}})
	mustOK(t, err)
	for _, g := range p.Items {
		if g.TargetKind == "space" && g.TargetID != w.support || g.TargetKind == "channel" && g.TargetID != w.allgemein && g.TargetID != w.eskalation {
			t.Errorf("Alex sees a grant outside Support: %+v", g)
		}
	}
	_, err = w.admin.ListGrants(ctx, alex, authz.ListGrantsQuery{TargetKind: "space", TargetID: w.marketing})
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.ListGrants(ctx, alex, authz.ListGrantsQuery{TargetKind: "host", TargetID: ids.New()})
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.GetGrant(ctx, alex, w.g2)
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.GetGrant(ctx, alex, ids.New())
	wantErr(t, err, errs.CodeNotFound, "")
	if g, err := w.admin.GetGrant(ctx, alex, w.g4); err != nil || g.SubjectID != w.leser || g.TargetID != w.allgemein {
		t.Fatalf("Alex reads G-4: %+v, %v", g, err)
	}
	if g, err := w.admin.GetGrant(ctx, owner, w.g2); err != nil || g.ID != w.g2 {
		t.Fatalf("owner reads G-2: %+v, %v", g, err)
	}
}
