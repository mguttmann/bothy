// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// m1EventCorpus is every event type an M1 producer emits (PLAN "M1 Foundation", brief M1-67):
// messages, reactions, pins, threads including the move of M1-62, spaces, channels, people,
// agents, the instance, rights, sessions, the personal sync events and typing. A type a later
// ticket adds to M1 belongs here; the test also requires each to be registered.
var m1EventCorpus = []string{
	"message.created", "message.updated", "message.deleted",
	"reaction.added", "reaction.removed",
	"pin.added", "pin.removed",
	"thread.created", "thread.updated", "thread.closed", "thread.reopened", "thread.moved", "thread.moved_out",
	"space.created", "space.updated", "space.archived",
	"channel.created", "channel.updated", "channel.archived",
	"user.updated", "agent.updated", "instance.updated",
	"grant.changed", "group.changed", "access.changed", "access.expiring",
	"session.revoked",
	"read_state.updated", "draft.updated", "draft.deleted", "settings.updated", "sidebar.updated",
	"typing.started",
}

// m1ServerOps is every op the M1 server sends (04 section 6.2).
var m1ServerOps = []string{
	"hello", "ready", "resumed", "dispatch", "cursor", "catchup.complete", "resync_required", "heartbeat_ack",
	"reauth_required", "error",
}

// conformanceWorld is the delivery world with REST calls that carry extra headers (If-Match).
type conformanceWorld struct {
	*dlWorld
	clients []*wstest.Client
}

// call calls the REST API of the process as p with a fresh step-up and the headers hdr; it
// requires a 2xx answer and returns body and headers.
func (w *conformanceWorld) call(t *testing.T, p gwPerson, method, path string, body any, hdr map[string]string) ([]byte, http.Header) {
	t.Helper()
	w.sql(`UPDATE auth_sessions SET step_up_at = now(), step_up_method = 'passkey', step_up_consumed_at = NULL WHERE id = $1`, p.set.Session.ID.String())
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, w.base+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(gwPublicURL + path)
	u.RawQuery = ""
	req.Header.Set("Authorization", "DPoP "+p.access)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{HTM: method, HTU: u.String(),
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(p.access)}))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("Idempotency-Key", ids.New().String())
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Fatalf("%s %s = %d %s", method, path, resp.StatusCode, b)
	}
	return b, resp.Header
}

// id returns the member id of a JSON answer.
func id(t *testing.T, b []byte) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &v); err != nil || v.ID == "" {
		t.Fatalf("no id in %s", b)
	}
	return v.ID
}

// ifMatch reads the ETag of path as p and returns the If-Match header for a change.
func (w *conformanceWorld) ifMatch(t *testing.T, p gwPerson, path string) map[string]string {
	t.Helper()
	_, h := w.call(t, p, http.MethodGet, path, nil, nil)
	tag := h.Get("ETag")
	if tag == "" {
		t.Fatalf("GET %s without ETag", path)
	}
	return map[string]string{"If-Match": tag}
}

// observed returns the union of the ops and event types every client of the world received.
func (w *conformanceWorld) observed() map[string]int {
	out := map[string]int{}
	for _, c := range w.clients {
		for k, n := range c.Observed() {
			out[k] += n
		}
	}
	return out
}

// waitFor waits until every key was observed by some client; it fails with the missing keys.
func (w *conformanceWorld) waitFor(t *testing.T, keys ...string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		seen := w.observed()
		var missing []string
		for _, k := range keys {
			if seen[k] == 0 {
				missing = append(missing, k)
			}
		}
		if len(missing) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("never observed %v", missing)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// track adds c to the clients whose frames count.
func (w *conformanceWorld) track(c *wstest.Client) *wstest.Client {
	w.clients = append(w.clients, c)
	return c
}

// withEnv returns env with the entries extra replacing those of the same name.
func withEnv(env []string, extra ...string) []string {
	out := slices.Clone(env)
	for _, e := range extra {
		name, _, _ := strings.Cut(e, "=")
		out = slices.DeleteFunc(out, func(have string) bool { return strings.HasPrefix(have, name+"=") })
		out = append(out, e)
	}
	return out
}

// TestConformance_M1EventCorpus is the corpus half of gate G9: one scenario against a started
// api process and a worker (PostgreSQL 18, all real modules) makes every event type of
// m1EventCorpus reach a client and the server send every op of m1ServerOps. The harness validates
// each frame strictly against the contract as it arrives; the test fails if a listed type or op
// was not observed or a listed type is not registered. Access tokens live 5m with
// sync.reauth_lead_time 298s, so every person's connection gets reauth_required after 2s.
func TestConformance_M1EventCorpus(t *testing.T) {
	t.Parallel()
	registered := map[string]bool{}
	for _, e := range wire.BuildRegistry(nil).Events() {
		registered[string(e.Type)] = true
	}
	for _, typ := range m1EventCorpus {
		if !registered[typ] {
			t.Errorf("event type %s of the M1 corpus is not registered", typ)
		}
	}
	w := &conformanceWorld{dlWorld: newDeliveryWorld(t, "BOTHY_AUTH__ACCESS_TOKEN_TTL=5m", "BOTHY_SYNC__REAUTH_LEAD_TIME=298s")}
	alex := w.owner

	// Alex's first connection observes the instance; its ready head is the cursor of the
	// catch-up below.
	a1, r1 := w.ready(t, w.params(alex))
	w.track(a1)
	if err := a1.Send("heartbeat", map[string]any{"last_seq": r1.HeadSeq}); err != nil {
		t.Fatal(err)
	}
	// typing.start on an unknown thread answers error not_found and keeps the connection.
	if err := a1.Send("typing.start", map[string]any{"thread_id": ids.New().String()}); err != nil {
		t.Fatal(err)
	}
	w.waitFor(t, "hello", "ready", "heartbeat_ack", "error", "reauth_required")

	// Robin observes his own access and types; he has no access yet.
	robin := w.newPerson("robin", identity.DeviceStateActive)
	rc, _ := w.ready(t, w.params(robin))
	w.track(rc)

	// Spaces, channels, threads and messages, as the Owner.
	space := w.newSpace("Engineering")
	w.call(t, alex, http.MethodPatch, "/api/v1/spaces/"+space, map[string]any{"name": "Research"}, w.ifMatch(t, alex, "/api/v1/spaces/"+space))
	support := w.newChannel(space, "support")
	marketing := w.newChannel(space, "marketing")
	w.call(t, alex, http.MethodPatch, "/api/v1/channels/"+support.id, map[string]any{"topic": "Questions"},
		w.ifMatch(t, alex, "/api/v1/channels/"+support.id))
	b, _ := w.call(t, alex, http.MethodPost, "/api/v1/threads/"+support.thread+"/messages",
		map[string]any{"client_msg_id": ids.New().String(), "content_md": "first"}, nil)
	msg := id(t, b)
	w.call(t, alex, http.MethodPatch, "/api/v1/messages/"+msg, map[string]any{"content_md": "first, edited"},
		w.ifMatch(t, alex, "/api/v1/messages/"+msg))
	emoji := url.PathEscape("\U0001F44D")
	w.call(t, alex, http.MethodPut, "/api/v1/messages/"+msg+"/reactions/"+emoji, nil, nil)
	w.call(t, alex, http.MethodDelete, "/api/v1/messages/"+msg+"/reactions/"+emoji, nil, nil)
	w.call(t, alex, http.MethodPut, "/api/v1/messages/"+msg+"/pin", nil, nil)
	w.call(t, alex, http.MethodDelete, "/api/v1/messages/"+msg+"/pin", nil, nil)
	b, _ = w.call(t, alex, http.MethodPost, "/api/v1/channels/"+support.id+"/threads", map[string]any{"parent_message_id": msg}, nil)
	var created struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(b, &created); err != nil || created.Thread.ID == "" {
		t.Fatalf("createThread answered %s", b)
	}
	thread := created.Thread.ID
	w.call(t, alex, http.MethodPut, "/api/v1/threads/"+thread+"/title", map[string]any{"title": "Release"},
		w.ifMatch(t, alex, "/api/v1/threads/"+thread))
	w.call(t, alex, http.MethodPost, "/api/v1/threads/"+thread+"/close", nil, nil)
	w.call(t, alex, http.MethodPost, "/api/v1/threads/"+thread+"/reopen", nil, nil)
	w.call(t, alex, http.MethodPost, "/api/v1/threads/"+thread+"/move", map[string]any{"target_channel_id": marketing.id, "reason": "topic"}, nil)
	b, _ = w.call(t, alex, http.MethodPost, "/api/v1/threads/"+support.thread+"/messages",
		map[string]any{"client_msg_id": ids.New().String(), "content_md": "second"}, nil)
	w.call(t, alex, http.MethodDelete, "/api/v1/messages/"+id(t, b), nil, nil)

	// Personal sync events of Alex.
	w.call(t, alex, http.MethodPut, "/api/v1/threads/"+support.thread+"/read-state", map[string]any{"last_read_seq": 1}, nil)
	w.call(t, alex, http.MethodPut, "/api/v1/threads/"+support.thread+"/draft", map[string]any{"content_md": "draft"}, nil)
	w.call(t, alex, http.MethodDelete, "/api/v1/threads/"+support.thread+"/draft", nil, nil)
	w.call(t, alex, http.MethodPut, "/api/v1/me/settings/appearance.theme", map[string]any{"value": "dark"}, nil)
	w.call(t, alex, http.MethodPut, "/api/v1/me/sidebar/spaces/"+space, map[string]any{"collapsed": true}, nil)

	// People, agents, the instance and rights.
	w.call(t, alex, http.MethodPatch, "/api/v1/me/profile", map[string]any{"display_name": "Alex"}, nil)
	w.call(t, alex, http.MethodPost, "/api/v1/agents", map[string]any{"handle": "helper", "display_name": "Helper", "adapter_kind": "custom"}, nil)
	w.call(t, alex, http.MethodPut, "/api/v1/instance/settings/locale.default", map[string]any{"value": "en"}, nil)
	w.call(t, alex, http.MethodPost, "/api/v1/groups", map[string]any{"name": "Support"}, nil)
	w.call(t, alex, http.MethodPost, "/api/v1/grants", map[string]any{"subject_kind": "user", "subject_id": robin.user.String(),
		"target_kind": "channel", "target_id": support.id, "level": "write", "reason": "test",
		"expires_at": time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)}, nil)
	w.waitFor(t, "dispatch access.changed")

	// typing.started: Alex focuses the main thread of support, Robin types in it.
	if err := a1.Send("focus", map[string]any{"thread_ids": []string{support.thread}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for w.observed()["dispatch typing.started"] == 0 {
		if time.Now().After(deadline) {
			t.Fatal("typing.started never reached the focusing connection")
		}
		if err := rc.Send("typing.start", map[string]any{"thread_id": support.thread}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
		if err := rc.Send("typing.stop", map[string]any{"thread_id": support.thread}); err != nil {
			t.Fatal(err)
		}
	}

	// access.expiring: a worker on the same database runs the reminder every second with a lead
	// of one hour, so Robin's grant of 30 minutes is due at once.
	worker := start(t, wire.RoleWorker, withEnv(w.env, "BOTHY_AUTHZ__EXPIRY_SWEEP_INTERVAL=1s", "BOTHY_AUTHZ__EXPIRY_REMINDER_LEAD=1h"), wire.Deps{})
	waitReadyz(t, worker.waitInternal(t), http.StatusOK)

	// Catch-up: Alex's second device identifies with the cursor of the first ready, so cursor
	// and catchup.complete follow.
	alex2 := w.addSession(alex)
	cursor := r1.HeadSeq
	p2 := w.params(alex2)
	p2.Cursor = &cursor
	a2, _ := w.ready(t, p2)
	w.track(a2)
	w.waitFor(t, "cursor", "catchup.complete")

	// resumed and resync_required: Sam resumes a session, then identifies with a foreign epoch.
	sam := w.newPerson("sam", identity.DeviceStateActive)
	s1, sr := w.ready(t, w.params(sam))
	w.track(s1)
	s1.Abort()
	s2 := w.track(w.dial(t))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := s2.Resume(ctx, wstest.ResumeParams{SessionID: sr.GatewaySessionId.String(), AccessToken: sam.access,
		Proof: w.proofFor(sam.key, sam.access, gwHTU), Cursor: sr.HeadSeq}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	s2.Close()
	ps := w.params(sam)
	samCursor := sr.HeadSeq
	ps.Cursor, ps.SyncEpoch = &samCursor, ids.New().String()
	s3, _ := w.ready(t, ps)
	w.track(s3)

	// session.revoked: Alex revokes the session of his second device; the first connection
	// receives the event, the second closes with 4012.
	w.call(t, alex, http.MethodPost, "/api/v1/sessions/"+alex2.set.Session.ID.String()+"/revoke", nil, nil)

	// The archives come last: they end the places of the events above.
	w.call(t, alex, http.MethodPost, "/api/v1/channels/"+marketing.id+"/archive", map[string]any{"reason": "done"}, nil)
	other := w.newSpace("Marketing")
	w.call(t, alex, http.MethodPost, "/api/v1/spaces/"+other+"/archive", map[string]any{"reason": "done"}, nil)

	want := slices.Clone(m1ServerOps)
	for _, typ := range m1EventCorpus {
		want = append(want, "dispatch "+typ)
	}
	w.waitFor(t, want...)
	seen := w.observed()
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("observed %s x%d", k, seen[k])
	}
}
