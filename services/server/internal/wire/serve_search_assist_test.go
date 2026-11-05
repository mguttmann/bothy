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

// assistFrame is a dispatch of search.assist_requested as the test reads it.
type assistFrame struct {
	Seq             *int64 `json:"seq"`
	Type            string `json:"type"`
	Audience        string `json:"audience"`
	InvocationToken string `json:"invocation_token"`
	Resource        struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"resource"`
	Payload struct {
		RequestID  string `json:"request_id"`
		Q          string `json:"q"`
		MaxPicks   int    `json:"max_picks"`
		Candidates []struct {
			Ref     string `json:"ref"`
			Excerpt string `json:"excerpt"`
		} `json:"candidates"`
	} `json:"payload"`
	raw string
}

// nextAssist reads frames of c until a dispatch of search.assist_requested, acknowledging the
// durable dispatches on the way; ok is false when none came within testkit.Patience.
func nextAssist(t *testing.T, c *wstest.Client) (assistFrame, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			return assistFrame{}, false
		}
		if err != nil {
			t.Fatal(err)
		}
		if op != "dispatch" {
			continue
		}
		var f assistFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		if f.Type != "search.assist_requested" {
			if f.Seq != nil && *f.Seq > 0 {
				sendAck(t, c, *f.Seq)
			}
			continue
		}
		f.raw = string(raw)
		return f, true
	}
}

// TestSearchAssistThroughTheGateway is the path of the model end to end (14 4.10):
// the assistant agent named in search.assist.agent_id reads Support; Sam asks, the agent's
// default connection gets search.assist_requested without a token and without ids (the harness
// validates the frame against the contract), its pick goes back through answerSearchAssist, and
// Sam gets the pick with its reason. Robin, who reads only Eskalation, gets nothing of Allgemein,
// and the payload of his question holds nothing of it either. Another agent's answer is 404 like an
// unknown request.
func TestSearchAssistThroughTheGateway(t *testing.T) {
	assistant := ids.New()
	w := newACWorld(t, "BOTHY_SEARCH__ASSIST__AGENT_ID="+assistant.String(),
		"BOTHY_SEARCH__ASSIST__TIMEOUT="+strconv.FormatInt(acWait.Milliseconds(), 10)+"ms")
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', 'finder', 'Finder')`, assistant.String())
	w.sql(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, assistant.String())
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	finder := consumerAgent{id: assistant, tok: w.agentToken(assistant, key, time.Now().Add(time.Hour)), key: key}
	w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "agent", "subject_id": assistant.String(), "target_kind": "channel", "target_id": w.allgemein.id, "level": "read",
		"reason": "search assistant"})
	msg, _ := w.say(w.sam, w.allgemein.thread, "The steering session moved from Discord to Larkspur today")
	c := w.connectAs(t, finder, "default")

	ask := func(p gwPerson, q string) <-chan acReply {
		out := make(chan acReply, 1)
		go func() { out <- w.rawRequest(&p, nil, "/api/v1/search/assist", map[string]any{"q": q}) }()
		return out
	}
	reply := ask(w.sam, "Where did the steering session move to Larkspur?")
	f, ok := nextAssist(t, c)
	if !ok {
		t.Fatal("the assistant got no search.assist_requested")
	}
	if f.Seq != nil || f.InvocationToken != "" || f.Audience != "self" || f.Resource.Kind != "agent" || f.Resource.ID != assistant.String() {
		t.Fatalf("frame %s", f.raw)
	}
	if strings.Contains(f.raw, msg) || strings.Contains(f.raw, w.allgemein.id) || len(f.Payload.Candidates) != 1 ||
		!strings.Contains(f.Payload.Candidates[0].Excerpt, "steering session") {
		t.Fatalf("payload %s, want one candidate without ids", f.raw)
	}
	path := "/api/v1/search-assist-requests/" + f.Payload.RequestID + "/answer"
	body := map[string]any{"picks": []any{map[string]any{"ref": "c1", "reason": "The thread where the steering session moved."}},
		"costs": []any{map[string]any{"provider": "anthropic", "model": "model-fast", "tokens_in": 900, "amount_micros": 700, "currency": "USD"}}}
	if r := w.rawRequest(nil, &w.lotse, path, body); r.code != http.StatusNotFound {
		t.Fatalf("another agent's answer = %d %s", r.code, r.body)
	}
	if r := w.rawRequest(nil, &finder, "/api/v1/search-assist-requests/"+ids.New().String()+"/answer", body); r.code != http.StatusNotFound {
		t.Fatalf("unknown request = %d %s", r.code, r.body)
	}
	if r := w.rawRequest(nil, &finder, path, body); r.code != http.StatusNoContent {
		t.Fatalf("answer = %d %s", r.code, r.body)
	}
	got := w.got(reply)
	var res struct {
		Mode string `json:"mode"`
		Hits []struct {
			MessageID string `json:"message_id"`
			Source    string `json:"source"`
			Reason    string `json:"reason"`
		} `json:"hits"`
		Cost struct {
			AmountMicros int64  `json:"amount_micros"`
			Currency     string `json:"currency"`
		} `json:"cost_estimate"`
	}
	if got.code != http.StatusOK || json.Unmarshal(got.body, &res) != nil {
		t.Fatalf("assistSearch = %d %s", got.code, got.body)
	}
	if res.Mode != "assisted" || len(res.Hits) != 1 || res.Hits[0].MessageID != msg || res.Hits[0].Source != "assistant" ||
		res.Hits[0].Reason == "" || res.Cost.AmountMicros != 700 || res.Cost.Currency != "USD" {
		t.Fatalf("assistSearch = %s", got.body)
	}

	reply = ask(w.robin, "Where did the steering session move to Larkspur?")
	f, ok = nextAssist(t, c)
	if !ok {
		t.Fatal("no request for Robin's question")
	}
	if len(f.Payload.Candidates) != 0 || strings.Contains(f.raw, "steering session moved") {
		t.Fatalf("Robin's payload holds what he may not read: %s", f.raw)
	}
	if r := w.rawRequest(nil, &finder, "/api/v1/search-assist-requests/"+f.Payload.RequestID+"/answer",
		map[string]any{"picks": []any{map[string]any{"ref": "c1", "reason": "Guess."}}}); r.code != http.StatusBadRequest {
		t.Fatalf("a pick of a reference that was not sent = %d %s", r.code, r.body)
	}
	if r := w.rawRequest(nil, &finder, "/api/v1/search-assist-requests/"+f.Payload.RequestID+"/answer", map[string]any{}); r.code != http.StatusNoContent {
		t.Fatalf("empty answer = %d %s", r.code, r.body)
	}
	got = w.got(reply)
	if got.code != http.StatusOK || strings.Contains(string(got.body), msg) || !strings.Contains(string(got.body), `"hits":[]`) {
		t.Fatalf("Robin got %d %s", got.code, got.body)
	}
}
