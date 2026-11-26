// SPDX-License-Identifier: Apache-2.0

//go:build computere2e

package computer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// agent is the agent of the test: a code runner of tool class host that operates the channel
// desk of the space Office, with a DPoP-bound API token for the runner.
type agent struct {
	id, channel, mainThread, instanceID string
	// token is the API token's secret and dpopPEM the PKCS#8 key it is bound to; both reach only
	// the runner machine.
	token   string
	dpopPEM []byte
}

// tokenTTL is the validity of the runner's API token (a test constant).
const tokenTTL = 24 * time.Hour

// newAgent creates the agent through the case's Owner: createAgent (adapter kind code_runner),
// the tool class host, a space and a text channel it operates, and an API token bound to a fresh
// P-256 key (createToken with dpop_jkt).
func newAgent(t *testing.T, owner *harness.Client) agent {
	t.Helper()
	var a agent
	var created struct {
		ID string `json:"id"`
	}
	sfx := time.Now().UTC().Format("150405")
	mustDo(t, owner, http.MethodPost, "/agents", map[string]any{"handle": "builder-" + sfx, "display_name": "Builder",
		"adapter_kind": "code_runner"}, nil, http.StatusCreated).JSON(t, &created)
	a.id = created.ID
	cur := mustDo(t, owner, http.MethodGet, "/agents/"+a.id, nil, nil, http.StatusOK)
	mustDo(t, owner, http.MethodPut, "/agents/"+a.id+"/tool-class", map[string]any{"tool_class": "host", "reason": "computer use end to end"},
		http.Header{"If-Match": []string{cur.Header.Get("ETag")}}, http.StatusOK)
	mustDo(t, owner, http.MethodPost, "/spaces", map[string]any{"name": "Office " + sfx}, nil, http.StatusCreated).JSON(t, &created)
	space := created.ID
	var ch struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	mustDo(t, owner, http.MethodPost, "/spaces/"+space+"/channels", map[string]any{"kind": "text", "name": "desk"}, nil, http.StatusCreated).JSON(t, &ch)
	a.channel, a.mainThread = ch.ID, ch.MainThreadID
	cur = mustDo(t, owner, http.MethodGet, "/channels/"+a.channel, nil, nil, http.StatusOK)
	mustDo(t, owner, http.MethodPut, "/channels/"+a.channel+"/operator", map[string]any{"mode": "agent", "agent_id": a.id, "reason": "computer use end to end"},
		http.Header{"If-Match": []string{cur.Header.Get("ETag")}}, http.StatusOK)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	a.dpopPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	var tok struct {
		Secret string `json:"secret"`
	}
	mustDo(t, owner, http.MethodPost, "/principals/"+a.id+"/tokens", map[string]any{"name": "runner", "dpop_jkt": thumbprint(t, &key.PublicKey),
		"expires_at": time.Now().Add(tokenTTL).UTC().Format("2006-01-02T15:04:05.000Z")}, nil, http.StatusCreated).JSON(t, &tok)
	if tok.Secret == "" {
		t.Fatal("createToken answered without a secret")
	}
	a.token = tok.Secret
	var disc struct {
		InstanceID string `json:"instance_id"`
	}
	owner.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/instance/discovery"}).JSON(t, &disc)
	a.instanceID = disc.InstanceID
	return a
}

// thumbprint returns the RFC 7638 thumbprint of a P-256 public key.
func thumbprint(t *testing.T, k *ecdsa.PublicKey) string {
	t.Helper()
	pub, err := k.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	raw := pub.Bytes() // 0x04 || x || y
	enc := base64.RawURLEncoding
	j := `{"crv":"P-256","kty":"EC","x":"` + enc.EncodeToString(raw[1:33]) + `","y":"` + enc.EncodeToString(raw[33:65]) + `"}`
	s := sha256.Sum256([]byte(j))
	return enc.EncodeToString(s[:])
}

// hostView is the part of getHost the test reads (the members of 27 12.4 for an active owner).
type hostView struct {
	Computer *struct {
		Helper     string `json:"helper"`
		Enabled    bool   `json:"enabled"`
		Generation int    `json:"generation"`
	} `json:"computer"`
	Listed bool `json:"computer_listed"`
}

// waitHost waits until getHost as reader satisfies cond.
func waitHost(t *testing.T, reader *harness.Client, hostID, what string, cond func(hostView) bool) {
	t.Helper()
	until(t, what, func() bool {
		resp, err := harness.DoWithStepUp(t.Context(), reader, harness.Request{Method: http.MethodGet, Path: "/hosts/" + hostID})
		if err != nil || resp.Status != http.StatusOK {
			return false
		}
		var h hostView
		return json.Unmarshal(resp.Body, &h) == nil && cond(h)
	})
}

// waitHeartbeat waits until the agent sent a heartbeat (the runner is connected and reports its
// tool class).
func waitHeartbeat(t *testing.T, owner *harness.Client, agentID string) {
	t.Helper()
	until(t, "a heartbeat of the runner", func() bool {
		resp, err := harness.DoWithStepUp(t.Context(), owner, harness.Request{Method: http.MethodGet, Path: "/agents/" + agentID})
		if err != nil || resp.Status != http.StatusOK {
			return false
		}
		var a struct {
			Presence struct {
				LastHeartbeatAt *string `json:"last_heartbeat_at"`
			} `json:"presence"`
		}
		return json.Unmarshal(resp.Body, &a) == nil && a.Presence.LastHeartbeatAt != nil
	})
}

// waitRun waits for the run of the thread and returns its ID (read-only SQL).
func waitRun(t *testing.T, env *suite.Env, thread string) string {
	t.Helper()
	var id string
	until(t, "the run of the owner's message", func() bool {
		rows, err := env.Stack.SQL(t.Context(), "SELECT id::text AS id FROM runs WHERE thread_id = '"+uuidOnly(t, thread)+"'", false)
		if err != nil || len(rows) != 1 {
			return false
		}
		id, _ = rows[0]["id"].(string)
		return id != ""
	})
	return id
}

// waitRunState waits until the run is in state.
func waitRunState(t *testing.T, env *suite.Env, runID, state string) {
	t.Helper()
	until(t, "the run "+state, func() bool {
		rows, err := env.Stack.SQL(t.Context(), "SELECT state FROM runs WHERE id = '"+uuidOnly(t, runID)+"'", false)
		return err == nil && len(rows) == 1 && rows[0]["state"] == state
	})
}

// uuidOnly fails unless s is a UUID, so it can stand in a SQL literal.
func uuidOnly(t *testing.T, s string) string {
	t.Helper()
	if len(s) != 36 {
		t.Fatalf("not a UUID: %d characters", len(s))
	}
	for i, c := range s {
		dash := i == 8 || i == 13 || i == 18 || i == 23
		if dash != (c == '-') || (!dash && (c < '0' || c > '9') && (c < 'a' || c > 'f')) {
			t.Fatal("not a UUID")
		}
	}
	return s
}

// mustDo sends the request as c (with a step-up when the server asks for one) and fails the test
// unless the status is want; a failure names the status and the problem code, never the body.
func mustDo(t *testing.T, c *harness.Client, method, path string, body any, h http.Header, want int) harness.Response {
	t.Helper()
	resp, err := harness.DoWithStepUp(t.Context(), c, harness.Request{Method: method, Path: path, Body: body, Header: h})
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	if resp.Status != want {
		var p struct {
			Code   string `json:"code"`
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(resp.Body, &p)
		t.Fatalf("%s %s: %d %s %s, want %d", method, path, resp.Status, p.Code, p.Reason, want)
	}
	return resp
}

// until waits for cond within the patience of the tests (testkit.Patience).
func until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.Deadline()
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("not within %s: %s", testkit.Patience, what)
		}
		time.Sleep(waitTick)
	}
}
