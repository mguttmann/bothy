// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestPeopleQueries covers the queries of M1-31: directory pages, the locked person, profile
// updates, enabling and the owner rows.
func TestPeopleQueries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	anna := e.mustAddUser(t, "anna", "anna@example.com")
	ben := e.mustAddUser(t, "ben", "")
	e.mustAddUser(t, "carl", "")
	page, err := e.repo.ListUsers(ctx, app.UserFilter{Limit: 2})
	if err != nil || len(page) != 2 || page[0].User.Principal.ID != anna.Principal.ID {
		t.Fatalf("ListUsers = %+v, %v", page, err)
	}
	next, err := e.repo.ListUsers(ctx, app.UserFilter{Limit: 5, AfterHandle: "ben", AfterID: ben.Principal.ID})
	if err != nil || len(next) != 1 || next[0].User.Principal.Handle.String() != "carl" {
		t.Fatalf("ListUsers after ben = %+v, %v", next, err)
	}
	if q, err := e.repo.ListUsers(ctx, app.UserFilter{Q: "B", Status: domain.StatusActive, Limit: 5}); err != nil || len(q) != 1 {
		t.Fatalf("ListUsers q=B = %+v, %v", q, err)
	}
	if _, _, err := e.repo.LockUser(ctx, anna.Principal.ID); err == nil {
		t.Fatal("LockUser outside a unit of work")
	}
	err = e.write(t, func(ctx context.Context) error {
		rec, found, err := e.repo.LockUser(ctx, anna.Principal.ID)
		if err != nil || !found {
			t.Fatalf("LockUser = %v, %v", found, err)
		}
		rec.User.Principal.DisplayName = domain.DisplayNameFromStore("Anna A")
		rec.User.Email, rec.User.Locale, rec.User.TimeZone = "", "fr", "UTC"
		if err := e.repo.UpdateUser(ctx, rec.User); err != nil {
			return err
		}
		if _, found, err := e.repo.LockUser(ctx, ids.New()); err != nil || found {
			t.Fatalf("LockUser unknown = %v, %v", found, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec, _, _ := e.repo.User(ctx, anna.Principal.ID); rec.User.Email != "" || rec.User.Locale != "fr" || rec.User.Principal.DisplayName.String() != "Anna A" {
		t.Fatalf("after UpdateUser %+v", rec)
	}
	// Owners: lock, read, revoke inside rights changes.
	if _, err := e.addOwner(t, anna.Principal.ID); err != nil {
		t.Fatal(err)
	}
	if o, found, err := e.repo.ActiveOwner(ctx, anna.Principal.ID); err != nil || !found || o.User.ID != anna.Principal.ID {
		t.Fatalf("ActiveOwner = %+v, %v, %v", o, found, err)
	}
	if _, found, err := e.repo.ActiveOwner(ctx, ben.Principal.ID); err != nil || found {
		t.Fatalf("ActiveOwner of a non-owner = %v, %v", found, err)
	}
	err = e.uow.Do(ctx, func(ctx context.Context) error {
		owners, err := e.repo.LockActiveOwners(ctx)
		if err != nil || len(owners) != 1 || owners[0] != anna.Principal.ID {
			t.Fatalf("LockActiveOwners = %v, %v", owners, err)
		}
		if err := e.repo.EnableUser(ctx, ben.Principal.ID); err != nil {
			return err
		}
		revoked, err := e.repo.RevokeOwner(ctx, anna.Principal.ID, ben.Principal.ID)
		if err != nil || !revoked {
			t.Fatalf("RevokeOwner = %v, %v", revoked, err)
		}
		again, err := e.repo.RevokeOwner(ctx, anna.Principal.ID, ben.Principal.ID)
		if err != nil || again {
			t.Fatalf("second RevokeOwner = %v, %v", again, err)
		}
		return nil
	}, txn.RightsChange(anna.Principal.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []func(context.Context) error{
		func(ctx context.Context) error { _, err := e.repo.LockActiveOwners(ctx); return err },
		func(ctx context.Context) error { return e.repo.EnableUser(ctx, ben.Principal.ID) },
		func(ctx context.Context) error {
			_, err := e.repo.RevokeOwner(ctx, ben.Principal.ID, ben.Principal.ID)
			return err
		},
		func(ctx context.Context) error { return e.repo.UpdateUser(ctx, ben) },
	} {
		if err := f(ctx); err == nil {
			t.Error("a write outside a unit of work succeeded")
		}
	}
}
