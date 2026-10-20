// SPDX-License-Identifier: Apache-2.0

// Package terminalsessions holds the cross-module integration tests of the terminal sessions
// (M2-44): the full server of test/harness (real modules through internal/wire, PostgreSQL 18
// through pgtest, HTTP and WebSockets on 127.0.0.1) with the Fake-Host of test/terminal/fakehost
// and its fake timers, so no test sleeps for terminal.open_timeout.
package terminalsessions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// waitLong bounds every wait of a test.
const waitLong = 20 * time.Second

// cek is a well-formed client_ephemeral_key_hash (43 base64url characters of 32 bytes).
var cek = base64.RawURLEncoding.EncodeToString(make([]byte, 32))

// world is the world of the brief: Robin (the first owner of the harness) with the assignment
// bothy-s-robin; host R1 active, assigned to Alex, organization, profiles shell and claude,
// reported session users bothy-s-alex (Project A, Project B), bothy-s-robin (Project C) and
// bothy-s-sam (none); host R2 active without assignee; Alex with a grant terminal on R1 and the
// devices D1 and D2; Sam with a grant terminal on R1 and an instance grant with trigger and every
// capability; the agent Lotse; the Fake-Host H1 holds R1's keys and is connected with its report.
type world struct {
	t                *testing.T
	s                *harness.Server
	timers           *fakehost.Timers
	robin, alex, sam *harness.Client
	alex2            *harness.Client
	lotse            *harness.Client
	r1, r2           ids.ID
	h1Key            hosttest.Key
	h1               *fakehost.Host
	// alexGrant is the id of Alex's grant terminal on R1.
	alexGrant string
	// beforeOpenCommit, when set, runs inside the unit of work of the next opening before its
	// commit (TestOpen_RevocationRaceRecheck); it is cleared when it runs.
	beforeOpenCommit atomic.Pointer[func()]
}

// newWorld builds the world with the configuration overrides cfg.
func newWorld(t *testing.T, cfg map[string]string) *world {
	t.Helper()
	return newWorldLogs(t, cfg, nil)
}

// worldSlots bounds the servers that run at once: each has its own database and connection pool
// on the shared pgtest server, whose connection slots a test per core would exhaust.
var worldSlots = make(chan struct{}, 3)

// newWorldLogs is newWorld with the log lines of the server also written to logs.
func newWorldLogs(t *testing.T, cfg map[string]string, logs io.Writer) *world {
	t.Helper()
	worldSlots <- struct{}{}
	// Registered first, so it runs after the cleanups of the server.
	t.Cleanup(func() { <-worldSlots })
	conf := harness.RaisedRateLimits()
	// Denials are aggregated per window; a short one writes them while the test waits.
	conf["audit.deny_aggregate_window"] = "50ms"
	for k, v := range cfg {
		conf[k] = v
	}
	w := &world{t: t, timers: &fakehost.Timers{}}
	w.s = harness.Start(t, harness.Options{Config: conf, TerminalTimers: w.timers, EgressDenyAll: true, LogWriter: logs,
		TerminalBeforeOpenCommit: func() {
			if f := w.beforeOpenCommit.Swap(nil); f != nil {
				(*f)()
			}
		}})
	w.robin = w.s.Owner
	w.alex = w.s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex"})
	w.sam = w.s.NewUser(t, harness.UserSpec{Handle: "sam", DisplayName: "Sam"})
	var err error
	if w.alex2, err = w.s.UserSession(t.Context(), w.alex.PrincipalID); err != nil {
		t.Fatal(err)
	}
	w.lotse = w.s.NewAgent(t, harness.AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	w.h1Key, w.r1 = w.pairedHost("Build server", w.alex.PrincipalID, map[string]string{
		w.alex.PrincipalID: "bothy-s-alex", w.robin.PrincipalID: "bothy-s-robin", w.sam.PrincipalID: "bothy-s-sam"})
	_, w.r2 = w.pairedHost("Lab machine", "", map[string]string{w.sam.PrincipalID: "bothy-s-sam"})
	w.alexGrant = w.grant(w.alex.PrincipalID, "host", w.r1.String(), "", "terminal")
	w.grant(w.sam.PrincipalID, "host", w.r1.String(), "", "terminal")
	w.grant(w.sam.PrincipalID, "instance", "", "trigger", "approve_actions", "upload", "export", "manage")
	w.h1 = w.connect(w.r1, w.h1Key, false)
	return w
}

// reportedUsers is the report of the session users of R1.
func reportedUsers(alexDirs ...string) []any {
	if alexDirs == nil {
		alexDirs = []string{"Project A", "Project B"}
	}
	return []any{
		map[string]any{"os_user": "bothy-s-alex", "workdirs": alexDirs},
		map[string]any{"os_user": "bothy-s-robin", "workdirs": []string{"Project C"}},
		map[string]any{"os_user": "bothy-s-sam", "workdirs": []string{}},
	}
}

// profiles are the profiles of R1.
func profiles() []any {
	return []any{
		map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"},
		map[string]any{"name": "claude", "display_name": "Claude Code", "kind": "claude"},
	}
}

// report is a report of R1 that passes, with the session users users and local confirmation.
func report(users []any, confirm bool) map[string]any {
	var doctor []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": c, "ok": true})
	}
	return map[string]any{"profiles": profiles(), "session_users": users, "doctor": doctor, "is_server_host": false,
		"require_local_confirm": confirm, "os_version": "Debian 13"}
}

// connect connects the Fake-Host of host with its report.
func (w *world) connect(host ids.ID, key hosttest.Key, confirm bool, live ...ids.ID) *fakehost.Host {
	return fakehost.Connect(w.t, w.s.PublicURL, host, key, report(reportedUsers(), confirm), live...)
}

// must sends r as c and fails unless the status is want.
func (w *world) must(c *harness.Client, method, path string, body any, want int) harness.Response {
	w.t.Helper()
	return harness.MustDo(w.t, c, harness.Request{Method: method, Path: path, Body: body}, want)
}

// pairedHost creates, pairs and confirms a host named name with the holder (empty for none) and
// the assignments users (person to session user); it returns the login key and the host id.
func (w *world) pairedHost(name, holder string, users map[string]string) (hosttest.Key, ids.ID) {
	w.t.Helper()
	body := map[string]any{"name": name, "reason": "terminal test"}
	if holder != "" {
		body["assigned_user_id"] = holder
	}
	var created struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	w.must(w.robin, http.MethodPost, "/hosts", body, http.StatusCreated).JSON(w.t, &created)
	key, noise := fakehost.NewKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.JWK(), &jwk); err != nil {
		w.t.Fatal(err)
	}
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	w.must(w.robin, http.MethodPost, "/hosts/pair", map[string]any{"pairing_code": created.PairingCode, "noise_public_key": fakehost.B64(noise),
		"auth_public_jwk": jwk, "platform": "linux", "host_version": "1.0.0", "profiles": profiles(), "session_users": reportedUsers()},
		http.StatusCreated).JSON(w.t, &paired)
	var su []any
	for u, os := range users {
		su = append(su, map[string]any{"user_id": u, "os_user": os})
	}
	prefix := strings.ReplaceAll(paired.KeyFingerprint, "-", "")[:8]
	get := w.must(w.robin, http.MethodGet, "/hosts/"+created.HostID, nil, http.StatusOK)
	resp, err := harness.DoWithStepUp(w.t.Context(), w.robin, harness.Request{Method: http.MethodPost, Path: "/hosts/" + created.HostID + "/confirm",
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}},
		Body: map[string]any{"fingerprint_prefix": prefix, "llm_credentials": "organization", "declared_server_host": false,
			"session_users": su, "reason": "terminal test"}})
	if err != nil || resp.Status != http.StatusOK {
		w.t.Fatalf("confirmHost: %v %d %s", err, resp.Status, resp.Body)
	}
	return key, ids.MustParse(created.HostID)
}

// grant creates a grant of subject on the target with level (empty for a host) and caps and
// returns its id.
func (w *world) grant(subject, targetKind, targetID, level string, caps ...string) string {
	w.t.Helper()
	body := map[string]any{"subject_kind": "user", "subject_id": subject, "target_kind": targetKind, "capabilities": caps, "reason": "terminal test"}
	if targetID != "" {
		body["target_id"] = targetID
	}
	if level != "" {
		body["level"] = level
	}
	var g struct {
		ID string `json:"id"`
	}
	w.must(w.robin, http.MethodPost, "/grants", body, http.StatusCreated).JSON(w.t, &g)
	return g.ID
}

// ctx is a context bounded by waitLong.
func (w *world) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), waitLong)
	w.t.Cleanup(cancel)
	return ctx
}

// grantBody is the decoded TerminalSessionGrant.
type grantBody struct {
	Session struct {
		ID, State, DeviceID string
		Version             int64
	}
	SessionToken          string `json:"session_token"`
	SessionTokenExpiresAt string `json:"session_token_expires_at"`
	StreamPath            string `json:"stream_path"`
	NoisePublicKey        string `json:"noise_public_key"`
	KeyFingerprint        string `json:"key_fingerprint"`
	KeyGeneration         int    `json:"key_generation"`
	NoiseProtocol         string `json:"noise_protocol"`
	PrologueVersion       string `json:"prologue_version"`
}

// openBody is the body of openTerminalSession for profile with the optional index.
func openBody(profile string, index ...int) map[string]any {
	b := map[string]any{"profile": profile, "cols": 120, "rows": 40, "client_ephemeral_key_hash": cek}
	if len(index) > 0 {
		b["workdir_index"] = index[0]
	}
	return b
}

// open sends openTerminalSession as c on host with a fresh step-up first and returns the response.
func (w *world) open(c *harness.Client, host ids.ID, body map[string]any) harness.Response {
	w.t.Helper()
	if err := c.StepUp(w.t.Context()); err != nil {
		w.t.Fatal(err)
	}
	return w.openRaw(c, host, body)
}

// openRaw sends openTerminalSession without a step-up.
func (w *world) openRaw(c *harness.Client, host ids.ID, body map[string]any) harness.Response {
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/hosts/" + host.String() + "/terminal/sessions", Body: body})
}

// opened opens a session and decodes the grant; it fails unless the answer is 201.
func (w *world) opened(c *harness.Client, host ids.ID, body map[string]any) grantBody {
	w.t.Helper()
	r := w.open(c, host, body)
	if r.Status != http.StatusCreated {
		w.t.Fatalf("openTerminalSession: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(w.t, &g)
	return g
}

// attach sends attachTerminalSession as c, with a fresh step-up when stepUp.
func (w *world) attach(c *harness.Client, session string, stepUp bool) harness.Response {
	w.t.Helper()
	if stepUp {
		if err := c.StepUp(w.t.Context()); err != nil {
			w.t.Fatal(err)
		}
	}
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/terminal/sessions/" + session + "/attach",
		Body: map[string]any{"cols": 120, "rows": 40, "client_ephemeral_key_hash": cek}})
}

// problem returns code and reason of a problem body.
func problem(r harness.Response) (code, reason string) {
	var p struct {
		Code, Reason string
	}
	_ = json.Unmarshal(r.Body, &p)
	return p.Code, p.Reason
}

// device opens the device stream of session as c (correct identify unless dp says otherwise).
func (w *world) device(c *harness.Client, session string, dp fakehost.DeviceParams) *fakehost.Device {
	w.t.Helper()
	if dp.DeviceID == "" {
		dp.DeviceID = c.DeviceID
	}
	d, status, err := fakehost.DialDevice(w.t, w.ctx(), w.s.PublicURL, w.s.PublicURL, ids.MustParse(session), c, dp)
	if err != nil {
		w.t.Fatalf("device stream: %d %v", status, err)
	}
	return d
}

// text returns a single text value of query on the database.
func (w *world) text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.s.AdminDB().QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// count returns an integer result of query.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.s.AdminDB().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// exec runs stmt on the database (fixtures outside the use cases: times).
func (w *world) exec(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.s.AdminDB().Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// state returns state and close_reason of a session ("closed:open_failed:open_timeout" style).
func (w *world) state(session string) string {
	return w.text(`SELECT state || coalesce(':' || close_reason, '') || coalesce(':' || close_detail, '') FROM terminal_sessions WHERE id = $1`, session)
}

// eventually waits until cond holds or waitLong passes.
func (w *world) eventually(what string, cond func() bool) {
	w.t.Helper()
	deadline := time.Now().Add(waitLong)
	for !cond() {
		if time.Now().After(deadline) {
			w.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitState waits until the session has the state string want.
func (w *world) waitState(session, want string) {
	w.t.Helper()
	w.eventually(fmt.Sprintf("session %s %s (now %s)", session, want, w.state(session)), func() bool { return w.state(session) == want })
}

// handshake runs message 1 and 2 and the first transport frame between d and the host side, which
// makes an Accept host send session.started.
func (w *world) handshake(d *fakehost.Device, token string) {
	w.t.Helper()
	ctx := w.ctx()
	if err := d.Send(ctx, []byte(token)); err != nil {
		w.t.Fatal(err)
	}
	if m, err := d.Recv(ctx); err != nil || string(m) != "message-2" {
		w.t.Fatalf("message 2: %q %v", m, err)
	}
	if err := d.Send(ctx, []byte("start")); err != nil {
		w.t.Fatal(err)
	}
	if m, err := d.Recv(ctx); err != nil || string(m) != "start" {
		w.t.Fatalf("echo of start: %q %v", m, err)
	}
}

// audits returns the details of the allow rows of action, oldest first.
func (w *world) audits(action string) []map[string]any {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(context.Background(), `SELECT details::text FROM audit_events WHERE action = $1 AND decision = 'allow' ORDER BY seq`,
		action)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			w.t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(raw), &m)
		out = append(out, m)
	}
	return out
}

// waitDenial waits until a deny entry of action with reason exists.
func (w *world) waitDenial(action, reason string) {
	w.t.Helper()
	w.eventually("deny entry "+action+" "+reason, func() bool {
		return w.count(`SELECT count(*) FROM audit_events WHERE action = $1 AND decision = 'deny' AND deny_reason = $2`, action, reason) > 0
	})
}

// tokenClaims decodes the payload and header of a compact JWS without checking it.
func tokenClaims(t *testing.T, tok string) (header, claims map[string]any, order []string) {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token with %d parts", len(parts))
	}
	h, _ := base64.RawURLEncoding.DecodeString(parts[0])
	p, _ := base64.RawURLEncoding.DecodeString(parts[1])
	_ = json.Unmarshal(h, &header)
	_ = json.Unmarshal(p, &claims)
	dec := json.NewDecoder(strings.NewReader(string(p)))
	_, _ = dec.Token()
	for dec.More() {
		k, _ := dec.Token()
		order = append(order, k.(string))
		var v json.RawMessage
		_ = dec.Decode(&v)
	}
	return header, claims, order
}
