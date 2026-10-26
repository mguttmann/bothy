// SPDX-License-Identifier: Apache-2.0

package maintenance_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	authzdomain "github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestMaintenance_MessageRejected (ADR 0065 "Confirmation"): in maintenance mode a message gets
// 503 maintenance with Retry-After, writes no row and no audit entry, and
// bothy.maintenance.rejected counts it with the action; reading continues. It observes the global
// telemetry of the server, so it never runs in parallel.
func TestMaintenance_MessageRejected(t *testing.T) {
	w := newWorld(t, nil)
	sinks := harness.NewTelemetrySinks()
	w.s.Restart(t, harness.RestartOptions{Config: map[string]string{"server.maintenance_mode": "read_only"}, Telemetry: sinks})
	alex := w.w.People["Alex"]
	before := w.tableState()
	audits := w.count(`SELECT count(*) FROM audit_events`)
	w.wantMaintenance("createMessage", w.do(alex, harness.Request{Method: http.MethodPost, Path: "/threads/" + w.thread + "/messages",
		Body: map[string]any{"client_msg_id": newID(), "content_md": "Hello during the move"}}))
	if resp := w.do(alex, harness.Request{Method: http.MethodGet, Path: "/threads/" + w.thread + "/messages"}); resp.Status != http.StatusOK {
		t.Fatalf("listMessages in maintenance mode: %d %s", resp.Status, resp.Body)
	}
	if got := w.count(`SELECT count(*) FROM audit_events`); got != audits {
		t.Fatalf("audit entries %d after the rejection, want %d", got, audits)
	}
	wantSameTables(t, before, w.tableState())
	if err := w.s.FlushTelemetry(t.Context()); err != nil {
		t.Fatal(err)
	}
	var rm metricdata.ResourceMetrics
	if err := sinks.Metrics.Collect(t.Context(), &rm); err != nil {
		t.Fatal(err)
	}
	if n := rejected(rm, "message.create"); n != 1 {
		t.Fatalf("bothy.maintenance.rejected{message.create} = %d, want 1", n)
	}
}

// rejected sums bothy.maintenance.rejected of action in rm.
func rejected(rm metricdata.ResourceMetrics, action string) int64 {
	var n int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != "bothy.maintenance.rejected" || !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				if v, ok := dp.Attributes.Value("bothy.action"); ok && v.AsString() == action {
					n += dp.Value
				}
			}
		}
	}
	return n
}

// wantSameTables fails t when a business table differs between the two fingerprints.
func wantSameTables(t *testing.T, before, after map[string]string) {
	t.Helper()
	for _, tbl := range slices.Sorted(maps.Keys(before)) {
		if after[tbl] != before[tbl] {
			t.Errorf("table %s changed in maintenance mode", tbl)
		}
	}
}

// TestMaintenance_RunCanFinish (ADR 0065 item 4): the agent with a running run patches it (REST and
// the gateway op run.update), posts
// its answer in the run's thread, uploads a file into the channel of that thread and finishes the
// run; the same agent gets 503 in another thread and for run.start, so no new run begins.
func TestMaintenance_RunCanFinish(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	lotse := w.w.Agents["Lotse"]
	queued := w.text(`SELECT id::text FROM runs WHERE agent_id = $1 AND state = 'queued' ORDER BY id LIMIT 1`, lotse.PrincipalID)
	other := w.w.Threads["Support/Eskalation"]
	w.maintenance()

	version := w.count(`SELECT version FROM runs WHERE id = $1`, w.run)
	harness.MustDo(t, lotse, harness.Request{Method: http.MethodPatch, Path: "/runs/" + w.run, Body: map[string]any{
		"base_version": version, "ops": []map[string]any{{"op": "replace", "path": "/status_line", "value": "Writing the answer"}},
	}}, http.StatusOK)
	harness.MustDo(t, lotse, harness.Request{Method: http.MethodPost, Path: "/agents/me/heartbeat", Body: map[string]any{"state": "online"}}, http.StatusOK)
	// The adapter may patch over the gateway as well (op run.update, D-M7-08-1).
	conn, err := lotse.Gateway(t.Context())
	if err != nil {
		t.Fatalf("gateway of the agent in maintenance mode: %v", err)
	}
	defer conn.Close()
	if err := conn.Send("run.update", map[string]any{"run_id": w.run, "base_version": version + 1,
		"ops": []map[string]any{{"op": "replace", "path": "/status_line", "value": "Almost done"}}}); err != nil {
		t.Fatal(err)
	}
	testkit.Until(t, "the run.update of the gateway", func() bool {
		return w.count(`SELECT version FROM runs WHERE id = $1`, w.run) == version+2
	})
	harness.MustDo(t, lotse, harness.Request{Method: http.MethodPost, Path: "/threads/" + w.thread + "/messages", Body: map[string]any{
		"client_msg_id": newID(), "content_md": "Here is the answer",
	}}, http.StatusCreated)
	w.upload(lotse, w.channel)
	w.wantMaintenance("message in another thread", w.do(lotse, harness.Request{Method: http.MethodPost, Path: "/threads/" + other + "/messages",
		Body: map[string]any{"client_msg_id": newID(), "content_md": "Elsewhere"}}))
	w.wantMaintenance("startRun", w.do(lotse, harness.Request{Method: http.MethodPost, Path: "/runs/" + queued + "/start", Body: map[string]any{
		"invocation_token": "x", "billing_mode_used": "organization", "tool_class_used": "chat",
	}}))
	harness.MustDo(t, lotse, harness.Request{Method: http.MethodPost, Path: "/runs/" + w.run + "/finish", Body: map[string]any{
		"state": "completed", "summary": "Answered",
	}}, http.StatusOK)
	if st := w.text(`SELECT state FROM runs WHERE id = $1`, w.run); st != "completed" {
		t.Fatalf("run %s is %s after finishRun, want completed", w.run, st)
	}
	if st := w.text(`SELECT state FROM runs WHERE id = $1`, queued); st != "queued" {
		t.Fatalf("the queued run is %s, want queued", st)
	}
	// Once the run ended, its agent is back under the rule: no answer after the end.
	w.wantMaintenance("message after the end of the run", w.do(lotse, harness.Request{Method: http.MethodPost,
		Path: "/threads/" + w.thread + "/messages", Body: map[string]any{"client_msg_id": newID(), "content_md": "Late"}}))
}

// upload creates, sends and completes a one-part upload as c in channel.
func (w *world) upload(c *harness.Client, channel string) {
	w.t.Helper()
	body := []byte("answer attachment")
	sum := sha256.Sum256(body)
	var created struct {
		FileID string `json:"file_id"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/uploads", Body: map[string]any{
		"name": "answer.txt", "size_bytes": len(body), "sha256": hex.EncodeToString(sum[:]),
	}}, http.StatusCreated).JSON(w.t, &created)
	h := http.Header{}
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":")
	var part struct {
		ETag string `json:"etag"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPut, Path: "/uploads/" + created.FileID + "/parts/1", Body: body, Header: h},
		http.StatusOK).JSON(w.t, &part)
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/uploads/" + created.FileID + "/complete", Body: map[string]any{
		"parts": []map[string]any{{"part_number": 1, "etag": part.ETag}},
	}}, http.StatusOK)
}

// sessionRows returns the rows of the session, its refresh families and tokens and its device as
// text, the state 08 7.4 keeps unchanged by a refresh in maintenance mode.
func (w *world) sessionRows(c *harness.Client) string {
	w.t.Helper()
	return w.text(`SELECT concat_ws('|',
		(SELECT string_agg(s::text, ',' ORDER BY s.id) FROM auth_sessions s WHERE s.id = $1),
		(SELECT string_agg(f::text, ',' ORDER BY f.id) FROM refresh_token_families f WHERE f.session_id = $1),
		(SELECT string_agg(r::text, ',' ORDER BY r.id) FROM refresh_tokens r JOIN refresh_token_families f ON f.id = r.family_id WHERE f.session_id = $1),
		(SELECT string_agg(d::text, ',' ORDER BY d.id) FROM devices d WHERE d.id = $2))`, c.SessionID, c.DeviceID)
}

// refresh sends refreshToken with token as c (the DPoP proof of its device key) and returns the
// response.
func (w *world) refresh(c *harness.Client, token string) harness.Response {
	w.t.Helper()
	return c.DoAnonymous(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/auth/token/refresh", Body: map[string]any{"refresh_token": token}})
}

// TestMaintenance_RefreshWithoutRotation (08 7.4): in maintenance mode a refresh returns a new
// access token and the presented refresh token, the rows of the session, the refresh family and
// the device stay byte-identical, and the new access token reads.
func TestMaintenance_RefreshWithoutRotation(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.maintenance()
	alex := w.w.People["Alex"]
	access, token := alex.Tokens()
	before := w.sessionRows(alex)
	resp := w.refresh(alex, token)
	if resp.Status != http.StatusOK {
		t.Fatalf("refreshToken in maintenance mode: %d %s", resp.Status, resp.Body)
	}
	var pair struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	resp.JSON(t, &pair)
	if pair.RefreshToken != token || pair.AccessToken == "" || pair.AccessToken == access {
		t.Fatalf("refresh in maintenance mode: same refresh token %v, new access token %v", pair.RefreshToken == token, pair.AccessToken != access && pair.AccessToken != "")
	}
	if after := w.sessionRows(alex); after != before {
		t.Fatalf("the session rows changed:\nbefore %s\nafter  %s", before, after)
	}
	if err := alex.Adopt(resp.Body); err != nil {
		t.Fatal(err)
	}
	harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	// A second refresh with the same token works again: it was never marked used.
	if again := w.refresh(alex, token); again.Status != http.StatusOK {
		t.Fatalf("second refresh: %d %s", again.Status, again.Body)
	}
}

// TestMaintenance_ReuseRevokesFamily (ADR 0065 item 4): the reuse of a rotated refresh token after
// auth.refresh_reuse_grace still revokes the family and the session in maintenance mode.
func TestMaintenance_ReuseRevokesFamily(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"auth.refresh_reuse_grace": "1s"})
	alex := w.w.People["Alex"]
	_, old := alex.Tokens()
	resp := w.refresh(alex, old)
	if resp.Status != http.StatusOK {
		t.Fatalf("rotation before the mode: %d %s", resp.Status, resp.Body)
	}
	if err := alex.Adopt(resp.Body); err != nil {
		t.Fatal(err)
	}
	w.clk.Advance(2 * time.Second)
	w.maintenance()
	reuse := w.refresh(alex, old)
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(reuse.Body, &p)
	if reuse.Status != http.StatusUnauthorized || p.Code != "session_revoked" {
		t.Fatalf("reuse in maintenance mode: %d %s, want 401 session_revoked", reuse.Status, reuse.Body)
	}
	if n := w.count(`SELECT count(*) FROM refresh_token_families WHERE session_id = $1 AND revoked_at IS NOT NULL`, alex.SessionID); n == 0 {
		t.Fatal("the family of the reused token is not revoked")
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND revoked_at IS NOT NULL`, alex.SessionID); n != 1 {
		t.Fatal("the session of the reused token is not revoked")
	}
}

// TestMaintenance_DeviceRevokeWorks (ADR 0065 item 4): in maintenance mode the owner revokes a
// device with step-up, and the device's gateway connection closes with 4012.
func TestMaintenance_DeviceRevokeWorks(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.maintenance()
	alex := w.w.People["Alex"]
	conn, err := alex.Gateway(t.Context())
	if err != nil {
		t.Fatalf("gateway in maintenance mode: %v", err)
	}
	defer conn.Close()
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/devices/" + alex.DeviceID + "/revoke",
		Body: map[string]any{"reason": "lost"}}, http.StatusOK)
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	for {
		if _, err := conn.Next(ctx); err != nil {
			var ce *wstest.CloseError
			if !errors.As(err, &ce) || ce.Code != 4012 {
				t.Fatalf("connection ended with %v, want close 4012", err)
			}
			break
		}
	}
	if st := w.text(`SELECT state FROM devices WHERE id = $1`, alex.DeviceID); st != "revoked" {
		t.Fatalf("device is %s, want revoked", st)
	}
}

// TestMaintenance_SecondLine (ADR 0065 item 5): a writing action the stage takes for a read (an
// unclassified write) reaches its use case, whose unit of work runs READ ONLY: PostgreSQL refuses
// the write, the answer is 503 maintenance with Retry-After, and the database is unchanged.
func TestMaintenance_SecondLine(t *testing.T) {
	t.Parallel()
	w := newWorldWith(t, nil, func(o *harness.Options) {
		o.MaintenanceWrites = func(action string) bool {
			spec, ok := authzdomain.LookupAction(action)
			return action != "space.create" && (!ok || spec.Effect == authzdomain.EffectWrite)
		}
	})
	w.maintenance()
	before := w.tableState()
	w.wantMaintenance("createSpace through the second line", w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces",
		Body: map[string]any{"name": "During the move"}}))
	wantSameTables(t, before, w.tableState())
}

// TestMaintenance_WorkerOnlyDeliveries (02 12.2): in maintenance mode the worker runs a job of the
// queue delivery and runs.cancel_timeout; a job of another kind on the queue maintenance and a job
// of the queue default stay unfinished, no periodic job of another kind is inserted, and the
// consumer push follows the log.
func TestMaintenance_WorkerOnlyDeliveries(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	alex := w.w.People["Alex"]
	w.maintenance()
	since := w.text(`SELECT now()::text`)
	insert := func(kind, queue, args string) int64 {
		return w.count(`INSERT INTO jobs.river_job (args, kind, max_attempts, queue, state, scheduled_at, priority)
			VALUES ($1::jsonb, $2, 3, $3, 'available', now(), 1) RETURNING id`, args, kind, queue)
	}
	delivery := insert("notify.badge_sync", "delivery", `{"user_id":"`+alex.PrincipalID+`","after_seq":0,"at":1}`)
	cancel := insert("runs.cancel_timeout", "maintenance", `{"run_id":"`+w.run+`"}`)
	cleanup := insert("identity.auth_cleanup", "maintenance", `{}`)
	expiry := insert("runs.queued_expiry", "default", `{"run_id":"`+w.run+`"}`)
	testkit.Until(t, "the delivery job and runs.cancel_timeout ran", func() bool {
		return w.count(`SELECT count(*) FROM jobs.river_job WHERE id = ANY($1) AND finalized_at IS NOT NULL`, []int64{delivery, cancel}) == 2
	})
	if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE id = ANY($1) AND (finalized_at IS NOT NULL OR cardinality(errors) > 0)`,
		[]int64{cleanup, expiry}); n != 0 {
		t.Fatalf("%d paused jobs ran", n)
	}
	if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE created_at > $1::timestamptz AND NOT (id = ANY($2)) AND queue <> 'delivery'`,
		since, []int64{delivery, cancel, cleanup, expiry}); n != 0 {
		t.Fatalf("%d jobs of other queues were inserted in maintenance mode", n)
	}
	// The consumer push follows the log: the answer of the running run appends message.created.
	harness.MustDo(t, w.w.Agents["Lotse"], harness.Request{Method: http.MethodPost, Path: "/threads/" + w.thread + "/messages", Body: map[string]any{
		"client_msg_id": newID(), "content_md": "Answer during the move",
	}}, http.StatusCreated)
	head := w.count(`SELECT max(seq) FROM event_log`)
	testkit.Until(t, "the consumer push reaches the head", func() bool {
		return w.count(`SELECT last_seq FROM event_consumer_cursors WHERE name = 'push'`) >= head
	})
}

// TestMaintenance_StatusShowsMaintenance (ADR 0065 item 6): getStatus reports every component of
// the instance and the overall state as maintenance, from the configuration, without writing
// status_components.
func TestMaintenance_StatusShowsMaintenance(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	testkit.Until(t, "the standard components", func() bool {
		return w.count(`SELECT count(*) FROM status_components WHERE agent_id IS NULL`) >= 4
	})
	w.maintenance()
	rows := w.text(`SELECT string_agg(c::text, ',' ORDER BY c.id) FROM status_components c`)
	var page struct {
		OverallState string `json:"overall_state"`
		Components   []struct {
			State   string  `json:"state"`
			AgentID *string `json:"agent_id"`
		} `json:"components"`
	}
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodGet, Path: "/status"}, http.StatusOK).JSON(t, &page)
	if page.OverallState != "maintenance" {
		t.Fatalf("overall_state %q, want maintenance", page.OverallState)
	}
	for _, c := range page.Components {
		if c.AgentID == nil && c.State != "maintenance" {
			t.Fatalf("component state %q, want maintenance", c.State)
		}
	}
	if after := w.text(`SELECT string_agg(c::text, ',' ORDER BY c.id) FROM status_components c`); after != rows {
		t.Fatal("status_components changed in maintenance mode")
	}
}

// TestMaintenance_QueuedRunsWait (ADR 0065 item 4, D-M7-08-13): in maintenance mode the delivery
// of a queued run's agent.invocation is held, also for an agent whose other run is running: its
// poll answers without the invocation, the queued run keeps its row, and after the mode the
// invocation arrives.
func TestMaintenance_QueuedRunsWait(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	lotse := w.w.Agents["Lotse"]
	thread := w.newThread(w.s.Owner, w.channel, "Second question for Lotse")
	queued := w.text(`SELECT id::text FROM runs WHERE thread_id = $1 AND state = 'queued'`, thread)
	row := func() string { return w.text(`SELECT r::text FROM runs r WHERE id = $1`, queued) }
	w.maintenance()
	before := row()
	var page struct {
		Events []struct {
			Type     string `json:"type"`
			Resource struct {
				ID string `json:"id"`
			} `json:"resource"`
		} `json:"events"`
	}
	harness.MustDo(t, lotse, harness.Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"}, http.StatusOK).JSON(t, &page)
	for _, e := range page.Events {
		if e.Type == "agent.invocation" && e.Resource.ID == queued {
			t.Fatal("the invocation of the queued run was delivered in maintenance mode")
		}
	}
	if after := row(); after != before {
		t.Fatalf("the queued run changed in maintenance mode:\nbefore %s\nafter  %s", before, after)
	}
	w.s.Restart(t, harness.RestartOptions{Config: map[string]string{"server.maintenance_mode": ""}})
	if token := w.invocationToken(lotse, queued); token == "" {
		t.Fatal("no invocation after the mode")
	}
}

// TestMaintenance_GatewayHeartbeatReadiness (D-M7-08-9): in maintenance mode a gateway heartbeat
// of an agent that reports the readiness of its project still stores it, like the REST heartbeat.
func TestMaintenance_GatewayHeartbeatReadiness(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	o, lotse := w.s.Owner, w.w.Agents["Lotse"]
	forum := w.w.Channels["Engineering/Projekt-A"]
	cur := harness.MustDo(t, o, harness.Request{Method: http.MethodGet, Path: "/channels/" + forum}, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	harness.MustDo(t, o, harness.Request{Method: http.MethodPut, Path: "/channels/" + forum + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": lotse.PrincipalID, "reason": "project of Lotse"}}, http.StatusOK)
	harness.MustDo(t, o, harness.Request{Method: http.MethodPut, Path: "/channels/" + forum + "/project-binding", Body: map[string]any{
		"workspace_ref": "ws", "repository_url": "https://git.example.com/team/project", "default_branch": "main",
		"runner_config": map[string]any{"model": "m", "allowed_tools": []string{"Read"}}, "reason": "project of Lotse",
	}}, http.StatusOK)
	ready := func() string { return w.text(`SELECT ready::text FROM project_bindings WHERE channel_id = $1`, forum) }
	w.maintenance()
	conn, err := lotse.Gateway(t.Context())
	if err != nil {
		t.Fatalf("gateway of the agent in maintenance mode: %v", err)
	}
	defer conn.Close()
	want := "true"
	if ready() == "true" {
		want = "false"
	}
	if err := conn.Send("heartbeat", map[string]any{"last_seq": 0, "agent": map[string]any{
		"state": "online", "project_readiness": []map[string]any{{"channel_id": forum, "ready": want == "true"}},
	}}); err != nil {
		t.Fatal(err)
	}
	testkit.Until(t, "the readiness of the gateway heartbeat is stored", func() bool { return ready() == want })
}
