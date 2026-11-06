// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// interviewFrame is a dispatch of agent.interview_requested as the test reads it.
type interviewFrame struct {
	Seq             *int64 `json:"seq"`
	Type            string `json:"type"`
	Audience        string `json:"audience"`
	InvocationToken string `json:"invocation_token"`
	Resource        struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"resource"`
	Payload struct {
		RequestID string `json:"request_id"`
		Turns     []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"turns"`
		Catalogue []any          `json:"catalogue"`
		Limits    map[string]any `json:"limits"`
		ExpiresAt string         `json:"expires_at"`
	} `json:"payload"`
	raw string
}

// nextInterview reads frames of c until a dispatch of agent.interview_requested, acknowledging the
// durable dispatches on the way; ok is false when none came within testkit.Patience.
func nextInterview(t *testing.T, c *wstest.Client) (interviewFrame, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			return interviewFrame{}, false
		}
		if err != nil {
			t.Fatal(err)
		}
		if op != "dispatch" {
			continue
		}
		var f interviewFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		if f.Type != "agent.interview_requested" {
			if f.Seq != nil && *f.Seq > 0 {
				sendAck(t, c, *f.Seq)
			}
			continue
		}
		f.raw = string(raw)
		return f, true
	}
}

// TestInterviewThroughTheGateway is the interview end to end (26 6.1, BA-T15): the owner's
// interview reaches the default connection of the builder agent named in agents.builder_agent_id
// as agent.interview_requested without a token (the harness validates the frame against the
// contract); another agent's answer and an unknown request are 404; a member outside the answer
// schema is refused by the request validation; a draft with an inline shell form ends the request
// with builder_invalid; questions come back with the cost estimate and a second answer is 409. A
// person who is no owner gets 403 and reaches no builder.
func TestInterviewThroughTheGateway(t *testing.T) {
	builderID := ids.New()
	w := newACWorld(t, "BOTHY_AGENTS__BUILDER_AGENT_ID="+builderID.String(),
		"BOTHY_AGENTS__INTERVIEW__TIMEOUT="+strconv.FormatInt((testkit.Patience/2).Milliseconds(), 10)+"ms")
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', 'builder', 'Builder')`, builderID.String())
	w.sql(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'code_runner')`, builderID.String())
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	builder := consumerAgent{id: builderID, tok: w.agentToken(builderID, key, time.Now().Add(time.Hour)), key: key}
	c := w.connectAs(t, builder, "default")

	turns := []any{map[string]any{"role": "owner", "text": "A writing helper for example.com"}}
	ask := func(p gwPerson) <-chan acReply {
		out := make(chan acReply, 1)
		go func() { out <- w.rawRequest(&p, nil, "/api/v1/agents/interview", map[string]any{"turns": turns}) }()
		return out
	}
	if r := <-ask(w.sam); r.code != http.StatusForbidden {
		t.Fatalf("a person who is no owner = %d %s", r.code, r.body)
	}

	reply := ask(w.owner)
	f, ok := nextInterview(t, c)
	if !ok {
		t.Fatal("the builder got no agent.interview_requested")
	}
	if f.Seq != nil || f.InvocationToken != "" || f.Audience != "self" || f.Resource.Kind != "agent" || f.Resource.ID != builderID.String() ||
		len(f.Payload.Turns) != 1 || f.Payload.Turns[0].Text != "A writing helper for example.com" || f.Payload.Catalogue == nil ||
		f.Payload.Limits["max_skills"] == nil || f.Payload.ExpiresAt == "" {
		t.Fatalf("frame %s", f.raw)
	}
	path := "/api/v1/agent-interview-requests/" + f.Payload.RequestID + "/answer"
	questions := map[string]any{"kind": "questions", "questions": []any{"Who reads the texts?"},
		"costs": []any{map[string]any{"provider": "anthropic", "model": "model-medium", "tokens_in": 900, "amount_micros": 4200, "currency": "USD"}}}
	if r := w.rawRequest(nil, &w.lotse, path, questions); r.code != http.StatusNotFound {
		t.Fatalf("another agent's answer = %d %s", r.code, r.body)
	}
	if r := w.rawRequest(nil, &builder, "/api/v1/agent-interview-requests/"+ids.New().String()+"/answer", questions); r.code != http.StatusNotFound {
		t.Fatalf("unknown request = %d %s", r.code, r.body)
	}
	if r := w.rawRequest(nil, &builder, path, map[string]any{"kind": "questions", "questions": []any{"a?"}, "hooks": map[string]any{}}); r.code != http.StatusBadRequest ||
		!strings.Contains(string(r.body), "unknown_member") {
		t.Fatalf("an answer with a member outside the schema = %d %s", r.code, r.body)
	}
	shell := map[string]any{"kind": "draft", "draft": map[string]any{
		"instructions": "Run !`id` first.", "skills": []any{}, "mcp_servers": []any{}, "settings": map[string]any{"tool_class_wish": "chat"},
		"origin": map[string]any{"kind": "interview"}, "name": "Writing helper", "handle": "writing-helper", "description": "Writes texts",
	}}
	if r := w.rawRequest(nil, &builder, path, shell); r.code != http.StatusBadRequest || !strings.Contains(string(r.body), "inline_shell") {
		t.Fatalf("a draft with an inline shell form = %d %s", r.code, r.body)
	}
	got := w.got(reply)
	if got.code != http.StatusOK || !strings.Contains(string(got.body), `"fallback_reason":"builder_invalid"`) || strings.Contains(string(got.body), "draft\":") {
		t.Fatalf("interviewAgent = %d %s", got.code, got.body)
	}

	reply = ask(w.owner)
	f, ok = nextInterview(t, c)
	if !ok {
		t.Fatal("no second agent.interview_requested")
	}
	path = "/api/v1/agent-interview-requests/" + f.Payload.RequestID + "/answer"
	if r := w.rawRequest(nil, &builder, path, questions); r.code != http.StatusNoContent {
		t.Fatalf("answer = %d %s", r.code, r.body)
	}
	if r := w.rawRequest(nil, &builder, path, questions); r.code != http.StatusConflict || !strings.Contains(string(r.body), "already_answered") {
		t.Fatalf("a second answer = %d %s", r.code, r.body)
	}
	got = w.got(reply)
	var res struct {
		Outcome   string   `json:"outcome"`
		Questions []string `json:"questions"`
		Cost      struct {
			AmountMicros int64  `json:"amount_micros"`
			Currency     string `json:"currency"`
		} `json:"cost_estimate"`
	}
	if got.code != http.StatusOK || json.Unmarshal(got.body, &res) != nil || res.Outcome != "questions" || len(res.Questions) != 1 ||
		res.Cost.AmountMicros != 4200 || res.Cost.Currency != "USD" {
		t.Fatalf("interviewAgent = %d %s", got.code, got.body)
	}
}
