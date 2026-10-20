// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// text runs a query as the database owner and returns its single text value.
func (w *world) text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// policyRow is the policy part of the agents row of id.
func (w *world) policyRow(id ids.ID) string {
	w.t.Helper()
	return w.text(`SELECT billing_mode || ':' || coalesce(credential_holder_id::text, '-') || ':' ||
		coalesce(billing_mode_changed_at::text, '-') || ':' || coalesce(billing_mode_changed_by::text, '-') || ':' || version
		FROM agents WHERE principal_id = $1`, id.String())
}

// lastUpdated returns the payload of the newest agent.updated of id without the members that
// name the agent (id, handle, display name), so payloads of different agents compare.
func (w *world) lastUpdated(id ids.ID) map[string]any {
	w.t.Helper()
	raw := w.text(`SELECT coalesce((SELECT payload::text FROM event_log WHERE type = 'agent.updated' AND resource_id = $1 ORDER BY seq DESC LIMIT 1), '')`, id.String())
	if raw == "" {
		return nil
	}
	var p struct {
		Agent map[string]any `json:"agent"`
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		w.t.Fatal(err)
	}
	for _, k := range []string{"id", "handle", "display_name"} {
		delete(p.Agent, k)
	}
	return p.Agent
}

// updates counts the agent.updated events of id.
func (w *world) updates(id ids.ID) string {
	w.t.Helper()
	return w.text(`SELECT count(*)::text FROM event_log WHERE type = 'agent.updated' AND resource_id = $1`, id.String())
}

// auditDetails returns the details of the allow entries of action on resource (empty for the
// instance), oldest first.
func (w *world) auditDetails(action string, resource ids.ID) []map[string]any {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT details::text FROM audit_events WHERE action = $1 AND decision = 'allow'
		AND coalesce(resource_id::text, '') = $2 ORDER BY seq`, action, idText(resource))
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			w.t.Fatal(err)
		}
		var d map[string]any
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

// idText is the text of id, empty for the zero id.
func idText(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// personal switches a to personal with holder as o.
func (w *world) personal(o actor.Actor, a agents.Agent, holder ids.ID) agents.Agent {
	w.t.Helper()
	got, err := w.svc.SetBillingMode(context.Background(), o, a.ID, agents.BillingModeCmd{
		BillingMode: agents.BillingPersonal, CredentialHolderID: holder, Reason: "Own", IfMatch: match(a.Version),
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return got
}

// switchInUnit runs SwitchToOrganization as by inside a unit of work declared as the rights
// change agent.billing_mode.set of list, the way authz runs it in the guided flow.
func (w *world) switchInUnit(by actor.Actor, list []ids.ID, reason string) error {
	ctx := actor.WithActor(context.Background(), by)
	return w.uow.Do(ctx, func(ctx context.Context) error {
		return w.svc.SwitchToOrganization(ctx, by, list, reason)
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "agent.billing_mode.set", Resource: txn.Ref{Kind: "agent", ID: list[0]}, Principals: list}))
}

// TestSwitchToOrganization (M2-21 design note 4): personal to organization clears the holder and
// sets changed-at, changed-by and the version, with one allow entry per agent; an agent on
// organization keeps its row and gets an entry with no_change; a disabled personal agent is
// switched; outside a unit of work the switch refuses.
func TestSwitchToOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	a := w.personal(owner, w.create(owner, "assistent"), owner.ID)
	b := w.create(owner, "lotse")
	c := w.personal(owner, w.create(owner, "texter"), owner.ID)
	if _, err := w.svc.Disable(ctx, owner, c.ID, "Pause"); err != nil {
		t.Fatal(err)
	}
	beforeB, eventsB := w.policyRow(b.ID), w.updates(b.ID)
	if err := w.switchInUnit(owner, []ids.ID{a.ID, b.ID, c.ID}, "Guided flow"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []ids.ID{a.ID, c.ID} {
		got, err := w.svc.Get(ctx, owner, id)
		if err != nil || got.BillingMode != agents.BillingOrganization || !got.CredentialHolderID.IsZero() || got.BillingModeChangedAt.IsZero() {
			t.Fatalf("switched agent %+v, %v", got, err)
		}
		if row := w.policyRow(id); !strings.HasSuffix(row, ":"+owner.ID.String()+":"+itoa(got.Version)) {
			t.Fatalf("row %s", row)
		}
		d := w.auditDetails("agent.billing_mode.set", id)
		last := d[len(d)-1]
		if last["from_billing_mode"] != "personal" || last["to_billing_mode"] != "organization" ||
			last["from_credential_holder_id"] != owner.ID.String() || last["reason"] != "Guided flow" || last["no_change"] != nil {
			t.Fatalf("audit %v", last)
		}
	}
	if w.policyRow(b.ID) != beforeB || w.updates(b.ID) != eventsB {
		t.Fatalf("agent on organization changed: %s", w.policyRow(b.ID))
	}
	if d := w.auditDetails("agent.billing_mode.set", b.ID); len(d) != 1 || d[0]["no_change"] != true || d[0]["reason"] != "Guided flow" {
		t.Fatalf("no-change entry %v", d)
	}
	wantCode(t, w.svc.SwitchToOrganization(ctx, owner, []ids.ID{a.ID}, "x"), errs.CodeInternalError)
	wantCode(t, w.switchInUnit(owner, []ids.ID{ids.New()}, "x"), errs.CodeNotFound)
}

// TestSwitchPathsShareApplyStep (M2-21 design note 4): the single switch, set_all to organization
// and SwitchToOrganization write equal rows and equal agent.updated payloads for agents in the
// same state; their audit entries differ only as the note says.
func TestSwitchPathsShareApplyStep(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	single := w.personal(owner, w.create(owner, "single"), owner.ID)
	all := w.personal(owner, w.create(owner, "all"), owner.ID)
	guided := w.personal(owner, w.create(owner, "guided"), owner.ID)
	if _, err := w.svc.SetBillingMode(ctx, owner, single.ID, agents.BillingModeCmd{BillingMode: agents.BillingOrganization, Reason: "Same", IfMatch: match(single.Version)}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{BillingMode: agents.BillingOrganization, AgentIDs: []ids.ID{all.ID}, AgentIDsGiven: true, Reason: "Same"}); err != nil {
		t.Fatal(err)
	}
	if err := w.switchInUnit(owner, []ids.ID{guided.ID}, "Same"); err != nil {
		t.Fatal(err)
	}
	rowOf := func(id ids.ID) string { return w.policyRow(id) }
	if rowOf(single.ID) != rowOf(all.ID) || rowOf(all.ID) != rowOf(guided.ID) {
		t.Fatalf("rows differ:\n%s\n%s\n%s", rowOf(single.ID), rowOf(all.ID), rowOf(guided.ID))
	}
	ps, pa, pg := w.lastUpdated(single.ID), w.lastUpdated(all.ID), w.lastUpdated(guided.ID)
	if !maps.EqualFunc(ps, pa, jsonEqual) || !maps.EqualFunc(pa, pg, jsonEqual) {
		t.Fatalf("payloads differ:\n%v\n%v\n%v", ps, pa, pg)
	}
	ds, dg := w.auditDetails("agent.billing_mode.set", single.ID), w.auditDetails("agent.billing_mode.set", guided.ID)
	if !maps.EqualFunc(ds[len(ds)-1], dg[len(dg)-1], jsonEqual) {
		t.Fatalf("single and guided audit differ: %v %v", ds[len(ds)-1], dg[len(dg)-1])
	}
	da := w.auditDetails("agent.billing_mode.set_all", ids.ID{})
	if last := da[len(da)-1]; strs(last["agent_ids"]) != all.ID.String() || last["reason"] != "Same" {
		t.Fatalf("set_all audit %v", last)
	}
}

// TestReasonRulesOfPolicyBodies (M2-21 design note 2): the reasons of BillingModeSet,
// BillingModeSetAll and CredentialHolderSet have at most audit.reason_max_chars characters.
func TestReasonRulesOfPolicyBodies(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	a := w.create(owner, "assistent")
	long := strings.Repeat("ä", 41)
	_, err := w.svc.SetBillingMode(ctx, owner, a.ID, agents.BillingModeCmd{BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, Reason: long, IfMatch: match(1)})
	wantTooLong(t, err)
	_, err = w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{BillingMode: agents.BillingOrganization, Reason: long})
	wantTooLong(t, err)
	a = w.personal(owner, a, owner.ID)
	_, err = w.svc.SetCredentialHolder(ctx, owner, a.ID, agents.CredentialHolderCmd{CredentialHolderID: owner.ID, Reason: long, IfMatch: match(a.Version)})
	wantTooLong(t, err)
	if _, err := w.svc.SetBillingMode(ctx, owner, a.ID, agents.BillingModeCmd{BillingMode: agents.BillingOrganization, Reason: strings.Repeat("ä", 40), IfMatch: match(a.Version)}); err != nil {
		t.Fatalf("a reason of exactly audit.reason_max_chars: %v", err)
	}
}

// wantTooLong checks 400 validation_failed too_long at /reason.
func wantTooLong(t *testing.T, err error) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "/reason" || e.Fields[0].Code != "too_long" {
		t.Fatalf("got %v, want too_long at /reason", err)
	}
}

// jsonEqual compares decoded JSON values.
func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// strs joins a decoded JSON array of strings.
func strs(v any) string {
	list, _ := v.([]any)
	var out []string
	for _, e := range list {
		s, _ := e.(string)
		out = append(out, s)
	}
	return strings.Join(out, ",")
}

// itoa formats v.
func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
