// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// otherJKT is a second well-formed thumbprint, no real key.
const otherJKT = "OTHERotherOTHERotherOTHERotherOTHERotherOT1"

// TestTokens_Hosted (A1-03, 26 3.3): the port identity.HostedTokens. IssueHosted runs only in a
// rights change, takes the key of the issuer's calling token, refuses a calling token without one
// (dpop_required) or of another principal (not_found), and keeps one active token per holder and
// issuer; RevokeHosted and RevokeIssued take exactly the tokens they name; a revoked agent token
// takes the hosted tokens it issued with the same key and leaves those of another key, the
// issuer's own tokens and every token a person created.
func TestTokens_Hosted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	now := time.Now().UTC()
	s := newTokens(t, f, clock.NewFake(now))
	ctx := context.Background()
	runner := f.add(domain.KindAgent, "runner")
	writer := f.add(domain.KindAgent, "writer")
	reader := f.add(domain.KindAgent, "reader")
	alex := f.add(domain.KindUser, "alex")
	token := func(holder, by ids.ID, jkt string) identity.APIToken {
		t.Helper()
		out, err := s.Issue(ctx, identity.IssueTokenCmd{PrincipalID: holder, Name: "Adapter", DPoPJKT: jkt, ExpiresAt: now.Add(time.Hour), CreatedBy: by})
		if err != nil {
			t.Fatal(err)
		}
		return out.Token
	}
	keyOne, keyTwo := token(runner.ID, runner.ID, testJKT), token(runner.ID, runner.ID, otherJKT)
	manual := token(writer.ID, alex.ID, testJKT)
	issue := func(holder ids.ID, caller identity.APIToken) (identity.HostedIssue, error) {
		var out identity.HostedIssue
		err := f.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			out, err = s.IssueHosted(ctx, identity.HostedTokenCmd{HolderID: holder, IssuerID: runner.ID, IssuerTokenID: caller.ID,
				Name: "hosted by runner", ExpiresAt: now.Add(time.Hour)})
			return err
		}, txn.RightsChange(holder))
		return out, err
	}
	active := func(holder, issuer ids.ID) int {
		n := 0
		page, err := s.List(ctx, holder, identity.Page{Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		for _, tk := range page.Items {
			if tk.CreatedBy == issuer && tk.RevokedAt.IsZero() {
				n++
			}
		}
		return n
	}
	if _, err := s.IssueHosted(ctx, identity.HostedTokenCmd{HolderID: writer.ID, IssuerID: runner.ID, IssuerTokenID: keyOne.ID}); codeOfErr(err) != string(errs.CodeInternalError) {
		t.Fatalf("outside a rights change: %v", err)
	}
	first, err := issue(writer.ID, keyOne)
	if err != nil {
		t.Fatal(err)
	}
	if tk := first.Issued.Token; tk.PrincipalID != writer.ID || tk.CreatedBy != runner.ID || tk.DPoPJKT != testJKT || tk.Kind != identity.TokenKindAgent ||
		first.Revoked != 0 {
		t.Fatalf("first %+v revoked %d", tk, first.Revoked)
	}
	second, err := issue(writer.ID, keyTwo)
	if err != nil || second.Revoked != 1 || second.Issued.Token.DPoPJKT != otherJKT || active(writer.ID, runner.ID) != 1 {
		t.Fatalf("second %+v %v, active %d", second, err, active(writer.ID, runner.ID))
	}
	if _, err := issue(reader.ID, keyOne); err != nil {
		t.Fatal(err)
	}
	if _, err := issue(reader.ID, manual); codeOfErr(err) != string(errs.CodeNotFound) {
		t.Fatalf("a calling token of another principal: %v", err)
	}
	unboundTokens := newTokensWith(t, f, clock.NewFake(now), nil)
	unbound, err := unboundTokens.Issue(ctx, identity.IssueTokenCmd{PrincipalID: runner.ID, Name: "Unbound", ExpiresAt: now.Add(time.Hour), CreatedBy: runner.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = issue(reader.ID, unbound.Token)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict || e.Reason != identity.ReasonDPoPRequired {
		t.Fatalf("an unbound calling token: %v", err)
	}
	// A revoked runner token takes the hosted tokens of its key only: reader's (keyOne), not
	// writer's (keyTwo); the person-created token of writer with the same key stays.
	if err := s.Revoke(ctx, keyOne.ID, alex.ID); err != nil {
		t.Fatal(err)
	}
	if active(reader.ID, runner.ID) != 0 || active(writer.ID, runner.ID) != 1 || active(writer.ID, alex.ID) != 1 || active(runner.ID, runner.ID) != 2 {
		t.Fatalf("after the revocation of keyOne: reader %d writer %d manual %d runner %d", active(reader.ID, runner.ID),
			active(writer.ID, runner.ID), active(writer.ID, alex.ID), active(runner.ID, runner.ID))
	}
	// A person's revoked token takes nothing it created.
	alexToken, err := unboundTokens.Issue(ctx, identity.IssueTokenCmd{PrincipalID: alex.ID, Name: "Personal", DPoPJKT: testJKT, ExpiresAt: now.Add(time.Hour), CreatedBy: alex.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(ctx, alexToken.Token.ID, alex.ID); err != nil {
		t.Fatal(err)
	}
	if active(writer.ID, alex.ID) != 1 {
		t.Fatal("a person's revoked token took a token the person created")
	}
	var n int
	err = f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.RevokeHosted(ctx, writer.ID, runner.ID, alex.ID)
		return err
	}, txn.RightsChange(writer.ID))
	if err != nil || n != 1 || active(writer.ID, runner.ID) != 0 || active(writer.ID, alex.ID) != 1 {
		t.Fatalf("RevokeHosted %d %v", n, err)
	}
	if _, err := issue(writer.ID, keyTwo); err != nil {
		t.Fatal(err)
	}
	if _, err := issue(reader.ID, keyTwo); err != nil {
		t.Fatal(err)
	}
	err = f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.RevokeIssued(ctx, runner.ID, alex.ID)
		return err
	}, txn.RightsChange(runner.ID))
	if err != nil || n != 2 || active(writer.ID, runner.ID) != 0 || active(reader.ID, runner.ID) != 0 || active(runner.ID, runner.ID) != 2 {
		t.Fatalf("RevokeIssued %d %v, runner's own %d", n, err, active(runner.ID, runner.ID))
	}
	if _, err := s.RevokeHosted(ctx, writer.ID, runner.ID, alex.ID); codeOfErr(err) != string(errs.CodeInternalError) {
		t.Fatalf("RevokeHosted outside a rights change: %v", err)
	}
	if _, err := s.RevokeIssued(ctx, runner.ID, alex.ID); codeOfErr(err) != string(errs.CodeInternalError) {
		t.Fatalf("RevokeIssued outside a rights change: %v", err)
	}
}

// newTokensWith is newTokens with auth.dpop_required_for set to required.
func newTokensWith(t *testing.T, f *fixture, clk clock.Clock, required []string) *app.Tokens {
	t.Helper()
	cfg := app.Config{APITokenMaxTTL: 90 * 24 * time.Hour, PersonalTokenMaxTTL: 30 * 24 * time.Hour,
		DPoPRequiredFor: required, PageSizeDefault: 50, PageSizeMax: 200}
	s, err := app.NewTokens(cfg, tokenPepper, app.TokensDeps{
		Store: f.repo, Principals: f.repo, UoW: f.uow, Authorizer: denyAll{}, Audit: noAudit{}, Cursors: nopCursors{},
		Switch: personalOff{}, Clock: clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
