// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestExpiryStore_Guards (M1-30): the selections lock rows, so they run only in a unit of work;
// the reported marks are rights writes; a reminded mark needs no rights change; a second mark of
// the same row is internal (the trigger and the selection under the lock rule it out); a limit
// below 1 is internal.
func TestExpiryStore_Guards(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.principal(t, "user", "alex")
	gr, err := domain.NewGrant(ids.New(), domain.Subject{Kind: domain.SubjectUser, PrincipalID: alex}, domain.Target{Kind: domain.TargetInstance},
		domain.LevelRead, 0, t0.Add(time.Hour), "reading", alex, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.InsertGrant(ctx, gr) }); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.DueExpiredGrants(ctx, t0.Add(2*time.Hour), 10); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("a locking read outside a unit of work: %v", err)
	}
	if err := e.plain(func(ctx context.Context) error {
		_, err := e.repo.DueExpiredGrants(ctx, t0.Add(2*time.Hour), 0)
		return err
	}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("limit 0: %v", err)
	}
	var due []domain.Grant
	if err := e.plain(func(ctx context.Context) (err error) {
		due, err = e.repo.DueExpiredGrants(ctx, t0.Add(2*time.Hour), 10)
		return err
	}); err != nil || len(due) != 1 || due[0].ID != gr.ID {
		t.Fatalf("due %v, %v", due, err)
	}
	mark := func(ctx context.Context) error {
		return e.repo.MarkGrantsExpiryReported(ctx, []ids.ID{gr.ID}, t0.Add(2*time.Hour))
	}
	wantError(t, e.plain(mark), errs.KindInternal, errs.CodeInternalError, "")
	if err := e.write(mark); err != nil {
		t.Fatal(err)
	}
	wantError(t, e.write(mark), errs.KindInternal, errs.CodeInternalError, "")
	remind := func(ctx context.Context) error { return e.repo.MarkGrantsExpiryReminded(ctx, []ids.ID{gr.ID}, t0) }
	if err := e.plain(remind); err != nil {
		t.Fatalf("a reminded mark without a rights change: %v", err)
	}
	if err := e.plain(func(ctx context.Context) (err error) {
		due, err = e.repo.DueExpiredGrants(ctx, t0.Add(2*time.Hour), 10)
		if err == nil && len(due) == 0 {
			due, err = e.repo.DueReminderGrants(ctx, t0, t0.Add(2*time.Hour), 10)
		}
		return err
	}); err != nil || len(due) != 0 {
		t.Fatalf("a marked grant was selected again: %v, %v", due, err)
	}
}

// TestExpiryStore_Memberships (M1-30): due expired memberships include deleted groups; due
// reminders of memberships and grants leave out deleted groups; marks set each marker once.
func TestExpiryStore_Memberships(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	live, gone := e.group(t, "Live", alex), e.group(t, "Gone", alex)
	member := func(g domain.Group, exp time.Time) domain.Membership {
		m, err := domain.NewMembership(ids.New(), g.ID, sam, alex, exp, t0)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.write(func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	ml, mg := member(live, t0.Add(time.Hour)), member(gone, t0.Add(time.Hour))
	gg, err := domain.NewGrant(ids.New(), domain.Subject{Kind: domain.SubjectGroup, GroupID: gone.ID}, domain.Target{Kind: domain.TargetInstance},
		domain.LevelRead, 0, t0.Add(time.Hour), "reading", alex, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.write(func(ctx context.Context) error {
		if err := e.repo.InsertGrant(ctx, gg); err != nil {
			return err
		}
		_, err := e.repo.SoftDeleteGroup(ctx, gone.ID, t0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var reminders []domain.MembershipWithGroup
	var grants []domain.Grant
	if err := e.plain(func(ctx context.Context) (err error) {
		if reminders, err = e.repo.DueReminderMemberships(ctx, t0, t0.Add(2*time.Hour), 10); err != nil {
			return err
		}
		grants, err = e.repo.DueReminderGrants(ctx, t0, t0.Add(2*time.Hour), 10)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(reminders) != 1 || reminders[0].Membership.ID != ml.ID || reminders[0].Group.ID != live.ID || len(grants) != 0 {
		t.Fatalf("reminders %+v, grants %+v", reminders, grants)
	}
	remind := func(ctx context.Context) error { return e.repo.MarkMembershipsExpiryReminded(ctx, []ids.ID{ml.ID}, t0) }
	if err := e.plain(remind); err != nil {
		t.Fatal(err)
	}
	wantError(t, e.plain(remind), errs.KindInternal, errs.CodeInternalError, "")
	var due []domain.MembershipWithGroup
	if err := e.plain(func(ctx context.Context) (err error) {
		due, err = e.repo.DueExpiredMemberships(ctx, t0.Add(2*time.Hour), 10)
		return err
	}); err != nil || len(due) != 2 || due[0].Membership.ID != ml.ID || due[1].Membership.ID != mg.ID {
		t.Fatalf("due %+v, %v", due, err)
	}
	report := func(ctx context.Context) error {
		return e.repo.MarkMembershipsExpiryReported(ctx, []ids.ID{ml.ID, mg.ID}, t0.Add(2*time.Hour))
	}
	wantError(t, e.plain(report), errs.KindInternal, errs.CodeInternalError, "")
	if err := e.write(report); err != nil {
		t.Fatal(err)
	}
	wantError(t, e.write(report), errs.KindInternal, errs.CodeInternalError, "")
	if err := e.plain(func(ctx context.Context) (err error) {
		if due, err = e.repo.DueExpiredMemberships(ctx, t0.Add(2*time.Hour), 10); err == nil && len(due) == 0 {
			due, err = e.repo.DueReminderMemberships(ctx, t0, t0.Add(2*time.Hour), 10)
		}
		return err
	}); err != nil || len(due) != 0 {
		t.Fatalf("marked memberships selected again: %+v, %v", due, err)
	}
	if _, err := e.repo.DueReminderMemberships(context.Background(), t0, t0, 1); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("a locking read outside a unit of work: %v", err)
	}
}
