// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestDeliveryMatrix checks every cell of M2-08 design note 2.
func TestDeliveryMatrix(t *testing.T) {
	t.Parallel()
	q, r, w := domain.StateQueued, domain.StateRunning, domain.StateWaitingInput
	type cell struct {
		typ    string
		state  domain.State
		record bool
		denial domain.DeliveryDenial
	}
	cells := []cell{
		{domain.EventAgentInvocation, q, true, domain.DenyFailRun},
		{domain.EventAgentInvocation, r, false, domain.DenyKeepRun},
		{domain.EventAgentInvocation, w, false, domain.DenyKeepRun},
		{domain.EventAgentFollowup, q, false, domain.DenyFailFollowup},
		{domain.EventAgentFollowup, r, false, domain.DenyFailFollowup},
		{domain.EventAgentFollowup, w, true, domain.DenyFailFollowup},
		{domain.EventInteractionCreated, q, false, domain.DenyFailFollowup},
		{domain.EventInteractionCreated, r, false, domain.DenyFailFollowup},
		{domain.EventInteractionCreated, w, true, domain.DenyFailFollowup},
		{domain.EventCommandInvoked, q, false, domain.DenyKeepRun},
		{domain.EventCommandInvoked, r, false, domain.DenyKeepRun},
		{domain.EventCommandInvoked, w, false, domain.DenyKeepRun},
	}
	for _, c := range cells {
		got, err := domain.DeliveryMatrix(c.typ, c.state)
		if err != nil || got != (domain.DeliveryRule{Record: c.record, Denial: c.denial}) {
			t.Errorf("%s %s = %+v, %v; want record %v denial %d", c.typ, c.state, got, err, c.record, c.denial)
		}
	}
	for _, typ := range []string{domain.EventAgentInvocation, domain.EventAgentFollowup, domain.EventInteractionCreated, domain.EventCommandInvoked} {
		if !domain.InvocationBearing(typ) {
			t.Errorf("%s is invocation-bearing", typ)
		}
		for _, st := range []domain.State{domain.StateCompleted, domain.StateFailed, domain.StateCancelled} {
			if got, err := domain.DeliveryMatrix(typ, st); err != nil || got != (domain.DeliveryRule{Skip: true}) {
				t.Errorf("%s %s = %+v, %v; want skip", typ, st, got, err)
			}
		}
	}
	for _, typ := range []string{"run.cancel_requested", "agent.config_changed", "message.created", ""} {
		if domain.InvocationBearing(typ) {
			t.Errorf("%s bears no invocation", typ)
		}
		if _, err := domain.DeliveryMatrix(typ, q); !errors.Is(err, domain.ErrUnknownDelivery) {
			t.Errorf("%s: err %v", typ, err)
		}
	}
	if _, err := domain.DeliveryMatrix(domain.EventAgentInvocation, "bogus"); !errors.Is(err, domain.ErrUnknownDelivery) {
		t.Errorf("unknown state: err %v", err)
	}
}

// TestDeliveryReasonMapping checks every row of M2-08 design note 6; unknown reasons give
// access_revoked, and every value is a server reason of failed.
func TestDeliveryReasonMapping(t *testing.T) {
	t.Parallel()
	rows := []struct {
		reason string
		active bool
		want   string
	}{
		{"billing_mode_personal_not_holder", true, "billing_mode_personal_not_holder"},
		{"tool_class_owner_only", true, "tool_class_owner_only"},
		{"project_not_ready", true, "project_not_ready"},
		{"channel_read_only", true, "channel_read_only"},
		{"channel_archived", true, "channel_read_only"},
		{"principal_inactive", true, "user_disabled"},
		{"agent_not_reachable", false, "agent_disabled"},
		{"agent_not_reachable", true, "access_revoked"},
		{"no_grant", true, "access_revoked"},
		{"level_insufficient", true, "access_revoked"},
		{"origin_not_human", true, "access_revoked"},
		{"run_active_other_agent", true, "access_revoked"},
		{"something_new", false, "access_revoked"},
	}
	for _, r := range rows {
		if got := domain.DeliveryReason(r.reason, r.active); got != r.want {
			t.Errorf("DeliveryReason(%s, %v) = %s, want %s", r.reason, r.active, got, r.want)
		}
	}
	for _, code := range domain.DeliveryReasons() {
		if err := domain.ValidateServerReason(domain.StateFailed, code); err != nil {
			t.Errorf("%s is not a server reason of failed: %v", code, err)
		}
	}
}

// TestQueuedAge: exactly at queued_at + run.queued_max_age the run is too old.
func TestQueuedAge(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	maxAge := 48 * time.Hour
	if domain.QueuedTooOld(at, at.Add(maxAge-time.Microsecond), maxAge) {
		t.Error("too old before the limit")
	}
	if !domain.QueuedTooOld(at, at.Add(maxAge), maxAge) || !domain.QueuedTooOld(at, at.Add(maxAge+time.Hour), maxAge) {
		t.Error("not too old at or after the limit")
	}
}

// TestInvocationClaims checks the claims of a delivery per trigger kind: sub, the triggering
// object, level trigger, the configured tool class, exp - iat = agent.invocation_token_ttl and no
// dev_environment_id.
func TestInvocationClaims(t *testing.T) {
	t.Parallel()
	ttl := 60 * time.Second
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	base := domain.DeliveryFacts{
		RunTriggerKind: "message", RunID: ids.New(), AgentID: ids.New(), TriggeredBy: ids.New(), ObjectID: ids.New(),
		ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(), BillingMode: "organization", ToolClass: "scoped",
		PermissionVersion: 7,
	}
	cases := []struct {
		typ, kind string
		object    func(domain.InvocationClaims) string
	}{
		{domain.EventAgentInvocation, "message", func(c domain.InvocationClaims) string { return c.MessageID }},
		{domain.EventAgentFollowup, "followup", func(c domain.InvocationClaims) string { return c.MessageID }},
		{domain.EventInteractionCreated, "interaction", func(c domain.InvocationClaims) string { return c.InteractionID }},
		{domain.EventCommandInvoked, "command", func(c domain.InvocationClaims) string { return c.CommandID }},
	}
	for _, c := range cases {
		f := base
		f.EventType = c.typ
		g, err := domain.DeliveryGrant(f)
		if err != nil {
			t.Fatalf("%s: %v", c.typ, err)
		}
		claims, err := domain.NewInvocationClaims(g, ids.New(), now, ttl)
		if err != nil {
			t.Fatalf("%s: %v", c.typ, err)
		}
		if claims.TriggerKind != c.kind || claims.Sub != f.TriggeredBy.String() || c.object(claims) != f.ObjectID.String() ||
			claims.Level != "trigger" || claims.ToolClass != "scoped" || claims.Exp-claims.Iat != int64(ttl/time.Second) ||
			claims.DevEnvironmentID != "" || claims.RunID != f.RunID.String() || claims.PermissionVersion != 7 {
			t.Errorf("%s: claims %+v", c.typ, claims)
		}
		raw, _ := json.Marshal(claims)
		if strings.Contains(string(raw), "dev_environment_id") {
			t.Errorf("%s: %s", c.typ, raw)
		}
	}
	f := base
	f.EventType = "message.created"
	if _, err := domain.DeliveryGrant(f); !errors.Is(err, domain.ErrUnknownDelivery) {
		t.Errorf("unknown type: %v", err)
	}
	f.EventType, f.RunTriggerKind = domain.EventAgentInvocation, "autocomplete"
	if _, err := domain.DeliveryGrant(f); !errors.Is(err, domain.ErrUnknownDelivery) {
		t.Errorf("autocomplete invocation: %v", err)
	}
}

// validateAgainst validates p against the schema name of the contract and refuses null.
func validateAgainst(t *testing.T, name string, p []byte) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), string(p)); !valid {
		for _, pr := range problems {
			t.Errorf("schema: %s %s", pr.Message, pr.Reason)
		}
		t.Fatalf("payload does not match %s: %s", name, p)
	}
	if testkit.HasJSONNull(p) {
		t.Fatalf("null in %s", p)
	}
}

// TestInvocationPayload: agent.invocation and agent.followup validate against their schemas, carry
// no null and no content.
func TestInvocationPayload(t *testing.T) {
	t.Parallel()
	in := domain.InvocationPayloadInput{
		RunID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(), MessageID: ids.New(),
		TriggeredBy: domain.PrincipalRefPayload{ID: ids.New().String(), Kind: "user", Handle: "alex", DisplayName: "Alex"},
		BillingMode: "organization", SettingsVersion: 3,
	}
	p, err := domain.InvocationPayload(in, domain.DecideSession(true, nil))
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "AgentInvocationPayload", p)
	for _, forbidden := range []string{"content", "prompt", "skills", "project"} {
		if strings.Contains(string(p), forbidden) {
			t.Errorf("payload carries %s: %s", forbidden, p)
		}
	}
	if m := members(t, p); m["session_mode"] != "new" || m["agent_session"] != nil {
		t.Errorf("operator run without a row: %s", p)
	}
	in.SettingsVersion, in.SpaceID = 0, ids.ID{}
	p, err = domain.InvocationPayload(in, domain.DecideSession(false, nil))
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "AgentInvocationPayload", p)
	if m := members(t, p); m["session_mode"] != nil || m["agent_session"] != nil {
		t.Errorf("guest run: %s", p)
	}
	in.MessageID = ids.ID{}
	if _, err := domain.InvocationPayload(in, domain.DecideSession(true, nil)); err == nil {
		t.Error("a payload without the message")
	}
	run, thread, msg := ids.New(), ids.New(), ids.New()
	fp, err := domain.FollowupPayload(run, thread, msg)
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "AgentFollowupPayload", fp)
	if got, err := domain.FollowupMessageID(fp); err != nil || got != msg {
		t.Errorf("FollowupMessageID = %s, %v", got, err)
	}
	if _, err := domain.FollowupMessageID([]byte(`{"run_id":"x"}`)); err == nil {
		t.Error("a follow-up payload without message_id")
	}
	if _, err := domain.FollowupPayload(run, ids.ID{}, msg); err == nil {
		t.Error("a follow-up payload without the thread")
	}
}

// members decodes the top-level members of a payload.
func members(t *testing.T, p []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(p, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestInvocationPayload_Session (M2-12 design note 9): every mode with its row validates against
// AgentInvocationPayload with agent_session in schema order before session_mode; the constructor
// refuses a missing decision, a guest run with a row, and resume or bind without a row.
func TestInvocationPayload_Session(t *testing.T) {
	t.Parallel()
	in := domain.InvocationPayloadInput{
		RunID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), MessageID: ids.New(),
		TriggeredBy: domain.PrincipalRefPayload{ID: ids.New().String(), Kind: "user", Handle: "alex", DisplayName: "Alex"},
		BillingMode: "organization",
	}
	// The schema AgentSession as agents.SessionJSON writes it; runs/domain never decodes it.
	raw := json.RawMessage(`{"id":"` + ids.New().String() + `","thread_id":"` + in.ThreadID.String() + `","agent_id":"` + ids.New().String() +
		`","external_session_ref":"sess-1","is_current":true,"started":true,"model":"model-a",` +
		`"cost":{"amount_micros":0,"currency":"USD","is_estimate":false},"created_at":"2026-10-14T09:00:00.000Z","version":2}`)
	for _, c := range []struct {
		row  *domain.SessionRow
		mode string
	}{
		{&domain.SessionRow{JSON: raw}, "new"},
		{&domain.SessionRow{JSON: raw, Started: true, Used: true}, "resume"},
		{&domain.SessionRow{JSON: raw, Started: true}, "bind"},
	} {
		p, err := domain.InvocationPayload(in, domain.DecideSession(true, c.row))
		if err != nil {
			t.Fatal(err)
		}
		validateAgainst(t, "AgentInvocationPayload", p)
		m := members(t, p)
		if m["session_mode"] != c.mode || m["agent_session"].(map[string]any)["external_session_ref"] != "sess-1" {
			t.Errorf("mode %s: %s", c.mode, p)
		}
		if a, b := strings.Index(string(p), `"agent_session"`), strings.Index(string(p), `"session_mode"`); a < 0 || a > b {
			t.Errorf("schema order: %s", p)
		}
	}
	for name, d := range map[string]domain.SessionDecision{
		"no decision":    {},
		"guest with row": {Mode: domain.SessionGuest, Row: raw},
		"resume no row":  {Mode: domain.SessionResume},
		"bind no row":    {Mode: domain.SessionBind},
		"unknown mode":   {Mode: "later", Row: raw},
	} {
		if _, err := domain.InvocationPayload(in, d); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
