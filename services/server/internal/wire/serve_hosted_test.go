// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// TestHostedTokens (A1-03, BA-T05 and BA-T06 end to end, docs/spec/26-builtin-agents.md 3.3): a
// hosting runner fetches the token of its built-in agent with its own DPoP-bound token; the hosted
// token connects to the gateway only with the runner's key; each revocation cause closes the
// hosted connection with 4012 through the existing revocation path, and a hosted agent's disable
// leaves the runner's own connection open.
func TestHostedTokens(t *testing.T) {
	t.Parallel()
	w := newConsumerWorld(t)
	runner := w.agent()
	other := w.agent()
	for _, r := range []consumerAgent{runner, other} {
		w.sql(`UPDATE agents SET adapter_kind = 'code_runner', reported_details = '{"capabilities":{"hosts_agents":"scoped"}}' WHERE principal_id = $1`,
			r.id.String())
	}
	// hosted creates a built-in agent on runner as the owner and returns it with a hosted token
	// fetched by runner, which carries runner's key.
	hosted := func(t *testing.T, r consumerAgent) consumerAgent {
		t.Helper()
		id := w.must(w.owner, http.MethodPost, "/api/v1/agents", map[string]any{
			"handle": "built" + ids.New().String()[30:], "display_name": "Built-in", "adapter_kind": "claude_agent", "hosted_by": r.id.String(),
		})
		code, b := w.call(w.base, r, http.MethodPost, "/api/v1/agents/me/hosted/"+id+"/token", nil)
		if code != http.StatusCreated {
			t.Fatalf("issueHostedAgentToken = %d %s", code, b)
		}
		var out struct {
			Secret string `json:"secret"`
		}
		if err := json.Unmarshal(b, &out); err != nil || out.Secret == "" {
			t.Fatalf("answer %s: %v", b, err)
		}
		w.remember(out.Secret)
		return consumerAgent{id: ids.MustParse(id), tok: out.Secret, key: r.key}
	}
	// beat sends a gateway heartbeat, which revalidates the credential (04 6.3).
	beat := func(t *testing.T, c *wstest.Client) {
		t.Helper()
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			t.Fatal(err)
		}
	}
	// runnerToken returns the id of the runner's own token bound to its key.
	runnerToken := func(r consumerAgent) string {
		var id string
		if err := w.queryRow(t.Context(), `SELECT id::text FROM api_tokens WHERE principal_id = $1 AND created_by = $1 AND revoked_at IS NULL
			ORDER BY created_at DESC LIMIT 1`, r.id.String()).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	t.Run("TestHostedTokenNeedsTheRunnersKey", func(t *testing.T) {
		a := hosted(t, runner)
		stranger, _ := identitytest.NewDeviceKey(jose.AlgES256)
		w.identifyErr(t, w.agentParams(consumerAgent{id: a.id, tok: a.tok, key: stranger}, ""), 4004)
		c, _ := w.ready(t, w.agentParams(a, ""))
		beat(t, c)
	})

	t.Run("TestRunnerTokenRevocationCloses4012", func(t *testing.T) {
		a := hosted(t, runner)
		c, _ := w.ready(t, w.agentParams(a, ""))
		w.must(w.owner, http.MethodDelete, "/api/v1/tokens/"+runnerToken(runner), nil)
		beat(t, c)
		w.expectClose(t, c, 4012)
		runner.tok = w.agentToken(runner.id, runner.key, time.Now().Add(time.Hour))
	})

	t.Run("TestHostChangeCloses4012", func(t *testing.T) {
		a := hosted(t, runner)
		c, _ := w.ready(t, w.agentParams(a, ""))
		code, b, etag := w.restIfMatch(w.owner, http.MethodGet, "/api/v1/agents/"+a.id.String(), "", nil)
		if code != http.StatusOK {
			t.Fatalf("getAgent = %d %s", code, b)
		}
		code, b, _ = w.restIfMatch(w.owner, http.MethodPut, "/api/v1/agents/"+a.id.String()+"/host", etag,
			map[string]any{"hosted_by": other.id.String(), "reason": "Move"})
		if code != http.StatusOK {
			t.Fatalf("setAgentHost = %d %s", code, b)
		}
		beat(t, c)
		w.expectClose(t, c, 4012)
	})

	t.Run("TestHostedAgentDisableCloses4012", func(t *testing.T) {
		a := hosted(t, runner)
		c, _ := w.ready(t, w.agentParams(a, ""))
		rc, _ := w.ready(t, w.agentParams(runner, ""))
		w.must(w.owner, http.MethodPost, "/api/v1/agents/"+a.id.String()+"/disable", map[string]any{"reason": "Pause"})
		beat(t, c)
		w.expectClose(t, c, 4012)
		// The runner's own connection and token are untouched.
		beat(t, rc)
		if code, b := w.call(w.base, runner, http.MethodGet, "/api/v1/agents/me/hosted", nil); code != http.StatusOK {
			t.Fatalf("listHostedAgents of the runner = %d %s", code, b)
		}
	})

	t.Run("TestRunnerDisableCloses4012", func(t *testing.T) {
		a := hosted(t, runner)
		c, _ := w.ready(t, w.agentParams(a, ""))
		w.must(w.owner, http.MethodPost, "/api/v1/agents/"+runner.id.String()+"/disable", map[string]any{"reason": "Retired"})
		beat(t, c)
		w.expectClose(t, c, 4012)
	})
}
