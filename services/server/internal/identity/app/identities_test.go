// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// unusedAuthorizer, unusedRecorder and unusedAuth stand for collaborators a test never reaches:
// Resolve authorizes nothing, writes no audit and revokes no session.
type (
	unusedAuthorizer struct{ authorize.Authorizer }
	unusedRecorder   struct{ auditing.Recorder }
	unusedAuth       struct{ identity.Auth }
)

// newIdentities returns the identity use cases over the fixture's repository.
func newIdentities(t *testing.T, f *fixture) *app.Identities {
	t.Helper()
	s, err := app.NewIdentities(app.IdentitiesDeps{
		Providers: f.repo, Identities: f.repo, People: f.repo, UoW: f.uow, Authorizer: unusedAuthorizer{},
		Audit: unusedRecorder{}, Auth: unusedAuth{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestResolve_IDT04_EmailNeverMatches: a person whose users.email equals the EmailHint of an
// external identity with an unknown subject stays unmatched, also when the display name and the
// e-mail at link of an existing identity of that person agree (ID-T04, R-ID-02).
func TestResolve_IDT04_EmailNeverMatches(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	alex := f.add(domain.KindUser, "alex")
	f.sql(`UPDATE users SET email = 'alex@example.com' WHERE principal_id = $1`, alex.ID.String())
	prov := ids.New()
	f.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', 'Company SSO', '{}')`, prov.String())
	f.sql(`INSERT INTO user_identities (id, user_id, provider_id, subject, issuer, email_at_link)
		VALUES ($1, $2, $3, 'sub-alex', 'https://login.example.com', 'alex@example.com')`, ids.New().String(), alex.ID.String(), prov.String())
	s := newIdentities(t, f)
	res, err := s.Resolve(context.Background(), identity.ExternalIdentity{
		ProviderID: prov, Issuer: "https://login.example.com", Subject: "sub-attacker",
		DisplayName: "Name of alex", EmailHint: "alex@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != identity.ResolutionUnmatched || !res.UserID.IsZero() || !res.IdentityID.IsZero() {
		t.Fatalf("Resolve = %+v, want unmatched without person", res)
	}
	res, err = s.Resolve(context.Background(), identity.ExternalIdentity{ProviderID: prov, Issuer: "https://login.example.com", Subject: "sub-alex"})
	if err != nil || res.Outcome != identity.ResolutionMatched || res.UserID != alex.ID {
		t.Fatalf("known subject: %+v, %v, want matched for alex", res, err)
	}
}

// TestIdentities_LinkResolveMarkLogin covers the paths of the login drivers: link with and without
// an OIDC issuer and e-mail hint, resolve with an unknown provider, an empty subject and another
// issuer, and the login mark of a known and an unknown identity.
func TestIdentities_LinkResolveMarkLogin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	oidc, header, off := ids.New(), ids.New(), ids.New()
	f.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', 'Company SSO', '{}')`, oidc.String())
	f.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'trusted_header', 'Proxy', '{}')`, header.String())
	f.sql(`INSERT INTO identity_providers (id, kind, name, config, enabled) VALUES ($1, 'oidc', 'Old SSO', '{}', false)`, off.String())
	s := newIdentities(t, f)
	const iss = "https://login.example.com"

	for name, subject := range map[string]string{"empty": "", "invalid UTF-8": "\xff", "NUL": "a\x00b"} {
		if _, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: oidc, Issuer: iss, Subject: subject}); errCode(err) != errs.CodeValidationFailed {
			t.Errorf("Link with a %s subject: %v", name, err)
		}
	}
	if _, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: oidc, Subject: "sub-1"}); errCode(err) != errs.CodeValidationFailed {
		t.Errorf("Link of an OIDC identity without issuer: %v", err)
	}
	if _, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: off, Issuer: iss, Subject: "sub-1"}); errCode(err) != errs.CodeNotFound {
		t.Errorf("Link at a disabled provider: %v", err)
	}
	linked, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: oidc, Issuer: iss, Subject: "sub-1", EmailHint: "alex@example.com"})
	if err != nil || linked.Issuer != iss || linked.EmailAtLink != "alex@example.com" || linked.ProviderKind != identity.ProviderOIDC {
		t.Fatalf("Link oidc = %+v, %v", linked, err)
	}
	viaHeader, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: header, Issuer: iss, Subject: "alex", EmailHint: "not an address"})
	if err != nil || viaHeader.Issuer != "" || viaHeader.EmailAtLink != "" {
		t.Fatalf("Link trusted_header = %+v, %v", viaHeader, err)
	}
	if _, err := s.Link(ctx, alex.ID, identity.ExternalIdentity{ProviderID: oidc, Issuer: iss, Subject: "sub-1"}); errCode(err) != errs.CodeConflict {
		t.Errorf("second active link of a subject: %v", err)
	}

	for name, ext := range map[string]identity.ExternalIdentity{
		"empty subject":    {ProviderID: oidc, Issuer: iss},
		"unknown provider": {ProviderID: ids.New(), Issuer: iss, Subject: "sub-1"},
		"other issuer":     {ProviderID: oidc, Issuer: "https://other.example.com", Subject: "sub-1"},
		"unknown subject":  {ProviderID: oidc, Issuer: iss, Subject: "sub-2"},
	} {
		if res, err := s.Resolve(ctx, ext); err != nil || res.Outcome != identity.ResolutionUnmatched {
			t.Errorf("Resolve with %s = %+v, %v", name, res, err)
		}
	}
	if res, err := s.Resolve(ctx, identity.ExternalIdentity{ProviderID: header, Subject: "alex"}); err != nil || res.IdentityID != viaHeader.ID {
		t.Errorf("Resolve trusted_header = %+v, %v", res, err)
	}

	if err := s.MarkLogin(ctx, linked.ID, time.Now()); err != nil {
		t.Errorf("MarkLogin: %v", err)
	}
	if err := s.MarkLogin(ctx, ids.New(), time.Now()); errCode(err) != errs.CodeNotFound {
		t.Errorf("MarkLogin of an unknown identity: %v", err)
	}
}

// TestProviders_EnabledAndDiscovery covers the two reads without authorization.
func TestProviders_EnabledAndDiscovery(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	on, off := ids.New(), ids.New()
	f.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', 'Company SSO', '{}')`, on.String())
	f.sql(`INSERT INTO identity_providers (id, kind, name, config, enabled) VALUES ($1, 'oidc', 'Old SSO', '{}', false)`, off.String())
	s, err := app.NewProviders(app.Config{DisplayNameMaxChars: 64}, app.ProvidersDeps{
		Store: f.repo, Rules: app.DefaultProviderRules(nil), UoW: f.uow, Authorizer: unusedAuthorizer{},
		Audit: unusedRecorder{}, Auth: unusedAuth{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p, err := s.Enabled(ctx, on); err != nil || p.ID != on {
		t.Fatalf("Enabled = %+v, %v", p, err)
	}
	if _, err := s.Enabled(ctx, off); errCode(err) != errs.CodeNotFound {
		t.Fatalf("Enabled of a disabled provider: %v", err)
	}
	list, err := s.ListForDiscovery(ctx)
	if err != nil || len(list) != 1 || list[0].ID != on || list[0].Kind != identity.ProviderOIDC || list[0].Name != "Company SSO" {
		t.Fatalf("ListForDiscovery = %+v, %v", list, err)
	}
}

// TestNewProvidersAndIdentities_Refuse checks the constructor errors.
func TestNewProvidersAndIdentities_Refuse(t *testing.T) {
	t.Parallel()
	if _, err := app.NewProviders(app.Config{DisplayNameMaxChars: 64}, app.ProvidersDeps{}); err == nil {
		t.Error("NewProviders without collaborators")
	}
	f := newFixture(t)
	deps := app.ProvidersDeps{
		Store: f.repo, Rules: app.DefaultProviderRules(nil), UoW: f.uow, Authorizer: unusedAuthorizer{},
		Audit: unusedRecorder{}, Auth: unusedAuth{},
	}
	if _, err := app.NewProviders(app.Config{}, deps); err == nil {
		t.Error("NewProviders with a display name limit of 0")
	}
	if _, err := app.NewIdentities(app.IdentitiesDeps{}); err == nil {
		t.Error("NewIdentities without collaborators")
	}
}

// errCode returns the public code of err, or "" when err is no *errs.Error.
func errCode(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}
