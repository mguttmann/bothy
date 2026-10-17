// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// writerCfg are the principal limits of the writer tests.
var writerCfg = app.Config{HandleMaxChars: 12, DisplayNameMaxChars: 20, PageSizeDefault: 2, PageSizeMax: 3}

// newWriter returns the PrincipalWriter of identity over the repository of e (M1-51).
func newWriter(t *testing.T, e env) *app.PrincipalWriter {
	t.Helper()
	w, err := app.NewPrincipalWriter(writerCfg, e.repo)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// TestPrincipalWriter_Create covers CreatePrincipal: an active agent with the given id, the
// handle rules of M1-26 (pattern, length, taken without regard to case, also by a person), the
// display name rules (SEC-T07), the refusal of people and of a call outside a unit of work.
func TestPrincipalWriter_Create(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := newWriter(t, e)
	ctx := context.Background()
	e.mustAddUser(t, "alex", "")
	id := ids.New()
	var got identity.PrincipalView
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		got, err = w.CreatePrincipal(ctx, identity.CreatePrincipalCmd{ID: id, Kind: actor.KindAgent, Handle: "lotse", DisplayName: "Lotse"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Kind != actor.KindAgent || got.Status != identity.StatusActive || got.Handle != "lotse" || got.CreatedAt.IsZero() {
		t.Fatalf("created %+v", got)
	}
	if rec, found, err := e.repo.Principal(ctx, id); err != nil || !found || rec.Principal.Kind != "agent" {
		t.Fatalf("stored principal %+v, %v, %v", rec, found, err)
	}
	create := func(cmd identity.CreatePrincipalCmd) error {
		return e.write(t, func(ctx context.Context) error {
			_, err := w.CreatePrincipal(ctx, cmd)
			return err
		})
	}
	wantError(t, create(identity.CreatePrincipalCmd{Kind: actor.KindAgent, Handle: "alex", DisplayName: "Alex Bot"}),
		errs.KindConflict, errs.CodeConflict, "/handle", "alex")
	wantError(t, create(identity.CreatePrincipalCmd{Kind: actor.KindAgent, Handle: "Alex", DisplayName: "Alex Bot"}),
		errs.KindBadRequest, errs.CodeValidationFailed, "/handle", "Alex")
	wantError(t, create(identity.CreatePrincipalCmd{Kind: actor.KindAgent, Handle: "toolonghandle1", DisplayName: "Bot"}),
		errs.KindBadRequest, errs.CodeValidationFailed, "/handle")
	wantError(t, create(identity.CreatePrincipalCmd{Kind: actor.KindAgent, Handle: "scout", DisplayName: "Sc\u202eout"}),
		errs.KindBadRequest, errs.CodeValidationFailed, "/display_name")
	wantError(t, create(identity.CreatePrincipalCmd{Kind: actor.KindAgent, Handle: "scout", DisplayName: "  "}),
		errs.KindBadRequest, errs.CodeValidationFailed, "/display_name")
	wantError(t, create(identity.CreatePrincipalCmd{Kind: actor.KindUser, Handle: "scout", DisplayName: "Scout"}),
		errs.KindInternal, errs.CodeInternalError, "")
	if _, err := w.CreatePrincipal(ctx, identity.CreatePrincipalCmd{Kind: actor.KindAgent, Handle: "scout", DisplayName: "Scout"}); err == nil {
		t.Fatal("CreatePrincipal outside a unit of work")
	}
}

// TestPrincipalWriter_Status covers SetPrincipalStatus: disabled sets disabled_at, active clears
// it, the same status is no change, a deleted principal and other targets are refused, people
// are refused, an unknown id is not_found.
func TestPrincipalWriter_Status(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := newWriter(t, e)
	ctx := context.Background()
	agent := e.mustAgent(t, w, "lotse")
	set := func(id ids.ID, s identity.Status) (identity.PrincipalView, bool, error) {
		var p identity.PrincipalView
		var changed bool
		err := e.write(t, func(ctx context.Context) error {
			var err error
			p, changed, err = w.SetPrincipalStatus(ctx, id, s)
			return err
		})
		return p, changed, err
	}
	if p, changed, err := set(agent, identity.StatusDisabled); err != nil || !changed || p.Status != identity.StatusDisabled {
		t.Fatalf("disable = %+v, %v, %v", p, changed, err)
	}
	var disabledSet bool
	if err := e.pool.QueryRow(ctx, `SELECT disabled_at IS NOT NULL FROM principals WHERE id = $1`, agent.String()).Scan(&disabledSet); err != nil || !disabledSet {
		t.Fatalf("disabled_at after disable: %v, %v", disabledSet, err)
	}
	if _, changed, err := set(agent, identity.StatusDisabled); err != nil || changed {
		t.Fatalf("disable again = %v, %v", changed, err)
	}
	if p, changed, err := set(agent, identity.StatusActive); err != nil || !changed || p.Status != identity.StatusActive {
		t.Fatalf("enable = %+v, %v, %v", p, changed, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT disabled_at IS NOT NULL FROM principals WHERE id = $1`, agent.String()).Scan(&disabledSet); err != nil || disabledSet {
		t.Fatalf("disabled_at after enable: %v, %v", disabledSet, err)
	}
	_, _, err := set(agent, identity.StatusDeleted)
	wantError(t, err, errs.KindConflict, errs.CodeInvalidStateTransition, "")
	_, _, err = set(ids.New(), identity.StatusDisabled)
	wantError(t, err, errs.KindNotFound, errs.CodeNotFound, "")
	person := e.mustAddUser(t, "alex", "")
	_, _, err = set(person.Principal.ID, identity.StatusDisabled)
	wantError(t, err, errs.KindInternal, errs.CodeInternalError, "")
	if _, _, err := w.SetPrincipalStatus(ctx, agent, identity.StatusDisabled); err == nil {
		t.Fatal("SetPrincipalStatus outside a unit of work")
	}
}

// TestPrincipalWriter_DisplayName covers SetDisplayName: a change, the same name without change,
// the SEC-IN-4 rules and the refusal of people.
func TestPrincipalWriter_DisplayName(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := newWriter(t, e)
	agent := e.mustAgent(t, w, "lotse")
	rename := func(id ids.ID, name string) (identity.PrincipalView, bool, error) {
		var p identity.PrincipalView
		var changed bool
		err := e.write(t, func(ctx context.Context) error {
			var err error
			p, changed, err = w.SetDisplayName(ctx, id, name)
			return err
		})
		return p, changed, err
	}
	if p, changed, err := rename(agent, "Lotse Zwei"); err != nil || !changed || p.DisplayName != "Lotse Zwei" {
		t.Fatalf("rename = %+v, %v, %v", p, changed, err)
	}
	if _, changed, err := rename(agent, "Lotse Zwei"); err != nil || changed {
		t.Fatalf("rename to the same name = %v, %v", changed, err)
	}
	_, _, err := rename(agent, "Lot\u200ese")
	wantError(t, err, errs.KindBadRequest, errs.CodeValidationFailed, "/display_name")
	_, _, err = rename(ids.New(), "Lotse")
	wantError(t, err, errs.KindNotFound, errs.CodeNotFound, "")
	person := e.mustAddUser(t, "alex", "")
	_, _, err = rename(person.Principal.ID, "Alex")
	wantError(t, err, errs.KindInternal, errs.CodeInternalError, "")
}

// mustAgent creates an active agent principal through w.
func (e env) mustAgent(t *testing.T, w *app.PrincipalWriter, handle string) ids.ID {
	t.Helper()
	var id ids.ID
	if err := e.write(t, func(ctx context.Context) error {
		p, err := w.CreatePrincipal(ctx, identity.CreatePrincipalCmd{Kind: actor.KindAgent, Handle: handle, DisplayName: handle})
		id = p.ID
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}
