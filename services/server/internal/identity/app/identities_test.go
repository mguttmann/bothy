// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
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
