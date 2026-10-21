// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The windows of the live-box world: a coalescing window long enough that a burst of gateway
// patches lands in one window, and a step bound small enough to drive the summarization.
const (
	lbCoalesce = 300 * time.Millisecond
	lbMaxSteps = 3
)

// lbBaseEnv is the environment of every live-box world: the windows of the consumer world
// (M2-07), the rate limits raised for the test (never disabled) and agent.cancel_grace short, so
// the job runs.cancel_timeout of a worker process ends a cancelled run within the test.
func lbBaseEnv() []string {
	return []string{
		"BOTHY_AGENT__ACK_TIMEOUT=" + consumerAckTimeout.String(), "BOTHY_AGENT__ACK_PERSIST_INTERVAL=" + consumerPersist.String(),
		"BOTHY_AGENT__MAX_INFLIGHT_EVENTS=" + strconv.Itoa(consumerInflight), "BOTHY_AUDIT__DENY_AGGREGATE_WINDOW=1s",
		"BOTHY_RATELIMIT__RL__MESSAGE__BURST=1000", "BOTHY_RATELIMIT__RL__ADMIN__BURST=1000", "BOTHY_RATELIMIT__RL__WRITE__BURST=1000",
		"BOTHY_RATELIMIT__RL__READ__BURST=5000", "BOTHY_RATELIMIT__RL__READ__PER_MINUTE=60000",
		"BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST=5000", "BOTHY_RATELIMIT__RL__AGENT_RUNTIME__PER_MINUTE=60000",
		"BOTHY_AGENT__CANCEL_GRACE=1s",
		"BOTHY_SYNC__RUN_PATCH_COALESCE_INTERVAL=" + lbCoalesce.String(), "BOTHY_RUN__MAX_STEPS_PERSISTED=" + strconv.Itoa(lbMaxSteps),
	}
}

// lbWorld is the invocation world of M2-08 (Support with the Owner Alex, Sam without owner role)
// with Robin, who has no grant, and the configuration of the live box.
type lbWorld struct {
	*invocationWorld
	robin gwPerson
}

// newLBWorld starts a live-box world with the environment of lbBaseEnv and extra.
func newLBWorld(t *testing.T, extra ...string) *lbWorld {
	t.Helper()
	dl := newDeliveryWorld(t, append(lbBaseEnv(), extra...)...)
	cw := &consumerWorld{dlWorld: dl, space: dl.newSpace("Support")}
	iw := &invocationWorld{consumerWorld: cw, sam: dl.newPerson("sam", identity.DeviceStateActive)}
	return &lbWorld{invocationWorld: iw, robin: dl.newPerson("robin", identity.DeviceStateActive)}
}

// lbRun is a running run of an operator agent (Lotse) in a channel of Support, with the agent's
// gateway connection.
type lbRun struct {
	id, thread, channel string
	agent               consumerAgent
	conn                *wstest.Client
}

// running creates the channel name operated by a new agent Lotse, lets Alex (the Owner) trigger
// it, delivers the invocation over the agent's gateway connection, acknowledges it (so no
// redelivery renews the token and moves the version) and starts the run with startRun (M2-09).
func (w *lbWorld) running(t *testing.T, name string) lbRun {
	t.Helper()
	ch, a, handle := w.operated(name, false)
	w.sql(`UPDATE principals SET display_name = 'Lotse' WHERE id = $1`, a.id.String())
	c, _ := w.ready(t, w.agentParams(a, ""))
	readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
	m, _ := w.say(w.owner, ch.thread, "@"+handle+" please check the export")
	f, ok := w.nextInvocation(c, 15*time.Second, true)
	if !ok {
		t.Fatal("no invocation reached Lotse")
	}
	sendAck(t, c, f.Seq)
	r, ok := w.runOf(m)
	if !ok {
		t.Fatal("no run for the trigger")
	}
	code, b := w.call(w.base, a, http.MethodPost, "/api/v1/runs/"+r.id+"/start",
		map[string]any{"invocation_token": f.InvocationToken, "billing_mode_used": "organization", "tool_class_used": "chat"})
	if code != http.StatusOK {
		t.Fatalf("startRun = %d %s", code, b)
	}
	if got, _ := w.runOf(m); got.state != "running" {
		t.Fatalf("run state %s after startRun", got.state)
	}
	return lbRun{id: r.id, thread: r.thread, channel: ch.id, agent: a, conn: c}
}

// version reads runs.version of run.
func (w *lbWorld) version(run string) int64 {
	w.t.Helper()
	var v int64
	if err := w.queryRow(context.Background(), `SELECT version FROM runs WHERE id = $1`, run).Scan(&v); err != nil {
		w.t.Fatal(err)
	}
	return v
}

// rowJSON reads the whole runs row of run as JSON, for "every column unchanged".
func (w *lbWorld) rowJSON(run string) string {
	w.t.Helper()
	var s string
	if err := w.queryRow(context.Background(), `SELECT to_jsonb(r)::text FROM runs r WHERE id = $1`, run).Scan(&s); err != nil {
		w.t.Fatal(err)
	}
	return s
}

// counter reads the value of the sequence counter name.
func (w *lbWorld) counter(name string) int64 {
	w.t.Helper()
	var v int64
	if err := w.queryRow(context.Background(), `SELECT value FROM sequence_counters WHERE name = $1`, name).Scan(&v); err != nil {
		w.t.Fatal(err)
	}
	return v
}

// lbResp is one HTTP answer with its headers.
type lbResp struct {
	code   int
	header http.Header
	body   []byte
}

// agentDo sends a REST request as the agent a with the raw JSON body (nil for none) and returns
// the answer; it never fails the test, so goroutines may call it.
func (w *lbWorld) agentDo(a consumerAgent, method, path string, body []byte) (lbResp, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, w.base+path, rd)
	if err != nil {
		return lbResp{}, err
	}
	req.Header.Set("Authorization", "DPoP "+a.tok)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(a.key, identitytest.ProofClaims{HTM: method, HTU: gwPublicURL + path,
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(a.tok)}))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return lbResp{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return lbResp{code: resp.StatusCode, header: resp.Header, body: b}, err
}

// patchBody encodes the body RunPatch {base_version, ops}.
func patchBody(t *testing.T, base int64, ops []map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"base_version": base, "ops": ops})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// patch sends patchRun as a on run and returns the answer.
func (w *lbWorld) patch(t *testing.T, a consumerAgent, run string, base int64, ops ...map[string]any) lbResp {
	t.Helper()
	r, err := w.agentDo(a, http.MethodPatch, "/api/v1/runs/"+run, patchBody(t, base, ops))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// patchOK sends patchRun and requires 200 with RunPatchResult.version = base + 1.
func (w *lbWorld) patchOK(t *testing.T, a consumerAgent, run string, base int64, ops ...map[string]any) int64 {
	t.Helper()
	r := w.patch(t, a, run, base, ops...)
	var res rest.RunPatchResult
	if r.code != http.StatusOK || wstest.Strict(r.body, &res) != nil || res.Version != base+1 {
		t.Fatalf("patchRun at %d = %d %s", base, r.code, r.body)
	}
	return res.Version
}

// op builds one RFC 6902 operation; value nil leaves the member out.
func op(kind, path string, value any) map[string]any {
	o := map[string]any{"op": kind, "path": path}
	if value != nil {
		o["value"] = value
	}
	return o
}

// getRun reads the run as Alex and returns the body decoded and raw.
func (w *lbWorld) getRun(t *testing.T, run string) (any, []byte) {
	t.Helper()
	code, b := w.rest(w.base, w.owner, http.MethodGet, "/api/v1/runs/"+run, nil)
	if code != http.StatusOK {
		t.Fatalf("getRun = %d %s", code, b)
	}
	return decodeAny(t, b), b
}

// decodeAny decodes JSON into the generic form the comparisons use.
func decodeAny(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decoding %s: %v", b, err)
	}
	return v
}

// lbProblem is the part of a problem the tests read.
type lbProblem struct {
	Code    string          `json:"code"`
	Status  int             `json:"status"`
	Reason  string          `json:"reason"`
	Detail  string          `json:"detail"`
	Current json.RawMessage `json:"current"`
	Errors  []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// problemOf decodes the problem of r and requires status and code.
func problemOf(t *testing.T, r lbResp, status int, code string) lbProblem {
	t.Helper()
	var p lbProblem
	if err := json.Unmarshal(r.body, &p); err != nil || r.code != status || p.Code != code {
		t.Fatalf("answer %d %s, want %d %s", r.code, r.body, status, code)
	}
	return p
}

// runUpdate sends the gateway op run.update on c.
func runUpdate(t *testing.T, c *wstest.Client, run string, base int64, ref string, ops ...map[string]any) {
	t.Helper()
	d := map[string]any{"run_id": run, "base_version": base, "ops": ops}
	if ref != "" {
		d["ref"] = ref
	}
	if err := c.Send("run.update", d); err != nil {
		t.Fatal(err)
	}
}

// nextError reads frames of c until an error frame arrives and returns it decoded strictly;
// dispatches on the way are acknowledged, so an agent consumer keeps its window open.
func nextError(t *testing.T, c *wstest.Client) rest.GatewayError {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for {
		op, d, err := c.Next(ctx)
		if err != nil {
			t.Fatalf("waiting for an error frame: %v", err)
		}
		switch op {
		case "dispatch":
			var ev struct {
				Seq *int64 `json:"seq"`
			}
			if json.Unmarshal(d, &ev) == nil && ev.Seq != nil {
				sendAck(t, c, *ev.Seq)
			}
		case "error":
			var e rest.GatewayError
			if err := wstest.Strict(d, &e); err != nil {
				t.Fatalf("error frame %s: %v", d, err)
			}
			return e
		}
	}
}

// wantOpError requires an error frame of run.update with code and the echoed ref.
func wantOpError(t *testing.T, c *wstest.Client, code, ref string) rest.GatewayError {
	t.Helper()
	e := nextError(t, c)
	if e.Op != "run.update" || e.Code != code || e.Ref == nil || *e.Ref != ref {
		t.Fatalf("error frame %+v (detail %v), want run.update %s with ref %s", e, deref(e.Detail), code, ref)
	}
	return e
}

// deref returns *s or "".
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// lbOp is one operation of a run.updated frame.
type lbOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// lbEnvelope is the dispatch envelope of run.updated as the tests read it.
type lbEnvelope struct {
	Seq               *int64 `json:"seq"`
	PermissionVersion *int64 `json:"permission_version"`
	Type              string `json:"type"`
	Truncated         bool   `json:"truncated"`
	Resource          struct {
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		Version int64  `json:"version"`
	} `json:"resource"`
	Payload struct {
		RunID       string          `json:"run_id"`
		BaseVersion *int64          `json:"base_version"`
		Version     int64           `json:"version"`
		Ops         []lbOp          `json:"ops"`
		Snapshot    json.RawMessage `json:"snapshot"`
	} `json:"payload"`
}

// lbFrame is one run.updated dispatch: the raw envelope and its decoded form.
type lbFrame struct {
	raw json.RawMessage
	ev  lbEnvelope
}

// snapshot reports whether f carries the whole document.
func (f lbFrame) snapshot() bool { return len(f.ev.Payload.Snapshot) > 0 }

// nextRunUpdated reads frames of c until a run.updated of run arrives or d passed (ok false).
// Every run.updated passes the ephemeral rules: no seq, no permission_version, resource {kind run,
// id, version} and no JSON null.
func nextRunUpdated(t *testing.T, c *wstest.Client, run string, d time.Duration) (lbFrame, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			return lbFrame{}, false
		}
		if err != nil {
			t.Fatalf("waiting for run.updated: %v", err)
		}
		if op != "dispatch" {
			continue
		}
		var ev lbEnvelope
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != "run.updated" {
			continue
		}
		if ev.Seq != nil || ev.PermissionVersion != nil || ev.Resource.Kind != "run" || ev.Resource.ID != ev.Payload.RunID ||
			ev.Resource.Version != ev.Payload.Version || bytes.Contains(raw, []byte("null")) {
			t.Fatalf("run.updated breaks the ephemeral rules: %s", raw)
		}
		if ev.Payload.RunID != run {
			continue
		}
		return lbFrame{raw: raw, ev: ev}, true
	}
}

// mustRunUpdated is nextRunUpdated that requires a frame within 15 seconds.
func mustRunUpdated(t *testing.T, c *wstest.Client, run string) lbFrame {
	t.Helper()
	f, ok := nextRunUpdated(t, c, run, 15*time.Second)
	if !ok {
		t.Fatalf("no run.updated of run %s", run)
	}
	return f
}

// noRunUpdated requires that no run.updated of run arrives on c within d.
func noRunUpdated(t *testing.T, c *wstest.Client, run string, d time.Duration, who string) {
	t.Helper()
	if f, ok := nextRunUpdated(t, c, run, d); ok {
		t.Fatalf("%s received run.updated %s", who, f.raw)
	}
}

// focus sends focus {thread_ids} on c.
func focus(t *testing.T, c *wstest.Client, threads ...string) {
	t.Helper()
	if threads == nil {
		threads = []string{}
	}
	if err := c.Send("focus", map[string]any{"thread_ids": threads}); err != nil {
		t.Fatal(err)
	}
}

// applyOps applies RFC 6902 add, replace and remove (sections 4.1 to 4.3) to the decoded
// document doc and returns the result; a replace or remove of a missing target, or a path through
// a missing container, is an error. It is the independent applier of design note 4, written from
// the RFC for this test.
func applyOps(doc any, ops []lbOp) (any, error) {
	for i, o := range ops {
		var v any
		if o.Op != "remove" {
			if err := json.Unmarshal(o.Value, &v); err != nil {
				return nil, fmt.Errorf("op %d: value: %w", i, err)
			}
		}
		var err error
		if doc, err = applyOne(doc, o.Op, o.Path, v); err != nil {
			return nil, fmt.Errorf("op %d %s %s: %w", i, o.Op, o.Path, err)
		}
	}
	return doc, nil
}

// applyOne applies one operation at the JSON Pointer path (RFC 6901) and returns the new root.
func applyOne(root any, kind, path string, v any) (any, error) {
	if path == "" {
		if kind == "remove" {
			return nil, errors.New("remove of the root")
		}
		return v, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, errors.New("pointer without leading slash")
	}
	segs := strings.Split(path[1:], "/")
	for i, s := range segs {
		segs[i] = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
	}
	parent := root
	for _, s := range segs[:len(segs)-1] {
		switch p := parent.(type) {
		case map[string]any:
			next, ok := p[s]
			if !ok {
				return nil, fmt.Errorf("missing member %q", s)
			}
			parent = next
		case []any:
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 || n >= len(p) {
				return nil, fmt.Errorf("bad index %q", s)
			}
			parent = p[n]
		default:
			return nil, fmt.Errorf("no container at %q", s)
		}
	}
	last := segs[len(segs)-1]
	switch p := parent.(type) {
	case map[string]any:
		_, exists := p[last]
		switch kind {
		case "add":
			p[last] = v
		case "replace":
			if !exists {
				return nil, fmt.Errorf("replace of missing member %q", last)
			}
			p[last] = v
		case "remove":
			if !exists {
				return nil, fmt.Errorf("remove of missing member %q", last)
			}
			delete(p, last)
		default:
			return nil, fmt.Errorf("op %q", kind)
		}
		return root, nil
	case []any:
		// Arrays are changed in place through their parent; the run document keys its
		// collections by id, so only whole-value replacements reach arrays in practice.
		return nil, errors.New("array targets are not used by run_document.v1 patches")
	}
	return nil, errors.New("no container for the last segment")
}

// rfc3339ms formats t as the API writes timestamps: UTC, milliseconds, suffix Z.
func rfc3339ms(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// TestLiveBoxAG12PatchStateRejected is AG-12 (09 "Required scenarios"), the path half, against a
// started api process: a patch on /state of Lotse's running run answers 400 validation_failed
// with errors[0] {pointer /ops/0, code patch_path_forbidden} over REST, and an error frame of
// run.update with validation_failed and the echoed ref over the gateway, whose connection stays
// open; runs.version and every column of the row are unchanged. It uses only REST, SQL and gateway
// frames, so it also runs against the code before M2-10 (the red run).
func TestLiveBoxAG12PatchStateRejected(t *testing.T) {
	t.Parallel()
	w := newLBWorld(t)
	r := w.running(t, "ag12-state")
	before, row := w.version(r.id), w.rowJSON(r.id)
	state := op("replace", "/state", "completed")

	p := problemOf(t, w.patch(t, r.agent, r.id, before, state), http.StatusBadRequest, "validation_failed")
	if len(p.Errors) == 0 || p.Errors[0].Pointer != "/ops/0" || p.Errors[0].Code != "patch_path_forbidden" {
		t.Fatalf("errors %+v", p.Errors)
	}

	runUpdate(t, r.conn, r.id, before, "ag12-state", state)
	e := wantOpError(t, r.conn, "validation_failed", "ag12-state")
	if d := deref(e.Detail); d != "/ops/0: patch_path_forbidden" {
		t.Fatalf("detail %q", d)
	}
	// The connection stays open: a second rejected op answers a second error frame.
	runUpdate(t, r.conn, r.id, before, "ag12-version", op("replace", "/version", 99))
	wantOpError(t, r.conn, "validation_failed", "ag12-version")

	if got := w.version(r.id); got != before {
		t.Fatalf("runs.version %d, want %d", got, before)
	}
	if got := w.rowJSON(r.id); got != row {
		t.Fatalf("the row changed:\n%s\n%s", row, got)
	}
}

// TestLiveBoxAG12StaleBaseVersion is AG-12, the version half: a patch with an old base_version
// answers 409 base_version_conflict whose current equals the body of getRun at that moment, and
// run.update answers an error frame with base_version_conflict. Like the path half it needs no
// symbol of M2-10.
func TestLiveBoxAG12StaleBaseVersion(t *testing.T) {
	t.Parallel()
	w := newLBWorld(t)
	r := w.running(t, "ag12-stale")
	v := w.version(r.id)
	line := op("replace", "/status_line", "Reading the export logs")

	resp := w.patch(t, r.agent, r.id, v-1, line)
	p := problemOf(t, resp, http.StatusConflict, "base_version_conflict")
	doc, raw := w.getRun(t, r.id)
	if len(p.Current) == 0 || !reflect.DeepEqual(decodeAny(t, p.Current), doc) {
		t.Fatalf("current %s, getRun %s", p.Current, raw)
	}

	runUpdate(t, r.conn, r.id, v+5, "ag12-stale", line)
	wantOpError(t, r.conn, "base_version_conflict", "ag12-stale")
	if got := w.version(r.id); got != v {
		t.Fatalf("runs.version %d after the rejected patches, want %d", got, v)
	}
}

// TestLiveBoxPatches is the integration suite of M2-10 against one started api process with real
// modules (PostgreSQL 18 via pgtest, HTTP through the real chain, the gateway through wstest):
// sync.run_patch_coalesce_interval 300ms and run.max_steps_persisted 3. The subtests run one after
// the other, because the load test measures the global counters of the world.
func TestLiveBoxPatches(t *testing.T) {
	t.Parallel()
	w := newLBWorld(t)

	t.Run("TestLiveBoxDurableEventsIndependentOfPatchRate", func(t *testing.T) {
		a := w.running(t, "rate-a")
		b := w.running(t, "rate-b")
		anchor := func(run string) (int64, int) {
			var id string
			var v int64
			if err := w.queryRow(context.Background(), `SELECT id::text, version FROM messages WHERE run_id = $1 AND kind = 'run'`, run).Scan(&id, &v); err != nil {
				t.Fatal(err)
			}
			return v, w.count(`SELECT count(*) FROM message_revisions WHERE message_id = $1`, id)
		}
		anchorA, revA := anchor(a.id)
		anchorB, revB := anchor(b.id)
		// LISTEN event_log on a connection of its own: no patch may send a NOTIFY.
		ctx := context.Background()
		listen, err := w.db.Pool(t, pg.RoleOwner).DialDedicated(ctx, "livebox-listen")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = listen.Close(context.Background()) }()
		if _, err := listen.Exec(ctx, `LISTEN event_log`); err != nil {
			t.Fatal(err)
		}
		events, audits := w.counter("event_seq"), w.counter("audit_seq")

		// Run A: three patches; run B: thirty, half over REST and half over run.update.
		drive := func(r lbRun, n int) {
			base := w.version(r.id)
			for i := range n / 2 {
				base = w.patchOK(t, r.agent, r.id, base, op("replace", "/status_line", "Step "+strconv.Itoa(i)),
					op("add", "/metrics/tokens_in", 100*(i+1)))
			}
			for i := range n - n/2 {
				runUpdate(t, r.conn, r.id, base, "", op("replace", "/status_line", "Gateway step "+strconv.Itoa(i)))
				base++
			}
			eventually(t, 15*time.Second, "the gateway patches of the run", func() bool { return w.version(r.id) == base })
		}
		drive(a, 3)
		drive(b, 30)

		if got := w.counter("event_seq"); got != events {
			t.Fatalf("event_seq moved from %d to %d during the patches", events, got)
		}
		if got := w.counter("audit_seq"); got != audits {
			t.Fatalf("audit_seq moved from %d to %d during the patches", audits, got)
		}
		wctx, cancel := context.WithTimeout(ctx, 2*lbCoalesce)
		n, err := listen.WaitForNotification(wctx)
		cancel()
		if err == nil {
			t.Fatalf("a NOTIFY on event_log during the patches: %+v", n)
		}
		if v, rev := anchor(a.id); v != anchorA || rev != revA {
			t.Fatalf("anchor of A: version %d revisions %d, before %d %d", v, rev, anchorA, revA)
		}
		if v, rev := anchor(b.id); v != anchorB || rev != revB {
			t.Fatalf("anchor of B: version %d revisions %d, before %d %d", v, rev, anchorB, revB)
		}
		for _, r := range []lbRun{a, b} {
			if code, body := w.call(w.base, r.agent, http.MethodPost, "/api/v1/runs/"+r.id+"/finish", map[string]any{"state": "completed"}); code != http.StatusOK {
				t.Fatalf("finishRun = %d %s", code, body)
			}
		}
		types := func(run string) string {
			var s string
			if err := w.queryRow(ctx, `SELECT coalesce(string_agg(type || '=' || n, ','), '') FROM (SELECT type, count(*) AS n FROM event_log
				WHERE resource_id = $1 GROUP BY type ORDER BY type) e`, run).Scan(&s); err != nil {
				t.Fatal(err)
			}
			return s
		}
		if ta, tb := types(a.id), types(b.id); ta != tb || ta == "" {
			t.Fatalf("durable events of A %q and of B %q", ta, tb)
		}
	})

	t.Run("TestLiveBoxPatchRunStoresDocument", func(t *testing.T) {
		r := w.running(t, "document")
		v := w.version(r.id)
		step := ids.New().String()
		finished := rfc3339ms(time.Now())
		retry := map[string]any{"attempt": 2, "error": "overloaded", "max_retries": 5, "retry_delay_ms": 4000}
		plan := map[string]any{"items": []any{
			map[string]any{"id": "step-1", "text": "Read the logs", "status": "in_progress"},
			map[string]any{"id": "step-2", "text": "Propose a fix", "status": "pending"},
		}}
		cost := map[string]any{"amount_micros": 12500, "currency": "EUR", "is_estimate": true}
		for _, o := range []map[string]any{
			op("replace", "/status_line", "Reading the export logs"),
			op("replace", "/model", "model-large"),
			op("add", "/metrics/tokens_in", 1200),
			op("replace", "/metrics/context_pct", 42.5),
			op("add", "/metrics/last_api_retry", retry),
			op("add", "/plan", plan),
			op("replace", "/cost", cost),
			op("add", "/steps/"+step, map[string]any{"kind": "tool_call", "name": "read_file", "title": "Reading export.log"}),
			op("replace", "/steps/"+step+"/status", "completed"),
			op("replace", "/steps/"+step+"/finished_at", finished),
		} {
			v = w.patchOK(t, r.agent, r.id, v, o)
		}
		if got := w.version(r.id); got != v {
			t.Fatalf("runs.version %d, last RunPatchResult %d", got, v)
		}
		_, raw := w.getRun(t, r.id)
		var doc rest.Run
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		s, ok := doc.Steps[step]
		switch {
		case doc.Version != v, deref(doc.StatusLine) != "Reading the export logs", deref(doc.Model) != "model-large":
			t.Fatalf("document %s", raw)
		case doc.Metrics.TokensIn == nil || *doc.Metrics.TokensIn != 1200 || doc.Metrics.ContextPct == nil || *doc.Metrics.ContextPct != 42.5:
			t.Fatalf("metrics %s", raw)
		case doc.Metrics.LastApiRetry == nil || doc.Metrics.LastApiRetry.Attempt != 2 || deref(doc.Metrics.LastApiRetry.Error) != "overloaded":
			t.Fatalf("last_api_retry %s", raw)
		case doc.Plan == nil || len(doc.Plan.Items) != 2 || doc.Plan.Items[0].Id != "step-1" || string(doc.Plan.Items[1].Status) != "pending":
			t.Fatalf("plan %s", raw)
		case doc.Cost.AmountMicros != 12500 || doc.Cost.Currency != "EUR" || !doc.Cost.IsEstimate:
			t.Fatalf("cost %s", raw)
		case !ok || s.Ordinal != 1 || s.Name != "read_file" || s.Title != "Reading export.log" || string(s.Status) != "completed" ||
			s.FinishedAt == nil || rfc3339ms(time.Time(*s.FinishedAt)) != finished:
			t.Fatalf("step %s", raw)
		}
	})

	t.Run("TestLiveBoxPatchRunConcurrentSameBase", func(t *testing.T) {
		r := w.running(t, "concurrent")
		base := w.version(r.id)
		lines := []string{"First writer", "Second writer"}
		resps := make([]lbResp, len(lines))
		errs := make([]error, len(lines))
		var wg gosync.WaitGroup
		for i, l := range lines {
			wg.Go(func() {
				resps[i], errs[i] = w.agentDo(r.agent, http.MethodPatch, "/api/v1/runs/"+r.id, patchBody(t, base, []map[string]any{op("replace", "/status_line", l)}))
			})
		}
		wg.Wait()
		var won []int
		for i, resp := range resps {
			if errs[i] != nil {
				t.Fatal(errs[i])
			}
			switch resp.code {
			case http.StatusOK:
				won = append(won, i)
			case http.StatusConflict:
				problemOf(t, resp, http.StatusConflict, "base_version_conflict")
			default:
				t.Fatalf("patch %d = %d %s", i, resp.code, resp.body)
			}
		}
		if len(won) != 1 {
			t.Fatalf("%d of two patches with the same base were stored", len(won))
		}
		var line string
		if err := w.queryRow(context.Background(), `SELECT coalesce(status_line, '') FROM runs WHERE id = $1`, r.id).Scan(&line); err != nil {
			t.Fatal(err)
		}
		if got := w.version(r.id); got != base+1 || line != lines[won[0]] {
			t.Fatalf("version %d status_line %q, want %d %q", got, line, base+1, lines[won[0]])
		}
	})

	t.Run("TestLiveBoxPatchRunNotOwnRun", func(t *testing.T) {
		r := w.running(t, "not-own")
		v := w.version(r.id)
		line := op("replace", "/status_line", "Not mine")
		// Texter is an agent of Marketing without any grant on Support.
		texter := w.agent()
		w.sql(`UPDATE principals SET display_name = 'Texter' WHERE id = $1`, texter.id.String())
		hidden := w.patch(t, texter, r.id, v, line)
		unknownID := ids.New().String()
		unknown := w.patch(t, texter, unknownID, v, line)
		problemOf(t, hidden, http.StatusNotFound, "not_found")
		problemOf(t, unknown, http.StatusNotFound, "not_found")
		// The bodies differ only in the request path and the correlation id.
		norm := func(b []byte, id string) any {
			m, _ := decodeAny(t, bytes.ReplaceAll(b, []byte(id), []byte("{run_id}"))).(map[string]any)
			delete(m, "correlation_id")
			return m
		}
		if a, b := norm(hidden.body, r.id), norm(unknown.body, unknownID); !reflect.DeepEqual(a, b) {
			t.Fatalf("hidden run %s, unknown run %s", hidden.body, unknown.body)
		}
		headers := func(h http.Header) []string {
			var out []string
			for k := range h {
				// The correlation id is per request, and Ratelimit counts the remaining tokens down.
				switch k {
				case "Date", "Content-Length", "Correlation-Id", "Ratelimit":
					continue
				}
				out = append(out, k+": "+strings.Join(h.Values(k), ","))
			}
			slices.Sort(out)
			return out
		}
		if a, b := headers(hidden.header), headers(unknown.header); !slices.Equal(a, b) {
			t.Fatalf("headers of the hidden run %v, of the unknown run %v", a, b)
		}
		// A third agent with write on the channel sees the run and is refused (not_self).
		writer := w.agent()
		w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
			"subject_kind": "agent", "subject_id": writer.id.String(), "target_kind": "channel", "target_id": r.channel, "level": "write", "reason": "test"})
		p := problemOf(t, w.patch(t, writer, r.id, v, line), http.StatusForbidden, "forbidden")
		if p.Reason != "not_self" {
			t.Fatalf("reason %q for another agent, want not_self", p.Reason)
		}
		// Alex with his session is a person (subject_kind_not_allowed).
		code, body := w.rest(w.base, w.owner, http.MethodPatch, "/api/v1/runs/"+r.id, map[string]any{"base_version": v, "ops": []any{line}})
		pa := problemOf(t, lbResp{code: code, body: body}, http.StatusForbidden, "forbidden")
		if pa.Reason != "subject_kind_not_allowed" {
			t.Fatalf("reason %q for a person, want subject_kind_not_allowed", pa.Reason)
		}
		if got := w.version(r.id); got != v {
			t.Fatalf("runs.version %d after the denials, want %d", got, v)
		}
		for name, who := range map[string]ids.ID{"Texter": texter.id, "the writer agent": writer.id, "Alex": w.owner.user} {
			eventually(t, 15*time.Second, "the deny entry of "+name, func() bool {
				return w.count(`SELECT count(*) FROM audit_events WHERE action = 'run.update' AND decision = 'deny' AND actor_principal_id = $1`, who.String()) > 0
			})
		}
	})

	t.Run("TestLiveBoxFocusFullDocumentOnce", func(t *testing.T) {
		r := w.running(t, "focus-once")
		alex, _ := w.ready(t, w.params(w.owner))
		focus(t, alex, r.thread)
		snap := mustRunUpdated(t, alex, r.id)
		doc, raw := w.getRun(t, r.id)
		if !snap.snapshot() || snap.ev.Payload.BaseVersion != nil || snap.ev.Payload.Ops != nil || !reflect.DeepEqual(decodeAny(t, snap.ev.Payload.Snapshot), doc) {
			t.Fatalf("snapshot %s, getRun %s", snap.raw, raw)
		}
		noRunUpdated(t, alex, r.id, 3*lbCoalesce, "Alex after the snapshot")
		// The same set again names no new thread, so no second snapshot.
		focus(t, alex, r.thread)
		noRunUpdated(t, alex, r.id, 3*lbCoalesce, "Alex after the same focus")
		w.patchOK(t, r.agent, r.id, snap.ev.Payload.Version, op("replace", "/status_line", "After the snapshot"))
		next := mustRunUpdated(t, alex, r.id)
		if next.snapshot() || next.ev.Payload.BaseVersion == nil || *next.ev.Payload.BaseVersion != snap.ev.Payload.Version {
			t.Fatalf("frame after the snapshot %s", next.raw)
		}
	})

	t.Run("TestLiveBoxRunUpdatedCoalesced", func(t *testing.T) {
		r := w.running(t, "coalesced")
		alex, _ := w.ready(t, w.params(w.owner))
		focus(t, alex, r.thread)
		snap := mustRunUpdated(t, alex, r.id)
		base := snap.ev.Payload.Version
		// The burst goes over run.update without waiting for answers, so it lands in one window.
		step := ids.New().String()
		burst := [][]map[string]any{
			{op("replace", "/status_line", "Reading")},
			{op("replace", "/status_line", "Reading the logs"), op("add", "/metrics/tokens_in", 10)},
			{op("add", "/steps/"+step, map[string]any{"kind": "tool_call", "name": "read_file"})},
			{op("replace", "/steps/"+step+"/title", "Reading export.log"), op("replace", "/metrics/tokens_in", 20)},
		}
		for i, ops := range burst {
			runUpdate(t, r.conn, r.id, base+int64(i), "", ops...)
		}
		f := mustRunUpdated(t, alex, r.id)
		if f.snapshot() || f.ev.Payload.BaseVersion == nil || *f.ev.Payload.BaseVersion != base || f.ev.Payload.Version != base+int64(len(burst)) {
			t.Fatalf("coalesced frame %s, want base %d version %d", f.raw, base, base+int64(len(burst)))
		}
		noRunUpdated(t, alex, r.id, 3*lbCoalesce, "Alex after the coalesced frame")
		applied, err := applyOps(decodeAny(t, snap.ev.Payload.Snapshot), f.ev.Payload.Ops)
		if err != nil {
			t.Fatalf("applying %s: %v", f.raw, err)
		}
		doc, raw := w.getRun(t, r.id)
		// The snapshot was taken at base; getRun's version is the frame's version.
		if m, ok := applied.(map[string]any); ok {
			m["version"] = float64(f.ev.Payload.Version)
		}
		if !reflect.DeepEqual(applied, doc) {
			got, _ := json.Marshal(applied)
			t.Fatalf("snapshot with the ops %s, getRun %s", got, raw)
		}
	})

	t.Run("TestLiveBoxRunUpdatedOnlyToFocusedReaders", func(t *testing.T) {
		r := w.running(t, "readers")
		w.grant(w.sam.user, "user", "channel", r.channel)
		alex, _ := w.ready(t, w.params(w.owner))
		sam, _ := w.ready(t, w.params(w.sam))
		robin, _ := w.ready(t, w.params(w.robin))
		focus(t, alex, r.thread)
		focus(t, robin, r.thread)
		snap := mustRunUpdated(t, alex, r.id)
		v := snap.ev.Payload.Version
		for i := range 3 {
			v = w.patchOK(t, r.agent, r.id, v, op("replace", "/status_line", "Working "+strconv.Itoa(i)))
			// Each patch waits for its frame, so every window holds one patch.
			if f := mustRunUpdated(t, alex, r.id); f.snapshot() || f.ev.Payload.Version != v {
				t.Fatalf("frame of patch %d: %s", i, f.raw)
			}
		}
		noRunUpdated(t, sam, r.id, 2*lbCoalesce, "Sam, who does not focus the thread")
		noRunUpdated(t, robin, r.id, 2*lbCoalesce, "Robin, who cannot read the thread")
	})

	t.Run("TestLiveBoxStepSummarizationBounded", func(t *testing.T) {
		r := w.running(t, "summary")
		alex, _ := w.ready(t, w.params(w.owner))
		focus(t, alex, r.thread)
		snap := mustRunUpdated(t, alex, r.id)
		doc := decodeAny(t, snap.ev.Payload.Snapshot)
		v := snap.ev.Payload.Version
		steps := make([]string, 5)
		statuses := []string{"completed", "completed", "running", "completed", "completed"}
		var frames []lbFrame
		for i := range steps {
			steps[i] = ids.New().String()
			v = w.patchOK(t, r.agent, r.id, v, op("add", "/steps/"+steps[i], map[string]any{"kind": "tool_call", "name": "step_" + strconv.Itoa(i), "status": statuses[i]}))
			if n := w.count(`SELECT count(*) FROM run_steps WHERE run_id = $1`, r.id); n > lbMaxSteps {
				t.Fatalf("the run holds %d steps after patch %d, more than run.max_steps_persisted %d", n, i, lbMaxSteps)
			}
			f := mustRunUpdated(t, alex, r.id)
			if f.snapshot() || f.ev.Payload.Version != v {
				t.Fatalf("frame of patch %d: %s", i, f.raw)
			}
			var err error
			if doc, err = applyOps(doc, f.ev.Payload.Ops); err != nil {
				t.Fatalf("applying %s: %v", f.raw, err)
			}
			frames = append(frames, f)
		}
		var summary string
		if err := w.queryRow(context.Background(), `SELECT id::text FROM run_steps WHERE run_id = $1 AND name = 'steps_summarized'`, r.id).Scan(&summary); err != nil {
			t.Fatalf("no summary step: %v", err)
		}
		has := func(f lbFrame, kind, path string) bool {
			return slices.ContainsFunc(f.ev.Payload.Ops, func(o lbOp) bool { return (kind == "" || o.Op == kind) && strings.HasPrefix(o.Path, path) })
		}
		f4, f5 := frames[3], frames[4]
		if !has(f4, "remove", "/steps/"+steps[0]) || !has(f4, "remove", "/steps/"+steps[1]) || !has(f4, "add", "/steps/"+summary) {
			t.Fatalf("frame of the fourth step %s", f4.raw)
		}
		if !has(f5, "remove", "/steps/"+steps[3]) || !has(f5, "", "/steps/"+summary) {
			t.Fatalf("frame of the fifth step %s", f5.raw)
		}
		code, b := w.rest(w.base, w.owner, http.MethodGet, "/api/v1/runs/"+r.id+"/steps", nil)
		var page rest.RunStepPage
		if code != http.StatusOK || json.Unmarshal(b, &page) != nil {
			t.Fatalf("listRunSteps = %d %s", code, b)
		}
		var names []string
		for i, s := range page.Items {
			if i > 0 && s.Ordinal <= page.Items[i-1].Ordinal {
				t.Fatalf("steps out of ordinal order: %s", b)
			}
			names = append(names, s.Name)
		}
		if !slices.Equal(names, []string{"steps_summarized", "step_2", "step_4"}) || page.Items[0].Id.String() != summary {
			t.Fatalf("steps %v: %s", names, b)
		}
		var detail struct {
			Summarized struct {
				Count  int            `json:"count"`
				ByKind map[string]int `json:"by_kind"`
			} `json:"summarized"`
		}
		db, _ := json.Marshal(page.Items[0].Detail)
		if err := json.Unmarshal(db, &detail); err != nil || detail.Summarized.Count != 3 || detail.Summarized.ByKind["tool_call"] != 3 {
			t.Fatalf("summary detail %s", db)
		}
		got, raw := w.getRun(t, r.id)
		if m, ok := doc.(map[string]any); ok {
			m["version"] = float64(v)
		}
		if !reflect.DeepEqual(doc, got) {
			b, _ := json.Marshal(doc)
			t.Fatalf("snapshot with every frame %s, getRun %s", b, raw)
		}
	})

	t.Run("TestLiveBoxPatchRunSchemas", func(t *testing.T) {
		r := w.running(t, "schemas")
		alex, _ := w.ready(t, w.params(w.owner))
		focus(t, alex, r.thread)
		snap := mustRunUpdated(t, alex, r.id)
		v := snap.ev.Payload.Version
		ok := w.patch(t, r.agent, r.id, v, op("replace", "/status_line", "Checking the schemas"))
		var res rest.RunPatchResult
		if ok.code != http.StatusOK || wstest.Strict(ok.body, &res) != nil {
			t.Fatalf("RunPatchResult %d %s", ok.code, ok.body)
		}
		frame := mustRunUpdated(t, alex, r.id)
		for _, f := range []lbFrame{snap, frame} {
			var ev rest.RunUpdatedEvent
			if err := wstest.Strict(f.raw, &ev); err != nil || ev.Type != "run.updated" {
				t.Fatalf("RunUpdatedEvent %s: %v", f.raw, err)
			}
		}
		for _, resp := range []lbResp{
			w.patch(t, r.agent, r.id, res.Version, op("replace", "/state", "failed")),
			w.patch(t, r.agent, r.id, res.Version-1, op("replace", "/status_line", "Stale")),
		} {
			var p rest.Problem
			if err := wstest.Strict(resp.body, &p); err != nil || p.Status != resp.code {
				t.Fatalf("Problem %d %s: %v", resp.code, resp.body, err)
			}
		}
	})

	t.Run("TestLiveBoxPatchContentNeverLogged", func(t *testing.T) {
		r := w.running(t, "content")
		marker := "Kennwort" + strings.ReplaceAll(ids.New().String(), "-", "")
		v := w.version(r.id)
		v = w.patchOK(t, r.agent, r.id, v, op("replace", "/status_line", "Status "+marker))
		v = w.patchOK(t, r.agent, r.id, v, op("add", "/steps/"+ids.New().String(),
			map[string]any{"kind": "tool_call", "name": "read_file", "detail": map[string]any{"note": "Detail " + marker}}))
		var bodies [][]byte
		for _, o := range []map[string]any{
			op("replace", "/status_line", "Bidi \u202e"+marker),
			op("replace", "/model", "Line\n"+marker),
			op("replace", "/state", marker),
		} {
			resp := w.patch(t, r.agent, r.id, v, o)
			problemOf(t, resp, http.StatusBadRequest, "validation_failed")
			bodies = append(bodies, resp.body)
		}
		bodies = append(bodies, w.patch(t, r.agent, r.id, v-1, op("replace", "/status_line", marker)).body)
		runUpdate(t, r.conn, r.id, v, "content", op("replace", "/status_line", "Bidi \u202e"+marker))
		e := wantOpError(t, r.conn, "validation_failed", "content")
		for i, b := range bodies {
			p := map[string]any{}
			_ = json.Unmarshal(b, &p)
			delete(p, "current") // the conflict carries the document, which holds the accepted status_line
			got, _ := json.Marshal(p)
			if strings.Contains(string(got), marker) {
				t.Errorf("problem %d carries the patch content: %s", i, got)
			}
		}
		if strings.Contains(deref(e.Detail), marker) {
			t.Errorf("the error frame carries the patch content: %q", deref(e.Detail))
		}
		if strings.Contains(w.p.logs.String(), marker) {
			t.Error("a log line carries the patch content")
		}
	})

	t.Run("TestLiveBoxPatchRunRejectedAfterEnd", func(t *testing.T) {
		// The job runs.cancel_timeout runs in a worker process on the same database; the api
		// process of the world only schedules it at cancelRun + agent.cancel_grace (1s here).
		worker := start(t, wire.RoleWorker, w.env, wire.Deps{})
		waitReadyz(t, worker.waitInternal(t), http.StatusOK)
		r := w.running(t, "after-end")
		w.must(w.owner, http.MethodPost, "/api/v1/runs/"+r.id+"/cancel", map[string]any{})
		// The adapter does not answer the cancel; the job ends the run.
		eventually(t, 30*time.Second, "the end of the run by runs.cancel_timeout", func() bool {
			got := w.runOfID(r.id)
			return got.state == "cancelled"
		})
		if got := w.runOfID(r.id); got.reason != "cancel_timeout" {
			t.Fatalf("run %+v, want cancelled by cancel_timeout", got)
		}
		v := w.version(r.id)
		p := problemOf(t, w.patch(t, r.agent, r.id, v, op("replace", "/status_line", "Too late")), http.StatusConflict, "conflict")
		if p.Reason != "run_not_active" {
			t.Fatalf("reason %q, want run_not_active", p.Reason)
		}
		runUpdate(t, r.conn, r.id, v, "after-end", op("replace", "/status_line", "Too late"))
		if e := wantOpError(t, r.conn, "conflict", "after-end"); deref(e.Detail) != "run_not_active" {
			t.Fatalf("detail %q", deref(e.Detail))
		}
		if got := w.version(r.id); got != v {
			t.Fatalf("runs.version %d after the rejected patches, want %d", got, v)
		}
		if strings.Contains(worker.logs.String(), "Too late") {
			t.Error("the worker logged patch content")
		}
	})
}

// runOfID reads the state and state_reason of the run id; the zero row when it is unknown.
func (w *lbWorld) runOfID(id string) runRow {
	w.t.Helper()
	var r runRow
	_ = w.queryRow(context.Background(), `SELECT id::text, state, coalesce(state_reason, ''), version FROM runs WHERE id = $1`, id).
		Scan(&r.id, &r.state, &r.reason, &r.version)
	return r
}

// TestLiveBoxQueueFullGivesSnapshot: Alex's client stops reading while a small
// sync.outbound_queue_max (8) and large run.updated frames (a last_text_excerpt of 50000
// characters) fill first the socket buffers and then the outbound queue; the patches go on, the
// connection is not closed (no 4014: gateway.write_timeout is raised so the blocked writer does
// not end it, and Alex sends no heartbeat that would need room for a control frame), and after
// Alex reads again the frames form an unbroken chain that ends with a snapshot of the newest
// version instead of the dropped patches. The size of the kernel buffers is not under the test's
// control, so the stall is repeated with more patches until a snapshot replaced dropped frames.
func TestLiveBoxQueueFullGivesSnapshot(t *testing.T) {
	t.Parallel()
	const excerpt = 50000
	w := newLBWorld(t, "BOTHY_SYNC__OUTBOUND_QUEUE_MAX=8", "BOTHY_SYNC__CURSOR_ADVANCE_EVERY=1000", "BOTHY_GATEWAY__WRITE_TIMEOUT=5m",
		"BOTHY_SYNC__HEARTBEAT_INTERVAL=2m", "BOTHY_AGENT__PRESENCE_TIMEOUT=5m", "BOTHY_SYNC__RUN_PATCH_COALESCE_INTERVAL=20ms", "BOTHY_RUN__EXCERPT_MAX_CHARS=60000")
	r := w.running(t, "queue")
	alex, _ := w.ready(t, w.params(w.owner), wstest.WithoutHeartbeat())
	focus(t, alex, r.thread)
	snap := mustRunUpdated(t, alex, r.id)
	last := snap.ev.Payload.Version
	replaced := false
	for round := 1; round <= 4 && !replaced; round++ {
		alex.Pause()
		v := last
		for i := range 60 * round {
			text := strings.Repeat(string(rune('a'+(i+round)%26)), excerpt)
			v = w.patchOK(t, r.agent, r.id, v, op("replace", "/last_text_excerpt", text))
		}
		alex.Unpause()
		var f lbFrame
		for last < v {
			f = mustRunUpdated(t, alex, r.id)
			switch {
			case f.snapshot():
				replaced = true
			case f.ev.Payload.BaseVersion == nil || *f.ev.Payload.BaseVersion != last:
				t.Fatalf("a frame with a gap after version %d: base %v version %d", last, f.ev.Payload.BaseVersion, f.ev.Payload.Version)
			}
			last = f.ev.Payload.Version
		}
		// Once frames were dropped, the stall ends with the snapshot of the newest version.
		if replaced && (!f.snapshot() || f.ev.Payload.Version != v) {
			t.Fatalf("the last frame of the stall: snapshot %v version %d, want the snapshot of version %d", f.snapshot(), f.ev.Payload.Version, v)
		}
		if replaced {
			// Exactly one snapshot: nothing follows while no patch comes.
			noRunUpdated(t, alex, r.id, 250*time.Millisecond, "Alex after the snapshot of the stall")
		}
		t.Logf("round %d: %d patches, snapshot instead of dropped frames: %v", round, 60*round, replaced)
	}
	if !replaced {
		t.Fatal("the outbound queue never filled; the socket buffers took every frame")
	}
	// Still open and back on the chain: the next patch arrives as ops on the snapshot's version.
	v := w.patchOK(t, r.agent, r.id, last, op("replace", "/status_line", "Reading again"))
	f := mustRunUpdated(t, alex, r.id)
	if f.snapshot() || f.ev.Payload.BaseVersion == nil || *f.ev.Payload.BaseVersion != last || f.ev.Payload.Version != v {
		t.Fatalf("frame after the stall %s", f.raw)
	}
}

// TestLiveBoxStepDetailNumbersBounded checks against PostgreSQL that number literals in a step
// detail cannot inflate what reads return: 1e-16000 is a few bytes as sent but thousands of
// digits as a jsonb numeric prints it, so a detail of such literals is stored as the fallback
// {"truncated": true}, a literal beyond the numeric range answers 200 instead of 500, and an
// ordinary fraction is stored in its plain decimal form.
func TestLiveBoxStepDetailNumbersBounded(t *testing.T) {
	t.Parallel()
	w := newLBWorld(t)
	r := w.running(t, "detail-numbers")
	many := make([]json.Number, 1800)
	for i := range many {
		many[i] = "1e-16000"
	}
	poisoned, overflow, plain := ids.New().String(), ids.New().String(), ids.New().String()
	w.patchOK(t, r.agent, r.id, w.version(r.id),
		op("add", "/steps/"+poisoned, map[string]any{"kind": "tool_call", "name": "read", "status": "completed", "detail": map[string]any{"n": many}}),
		op("add", "/steps/"+overflow, map[string]any{"kind": "tool_call", "name": "read", "status": "completed", "detail": map[string]any{"n": json.Number("1e-20000")}}),
		op("add", "/steps/"+plain, map[string]any{"kind": "tool_call", "name": "read", "status": "completed", "detail": map[string]any{"n": json.Number("1.5e-3")}}))
	for id, want := range map[string]string{poisoned: `{"truncated": true}`, overflow: `{"truncated": true}`, plain: `{"n": 0.0015}`} {
		var got string
		if err := w.queryRow(context.Background(), `SELECT detail::text FROM run_steps WHERE id = $1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("step %s detail %.120s, want %s", id, got, want)
		}
	}
}
