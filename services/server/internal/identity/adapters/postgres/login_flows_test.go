// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestLoginFlowStoreRoundTrip covers every method of app.LoginFlowStore against PostgreSQL 18 and
// the lock of an enrollment code by id: insert, the read by state hash outside a unit, the issue
// of the login code only from started, the lock by code hash, completion and failure only from
// their states, and the deletion of expired flows.
func TestLoginFlowStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	prov := e.mustInsertProvider(t, provider("Company", true))
	alex := e.mustAddUser(t, "alex", "alex@example.com")
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	code := app.EnrollmentCodeRecord{
		ID: ids.New(), UserID: alex.Principal.ID, Purpose: app.PurposeIdentityLink, Hash: []byte("code-EXAMPLE"),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	login := app.LoginFlowRecord{
		ID: ids.New(), ProviderID: prov.ID, Mode: oidcflow.ModeLogin, StateHash: []byte("state-1"), NonceHash: []byte("nonce-1"),
		ClientChallenge: "challenge", AppRedirectURI: "http://127.0.0.1:4711/callback", EnrollmentCodeID: code.ID, CreatedAt: now,
		ExpiresAt: now.Add(10 * time.Minute),
	}
	link := app.LoginFlowRecord{
		ID: ids.New(), ProviderID: prov.ID, Mode: oidcflow.ModeLink, UserID: alex.Principal.ID, StateHash: []byte("state-2"),
		NonceHash: []byte("nonce-2"), ClientChallenge: "challenge", AppRedirectURI: "com.example.chat:/oidc", CreatedAt: now,
		ExpiresAt: now.Add(-time.Second),
	}
	if err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.InsertEnrollmentCode(ctx, code); err != nil {
			return err
		}
		if err := e.repo.InsertLoginFlow(ctx, login); err != nil {
			return err
		}
		return e.repo.InsertLoginFlow(ctx, link)
	}); err != nil {
		t.Fatal(err)
	}
	got, found, err := e.repo.LoginFlowByStateHash(ctx, login.StateHash)
	if err != nil || !found || got.ID != login.ID || got.Status != oidcflow.StatusStarted || got.EnrollmentCodeID != code.ID || got.External != nil ||
		!got.ExpiresAt.Equal(login.ExpiresAt) {
		t.Fatalf("by state = %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.repo.LoginFlowByStateHash(ctx, []byte("unknown")); err != nil || found {
		t.Fatalf("unknown state: %v %v", found, err)
	}
	if _, err := e.repo.IssueLoginCode(ctx, login.ID, oidcflow.ExternalIdentity{}, nil, now); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("write outside a unit: %v", err)
	}
	ext := oidcflow.ExternalIdentity{Issuer: "https://idp.example.com", Subject: "alex-sub", DisplayName: "Alex", AuthTime: now, AMR: []string{"pwd"}}
	var issued, again, completed, failedDone, failedLink bool
	var locked app.LoginFlowRecord
	var codeLocked app.EnrollmentCodeRecord
	var codeFound bool
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		if issued, err = e.repo.IssueLoginCode(ctx, login.ID, ext, []byte("login-code"), now.Add(time.Minute)); err != nil {
			return err
		}
		if again, err = e.repo.IssueLoginCode(ctx, login.ID, ext, []byte("login-code-2"), now.Add(time.Minute)); err != nil {
			return err
		}
		if locked, _, err = e.repo.LockLoginFlowByCodeHash(ctx, []byte("login-code")); err != nil {
			return err
		}
		if codeLocked, codeFound, err = e.repo.LockEnrollmentCode(ctx, code.ID); err != nil {
			return err
		}
		if completed, err = e.repo.CompleteLoginFlow(ctx, login.ID); err != nil {
			return err
		}
		if failedDone, err = e.repo.FailLoginFlow(ctx, login.ID, oidcflow.ErrorLoginFailed); err != nil {
			return err
		}
		failedLink, err = e.repo.FailLoginFlow(ctx, link.ID, oidcflow.ErrorFlowExpired)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !issued || again || !completed || failedDone || !failedLink {
		t.Fatalf("issued %v again %v completed %v failed after completion %v failed link %v", issued, again, completed, failedDone, failedLink)
	}
	if locked.Status != oidcflow.StatusCodeIssued || locked.External == nil || locked.External.Subject != "alex-sub" ||
		!locked.External.AuthTime.Equal(now) || !locked.LoginCodeExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("locked = %+v", locked)
	}
	if !codeFound || codeLocked.ID != code.ID {
		t.Fatalf("code lock = %+v %v", codeLocked, codeFound)
	}
	got, _, _ = e.repo.LoginFlowByStateHash(ctx, link.StateHash)
	if got.Status != oidcflow.StatusFailed || got.Error != oidcflow.ErrorFlowExpired || got.UserID != alex.Principal.ID {
		t.Fatalf("failed link = %+v", got)
	}
	var deleted int64
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		deleted, err = e.repo.DeleteExpiredLoginFlows(ctx, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted %d, want the expired link flow", deleted)
	}
	if _, found, _ := e.repo.LoginFlowByStateHash(ctx, login.StateHash); !found {
		t.Fatal("the live flow was deleted")
	}
}
