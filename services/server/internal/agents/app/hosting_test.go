// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The hosted agents of docs/spec/26-builtin-agents.md 3 (A1-03) over the real identity, authz,
// sync and audit modules: BA-T04 (hosting part) to BA-T07 and BR-14 of chapter 10.

// testJKT and otherJKT are well-formed RFC 7638 thumbprints (43 base64url characters), no real keys.
const (
	testJKT  = "EXAMPLEexampleEXAMPLEexampleEXAMPLEexample0"
	otherJKT = "OTHERotherOTHERotherOTHERotherOTHERotherOT1"
)

// runner creates a Claude Code runner as o; a ceiling other than "" is stored as the capability
// hosts_agents of its last heartbeat, with both billing modes supported.
func (w *world) runner(o actor.Actor, handle, ceiling string) agents.Agent {
	w.t.Helper()
	a, err := w.svc.Create(context.Background(), o, agents.CreateCmd{Handle: handle, DisplayName: handle, AdapterKind: "code_runner"})
	if err != nil {
		w.t.Fatal(err)
	}
	if ceiling != "" {
		w.sql(`UPDATE agents SET reported_details = jsonb_build_object('capabilities', jsonb_build_object('hosts_agents', $2::text),
			'billing_modes_supported', '["organization","personal"]'::jsonb), last_heartbeat_at = now() WHERE principal_id = $1`,
			a.ID.String(), ceiling)
	}
	return a
}

// hosted creates a built-in agent hosted by host as o.
func (w *world) hosted(o actor.Actor, handle string, host ids.ID) agents.Agent {
	w.t.Helper()
	a, err := w.svc.Create(context.Background(), o, agents.CreateCmd{Handle: handle, DisplayName: handle, AdapterKind: "claude_agent", HostedBy: host})
	if err != nil {
		w.t.Fatal(err)
	}
	return a
}

// tokenActor issues a real API token of agent id bound to jkt ("" for none) and returns the agent
// as it authenticates with that token.
func (w *world) tokenActor(id ids.ID, jkt string) actor.Actor {
	w.t.Helper()
	issued, err := w.id.Tokens.Issue(context.Background(), identity.IssueTokenCmd{
		PrincipalID: id, Name: "Adapter", DPoPJKT: jkt, ExpiresAt: start.Add(time.Hour), CreatedBy: id,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	w.creds.PutToken(issued.Token.ID, fake.Token{PrincipalID: id, ExpiresAt: start.Add(time.Hour)})
	return actor.Actor{ID: id, Kind: actor.KindAgent, TokenID: issued.Token.ID}
}

// hostingEvents counts agent.hosting_changed to runner about agent.
func (w *world) hostingEvents(runner, agent ids.ID) int {
	return w.count(`SELECT count(*)::int FROM event_log WHERE type = 'agent.hosting_changed' AND audience_principal_id = $1
		AND resource_id = $1 AND receive_action = 'agent.hosted.list' AND payload->>'agent_id' = $2`, runner.String(), agent.String())
}

// activeTokens counts the active tokens of holder created by issuer.
func (w *world) activeTokens(holder, issuer ids.ID) int {
	return w.count(`SELECT count(*)::int FROM api_tokens WHERE principal_id = $1 AND created_by = $2 AND revoked_at IS NULL`,
		holder.String(), issuer.String())
}

// hostedDenials flushes the aggregated denials and counts those of action with reason ("" for any) on
// resource.
func (w *world) hostedDenials(action, reason string, resource ids.ID) int {
	w.t.Helper()
	if err := w.rec.Aggregator().Flush(context.Background()); err != nil {
		w.t.Fatal(err)
	}
	return w.count(`SELECT count(*)::int FROM audit_events WHERE action = $1 AND decision = 'deny' AND ($2 = '' OR deny_reason = $2)
		AND resource_id = $3`, action, reason, resource.String())
}

// wantHostedByError requires validation_failed with one field error at /hosted_by with code.
func wantHostedByError(t *testing.T, err error, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "/hosted_by" || e.Fields[0].Code != code {
		t.Fatalf("error %v, want validation_failed /hosted_by %s", err, code)
	}
}

// wantConflict requires 409 conflict with reason.
func wantConflict(t *testing.T, err error, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeConflict || e.Reason != reason {
		t.Fatalf("error %v, want conflict %s", err, reason)
	}
}

// TestCreateHostedAgent (BA-T07, R-DM-58, 26 3.1): a built-in agent is created with a capable
// runner, carries hosted_by and tells its runner with agent.hosting_changed; every other runner
// value is host_not_capable on /hosted_by, claude_agent without a runner is required and a runner
// for another kind not_allowed. Nothing is created by a refused call.
func TestCreateHostedAgent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	r := w.runner(owner, "runner", "scoped")
	a := w.hosted(owner, "writer", r.ID)
	if a.HostedBy != r.ID || a.AdapterKind != "claude_agent" || a.ToolClass != agents.ToolChat || a.BillingMode != agents.BillingOrganization {
		t.Fatalf("created %+v", a)
	}
	if n := w.hostingEvents(r.ID, a.ID); n != 1 {
		t.Fatalf("agent.hosting_changed to the runner: %d", n)
	}
	if d := w.auditDetails("agent.create", a.ID); len(d) != 1 || d[0]["hosted_by"] != r.ID.String() {
		t.Fatalf("audit %v", d)
	}
	noCap := w.runner(owner, "plain-runner", "")
	other := w.create(owner, "assistant")
	w.sql(`UPDATE agents SET reported_details = '{"capabilities":{"hosts_agents":"host"}}' WHERE principal_id = $1`, other.ID.String())
	gone := w.runner(owner, "gone-runner", "host")
	if _, err := w.svc.Disable(ctx, owner, gone.ID, "Retired"); err != nil {
		t.Fatal(err)
	}
	before := w.count(`SELECT count(*)::int FROM agents`)
	for name, host := range map[string]ids.ID{
		"a runner without hosts_agents": noCap.ID, "a hosted agent": a.ID, "an agent of another runtime": other.ID,
		"a disabled runner": gone.ID, "an unknown id": ids.New(), "the creating person": owner.ID,
	} {
		_, err := w.svc.Create(ctx, owner, agents.CreateCmd{Handle: "x", DisplayName: "x", AdapterKind: "claude_agent", HostedBy: host})
		t.Run(name, func(t *testing.T) { wantHostedByError(t, err, "host_not_capable") })
	}
	_, err := w.svc.Create(ctx, owner, agents.CreateCmd{Handle: "x", DisplayName: "x", AdapterKind: "claude_agent"})
	wantHostedByError(t, err, "required")
	_, err = w.svc.Create(ctx, owner, agents.CreateCmd{Handle: "x", DisplayName: "x", AdapterKind: "hermes", HostedBy: r.ID})
	wantHostedByError(t, err, "not_allowed")
	if after := w.count(`SELECT count(*)::int FROM agents`); after != before {
		t.Fatalf("refused calls created %d agents", after-before)
	}
}

// TestSetHost (26 3.1 item 4, BA-T04, BA-T06): the owner moves a built-in agent; the tokens the
// old runner holds end in the same transaction, the agent's other tokens and the runners' own
// stay, and both runners get agent.hosting_changed. The agent itself, a runner without the
// capability, an agent that is not built in, a runner below the agent's class and a caller who is
// not an owner are refused.
func TestSetHost(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	alex := w.person("alex", false)
	r1, r2 := w.runner(owner, "runner-one", "host"), w.runner(owner, "runner-two", "scoped")
	a := w.hosted(owner, "writer", r1.ID)
	run1 := w.tokenActor(r1.ID, testJKT)
	if _, err := w.svc.IssueHostedToken(ctx, run1, a.ID); err != nil {
		t.Fatal(err)
	}
	w.tokenActor(a.ID, "")
	set := func(by actor.Actor, id, host ids.ID, version int64) (agents.Agent, error) {
		return w.svc.SetHost(ctx, by, id, agents.HostCmd{HostedBy: host, Reason: "Move", IfMatch: match(version)})
	}
	// 07 2.13: owner only; the agent is not visible to them, so the answer is 404 (07 4.3).
	if _, err := set(alex, a.ID, r2.ID, a.Version); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("a person who is not an owner: %v", err)
	}
	if _, err := set(run1, a.ID, r2.ID, a.Version); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("the hosting runner: %v", err)
	}
	_, err := set(owner, a.ID, a.ID, a.Version)
	wantHostedByError(t, err, "host_not_capable")
	_, err = set(owner, r2.ID, r1.ID, r2.Version)
	wantConflict(t, err, "not_a_builtin_agent")
	moved, err := set(owner, a.ID, r2.ID, a.Version)
	if err != nil {
		t.Fatal(err)
	}
	if moved.HostedBy != r2.ID || moved.Version != a.Version+1 {
		t.Fatalf("moved %+v", moved)
	}
	if n := w.activeTokens(a.ID, r1.ID); n != 0 {
		t.Fatalf("tokens of the old runner after the move: %d", n)
	}
	if n := w.activeTokens(a.ID, a.ID); n != 1 {
		t.Fatalf("the agent's own token: %d", n)
	}
	if n := w.activeTokens(r1.ID, r1.ID); n != 1 {
		t.Fatalf("the old runner's own token: %d", n)
	}
	if w.hostingEvents(r1.ID, a.ID) != 2 || w.hostingEvents(r2.ID, a.ID) != 1 {
		t.Fatalf("agent.hosting_changed: old %d, new %d", w.hostingEvents(r1.ID, a.ID), w.hostingEvents(r2.ID, a.ID))
	}
	d := w.auditDetails("agent.host.set", a.ID)
	if len(d) != 1 || d[0]["from_hosted_by"] != r1.ID.String() || d[0]["to_hosted_by"] != r2.ID.String() || d[0]["revoked_tokens"] != float64(1) {
		t.Fatalf("audit %v", d)
	}
	same, err := set(owner, a.ID, r2.ID, moved.Version)
	if err != nil || same.Version != moved.Version {
		t.Fatalf("the same runner: %+v %v", same, err)
	}
	// Ceiling: an agent of class host cannot move to a runner whose ceiling is scoped.
	b := w.hosted(owner, "builder", r1.ID)
	b, err = w.svc.SetToolClass(ctx, owner, b.ID, agents.ToolClassCmd{ToolClass: agents.ToolHost, Reason: "Shell", IfMatch: match(b.Version)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = set(owner, b.ID, r2.ID, b.Version)
	wantConflict(t, err, "host_tool_class_ceiling")
}

// TestToolClassCeiling (26 3.1 item 3): agent.tool_class.set above the ceiling of the hosting
// runner is host_tool_class_ceiling; a runner that stopped reporting hosts_agents admits only chat
// (D-A1-03-6); agents that are not built in are not limited.
func TestToolClassCeiling(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	r := w.runner(owner, "runner", "scoped")
	a := w.hosted(owner, "writer", r.ID)
	setClass := func(id ids.ID, class string, version int64) (agents.Agent, error) {
		return w.svc.SetToolClass(ctx, owner, id, agents.ToolClassCmd{ToolClass: class, Reason: "Tools", IfMatch: match(version)})
	}
	a, err := setClass(a.ID, agents.ToolScoped, a.Version)
	if err != nil {
		t.Fatal(err)
	}
	_, err = setClass(a.ID, agents.ToolHost, a.Version)
	wantConflict(t, err, "host_tool_class_ceiling")
	w.sql(`UPDATE agents SET reported_details = '{}' WHERE principal_id = $1`, r.ID.String())
	if _, err := setClass(a.ID, agents.ToolChat, a.Version); err != nil {
		t.Fatalf("down to chat under a silent runner: %v", err)
	}
	other := w.create(owner, "assistant")
	if _, err := setClass(other.ID, agents.ToolHost, other.Version); err != nil {
		t.Fatalf("an agent that is not built in: %v", err)
	}
}

// TestHostedBilling (BR-14, 10 section 10.3): personal for a built-in agent only with its runner's
// holder; a runner that changes its own mode or holder while a hosted agent keeps the old holder
// is refused; moving a personal agent to a runner with another holder is refused; switching both
// together to organization works. Every refusal changes nothing.
func TestHostedBilling(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	sam := w.person("sam", false)
	r := w.runner(owner, "runner", "scoped")
	a := w.hosted(owner, "writer", r.ID)
	billing := func(id ids.ID, mode string, holder ids.ID, version int64) (agents.Agent, error) {
		return w.svc.SetBillingMode(ctx, owner, id, agents.BillingModeCmd{BillingMode: mode, CredentialHolderID: holder, Reason: "Plan",
			IfMatch: match(version)})
	}
	_, err := billing(a.ID, agents.BillingPersonal, owner.ID, a.Version)
	wantConflict(t, err, "hosted_holder_mismatch")
	r = w.personal(owner, r, owner.ID)
	_, err = billing(a.ID, agents.BillingPersonal, sam.ID, a.Version)
	wantConflict(t, err, "hosted_holder_mismatch")
	a = w.personal(owner, a, owner.ID)
	snapshot := w.policyRow(r.ID)
	_, err = billing(r.ID, agents.BillingOrganization, ids.ID{}, r.Version)
	wantConflict(t, err, "hosted_holder_mismatch")
	_, err = w.svc.SetCredentialHolder(ctx, owner, r.ID, agents.CredentialHolderCmd{CredentialHolderID: sam.ID, Reason: "Plan", IfMatch: match(r.Version)})
	wantConflict(t, err, "hosted_holder_mismatch")
	_, err = w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{BillingMode: agents.BillingOrganization, AgentIDs: []ids.ID{r.ID},
		AgentIDsGiven: true, Reason: "Plan"})
	wantConflict(t, err, "hosted_holder_mismatch")
	if err := w.switchInUnit(owner, []ids.ID{r.ID}, "Plan"); err == nil {
		t.Fatal("guided switch of the runner alone succeeded")
	}
	if got := w.policyRow(r.ID); got != snapshot {
		t.Fatalf("runner changed by refused calls: %s -> %s", snapshot, got)
	}
	r2 := w.runner(owner, "runner-two", "scoped")
	_, err = w.svc.SetHost(ctx, owner, a.ID, agents.HostCmd{HostedBy: r2.ID, Reason: "Move", IfMatch: match(a.Version)})
	wantConflict(t, err, "hosted_holder_mismatch")
	changed, err := w.svc.SetBillingModeAll(ctx, owner, agents.BillingModeAllCmd{BillingMode: agents.BillingOrganization, Reason: "Plan"})
	if err != nil || len(changed) != 2 {
		t.Fatalf("both to organization: %v %v", changed, err)
	}
}

// TestListHosted (26 3.2, BA-T04): a runner lists the active agents it hosts in pages; another
// runner sees none of them; a person cannot call it.
func TestListHosted(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	r1, r2 := w.runner(owner, "runner-one", "chat"), w.runner(owner, "runner-two", "chat")
	a, b, c := w.hosted(owner, "one", r1.ID), w.hosted(owner, "two", r1.ID), w.hosted(owner, "three", r1.ID)
	w.hosted(owner, "four", r2.ID)
	if _, err := w.svc.Disable(ctx, owner, b.ID, "Pause"); err != nil {
		t.Fatal(err)
	}
	run1 := actor.Actor{ID: r1.ID, Kind: actor.KindAgent}
	page, err := w.svc.ListHosted(ctx, run1, agents.Page{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("first page %+v %v", page, err)
	}
	rest, err := w.svc.ListHosted(ctx, run1, agents.Page{Cursor: page.NextCursor, Limit: 3})
	if err != nil || len(rest.Items) != 1 || rest.NextCursor != "" {
		t.Fatalf("second page %+v %v", rest, err)
	}
	got := map[ids.ID]bool{page.Items[0].ID: true, rest.Items[0].ID: true}
	if !got[a.ID] || !got[c.ID] || got[b.ID] {
		t.Fatalf("listed %v", got)
	}
	if none, err := w.svc.ListHosted(ctx, actor.Actor{ID: a.ID, Kind: actor.KindAgent}, agents.Page{}); err != nil || len(none.Items) != 0 {
		t.Fatalf("an agent that hosts nothing: %+v %v", none, err)
	}
	if _, err := w.svc.ListHosted(ctx, owner, agents.Page{}); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("a person: %v", err)
	}
}

// TestIssueHostedToken (BA-T05, 26 3.3 item 1): the hosting runner gets a token of kind agt for
// the agent, bound to the key of its calling token, expiring after agents.hosted_token_ttl, with
// one allow entry without the value; a second issue revokes the first. Another runner, a runner
// for an agent hosted elsewhere, a disabled agent, a person, the agent itself and a runner without
// hosts_agents get not_found with a deny entry; a calling token without a key is dpop_required.
func TestIssueHostedToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	r1, r2 := w.runner(owner, "runner-one", "scoped"), w.runner(owner, "runner-two", "scoped")
	a, elsewhere := w.hosted(owner, "writer", r1.ID), w.hosted(owner, "reader", r2.ID)
	run1, run2 := w.tokenActor(r1.ID, testJKT), w.tokenActor(r2.ID, otherJKT)
	first, err := w.svc.IssueHostedToken(ctx, run1, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	tok := first.Token
	if tok.PrincipalID != a.ID || tok.CreatedBy != r1.ID || tok.DPoPJKT != testJKT || tok.Kind != identity.TokenKindAgent ||
		!tok.ExpiresAt.Equal(start.Add(hostedTTL)) || tok.Name != "hosted by runner-one" || first.Secret.IsZero() {
		t.Fatalf("issued %+v", tok)
	}
	d := w.auditDetails("agent.hosted_token.issue", a.ID)
	if len(d) != 1 || d[0]["hosted_by"] != r1.ID.String() || d[0]["token_id"] != tok.ID.String() {
		t.Fatalf("audit %v", d)
	}
	raw := w.text(`SELECT details::text FROM audit_events WHERE action = 'agent.hosted_token.issue'`)
	if secret := first.Secret.Reveal(); len(raw) == 0 || strings.Contains(raw, secret) || strings.Contains(raw, secret[len(secret)-12:]) {
		t.Fatalf("the audit entry holds the token: %s", raw)
	}
	second, err := w.svc.IssueHostedToken(ctx, run1, a.ID)
	if err != nil || second.Token.ID == tok.ID {
		t.Fatalf("second issue %+v %v", second.Token, err)
	}
	if n := w.activeTokens(a.ID, r1.ID); n != 1 {
		t.Fatalf("active hosted tokens after the second issue: %d", n)
	}
	gone := w.hosted(owner, "gone", r1.ID)
	if _, err := w.svc.Disable(ctx, owner, gone.ID, "Pause"); err != nil {
		t.Fatal(err)
	}
	noCap := w.runner(owner, "plain-runner", "")
	silent := w.hosted(owner, "silent", r1.ID)
	w.sql(`UPDATE agents SET hosted_by = $2 WHERE principal_id = $1`, silent.ID.String(), noCap.ID.String())
	for _, c := range []struct {
		name   string
		by     actor.Actor
		target ids.ID
		reason string
	}{
		{"another runner", run2, a.ID, ""},
		{"an agent hosted elsewhere", run1, elsewhere.ID, ""},
		{"a disabled agent", run1, gone.ID, "principal_inactive"},
		{"the agent itself", w.tokenActor(a.ID, testJKT), a.ID, "not_self"},
		{"a runner without hosts_agents", w.tokenActor(noCap.ID, testJKT), silent.ID, "capability_missing"},
		{"an unknown agent", run1, ids.New(), ""},
		{"a person", owner, a.ID, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := w.svc.IssueHostedToken(ctx, c.by, c.target)
			if errs.KindOf(err) != errs.KindNotFound {
				t.Fatalf("error %v, want not_found", err)
			}
			// authz audits its own refusals (another runner, a person) with its reason.
			if w.hostedDenials("agent.hosted_token.issue", c.reason, c.target) == 0 {
				t.Fatalf("no deny entry %q", c.reason)
			}
		})
	}
	if w.hostedDenials("agent.hosted_token.issue", "subject_kind_not_allowed", a.ID) == 0 {
		t.Fatal("no deny entry for the person")
	}
	unbound := w.tokenActor(r1.ID, "")
	_, err = w.svc.IssueHostedToken(ctx, unbound, a.ID)
	wantConflict(t, err, "dpop_required")
	if n := w.activeTokens(a.ID, r1.ID); n != 1 {
		t.Fatalf("active hosted tokens after the refusals: %d", n)
	}
}

// TestHostedRevocation (BA-T06, 26 3.3 item 3): each cause revokes exactly the hosted tokens it
// names in the same transaction: a revoked runner token those with its key, a disabled runner all
// it issued, a disabled hosted agent its own; the runner's own tokens survive a hosted agent's
// disable. createToken for a built-in agent is refused (hosted_agent_token, through the bound
// source of identity).
func TestHostedRevocation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	r := w.runner(owner, "runner", "scoped")
	a, b := w.hosted(owner, "writer", r.ID), w.hosted(owner, "reader", r.ID)
	keyOne, keyTwo := w.tokenActor(r.ID, testJKT), w.tokenActor(r.ID, otherJKT)
	for _, by := range []actor.Actor{keyOne, keyTwo} {
		for _, target := range []ids.ID{a.ID, b.ID} {
			if _, err := w.svc.IssueHostedToken(ctx, by, target); err != nil {
				t.Fatal(err)
			}
		}
	}
	// One active token per agent and runner (design note 3 of the brief): the newest, bound to the
	// second key.
	if w.activeTokens(a.ID, r.ID) != 1 || w.activeTokens(b.ID, r.ID) != 1 {
		t.Fatalf("one hosted token per agent and runner: %d %d", w.activeTokens(a.ID, r.ID), w.activeTokens(b.ID, r.ID))
	}
	if err := w.id.Tokens.Revoke(ctx, keyOne.TokenID, owner.ID); err != nil {
		t.Fatal(err)
	}
	if w.activeTokens(a.ID, r.ID) != 1 || w.activeTokens(b.ID, r.ID) != 1 {
		t.Fatal("a revoked runner token took hosted tokens bound to another key")
	}
	if err := w.id.Tokens.Revoke(ctx, keyTwo.TokenID, owner.ID); err != nil {
		t.Fatal(err)
	}
	if w.activeTokens(a.ID, r.ID) != 0 || w.activeTokens(b.ID, r.ID) != 0 {
		t.Fatal("a revoked runner token left hosted tokens with its key")
	}
	keyThree := w.tokenActor(r.ID, testJKT)
	for _, target := range []ids.ID{a.ID, b.ID} {
		if _, err := w.svc.IssueHostedToken(ctx, keyThree, target); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.svc.Disable(ctx, owner, a.ID, "Pause"); err != nil {
		t.Fatal(err)
	}
	if w.activeTokens(a.ID, r.ID) != 0 || w.activeTokens(b.ID, r.ID) != 1 || w.activeTokens(r.ID, r.ID) != 1 {
		t.Fatalf("after the hosted agent's disable: %d %d %d", w.activeTokens(a.ID, r.ID), w.activeTokens(b.ID, r.ID), w.activeTokens(r.ID, r.ID))
	}
	if w.hostingEvents(r.ID, a.ID) != 2 {
		t.Fatalf("agent.hosting_changed on disable: %d", w.hostingEvents(r.ID, a.ID))
	}
	if _, err := w.svc.Disable(ctx, owner, r.ID, "Retired"); err != nil {
		t.Fatal(err)
	}
	if w.activeTokens(b.ID, r.ID) != 0 {
		t.Fatal("a disabled runner left a hosted token")
	}
	if d := w.auditDetails("agent.disable", r.ID); len(d) != 1 || d[0]["revoked_hosted_tokens"] != float64(1) {
		t.Fatalf("audit of the runner's disable %v", d)
	}
	_, err := w.id.Tokens.CreateToken(ctx, owner, b.ID, identity.CreateTokenCmd{Name: "Manual", DPoPJKT: testJKT, ExpiresAt: start.Add(time.Hour)})
	wantConflict(t, err, "hosted_agent_token")
	plain := w.create(owner, "assistant")
	if _, err := w.id.Tokens.CreateToken(ctx, owner, plain.ID, identity.CreateTokenCmd{Name: "Manual", DPoPJKT: testJKT,
		ExpiresAt: start.Add(time.Hour)}); err != nil {
		t.Fatalf("createToken for an agent that is not built in: %v", err)
	}
}
