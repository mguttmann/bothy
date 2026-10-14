// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var errFault = errors.New("injected fault")

// faults names the one call that fails; empty means none.
type faults struct{ fail string }

func (f *faults) hit(name string) error {
	if f.fail == name {
		return errFault
	}
	return nil
}

type faultyExpiry struct {
	app.ExpiryStore
	f *faults
}

func (s faultyExpiry) DueReminderGrants(ctx context.Context, t, until time.Time, limit int) ([]domain.Grant, error) {
	if err := s.f.hit("DueReminderGrants"); err != nil {
		return nil, err
	}
	return s.ExpiryStore.DueReminderGrants(ctx, t, until, limit)
}

func (s faultyExpiry) MarkGrantsExpiryReminded(ctx context.Context, grantIDs []ids.ID, at time.Time) error {
	if err := s.f.hit("MarkGrantsExpiryReminded"); err != nil {
		return err
	}
	return s.ExpiryStore.MarkGrantsExpiryReminded(ctx, grantIDs, at)
}

func (s faultyExpiry) DueReminderMemberships(ctx context.Context, t, until time.Time, limit int) ([]domain.MembershipWithGroup, error) {
	if err := s.f.hit("DueReminderMemberships"); err != nil {
		return nil, err
	}
	return s.ExpiryStore.DueReminderMemberships(ctx, t, until, limit)
}

func (s faultyExpiry) MarkMembershipsExpiryReminded(ctx context.Context, membershipIDs []ids.ID, at time.Time) error {
	if err := s.f.hit("MarkMembershipsExpiryReminded"); err != nil {
		return err
	}
	return s.ExpiryStore.MarkMembershipsExpiryReminded(ctx, membershipIDs, at)
}

type faultyPrincipals struct {
	authz.PrincipalSource
	f *faults
}

func (s faultyPrincipals) Principal(ctx context.Context, id ids.ID) (authz.PrincipalFacts, bool, error) {
	if err := s.f.hit("Principal"); err != nil {
		return authz.PrincipalFacts{}, false, err
	}
	return s.PrincipalSource.Principal(ctx, id)
}

func (s faultyPrincipals) ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error) {
	if err := s.f.hit("ActiveOwnerIDs"); err != nil {
		return nil, err
	}
	return s.PrincipalSource.ActiveOwnerIDs(ctx)
}

type faultyEvents struct {
	eventlog.Appender
	f *faults
}

func (s faultyEvents) Append(ctx context.Context, events ...eventlog.Event) error {
	if err := s.f.hit("Append"); err != nil {
		return err
	}
	return s.Appender.Append(ctx, events...)
}

func TestNewExpiryService_RejectsMissingDepsAndBadConfig(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	deps := app.ExpiryDeps{UoW: w.uow, Expiry: w.store, Rights: w.hook, Principals: w.principals, Events: w.appender, Clock: w.clk}
	if _, err := app.NewExpiryService(app.ExpiryConfig{BatchMax: 1, ReminderLead: time.Hour}, app.ExpiryDeps{}); err == nil {
		t.Error("missing dependencies accepted")
	}
	if _, err := app.NewExpiryService(app.ExpiryConfig{BatchMax: 0, ReminderLead: time.Hour}, deps); err == nil {
		t.Error("batch 0 accepted")
	}
	if _, err := app.NewExpiryService(app.ExpiryConfig{BatchMax: 1}, deps); err == nil {
		t.Error("reminder lead 0 accepted")
	}
	if _, err := app.NewExpiryService(app.ExpiryConfig{BatchMax: 1, ReminderLead: time.Hour}, deps); err != nil {
		t.Errorf("nil Log refused: %v", err)
	}
}

// TestExpiryReminder_FailuresKeepNothing: every failing store, source or appender call ends the
// run with an error and leaves its batch unmarked, so the next run reminds the rows.
func TestExpiryReminder_FailuresKeepNothing(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(72 * time.Hour)
	// A group grant (owners read) and Sam's membership in Leser are due for a reminder at now.
	w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: w.leser}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.marketing},
		domain.LevelRead, exp)
	w.clk.Set(exp.Add(-reminderLead + time.Second))
	f := &faults{}
	svc, err := app.NewExpiryService(app.ExpiryConfig{BatchMax: 500, ReminderLead: reminderLead}, app.ExpiryDeps{
		UoW: w.uow, Expiry: faultyExpiry{w.store, f}, Rights: w.hook, Principals: faultyPrincipals{w.principals, f},
		Events: faultyEvents{w.appender, f}, Clock: w.clk, Log: slog.New(slog.DiscardHandler),
	})
	mustOK(t, err)
	// The grant step fails first; after the clean grant step the membership step fails.
	for _, name := range []string{
		"DueReminderGrants", "ActiveOwnerIDs", "Principal", "Append", "MarkGrantsExpiryReminded",
		"DueReminderMemberships", "Principal", "Append", "MarkMembershipsExpiryReminded",
	} {
		f.fail = name
		if _, err := svc.Remind(context.Background()); !errors.Is(err, errFault) {
			t.Fatalf("Remind with %s failing: err %v, want the fault", name, err)
		}
	}
	f.fail = ""
	res, err := svc.Remind(context.Background())
	mustOK(t, err)
	if res.Grants != 0 || res.Memberships != 1 {
		t.Fatalf("final run %+v, want only the membership left", res)
	}
}
