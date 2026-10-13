// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// noAudit is an audit recorder the port methods of Tokens never call.
type noAudit struct{ auditing.Recorder }

// testJKT is a well-formed RFC 7638 thumbprint (43 base64url characters), no real key.
const testJKT = "EXAMPLEexampleEXAMPLEexampleEXAMPLEexample0"

// newTokens returns the token use cases over the fixture's database at clk, with DPoP required
// for agents, personal tokens off and the cursors of nopCursors.
func newTokens(t *testing.T, f *fixture, clk clock.Clock) *app.Tokens {
	t.Helper()
	cfg := app.Config{APITokenMaxTTL: 90 * 24 * time.Hour, PersonalTokenMaxTTL: 30 * 24 * time.Hour,
		DPoPRequiredFor: []string{app.DPoPForAgent}, PageSizeDefault: 50, PageSizeMax: 200}
	s, err := app.NewTokens(cfg, tokenPepper, app.TokensDeps{
		Store: f.repo, Principals: f.repo, UoW: f.uow, Authorizer: denyAll{}, Audit: noAudit{}, Cursors: nopCursors{},
		Switch: app.PersonalTokensDefault{}, Clock: clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestTokens_Port (M1-44): the port identity.Tokens without HTTP. Issue derives the kind from the
// holder, validates name, scopes, expiry (both maxima) and binding, stores only the hash and
// returns a secret that parses; List pages the active tokens newest first and refuses a cursor
// without a position; Revoke and RevokeAllForPrincipal refuse to run inside a unit of work that is
// no rights change, and an unknown token is not_found.
func TestTokens_Port(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	now := time.Now().UTC()
	s := newTokens(t, f, clock.NewFake(now))
	ctx := context.Background()
	lotse := f.add(domain.KindAgent, "lotse")
	alex := f.add(domain.KindUser, "alex")
	cmd := func(holder ids.ID, mod func(*identity.IssueTokenCmd)) identity.IssueTokenCmd {
		c := identity.IssueTokenCmd{PrincipalID: holder, Name: "Adapter", DPoPJKT: testJKT, ExpiresAt: now.Add(time.Hour), CreatedBy: alex.ID}
		if mod != nil {
			mod(&c)
		}
		return c
	}

	issued, err := s.Issue(ctx, cmd(lotse.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	secret := issued.Secret.Reveal()
	p, err := apitoken.Parse(secret)
	if err != nil || p.Kind != apitoken.KindAgent || issued.Token.Kind != identity.TokenKindAgent || issued.Token.TokenPrefix != p.Prefix() ||
		issued.Token.DPoPJKT != testJKT || issued.Token.PrincipalID != lotse.ID || strings.Contains(issued.Token.TokenPrefix, secret) {
		t.Fatalf("issued %+v, parse %v", issued.Token, err)
	}
	stored, ok, err := f.repo.APIToken(ctx, issued.Token.ID)
	if err != nil || !ok || string(stored.Hash) != string(apitoken.Hash(tokenPepper, secret)) {
		t.Fatalf("stored %+v, %v, %v", stored, ok, err)
	}
	if _, err := s.Issue(ctx, cmd(lotse.ID, func(c *identity.IssueTokenCmd) { c.Name = "Second" })); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		cmd  identity.IssueTokenCmd
		code errs.Code
	}{
		"unknown holder":         {cmd(ids.New(), nil), errs.CodeNotFound},
		"agent without binding":  {cmd(lotse.ID, func(c *identity.IssueTokenCmd) { c.DPoPJKT = "" }), errs.CodeValidationFailed},
		"agent with scopes":      {cmd(lotse.ID, func(c *identity.IssueTokenCmd) { c.Scopes = []string{"messages:read"} }), errs.CodeValidationFailed},
		"empty name":             {cmd(lotse.ID, func(c *identity.IssueTokenCmd) { c.Name = "" }), errs.CodeValidationFailed},
		"no expiry":              {cmd(lotse.ID, func(c *identity.IssueTokenCmd) { c.ExpiresAt = time.Time{} }), errs.CodeValidationFailed},
		"agent past its maximum": {cmd(lotse.ID, func(c *identity.IssueTokenCmd) { c.ExpiresAt = now.Add(91 * 24 * time.Hour) }), errs.CodeValidationFailed},
		"personal past its maximum": {cmd(alex.ID, func(c *identity.IssueTokenCmd) {
			c.DPoPJKT, c.ExpiresAt = "", now.Add(31*24*time.Hour)
		}), errs.CodeValidationFailed},
		"malformed binding": {cmd(lotse.ID, func(c *identity.IssueTokenCmd) { c.DPoPJKT = "short" }), errs.CodeValidationFailed},
	} {
		if _, err := s.Issue(ctx, tc.cmd); codeOfErr(err) != string(tc.code) {
			t.Errorf("%s: %v, want %s", name, err, tc.code)
		}
	}
	personal, err := s.Issue(ctx, cmd(alex.ID, func(c *identity.IssueTokenCmd) { c.DPoPJKT = "" }))
	if err != nil || personal.Token.Kind != identity.TokenKindPersonal {
		t.Fatalf("personal token %+v, %v", personal.Token, err)
	}

	page, err := s.List(ctx, lotse.ID, identity.Page{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].Name != "Second" || page.NextCursor == "" {
		t.Fatalf("first page %+v, %v", page, err)
	}
	if all, err := s.List(ctx, lotse.ID, identity.Page{}); err != nil || len(all.Items) != 2 || all.NextCursor != "" {
		t.Fatalf("whole list %+v, %v", all, err)
	}
	if _, err := s.List(ctx, lotse.ID, identity.Page{Cursor: "c"}); codeOfErr(err) != string(errs.CodeCursorInvalid) {
		t.Fatalf("cursor without a position: %v", err)
	}

	if err := s.Revoke(ctx, ids.New(), alex.ID); codeOfErr(err) != string(errs.CodeNotFound) {
		t.Fatalf("revoke unknown: %v", err)
	}
	if _, err := s.RevokeAllForPrincipal(ctx, lotse.ID, alex.ID); codeOfErr(err) != string(errs.CodeInternalError) {
		t.Fatalf("revoke all outside a rights change: %v", err)
	}
	err = f.uow.Do(ctx, func(ctx context.Context) error {
		if err := s.Revoke(ctx, issued.Token.ID, alex.ID); codeOfErr(err) != string(errs.CodeInternalError) {
			t.Errorf("revoke inside a unit that is no rights change: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _ := f.repo.APIToken(ctx, issued.Token.ID); !r.RevokedAt.IsZero() {
		t.Fatal("a refused revocation changed the token")
	}
}
