// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/runs"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
)

// tokenConfig is the configuration of the invocation tokens of the world: agent.invocation_token_ttl
// and auth.clock_leeway as the runs use cases are configured with.
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
// the job runtime of the world runs no job, so a test calls the use case itself.
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

// runOf reads the run whose triggering message is message; ok is false without one.
func (w *world) runOf(message string) (string, bool) {
	w.t.Helper()
	var run string
	err := w.db.QueryRow(context.Background(), `SELECT coalesce(max(id::text), '') FROM runs WHERE triggering_message_id = $1`, message).Scan(&run)
	must(w.t, err)
	return run, run != ""
}

// deliver prepares the delivery of agent.invocation of run through runs.Delivery (M2-08) and
// returns the fresh token; the world has no gateway, so this stands in for pollAgentEvents.
func (w *world) deliver(run string) string {
	w.t.Helper()
	agent := w.text(`SELECT agent_id::text FROM runs WHERE id = $1`, run)
	out, err := w.runs.Prepare(context.Background(), runs.DeliveryInput{AgentID: ids.MustParse(agent), EventID: ids.New(), Type: "agent.invocation",
		ResourceKind: "run", ResourceID: ids.MustParse(run)})
	must(w.t, err)
	if !out.Send || out.InvocationToken == "" {
		w.t.Fatalf("the delivery of run %s sent nothing", run)
	}
	return out.InvocationToken
}

// startBody is the body RunStart with the mode organization and the tool class chat.
func startBody(token string) string {
	b, _ := json.Marshal(token)
	return `{"invocation_token":` + string(b) + `,"billing_mode_used":"organization","tool_class_used":"chat"}`
}

// start sends startRun as the agent handle.
func (w *world) start(handle, run, token string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/start", startBody(token))
}

// await sends awaitRunInput as the agent handle.
func (w *world) await(handle, run, line string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/await-input", `{"status_line":"`+line+`"}`)
}

// finish sends finishRun as the agent lotse with the JSON members body.
func (w *world) finish(run, body string) *httpservertest.Response {
	return w.do("lotse", http.MethodPost, "/api/v1/runs/"+run+"/finish", "{"+body+"}")
}

// cancel sends cancelRun as name with a fresh Idempotency-Key.
func (w *world) cancel(name, run string) *httpservertest.Response {
	return w.do(name, http.MethodPost, "/api/v1/runs/"+run+"/cancel", `{}`, key()...)
}
