// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otelapi "go.opentelemetry.io/otel"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/canary"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// cfTelemetryConfig is the telemetry of Design note 13: every span sampled, logs at debug to
// stdout and OTLP.
func cfTelemetryConfig() map[string]string {
	return map[string]string{
		"telemetry.traces.sampler_ratio": "1.0",
		"telemetry.logs.exporter":        "both",
		"telemetry.logs.level":           "debug",
	}
}

// cfWorld is the world of world_test.go started with the in-process telemetry sinks ts and an
// extra log writer logs; world_test.go cannot set Options.Telemetry. otel.Setup installs global
// providers, so a test with a cfWorld must not run in parallel with any other harness start of this
// binary (test/harness TelemetrySinks): its callers never call t.Parallel.
func cfWorld(t *testing.T, cfg map[string]string, ts *harness.TelemetrySinks, logs io.Writer) *world {
	t.Helper()
	return cfWorldRole(t, cfg, ts, logs, "")
}

// cfWorldRole is cfWorld with the server running as role (wire.RoleAll when empty).
func cfWorldRole(t *testing.T, cfg map[string]string, ts *harness.TelemetrySinks, logs io.Writer, role wire.Role) *world {
	t.Helper()
	worldSlots <- struct{}{}
	t.Cleanup(func() { <-worldSlots })
	clk := clock.NewFake(time.Now().UTC().Truncate(time.Second))
	w := &world{t: t, clk: clk, timers: fakehost.NewClockTimers(clk), metrics: newSessionMetrics(), logs: &syncBuffer{}}
	var out io.Writer = w.logs
	if logs != nil {
		out = io.MultiWriter(w.logs, logs)
	}
	w.s = harness.Start(t, harness.Options{Config: testConfig(cfg), Clock: clk, TerminalTimers: w.timers, EgressDenyAll: true, Role: role,
		LogWriter: out, Telemetry: ts, TerminalSessionMetrics: w.metrics,
		JobModules: func(m *wire.JobModules) { w.svcMu.Lock(); w.jobs = m; w.svcMu.Unlock() }})
	w.robin = w.s.Owner
	w.alex = w.s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex"})
	w.sam = w.s.NewUser(t, harness.UserSpec{Handle: "sam", DisplayName: "Sam"})
	var err error
	if w.alex2, err = w.s.UserSession(t.Context(), w.alex.PrincipalID); err != nil {
		t.Fatal(err)
	}
	w.k1, w.r1 = w.pairedHost("r1", w.alex.PrincipalID, r1Users(), map[string]string{
		w.alex.PrincipalID: "term-alex", w.sam.PrincipalID: "term-sam", w.robin.PrincipalID: "term-robin"})
	w.k2, w.r2 = w.pairedHost("r2", w.sam.PrincipalID, r2Users(), map[string]string{
		w.sam.PrincipalID: "term-sam", w.robin.PrincipalID: "term-robin"})
	w.alexGrant = w.grant("user", w.alex.PrincipalID, w.r1.String(), "")
	w.engineering = w.group("Engineering", w.sam.PrincipalID, "")
	w.engGrant = w.grant("group", w.engineering, w.r2.String(), "")
	w.h1 = w.connect(w.r1, w.k1, r1Users())
	w.h2 = w.connect(w.r2, w.k2, r2Users())
	return w
}

// cfNoiseHost pairs and confirms a host named name whose Fake-Host speaks Noise (a copy of
// world.pairedHost with the real X25519 key of nk as noise_public_key), assigned to holder with
// the session users of assign, connects it and sets the script NoiseAccept with the output out
// (nil echoes). It returns the host id and its Fake-Host.
func cfNoiseHost(w *world, name, holder string, reported []any, assign map[string]string, out []byte) (ids.ID, *fakehost.Host) {
	w.t.Helper()
	nk := fakehost.NewNoiseKey()
	body := map[string]any{"name": name, "reason": "terminal test"}
	if holder != "" {
		body["assigned_user_id"] = holder
	}
	var created struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	w.must(w.robin, http.MethodPost, "/hosts", body, http.StatusCreated).JSON(w.t, &created)
	var jwk map[string]any
	if err := json.Unmarshal(nk.Login.JWK(), &jwk); err != nil {
		w.t.Fatal(err)
	}
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	w.must(w.robin, http.MethodPost, "/hosts/pair", map[string]any{"pairing_code": created.PairingCode,
		"noise_public_key": fakehost.B64(nk.Public[:]), "auth_public_jwk": jwk, "platform": "linux", "host_version": "1.0.0",
		"profiles": profiles(), "session_users": reported}, http.StatusCreated).JSON(w.t, &paired)
	people := make([]string, 0, len(assign))
	for u := range assign {
		people = append(people, u)
	}
	sort.Strings(people)
	var su []any
	for _, u := range people {
		su = append(su, map[string]any{"user_id": u, "os_user": assign[u]})
	}
	prefix := strings.ReplaceAll(paired.KeyFingerprint, "-", "")[:8]
	get := w.must(w.robin, http.MethodGet, "/hosts/"+created.HostID, nil, http.StatusOK)
	resp, err := harness.DoWithStepUp(w.t.Context(), w.robin, harness.Request{Method: http.MethodPost,
		Path:   "/hosts/" + created.HostID + "/confirm",
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}},
		Body: map[string]any{"fingerprint_prefix": prefix, "llm_credentials": "organization", "declared_server_host": false,
			"session_users": su, "reason": "terminal test"}})
	if err != nil || resp.Status != http.StatusOK {
		w.t.Fatalf("confirmHost: %v %d %s", err, resp.Status, resp.Body)
	}
	host := ids.MustParse(created.HostID)
	h := w.connect(host, nk.Login, reported)
	h.UseNoise(nk)
	h.Default(fakehost.Script{Kind: fakehost.NoiseAccept, Output: out})
	return host, h
}

// cfSession is a session whose device side finished the Noise handshake.
type cfSession struct {
	g      grantBody
	id     ids.ID
	dev    *fakehost.Device
	stream *fakehost.NoiseStream
}

// cfOpen opens a shell of c on host with the hash of a fresh ephemeral key as
// client_ephemeral_key_hash, connects the device, runs the Noise handshake with the session token
// as the payload of message 1 and sends start; the session is active when it returns.
func cfOpen(w *world, c *harness.Client, host ids.ID) cfSession {
	w.t.Helper()
	eph := fakehost.NewEphemeral()
	w.syncClock()
	if err := c.StepUp(w.t.Context()); err != nil {
		w.t.Fatal(err)
	}
	r := c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/hosts/" + host.String() + "/terminal/sessions",
		Body: map[string]any{"profile": "shell", "cols": 120, "rows": 40, "client_ephemeral_key_hash": eph.Hash()}})
	if r.Status != http.StatusCreated {
		w.t.Fatalf("openTerminalSession: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(w.t, &g)
	static, err := base64.RawURLEncoding.DecodeString(g.NoisePublicKey)
	if err != nil || len(static) != 32 {
		w.t.Fatalf("noise_public_key of the grant: %d bytes, %v", len(static), err)
	}
	dev := w.device(c, g.Session.ID)
	p := fakehost.NoiseParams{HostID: host, Ephemeral: eph, Token: g.SessionToken}
	copy(p.HostStatic[:], static)
	ns, err := dev.Handshake(w.ctx(), p)
	if err != nil {
		w.t.Fatalf("noise handshake: %v", err)
	}
	if err := ns.Send(w.ctx(), fakehost.StartFrame(120, 40)); err != nil {
		w.t.Fatal(err)
	}
	w.waitState(g.Session.ID, "active")
	return cfSession{g: g, id: ids.MustParse(g.Session.ID), dev: dev, stream: ns}
}

// cfRecvData returns the payload of the next data frame the device decrypts.
func cfRecvData(w *world, s cfSession) []byte {
	w.t.Helper()
	typ, payload, err := s.stream.RecvFrame(w.ctx())
	if err != nil || typ != fakehost.FrameData {
		w.t.Fatalf("frame from the host: type %d, %v", typ, err)
	}
	return payload
}

// cfWireRecords adds every binary WebSocket message of msgs (what passed the relay, as the fake saw
// it) as one record of the sink "relay".
func cfWireRecords(sinks *canary.Sinks, who string, msgs []fakehost.WireMessage) {
	for i, m := range msgs {
		dir := "received"
		if m.Sent {
			dir = "sent"
		}
		sinks.Add(canary.Record{Sink: "relay", Source: fmt.Sprintf("%s message %d %s", who, i, dir),
			Fields: []canary.Field{{Location: "/bytes", Text: string(m.Bytes)}}})
	}
}

// cfCollectDatabase adds every row of every table of the domain schema and of the River schema
// (every base table outside the catalogs), read as text through AdminDB, to the sink "database":
// what a backup would hold. event_log payloads and audit_events.details are among them; they are
// added once more as their own sinks with their JSON members as fields.
func cfCollectDatabase(t *testing.T, w *world, sinks *canary.Sinks) int {
	t.Helper()
	ctx := w.ctx()
	db := w.s.AdminDB()
	rows, err := db.Query(ctx, `SELECT table_schema, table_name FROM information_schema.tables
		WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema') ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	var tables [][2]string
	for rows.Next() {
		var s, n string
		if err := rows.Scan(&s, &n); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, [2]string{s, n})
	}
	rows.Close()
	schemas, river := map[string]bool{}, false
	for _, tb := range tables {
		schemas[tb[0]] = true
		river = river || tb[1] == "river_job"
	}
	if !schemas["public"] || !river {
		t.Fatalf("tables %d in the schemas %v; want the domain schema and the River schema (river_job)", len(tables), schemas)
	}
	t.Logf("database: %d tables in the schemas %v", len(tables), schemas)
	n := 0
	for _, tb := range tables {
		q := fmt.Sprintf(`SELECT row_to_json(x)::text FROM %q.%q x`, tb[0], tb[1])
		r, err := db.Query(ctx, q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		i := 0
		for r.Next() {
			var row string
			if err := r.Scan(&row); err != nil {
				t.Fatal(err)
			}
			i++
			sinks.AddJSON("database", fmt.Sprintf("%s.%s row %d", tb[0], tb[1], i), "", "", []byte(row))
		}
		r.Close()
		n += i
	}
	for _, q := range []struct{ sink, sql string }{
		{"event_log", `SELECT payload::text FROM event_log`},
		{canary.SinkAudit, `SELECT details::text FROM audit_events`},
	} {
		r, err := db.Query(ctx, q.sql)
		if err != nil {
			t.Fatal(err)
		}
		i := 0
		for r.Next() {
			var row *string
			if err := r.Scan(&row); err != nil {
				t.Fatal(err)
			}
			if row != nil {
				i++
				sinks.AddJSON(q.sink, fmt.Sprintf("%s %d", q.sink, i), "", "/details", []byte(*row))
			}
		}
		r.Close()
	}
	return n
}

// cfCollect gathers the telemetry (after a flush), the rest of the log stream and the audit rows
// into sinks.
func cfCollect(t *testing.T, w *world, ts *harness.TelemetrySinks, lw *canary.LineWriter, sinks *canary.Sinks) {
	t.Helper()
	ctx := w.ctx()
	if err := w.s.FlushTelemetry(ctx); err != nil {
		t.Fatalf("flush telemetry: %v", err)
	}
	lw.Flush()
	if err := canary.CollectTelemetry(ctx, sinks, ts); err != nil {
		t.Fatalf("collect telemetry: %v", err)
	}
	if _, err := canary.CollectAudit(ctx, sinks, w.s.AdminDB()); err != nil {
		t.Fatalf("collect audit: %v", err)
	}
}

// TestTMT06_NoContentAnywhere proves TM-T06 and the terminal part of SEC-T10 (24 sections 5 and
// 10.4, R-DM-36, threat T-TM-01; brief M2-46 Design note 13 and requirement 24): Alex opens a shell
// on a host whose Fake-Host speaks Noise_NK_25519_ChaChaPoly_SHA256 with the prologue of 24
// section 5.2; the device sends canary keystrokes (class content) in a data frame and a resize,
// the Fake-Host answers with output carrying a second canary, the device closes. No canary is in
// the raw binary messages both fakes sent and received (what passed the relay), in the server logs
// (stdout JSON and OTLP at debug), in spans, in metrics, in any row of any table read as text (the
// domain schema and the River schema: what a backup would hold), in event_log payloads or in
// audit_events.details. The session token (class secret) is in none of them either: it travels
// only in the answer of openTerminalSession and inside the encrypted message 1. Positive controls:
// both canaries arrive in clear after decryption, the token is the payload of message 1 at the
// host, and a canary logged on purpose and a plaintext frame in the relay sink are found.
func TestTMT06_NoContentAnywhere(t *testing.T) {
	ts := harness.NewTelemetrySinks()
	reg := canary.NewRegistry()
	sinks := canary.NewSinks()
	lw := canary.NewLineWriter(sinks, canary.SinkLog, "stdout")
	w := cfWorld(t, cfTelemetryConfig(), ts, lw)
	input := reg.New(canary.ClassContent, "", "terminal input")
	output := reg.New(canary.ClassContent, "", "terminal output")
	r3, h3 := cfNoiseHost(w, "r3", w.alex.PrincipalID, users("term-alex"), map[string]string{w.alex.PrincipalID: "term-alex"},
		[]byte("$ "+output+"\r\n"))
	w.grant("user", w.alex.PrincipalID, r3.String(), "")

	s := cfOpen(w, w.alex, r3)
	reg.Register(s.g.SessionToken, canary.ClassSecret, "", "terminal session token")
	ctx := w.ctx()
	if err := s.stream.Send(ctx, fakehost.DataFrame([]byte(input+"\r"))); err != nil {
		t.Fatal(err)
	}
	if got := cfRecvData(w, s); !strings.Contains(string(got), output) {
		t.Fatalf("the device decrypted %d bytes without the output canary", len(got))
	}
	if err := s.stream.Send(ctx, fakehost.ResizeFrame(100, 30)); err != nil {
		t.Fatal(err)
	}
	if err := s.stream.Send(ctx, fakehost.CloseFrame()); err != nil {
		t.Fatal(err)
	}
	w.waitState(s.id.String(), "closed:client_closed")
	w.closeCode("device stream", s.dev.CloseCode, 4100, "client_closed")

	// Positive controls: the canaries in clear behind the encryption, the token inside message 1.
	var sawInput, sawResize bool
	for _, f := range h3.Plain(s.id) {
		switch {
		case f[0] == fakehost.FrameData && strings.Contains(string(f[1:]), input):
			sawInput = true
		case f[0] == fakehost.FrameResize:
			sawResize = true
		}
	}
	if !sawInput || !sawResize {
		t.Fatalf("the Fake-Host decrypted input %v, resize %v; want both", sawInput, sawResize)
	}
	if tok := h3.Tokens(); len(tok) != 1 || tok[0] != s.g.SessionToken {
		t.Fatalf("the Fake-Host got %d tokens from message 1, want the session token", len(tok))
	}
	data := h3.Data(s.id)
	if data == nil {
		t.Fatal("no data connection at the Fake-Host")
	}
	hostWire, devWire := data.Wire(), s.dev.Wire()
	if len(hostWire) < 5 || len(devWire) < 5 {
		t.Fatalf("relay capture: host %d, device %d messages; want the handshake and the frames", len(hostWire), len(devWire))
	}
	cfWireRecords(sinks, "host", hostWire)
	cfWireRecords(sinks, "device", devWire)

	cfCollect(t, w, ts, lw, sinks)
	rows := cfCollectDatabase(t, w, sinks)
	if f := canary.Scan(reg, sinks, canary.AllowRules{}); len(f) > 0 {
		t.Errorf("%d findings:\n%s", len(f), canary.Report(f))
	}
	t.Logf("scanned: log %d, trace %d, metric %d, audit %d, database %d rows, event_log %d, relay %d",
		sinks.Count(canary.SinkLog), sinks.Count(canary.SinkTrace), sinks.Count(canary.SinkMetric), sinks.Count(canary.SinkAudit),
		rows, sinks.Count("event_log"), sinks.Count("relay"))
	for sink, min := range map[string]int{canary.SinkLog: 1, canary.SinkTrace: 1, canary.SinkMetric: 1, "event_log": 1} {
		if sinks.Count(sink) < min {
			t.Errorf("sink %s has %d records: it was not exercised", sink, sinks.Count(sink))
		}
	}

	// Detector meta test: a canary logged on purpose through the server's log pipeline, and the
	// clear input frame put into the relay sink, are found.
	probe := reg.New(canary.ClassContent, "", "detector probe")
	slog.New(otelslog.NewHandler("tmt06-meta", otelslog.WithLoggerProvider(otelapi.GetLoggerProvider()))).
		LogAttrs(ctx, slog.LevelInfo, "detector probe", slog.String("error.type", probe))
	meta := canary.NewSinks()
	if err := w.s.FlushTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	if err := canary.CollectTelemetry(ctx, meta, ts); err != nil {
		t.Fatal(err)
	}
	meta.Add(canary.Record{Sink: "relay", Source: "meta clear frame", Fields: []canary.Field{{Location: "/bytes",
		Text: string(fakehost.DataFrame([]byte(input)))}}})
	got := map[string]bool{}
	for _, f := range canary.Scan(reg, meta, canary.AllowRules{}) {
		got[f.Sink+"/"+f.Label] = true
	}
	if !got[canary.SinkLog+"/detector probe"] || !got["relay/terminal input"] {
		t.Fatalf("detector meta test: findings %v, want the probe in the log and the clear input in the relay sink", got)
	}
}

// TestTelemetry_NoSpanPerFrame proves requirement 24 of brief M2-46 (no span per frame; signals.md
// 4.3a): streaming 10 and then 100 data frames through the relay creates the same number of spans,
// and no log line or span attribute carries the bytes of a frame (the plaintexts and the
// ciphertexts the device sent and received are canaries scanned in the log and trace sinks).
//
// The span count is deterministic on a loaded machine: the server runs as role api (no River
// worker, so no job such as the RunOnStart jobs lands in a window), and only spans of traces that
// began with or after the session open count. The traces seen before it are the control
// connections of the Fake-Hosts, whose heartbeat every real second (fakehost.Connect) adds
// spans to whichever window it falls in; frames never travel over a control connection. The
// first window waits until such a heartbeat span arrived inside it, so every run exercises the
// exclusion (keeper: in CI the first window counted 5 to 16 such spans and the test failed).
func TestTelemetry_NoSpanPerFrame(t *testing.T) {
	ts := harness.NewTelemetrySinks()
	reg := canary.NewRegistry()
	sinks := canary.NewSinks()
	lw := canary.NewLineWriter(sinks, canary.SinkLog, "stdout")
	w := cfWorldRole(t, cfTelemetryConfig(), ts, lw, wire.RoleAPI)
	r3, _ := cfNoiseHost(w, "r3", w.alex.PrincipalID, users("term-alex"), map[string]string{w.alex.PrincipalID: "term-alex"}, nil)
	w.grant("user", w.alex.PrincipalID, r3.String(), "")
	// spans returns the number of exported spans of the traces in background and of the others.
	spans := func(background map[string]bool) (bg, other int) {
		t.Helper()
		if err := w.s.FlushTelemetry(w.ctx()); err != nil {
			t.Fatal(err)
		}
		for _, sp := range ts.Spans.GetSpans() {
			if background[sp.SpanContext.TraceID().String()] {
				bg++
			} else {
				other++
			}
		}
		return bg, other
	}
	background := map[string]bool{}
	if err := w.s.FlushTelemetry(w.ctx()); err != nil {
		t.Fatal(err)
	}
	for _, sp := range ts.Spans.GetSpans() {
		background[sp.SpanContext.TraceID().String()] = true
	}
	s := cfOpen(w, w.alex, r3)
	stream := func(n int, waitBackground bool) int {
		t.Helper()
		bg0, before := spans(background)
		for range n {
			payload := reg.New(canary.ClassContent, "", "frame payload")
			if err := s.stream.Send(w.ctx(), fakehost.DataFrame([]byte(payload))); err != nil {
				t.Fatal(err)
			}
			if got := cfRecvData(w, s); string(got) != payload {
				t.Fatalf("echo %q, want the payload", got)
			}
		}
		if waitBackground {
			testkit.Until(t, "a heartbeat span of a control connection in the window", func() bool {
				bg, _ := spans(background)
				return bg > bg0
			})
		}
		_, after := spans(background)
		return after - before
	}
	ten, hundred := stream(10, true), stream(100, false)
	t.Logf("spans: 10 frames %d, 100 frames %d", ten, hundred)
	if ten != hundred {
		t.Errorf("10 frames created %d spans, 100 frames %d; want the same number", ten, hundred)
	}
	for _, sp := range ts.Spans.GetSpans() {
		if strings.HasPrefix(sp.Name, "job ") {
			t.Fatalf("span %q: the role api server ran a job", sp.Name)
		}
	}
	for _, m := range s.dev.Wire() {
		reg.Register(string(m.Bytes), canary.ClassSecret, "", "frame bytes")
	}
	cfCollect(t, w, ts, lw, sinks)
	var f []canary.Finding
	for _, x := range canary.Scan(reg, sinks, canary.AllowRules{}) {
		if x.Sink == canary.SinkLog || x.Sink == canary.SinkTrace {
			f = append(f, x)
		}
	}
	if len(f) > 0 {
		t.Errorf("%d frame bytes in logs or spans:\n%s", len(f), canary.Report(f))
	}
}

// cfRepoRoot returns the repository root: the first directory above the working directory that
// holds docs/spec.
func cfRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no repository root with docs/spec above the working directory")
		}
		dir = parent
	}
}

// cfDocColumns reads the table of #### `table` in the data model (03 section 4.14) and returns its
// rows as "name type nullable" (nullable yes or no; the types as information_schema names them).
func cfDocColumns(t *testing.T, path, table string) []string {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	types := map[string]string{"uuid": "uuid", "text": "text", "integer": "integer", "bigint": "bigint",
		"timestamptz": "timestamp with time zone", "boolean": "boolean", "jsonb": "jsonb", "bytea": "bytea"}
	nullable := map[string]string{"no": "no", "yes": "yes"}
	var out []string
	in, table2 := false, false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "#### "):
			in = strings.TrimSpace(strings.TrimPrefix(line, "#### ")) == "`"+table+"`"
			continue
		case !in:
			continue
		case strings.HasPrefix(line, "| Field "):
			table2 = true
			continue
		case !table2 || strings.HasPrefix(line, "|---"):
			continue
		case !strings.HasPrefix(line, "| `"):
			if len(out) > 0 {
				return out
			}
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 5 {
			t.Fatalf("row of %s: %q", table, line)
		}
		name := strings.Trim(strings.TrimSpace(cells[1]), "`")
		typ, ok := types[strings.Trim(strings.TrimSpace(cells[2]), "`")]
		null, ok2 := nullable[strings.TrimSpace(cells[3])]
		if !ok || !ok2 {
			t.Fatalf("row of %s with an unknown type or null column: %q", table, line)
		}
		out = append(out, name+" "+typ+" "+null)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestTerminalSessionsHoldOnlyMetadata proves requirement 24 and Design note 14 of brief M2-46
// (R-DM-36, 24 section 10.4): the columns of terminal_sessions in the migrated database (name,
// type and nullability, in order) equal the list of docs/spec/03-data-model.md section 4.14, so no
// column can hold terminal content.
func TestTerminalSessionsHoldOnlyMetadata(t *testing.T) {
	t.Parallel()
	want := cfDocColumns(t, filepath.Join(cfRepoRoot(t), "docs", "spec", "03-data-model.md"), "terminal_sessions")
	if len(want) == 0 {
		t.Fatal("no columns of terminal_sessions in 03")
	}
	worldSlots <- struct{}{}
	t.Cleanup(func() { <-worldSlots })
	s := harness.Start(t, harness.Options{Config: harness.RaisedRateLimits(), EgressDenyAll: true})
	ctx, cancel := context.WithTimeout(context.Background(), waitLong)
	defer cancel()
	rows, err := s.AdminDB().Query(ctx, `SELECT column_name, data_type, lower(is_nullable) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'terminal_sessions' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var n, typ, null string
		if err := rows.Scan(&n, &typ, &null); err != nil {
			t.Fatal(err)
		}
		got = append(got, n+" "+typ+" "+null)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("columns of terminal_sessions:\n%s\nwant (03 section 4.14):\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
