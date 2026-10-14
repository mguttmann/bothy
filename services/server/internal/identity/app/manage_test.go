// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// entries records the audit entries of a test.
type entries struct {
	auditing.Recorder
	mu   sync.Mutex
	list []auditing.Entry
}

func (r *entries) Record(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = append(r.list, e)
	return nil
}

func (r *entries) last(t *testing.T) auditing.Entry {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.list) == 0 {
		t.Fatal("no audit entry")
	}
	return r.list[len(r.list)-1]
}

// echoRules accepts every configuration unchanged, so these tests need no schema of 08 section 2.6.
type echoRules struct{}

func (echoRules) Validate(raw json.RawMessage) (json.RawMessage, error) { return raw, nil }

// TestProviders_ManageLifecycle runs create, list, get, a change without effect, rename, disable
// and delete through the use cases with the real repository.
func TestProviders_ManageLifecycle(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	owner := f.add(domain.KindUser, "owner")
	a := actor.Actor{ID: owner.ID, Kind: actor.KindUser}
	audit := &entries{}
	s, err := app.NewProviders(app.Config{DisplayNameMaxChars: 64}, app.ProvidersDeps{
		Store: f.repo, Rules: map[identity.ProviderKind]app.ProviderKindRules{identity.ProviderOIDC: echoRules{}},
		UoW: f.uow, Authorizer: allowAll{}, Audit: audit, Auth: unusedAuth{},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := json.RawMessage(`{"issuer": "https://login.example.com", "client_id": "bothy"}`)
	off := false

	if _, err := s.Create(ctx, a, identity.CreateProvider{Kind: identity.ProviderLocal, Name: "Local", Config: cfg}); errCode(err) != errs.CodeValidationFailed {
		t.Fatalf("Create of a kind without rules: %v", err)
	}
	if _, err := s.Create(ctx, a, identity.CreateProvider{Kind: identity.ProviderOIDC, Name: "SSO"}); errCode(err) != errs.CodeValidationFailed {
		t.Fatalf("Create without configuration: %v", err)
	}
	if _, err := s.Create(ctx, a, identity.CreateProvider{Kind: identity.ProviderOIDC, Name: "", Config: cfg}); errCode(err) != errs.CodeValidationFailed {
		t.Fatalf("Create without name: %v", err)
	}
	p, err := s.Create(ctx, a, identity.CreateProvider{Kind: identity.ProviderOIDC, Name: "SSO", Config: cfg, Enabled: &off, AllowProvisioning: &off})
	if err != nil || p.Enabled || p.AllowProvisioning || p.Name != "SSO" {
		t.Fatalf("Create = %+v, %v", p, err)
	}
	if e := audit.last(t); e.Action != string(authorize.ActionIdentityProviderManage) || e.ResourceID != p.ID {
		t.Fatalf("audit of Create = %+v", e)
	}

	list, err := s.List(ctx, a)
	if err != nil || len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if got, err := s.Get(ctx, a, p.ID); err != nil || got.ID != p.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := s.Get(ctx, a, ids.New()); errCode(err) != errs.CodeNotFound {
		t.Fatalf("Get of an unknown id: %v", err)
	}

	// The stored jsonb orders members differently; the same document is no change.
	same := json.RawMessage(`{"client_id":"bothy","issuer":"https://login.example.com"}`)
	name := "SSO"
	if got, err := s.Update(ctx, a, p.ID, identity.UpdateProvider{Name: &name, Config: &same, Enabled: &off}); err != nil || !got.UpdatedAt.Equal(p.UpdatedAt) {
		t.Fatalf("Update without effect = %+v, %v", got, err)
	}
	if _, ok := audit.last(t).Attrs["no_change"]; !ok {
		t.Fatalf("audit of an update without effect: %+v", audit.last(t))
	}
	bad := ""
	if _, err := s.Update(ctx, a, p.ID, identity.UpdateProvider{Name: &bad}); errCode(err) != errs.CodeValidationFailed {
		t.Fatalf("Update with an empty name: %v", err)
	}
	if _, err := s.Update(ctx, a, ids.New(), identity.UpdateProvider{Name: &name}); errCode(err) != errs.CodeNotFound {
		t.Fatalf("Update of an unknown id: %v", err)
	}
	on, renamed := true, "Company SSO"
	other := json.RawMessage(`{"issuer":"https://other.example.com"}`)
	got, err := s.Update(ctx, a, p.ID, identity.UpdateProvider{Name: &renamed, Config: &other, Enabled: &on, AllowProvisioning: &on})
	if err != nil || got.Name != renamed || !got.Enabled || !got.AllowProvisioning {
		t.Fatalf("Update = %+v, %v", got, err)
	}
	if got, err = s.Update(ctx, a, p.ID, identity.UpdateProvider{Enabled: &off}); err != nil || got.Enabled {
		t.Fatalf("disable = %+v, %v", got, err)
	}
	if _, ok := audit.last(t).Attrs["sessions_revoked"]; !ok {
		t.Fatalf("audit of a disable: %+v", audit.last(t))
	}

	if err := s.Delete(ctx, a, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, a, p.ID); errCode(err) != errs.CodeNotFound {
		t.Fatalf("second Delete: %v", err)
	}
}

// TestIdentities_CredentialsAndUnlink covers the overview and unlink through the use cases: an
// unknown person, the overview, the refusal of the last method, an unlink and its repetition.
func TestIdentities_CredentialsAndUnlink(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	a := actor.Actor{ID: alex.ID, Kind: actor.KindUser}
	p1, p2 := ids.New(), ids.New()
	f.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', 'One', '{}')`, p1.String())
	f.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'trusted_header', 'Two', '{}')`, p2.String())
	audit := &entries{}
	s, err := app.NewIdentities(app.IdentitiesDeps{
		Providers: f.repo, Identities: f.repo, People: f.repo, UoW: f.uow, Authorizer: allowAll{}, Audit: audit, Auth: unusedAuth{},
	})
	if err != nil {
		t.Fatal(err)
	}
	const iss = "https://login.example.com"
	one, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: p1, Issuer: iss, Subject: "sub-1"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Credentials(ctx, a, ids.New()); errCode(err) != errs.CodeNotFound {
		t.Fatalf("Credentials of an unknown person: %v", err)
	}
	over, err := s.Credentials(ctx, a, alex.ID)
	if err != nil || len(over.Identities) != 1 || over.Identities[0].ID != one.ID || over.Passkeys == nil {
		t.Fatalf("Credentials = %+v, %v", over, err)
	}

	err = s.Unlink(ctx, a, alex.ID, one.ID)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict || e.Reason != "no_login_method_left" {
		t.Fatalf("Unlink of the last method: %v", err)
	}
	if err := s.Unlink(ctx, a, alex.ID, ids.New()); errCode(err) != errs.CodeNotFound {
		t.Fatalf("Unlink of an unknown identity: %v", err)
	}
	if _, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: p2, Subject: "alex"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Unlink(ctx, a, alex.ID, one.ID); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if e := audit.last(t); e.Action != string(authorize.ActionIdentityUnlink) || e.ResourceID != alex.ID {
		t.Fatalf("audit of Unlink = %+v", e)
	}
	if err := s.Unlink(ctx, a, alex.ID, one.ID); err != nil {
		t.Fatalf("repeated Unlink: %v", err)
	}
	if _, ok := audit.last(t).Attrs["no_change"]; !ok {
		t.Fatalf("audit of a repeated unlink: %+v", audit.last(t))
	}
	if over, err = s.Credentials(ctx, a, alex.ID); err != nil || len(over.Identities) != 1 {
		t.Fatalf("Credentials after unlink = %+v, %v", over, err)
	}
}
