// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestNewRefusesMissingCollaborators: every collaborator and a valid page size are required.
func TestNewRefusesMissingCollaborators(t *testing.T) {
	t.Parallel()
	if _, err := app.New(app.Config{PageSizeDefault: 1, PageSizeMax: 1}, app.Deps{}); err == nil {
		t.Fatal("New without collaborators")
	}
}

// TestCreateAndUpdate: create validates adapter kind, description and configuration; update
// changes display name, description and configuration with one version step, and an update
// without change keeps the version; tool class changes with a version step.
func TestCreateAndUpdate(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	for _, cmd := range []agents.CreateCmd{
		{Handle: "x1", DisplayName: "X", AdapterKind: "Bad"},
		{Handle: "x2", DisplayName: "X", AdapterKind: "hermes", Description: "a\u0007"},
		{Handle: "x3", DisplayName: "X", AdapterKind: "hermes", OperatorConfig: map[string]any{"secret": "x"}},
		{Handle: "Upper", DisplayName: "X", AdapterKind: "hermes"},
	} {
		_, err := w.svc.Create(ctx, owner, cmd)
		wantCode(t, err, errs.CodeValidationFailed)
	}
	a := w.create(owner, "lotse")
	name, desc := "Lotse Zwei", "Pilot"
	got, err := w.svc.Update(ctx, owner, a.ID, agents.UpdateCmd{DisplayName: &name, Description: &desc,
		OperatorConfig: map[string]any{"supports_title": false}, IfMatch: match(1)})
	if err != nil || got.Version != 2 || got.DisplayName != name || got.Description != desc || got.OperatorConfig["supports_title"] != false {
		t.Fatalf("update %+v, %v", got, err)
	}
	got, err = w.svc.Update(ctx, owner, a.ID, agents.UpdateCmd{DisplayName: &name, IfMatch: match(2)})
	if err != nil || got.Version != 2 {
		t.Fatalf("no-op update %+v, %v", got, err)
	}
	newName := "Lotse Drei"
	if got, err = w.svc.Update(ctx, owner, a.ID, agents.UpdateCmd{DisplayName: &newName, IfMatch: match(2)}); err != nil || got.Version != 3 {
		t.Fatalf("rename %+v, %v", got, err)
	}
	_, err = w.svc.Update(ctx, owner, a.ID, agents.UpdateCmd{OperatorConfig: map[string]any{"supports_title": 1.0}, IfMatch: match(3)})
	wantCode(t, err, errs.CodeValidationFailed)
	if got, err = w.svc.SetToolClass(ctx, owner, a.ID, agents.ToolClassCmd{ToolClass: agents.ToolHost, Reason: "Shell", IfMatch: match(3)}); err != nil ||
		got.ToolClass != agents.ToolHost || got.Version != 4 {
		t.Fatalf("tool class %+v, %v", got, err)
	}
}

// TestAZM22_FakeStructure is AZ-M22 on the fake-source harness: Assistent serves Research and
// Notizen; Alex has trigger on Notizen; the switch of Assistent to personal for the sole owner is
// an InvariantError naming Alex and leaves the agent as it was; Research alone with a read grant
// for Alex is in order.
func TestAZM22_FakeStructure(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	alex := w.person("alex", false)
	assistent := w.create(owner, "assistent")
	space, research, notizen := ids.New(), ids.New(), ids.New()
	w.structure.PutSpace(space)
	w.structure.PutChannel(authz.ChannelFacts{ID: research, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: assistent.ID})
	w.structure.PutChannel(authz.ChannelFacts{ID: notizen, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: assistent.ID})
	grant := func(ch ids.ID, level string) {
		w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, reason, granted_by)
			VALUES ($1, 'user', $2, 'channel', $3, $4, 'fixture', $2)`, ids.New().String(), alex.ID.String(), ch.String(), level)
	}
	grant(research, "read")
	grant(notizen, "trigger")
	_, err := w.svc.SetBillingMode(ctx, owner, assistent.ID, agents.BillingModeCmd{
		BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, Reason: "Own use", IfMatch: match(1),
	})
	var inv *authz.InvariantError
	if !errors.As(err, &inv) {
		t.Fatalf("switch with a foreign trigger: %v", err)
	}
	wantCode(t, err, errs.CodeI1Violation)
	got, err := w.svc.Get(ctx, owner, assistent.ID)
	if err != nil || got.BillingMode != agents.BillingOrganization || got.Version != 1 {
		t.Fatalf("after the refusal %+v, %v", got, err)
	}
	w.sql(`UPDATE grants SET revoked_at = now(), revoked_by = subject_principal_id, revoke_reason = 'fixture' WHERE level = 'trigger'`)
	got, err = w.svc.SetBillingMode(ctx, owner, assistent.ID, agents.BillingModeCmd{
		BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, Reason: "Own use", IfMatch: match(1),
	})
	if err != nil || got.BillingMode != agents.BillingPersonal || got.CredentialHolderID != owner.ID || got.Version != 2 {
		t.Fatalf("switch without a foreign trigger %+v, %v", got, err)
	}
}

// TestPreconditionAndCaller: a missing If-Match is precondition_required, a malformed or stale one
// version_conflict with the current agent; a caller other than the principal of the request is
// an internal error.
func TestPreconditionAndCaller(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	a := w.create(owner, "lotse")
	desc := "Pilot"
	_, err := w.svc.Update(ctx, owner, a.ID, agents.UpdateCmd{Description: &desc})
	wantCode(t, err, errs.CodePreconditionRequired)
	_, err = w.svc.Update(ctx, owner, a.ID, agents.UpdateCmd{Description: &desc, IfMatch: agents.Precondition{Present: true}})
	wantCode(t, err, errs.CodeVersionConflict)
	_, err = w.svc.SetToolClass(ctx, owner, a.ID, agents.ToolClassCmd{ToolClass: agents.ToolHost, Reason: "x", IfMatch: match(9)})
	wantCode(t, err, errs.CodeVersionConflict)
	if e, _ := errs.As(err); e.Current == nil {
		t.Fatal("version_conflict without current")
	}
	other := actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser})
	_, err = w.svc.Update(other, owner, a.ID, agents.UpdateCmd{Description: &desc, IfMatch: match(1)})
	wantCode(t, err, errs.CodeInternalError)
	_, err = w.svc.Get(ctx, owner, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	_, err = w.svc.Update(ctx, owner, ids.New(), agents.UpdateCmd{Description: &desc, IfMatch: match(1)})
	wantCode(t, err, errs.CodeNotFound)
	bad := "a\u202eb"
	_, err = w.svc.Update(ctx, owner, a.ID, agents.UpdateCmd{Description: &bad, IfMatch: match(1)})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = w.svc.SetToolClass(ctx, owner, a.ID, agents.ToolClassCmd{ToolClass: "shell", Reason: "x", IfMatch: match(1)})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = w.svc.SetBillingMode(ctx, owner, a.ID, agents.BillingModeCmd{BillingMode: "shared", Reason: "x", IfMatch: match(1)})
	wantCode(t, err, errs.CodeValidationFailed)
}

// TestLifecycleAndHolder: disable and enable with and without effect, the holder change of a
// disabled personal agent, the refusal of a holder that is no person, and a deleted agent.
func TestLifecycleAndHolder(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	sam := w.person("sam", false)
	a := w.create(owner, "lotse")
	if _, err := w.svc.Enable(ctx, owner, a.ID, "Already"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.SetBillingMode(ctx, owner, a.ID, agents.BillingModeCmd{
		BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, Reason: "Own", IfMatch: match(1),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := w.svc.Disable(ctx, owner, a.ID, "Pause")
	if err != nil || got.Status != "disabled" || got.Version != 3 {
		t.Fatalf("disable %+v, %v", got, err)
	}
	_, err = w.svc.SetCredentialHolder(ctx, owner, a.ID, agents.CredentialHolderCmd{CredentialHolderID: a.ID, Reason: "x", IfMatch: match(3)})
	wantCode(t, err, errs.CodeValidationFailed)
	got, err = w.svc.SetCredentialHolder(ctx, owner, a.ID, agents.CredentialHolderCmd{CredentialHolderID: sam.ID, Reason: "Handover", IfMatch: match(3)})
	if err != nil || got.CredentialHolderID != sam.ID || got.Version != 4 {
		t.Fatalf("holder %+v, %v", got, err)
	}
	got, err = w.svc.SetCredentialHolder(ctx, owner, a.ID, agents.CredentialHolderCmd{CredentialHolderID: sam.ID, Reason: "Same", IfMatch: match(4)})
	if err != nil || got.Version != 4 {
		t.Fatalf("same holder %+v, %v", got, err)
	}
	_, err = w.svc.Enable(ctx, owner, a.ID, "Back")
	wantCode(t, err, errs.CodeI1Violation)
	w.sql(`UPDATE principals SET status = 'deleted' WHERE id = $1`, a.ID.String())
	_, err = w.svc.Disable(ctx, owner, a.ID, "Gone")
	wantCode(t, err, errs.CodeInvalidStateTransition)
	_, err = w.svc.Disable(ctx, owner, a.ID, "")
	wantCode(t, err, errs.CodeValidationFailed)
}

// TestSetAllPaths: duplicates count once, an empty given list switches nothing, an unknown id is
// refused on its index, personal needs a list, and organization without a list switches every
// personal agent.
func TestSetAllPaths(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	a := w.create(owner, "assistent")
	b := w.create(owner, "lotse")
	changed, err := w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{
		BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, AgentIDs: []ids.ID{b.ID, a.ID, b.ID}, AgentIDsGiven: true, Reason: "Own",
	})
	want := []ids.ID{a.ID, b.ID}
	slices.SortFunc(want, func(x, y ids.ID) int { return slices.Compare(x[:], y[:]) })
	if err != nil || !slices.Equal(changed, want) {
		t.Fatalf("personal %v, %v", changed, err)
	}
	changed, err = w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{BillingMode: agents.BillingOrganization, AgentIDsGiven: true, Reason: "None"})
	if err != nil || len(changed) != 0 {
		t.Fatalf("empty list %v, %v", changed, err)
	}
	_, err = w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{
		BillingMode: agents.BillingOrganization, AgentIDs: []ids.ID{a.ID, ids.New()}, AgentIDsGiven: true, Reason: "x",
	})
	wantCode(t, err, errs.CodeValidationFailed)
	if e, _ := errs.As(err); len(e.Fields) != 1 || e.Fields[0].Pointer != "/agent_ids/1" {
		t.Fatalf("fields %+v", e.Fields)
	}
	_, err = w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, Reason: "x"})
	wantCode(t, err, errs.CodeValidationFailed)
	changed, err = w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{BillingMode: agents.BillingOrganization, Reason: "Emergency"})
	if err != nil || !slices.Equal(changed, want) {
		t.Fatalf("organization %v, %v", changed, err)
	}
	sam := w.person("sam", false)
	_, err = w.svc.SetBillingModeAll(ctx, sam, agents.BillingModeAllCmd{BillingMode: agents.BillingOrganization, Reason: "x"})
	wantCode(t, err, errs.CodeForbidden)
}

// TestListPaging: the owner pages through more agents than a batch, a caller who sees only the
// last agent still finds it behind invisible candidates, a foreign cursor is refused and an
// anonymous caller is unauthenticated.
func TestListPaging(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	sam := w.person("sam", false)
	var all []ids.ID
	for _, h := range []string{"a1", "a2", "a3", "a4", "a5"} {
		all = append(all, w.create(owner, h).ID)
	}
	ch := ids.New()
	space := ids.New()
	w.structure.PutSpace(space)
	w.structure.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: all[4]})
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, reason, granted_by)
		VALUES ($1, 'user', $2, 'channel', $3, 'read', 'fixture', $2)`, ids.New().String(), sam.ID.String(), ch.String())
	var seen []ids.ID
	cursor := ""
	for range 5 {
		p, err := w.svc.List(ctx, owner, agents.Page{Cursor: cursor, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range p.Items {
			seen = append(seen, a.ID)
		}
		if cursor = p.NextCursor; cursor == "" {
			break
		}
		if len(p.Items) != 3 {
			t.Fatalf("page size %d, want the maximum 3", len(p.Items))
		}
	}
	if !slices.Equal(seen, all) {
		t.Fatalf("owner saw %v, want %v", seen, all)
	}
	p, err := w.svc.List(ctx, sam, agents.Page{Limit: 1})
	if err != nil || len(p.Items) != 1 || p.Items[0].ID != all[4] || p.NextCursor != "" {
		t.Fatalf("Sam %+v, %v", p, err)
	}
	first, err := w.svc.List(ctx, owner, agents.Page{Limit: 1})
	if err != nil || first.NextCursor == "" {
		t.Fatal(err)
	}
	_, err = w.svc.List(ctx, sam, agents.Page{Cursor: first.NextCursor})
	wantCode(t, err, errs.CodeCursorInvalid)
	_, err = w.svc.List(ctx, actor.Anonymous(), agents.Page{})
	wantCode(t, err, errs.CodeUnauthenticated)
}
