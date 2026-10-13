// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// tokenAt is an API token record of holder created by creator at created, with a hash unique to
// its ID.
func tokenAt(holder, creator ids.ID, created time.Time, jkt string) app.APITokenRecord {
	id := ids.New()
	return app.APITokenRecord{
		ID: id, PrincipalID: holder, Name: "ci", Prefix: "bothy_agt_EXAMPL", Hash: []byte(id.String()),
		DPoPJKT: jkt, ExpiresAt: created.Add(24 * time.Hour), CreatedBy: creator, CreatedAt: created,
	}
}

// TestAPITokens_StoreRoundTrip (M1-44): the API token store of the repository writes and reads
// back every column, finds a token by hash with the kind of its holder, pages the active tokens
// newest first, revokes one and all of a principal exactly once, and writes last_used_at only
// past the threshold. Writes need a unit of work.
func TestAPITokens_StoreRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sam := e.mustAddUser(t, "sam", "sam@example.com").Principal
	other := e.mustAddUser(t, "kim", "kim@example.com").Principal
	base := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	t1 := tokenAt(sam.ID, sam.ID, base, "")
	t2 := tokenAt(sam.ID, sam.ID, base.Add(time.Minute), "jkt-thumbprint")
	t3 := tokenAt(sam.ID, sam.ID, base.Add(2*time.Minute), "")
	t4 := tokenAt(other.ID, other.ID, base, "")

	if err := e.repo.InsertAPIToken(ctx, t1); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("insert outside a unit of work: %v", err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		for _, tk := range []app.APITokenRecord{t1, t2, t3, t4} {
			if err := e.repo.InsertAPIToken(ctx, tk); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dup := tokenAt(sam.ID, sam.ID, base, "")
	dup.Hash = t1.Hash
	if err := e.write(t, func(ctx context.Context) error { return e.repo.InsertAPIToken(ctx, dup) }); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("duplicate hash: %v", err)
	}

	got, ok, err := e.repo.APIToken(ctx, t2.ID)
	if err != nil || !ok || got.DPoPJKT != "jkt-thumbprint" || !got.ExpiresAt.Equal(t2.ExpiresAt) || got.Scopes == nil || len(got.Scopes) != 0 ||
		got.PrincipalID != sam.ID || !got.LastUsedAt.IsZero() || !got.RevokedAt.IsZero() {
		t.Fatalf("APIToken = %+v, %v, %v", got, ok, err)
	}
	if _, ok, err := e.repo.APIToken(ctx, ids.New()); ok || err != nil {
		t.Fatalf("unknown id: %v, %v", ok, err)
	}
	auth, ok, err := e.repo.APITokenByHash(ctx, t1.Hash)
	if err != nil || !ok || auth.ID != t1.ID || auth.PrincipalKind != domain.KindUser || !bytes.Equal(auth.Hash, t1.Hash) {
		t.Fatalf("APITokenByHash = %+v, %v, %v", auth, ok, err)
	}
	if _, ok, err := e.repo.APITokenByHash(ctx, []byte("no such hash")); ok || err != nil {
		t.Fatalf("unknown hash: %v, %v", ok, err)
	}

	page, err := e.repo.ActiveAPITokens(ctx, sam.ID, time.Time{}, ids.ID{}, 2)
	if err != nil || len(page) != 2 || page[0].ID != t3.ID || page[1].ID != t2.ID {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	page, err = e.repo.ActiveAPITokens(ctx, sam.ID, page[1].CreatedAt, page[1].ID, 2)
	if err != nil || len(page) != 1 || page[0].ID != t1.ID {
		t.Fatalf("second page = %+v, %v", page, err)
	}

	if _, _, err := e.repo.LockAPIToken(ctx, t1.ID); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit of work: %v", err)
	}
	at := base.Add(time.Hour)
	if err := e.write(t, func(ctx context.Context) error {
		locked, ok, err := e.repo.LockAPIToken(ctx, t1.ID)
		if err != nil || !ok || locked.ID != t1.ID {
			t.Errorf("LockAPIToken = %+v, %v, %v", locked, ok, err)
		}
		if _, ok, err := e.repo.LockAPIToken(ctx, ids.New()); ok || err != nil {
			t.Errorf("lock unknown id: %v, %v", ok, err)
		}
		for i, want := range []bool{true, false} {
			if done, err := e.repo.RevokeAPIToken(ctx, t1.ID, other.ID, at); err != nil || done != want {
				t.Errorf("revoke %d: %v, %v; want %v", i, done, err, want)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, _, _ = e.repo.APIToken(ctx, t1.ID)
	if !got.RevokedAt.Equal(at) || got.RevokedBy != other.ID {
		t.Fatalf("revoked token = %+v", got)
	}
	if _, err := e.repo.RevokeAPIToken(ctx, t2.ID, ids.ID{}, at); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("revoke outside a unit of work: %v", err)
	}
	if _, err := e.repo.RevokeAPITokensOfPrincipal(ctx, sam.ID, ids.ID{}, at); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("revoke all outside a unit of work: %v", err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		for i, want := range []int{2, 0} {
			if n, err := e.repo.RevokeAPITokensOfPrincipal(ctx, sam.ID, ids.ID{}, at); err != nil || n != want {
				t.Errorf("revoke all %d: %d, %v; want %d", i, n, err, want)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if page, err := e.repo.ActiveAPITokens(ctx, sam.ID, time.Time{}, ids.ID{}, 10); err != nil || len(page) != 0 {
		t.Fatalf("active after revoke all = %+v, %v", page, err)
	}
	if page, err := e.repo.ActiveAPITokens(ctx, other.ID, time.Time{}, ids.ID{}, 10); err != nil || len(page) != 1 {
		t.Fatalf("other principal's tokens = %+v, %v", page, err)
	}

	used := base.Add(2 * time.Hour)
	for i, step := range []struct {
		at, threshold, want time.Time
	}{
		{used, used.Add(-time.Minute), used},
		{used.Add(30 * time.Second), used.Add(-30 * time.Second), used},
		{used.Add(2 * time.Minute), used.Add(time.Minute), used.Add(2 * time.Minute)},
	} {
		if err := e.repo.TouchAPIToken(ctx, t4.ID, step.at, step.threshold); err != nil {
			t.Fatalf("touch %d: %v", i, err)
		}
		got, _, _ := e.repo.APIToken(ctx, t4.ID)
		if !got.LastUsedAt.Equal(step.want) {
			t.Fatalf("touch %d: last_used_at %v, want %v", i, got.LastUsedAt, step.want)
		}
	}
}
