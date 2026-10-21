// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/runs"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
)

// tokenConfig is the configuration of the invocation tokens of the world: agent.invocation_token_ttl
// and auth.clock_leeway as the runs use cases are configured with (Config.ClockLeeway).
var tokenConfig = runsapp.InvocationTokenConfig{TTL: 90 * time.Second, ClockLeeway: 30 * time.Second}

// testKey is one Ed25519 key of testKeys.
type testKey struct {
	kid     string
	purpose instance.Purpose
	priv    ed25519.PrivateKey
}

// testKeys stands in for instance.Keys: one active key per purpose, every key published.
type testKeys struct {
	mu        sync.Mutex
	active    map[instance.Purpose]*testKey
	published map[string]*testKey
}

// newTestKeys returns keys with an active key of the purposes invocation and access_token.
func newTestKeys(t *testing.T) *testKeys {
	t.Helper()
	k := &testKeys{active: map[instance.Purpose]*testKey{}, published: map[string]*testKey{}}
	for _, p := range []instance.Purpose{instance.PurposeInvocation, instance.PurposeAccessToken} {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		must(t, err)
		key := &testKey{kid: fmt.Sprintf("%s-%x", p, priv.Public().(ed25519.PublicKey)[:6]), purpose: p, priv: priv}
		k.active[p], k.published[key.kid] = key, key
	}
	return k
}

// testSigner is the instance.Signer of a testKey.
type testSigner struct{ key *testKey }

func (s testSigner) KeyID() string { return s.key.kid }
func (testSigner) Alg() string     { return instance.AlgEdDSA }
func (s testSigner) Sign(_ context.Context, in []byte) ([]byte, error) {
	return ed25519.Sign(s.key.priv, in), nil
}

// Signer implements runsapp.InvocationKeys.
func (k *testKeys) Signer(_ context.Context, p instance.Purpose) (instance.Signer, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return testSigner{key: k.active[p]}, nil
}

// VerificationKey implements runsapp.InvocationKeys.
func (k *testKeys) VerificationKey(_ context.Context, p instance.Purpose, kid string) (instance.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	key, ok := k.published[kid]
	if !ok || key.purpose != p {
		return instance.PublicKey{}, errs.New(errs.CodeNotFound, "")
	}
	return instance.PublicKey{KID: kid, Purpose: p, Alg: instance.AlgEdDSA, State: instance.KeyActive, Key: key.priv.Public().(ed25519.PublicKey)}, nil
}

// fixedInstance is instance.State.Load with a fixed instance_id, the issuer of the tokens.
type fixedInstance struct{ id ids.ID }

func (f fixedInstance) Load(context.Context) (instance.Record, error) {
	return instance.Record{InstanceID: f.id}, nil
}

// cancelJob is one scheduled runs.cancel_timeout.
type cancelJob struct {
	Run ids.ID
	At  time.Time
}

// cancelJobs records the scheduled jobs runs.cancel_timeout (runsapp.CancelTimeoutScheduler);
// the world has no job runtime, so the tests call the handler's use case directly.
type cancelJobs struct {
	mu   sync.Mutex
	list []cancelJob
}

// Schedule implements runsapp.CancelTimeoutScheduler.
func (c *cancelJobs) Schedule(_ context.Context, run ids.ID, at time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list = append(c.list, cancelJob{Run: run, At: at})
	return nil
}

// of returns the jobs of run.
func (c *cancelJobs) of(run string) []cancelJob {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []cancelJob
	for _, j := range c.list {
		if j.Run.String() == run {
			out = append(out, j)
		}
	}
	return out
}

// deliver prepares the delivery of agent.invocation of run through runs.Delivery (M2-08) and
// returns the fresh token; the world has no gateway, so this stands in for pollAgentEvents.
func (w *world) deliver(run string) string {
	w.t.Helper()
	return w.deliverAs(w.runs, run)
}

// deliverAs is deliver through the runs use cases svc (a second server instance).
func (w *world) deliverAs(svc *runsapp.Service, run string) string {
	w.t.Helper()
	agent := w.text(`SELECT agent_id::text FROM runs WHERE id = $1`, run)
	out, err := svc.Prepare(context.Background(), runs.DeliveryInput{AgentID: ids.MustParse(agent), EventID: ids.New(), Type: "agent.invocation",
		ResourceKind: "run", ResourceID: ids.MustParse(run)})
	must(w.t, err)
	if !out.Send || out.InvocationToken == "" {
		w.t.Fatalf("the delivery of run %s sent nothing", run)
	}
	return out.InvocationToken
}

// startBody is the body RunStart with the configured mode organization and tool class tool.
func startBody(token, mode, tool string, extra ...string) string {
	b, _ := json.Marshal(token)
	body := `{"invocation_token":` + string(b) + `,"billing_mode_used":"` + mode + `","tool_class_used":"` + tool + `"`
	for _, e := range extra {
		body += "," + e
	}
	return body + "}"
}

// start sends startRun as the agent handle.
func (w *world) start(handle, run, token string, extra ...string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/start", startBody(token, "organization", "chat", extra...))
}

// startWith sends startRun with the mode and the tool class.
func (w *world) startWith(handle, run, token, mode, tool string, extra ...string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/start", startBody(token, mode, tool, extra...))
}

// resume sends resumeRun as the agent handle.
func (w *world) resume(handle, run, token string) *httpservertest.Response { //nolint:unparam // the request helpers name the agent handle like start and cancel do.
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/resume", startBody(token, "organization", "chat"))
}

// renew sends renewInvocationToken as the agent handle.
func (w *world) renew(handle, run, token string) *httpservertest.Response { //nolint:unparam // the request helpers name the agent handle like start and cancel do.
	b, _ := json.Marshal(token)
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/invocation-token", `{"invocation_token":`+string(b)+`}`)
}

// await sends awaitRunInput as the agent handle.
func (w *world) await(handle, run, line string) *httpservertest.Response { //nolint:unparam // the request helpers name the agent handle like start and cancel do.
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/await-input", `{"status_line":"`+line+`"}`)
}

// finish sends finishRun as the agent handle with the JSON members body.
func (w *world) finish(handle, run, body string) *httpservertest.Response { //nolint:unparam // the request helpers name the agent handle like start and cancel do.
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/finish", "{"+body+"}")
}

// cancel sends cancelRun as name with a fresh Idempotency-Key.
func (w *world) cancel(name, run string) *httpservertest.Response {
	return w.do(name, http.MethodPost, "/api/v1/runs/"+run+"/cancel", `{}`, key()...)
}

// wantReason checks a problem with code and reason.
func (w *world) wantReason(r *httpservertest.Response, status int, code, reason string) { //nolint:unparam // the status stays explicit next to the code at each call.
	w.t.Helper()
	p := w.want(r, status, code)
	if p["reason"] != reason {
		w.t.Fatalf("reason %v, want %s: %s", p["reason"], reason, r.Body)
	}
}

// lifecycleRow is the part of a runs row the lifecycle tests read.
type lifecycleRow struct {
	State, Reason, Invocation, Started, Renewed, Used, Model, Evidence, StatusLine, Summary, Metrics string
	Version                                                                                          int64
	StartedAt, FinishedAt                                                                            time.Time
	Duration                                                                                         int64
}

// lifecycle reads the run id.
func (w *world) lifecycle(id string) lifecycleRow {
	w.t.Helper()
	var r lifecycleRow
	var started, finished *time.Time
	var duration *int64
	must(w.t, w.db.QueryRow(context.Background(), `SELECT state, coalesce(state_reason, ''), invocation_jti::text, coalesce(started_jti::text, ''),
		coalesce(renewed_from_jti::text, ''), coalesce(billing_mode_used, ''), coalesce(model, ''), billing_evidence::text,
		coalesce(status_line, ''), coalesce(summary, ''), metrics::text, version, started_at, finished_at, duration_ms FROM runs WHERE id = $1`, id).
		Scan(&r.State, &r.Reason, &r.Invocation, &r.Started, &r.Renewed, &r.Used, &r.Model, &r.Evidence, &r.StatusLine, &r.Summary, &r.Metrics,
			&r.Version, &started, &finished, &duration))
	if started != nil {
		r.StartedAt = *started
	}
	if finished != nil {
		r.FinishedAt = *finished
	}
	if duration != nil {
		r.Duration = *duration
	}
	return r
}

// jtiOf returns the jti of a compact token without verifying it.
func jtiOf(t *testing.T, token string) string {
	t.Helper()
	parts := splitJWS(token)
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	must(t, err)
	var c struct {
		Jti string `json:"jti"`
	}
	must(t, json.Unmarshal(raw, &c))
	return c.Jti
}

// splitJWS splits a compact JWS into its three parts.
func splitJWS(token string) []string {
	var out []string
	start := 0
	for i := range len(token) {
		if token[i] == '.' {
			out = append(out, token[start:i])
			start = i + 1
		}
	}
	return append(out, token[start:])
}

// forge signs claims with header as a compact JWS with the active invocation key (or a fresh
// key with foreign), for the token-invalid cases.
func (w *world) forge(header map[string]any, claims map[string]any, foreign bool) string {
	w.t.Helper()
	key := w.keys.active[instance.PurposeInvocation]
	if foreign {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		must(w.t, err)
		key = &testKey{kid: key.kid, purpose: key.purpose, priv: priv}
	}
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(claims)
	in := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	return in + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key.priv, []byte(in)))
}

// claimsOf decodes the claims of a token without verifying it.
func claimsOf(t *testing.T, token string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(splitJWS(token)[1])
	must(t, err)
	var m map[string]any
	must(t, json.Unmarshal(raw, &m))
	return m
}

// headerOf is the header of a valid invocation token of the world.
func (w *world) headerOf() map[string]any {
	return map[string]any{"alg": jose.AlgEdDSA, "typ": "agent-invocation+jwt", "kid": w.keys.active[instance.PurposeInvocation].kid}
}

// eventsOf returns the payloads of the events of typ about run.
func (w *world) eventsOf(typ, run string) []map[string]any { //nolint:unparam // the event type stays explicit at each call.
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT payload::text FROM event_log WHERE type = $1 AND resource_id = $2 ORDER BY seq`, typ, run)
	must(w.t, err)
	defer rs.Close()
	var out []map[string]any
	for rs.Next() {
		var raw string
		must(w.t, rs.Scan(&raw))
		var m map[string]any
		must(w.t, json.Unmarshal([]byte(raw), &m))
		out = append(out, m)
	}
	return out
}

// noTokenAnywhere fails when one of tokens or its jti is in event_log, audit_events or the log.
func (w *world) noTokenAnywhere(tokens ...string) {
	w.t.Helper()
	for _, tok := range tokens {
		jti := jtiOf(w.t, tok)
		for _, s := range []string{tok, jti} {
			if n := w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, s); n != 0 {
				w.t.Fatal("a token or jti is in event_log")
			}
			if n := w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, s); n != 0 {
				w.t.Fatal("a token or jti is in audit_events")
			}
			if w.log.Contains(s) {
				w.t.Fatal("a token or jti is in the log")
			}
		}
	}
}

// deliveryInput is the input of runs.Delivery for an event of typ about run with payload.
func deliveryInput(agent ids.ID, run, typ string, payload []byte) runs.DeliveryInput {
	return runs.DeliveryInput{AgentID: agent, EventID: ids.New(), Type: typ, ResourceKind: "run", ResourceID: ids.MustParse(run), Payload: payload}
}
