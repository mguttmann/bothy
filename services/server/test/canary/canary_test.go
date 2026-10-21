// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// collectWait bounds the flush and collection at the end of a run.
const collectWait = 2 * time.Minute

// serverLog returns a logger through the OTLP log pipeline of the running server (the global
// logger provider otel.Setup installed): its records pass the catalogue filter and the bounded
// queue of the server like every record of the server's own logger.
func serverLog() *slog.Logger {
	return slog.New(otelslog.NewHandler("canary-meta", otelslog.WithLoggerProvider(otelapi.GetLoggerProvider())))
}

// findingsIn returns the findings of sink.
func findingsIn(f []Finding, sink Sink) []Finding {
	var out []Finding
	for _, x := range f {
		if x.Sink == sink {
			out = append(out, x)
		}
	}
	return out
}

// TestSEC_T10_DetectorFindsInjectedLog: a canary logged through the server's log pipeline as an
// attribute is exactly one finding in the log sink, at that attribute key.
func TestSEC_T10_DetectorFindsInjectedLog(t *testing.T) {
	r := StartRun(t, RunOptions{})
	v := r.Reg.New(ClassContent, "", "injected log attribute")
	serverLog().LogAttrs(t.Context(), slog.LevelInfo, "canary meta", slog.String("error.type", v))
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	got := findingsIn(Scan(r.Reg, r.Sinks, AllowRules{}), SinkLog)
	if len(got) != 1 || !strings.Contains(got[0].Location, "error.type") || got[0].Label != "injected log attribute" {
		t.Fatalf("log findings:\n%s\nwant exactly one at error.type", Report(got))
	}
}

// TestSEC_T10_DetectorFindsInjectedSpanAttribute: a canary set as a span attribute through the
// server's tracer provider is a finding in the trace sink at that key.
func TestSEC_T10_DetectorFindsInjectedSpanAttribute(t *testing.T) {
	r := StartRun(t, RunOptions{})
	v := r.Reg.New(ClassSecret, "", "injected span attribute")
	_, span := otelapi.Tracer("canary-meta").Start(t.Context(), "canary meta")
	span.SetAttributes(attribute.String("bothy.thread.id", v))
	span.End()
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	got := findingsIn(Scan(r.Reg, r.Sinks, AllowRules{}), SinkTrace)
	if len(got) != 1 || !strings.Contains(got[0].Location, "bothy.thread.id") || got[0].Class != string(ClassSecret) {
		t.Fatalf("trace findings:\n%s\nwant exactly one at bothy.thread.id", Report(got))
	}
}

// TestSEC_T10_DetectorFindsClearPrincipalIDInLog: a human principal ID in clear in a log
// attribute is an identity finding in the log sink.
func TestSEC_T10_DetectorFindsClearPrincipalIDInLog(t *testing.T) {
	r := StartRun(t, RunOptions{})
	alex := r.Watch(r.Server.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"}), "alex")
	serverLog().LogAttrs(t.Context(), slog.LevelInfo, "canary meta", slog.String("bothy.thread.id", alex.PrincipalID))
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	r.RegisterIdentities(ctx)
	var hits []Finding
	for _, f := range findingsIn(Scan(r.Reg, r.Sinks, AllowRules{}), SinkLog) {
		if f.Class == string(ClassIdentity) && strings.Contains(f.Location, "bothy.thread.id") {
			hits = append(hits, f)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("identity findings at bothy.thread.id:\n%s\nwant exactly one", Report(hits))
	}
}

// TestSEC_T10_CanaryAcrossAllOutputs: the full scenario, then Scan; zero findings. It also holds
// what makes the zero meaningful: every class has values, every file of the secrets directory
// and of the key store is registered, the network canaries reached the audit IP path, the
// message delete left a redacted event, and the permitted places did see their values.
func TestSEC_T10_CanaryAcrossAllOutputs(t *testing.T) {
	res := scenario(t)
	for _, c := range []Class{ClassContent, ClassAdminReason, ClassSecret, ClassNetwork, ClassIdentity, ClassManageOnly} {
		if res.run.Reg.Count(c) == 0 {
			t.Errorf("no canary of class %s", c)
		}
	}
	if res.secretFiles == 0 || res.keyFiles == 0 {
		t.Errorf("secret files %d, key files %d, want both above 0", res.secretFiles, res.keyFiles)
	}
	if res.ipHMACs < 2 {
		t.Errorf("the owner's audit rows carry %d distinct ip_hmac values, want the loopback and the network canary", res.ipHMACs)
	}
	if res.alexIPRows == 0 {
		t.Error("no audit row of Alex (IPv4 canary) with ip_hmac and ip_key_id")
	}
	refresh := 0
	for _, c := range res.run.Reg.Canaries() {
		if c.Label == "refresh token" {
			refresh++
		}
	}
	if refresh < 2 {
		t.Errorf("%d refresh tokens registered, want those of the harness clients and their rotations", refresh)
	}
	if !res.redacted {
		t.Error("the message delete left no redacted event")
	}
	seen := map[string]bool{}
	for _, m := range Matches(res.run.Reg, res.run.Sinks) {
		seen[m.Sink+"/"+m.Class] = true
	}
	// The 412 of putChannelBehavior echoes the stored canary prompt in current (M2-02).
	for _, want := range []string{SinkAudit + "/" + string(ClassAdminReason), SinkForeign + "/" + string(ClassContent), SinkError + "/" + string(ClassManageOnly)} {
		if !seen[want] {
			t.Errorf("no permitted match in %s: the sink was not exercised", want)
		}
	}
	if f := Scan(res.run.Reg, res.run.Sinks, AllowRules{Readable: res.sc.Readable}); len(f) > 0 {
		t.Errorf("%d findings:\n%s", len(f), Report(f))
	}
}

// TestSEC_T10_ScenarioCoversEveryEnabledOperation: every operation of the enabled list of
// api/operations.yaml was called, and every client gateway op M1 registers was sent.
func TestSEC_T10_ScenarioCoversEveryEnabledOperation(t *testing.T) {
	res := scenario(t)
	enabled, err := EnabledOperations()
	if err != nil {
		t.Fatal(err)
	}
	if len(Exempt) != 0 {
		t.Logf("exempt operations: %v", Exempt)
	}
	if missing := res.run.Cov.Missing(enabled, Exempt); len(missing) > 0 {
		t.Errorf("%d enabled operations the scenario never calls (add them to scenario.go):\n%s", len(missing), strings.Join(missing, "\n"))
	}
	var ops []string
	for op := range wire.GatewayOpClasses() {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	for _, op := range ops {
		if res.sc.GatewaySent[op] == 0 {
			t.Errorf("client gateway op %s never sent", op)
		}
	}
}

// TestDP_T09_NoClearPrincipalOrDeviceIDsInTelemetry: no principal or device ID of the run in logs,
// traces or metrics, and bothy.principal.pseudo_id on at least one request span.
func TestDP_T09_NoClearPrincipalOrDeviceIDsInTelemetry(t *testing.T) {
	res := scenario(t)
	var f []Finding
	for _, x := range Scan(res.run.Reg, res.run.Sinks, AllowRules{Readable: res.sc.Readable}) {
		if x.Class == string(ClassIdentity) {
			f = append(f, x)
		}
	}
	if len(f) > 0 {
		t.Errorf("%d clear IDs in telemetry:\n%s", len(f), Report(f))
	}
	pseudo := 0
	for _, r := range res.run.Sinks.Records() {
		if r.Sink != SinkTrace {
			continue
		}
		for _, fl := range r.Fields {
			if fl.Location == "/attributes/bothy.principal.pseudo_id" && len(fl.Text) == pseudoIDLen {
				pseudo++
			}
		}
	}
	if pseudo == 0 {
		t.Error("no span carries bothy.principal.pseudo_id")
	}
}

// pseudoIDLen is the length of a pseudonym (privacy.md "Pseudonyms" item 1: 32 hex characters).
const pseudoIDLen = 32

// scenarioResult is a finished scenario run: its collected sinks and the positive checks.
type scenarioResult struct {
	run *Run
	sc  *Scenario
	// secretFiles and keyFiles count the registered files of the secrets directory and the key
	// store; ipHMACs is the number of distinct ip_hmac values of the owner's audit rows,
	// alexIPRows the audit rows of Alex with ip_hmac and ip_key_id; redacted reports a redacted
	// event in the owner's event log.
	secretFiles, keyFiles, ipHMACs, alexIPRows int
	redacted                                   bool
}

var (
	scenarioOnce sync.Once
	scenarioRes  *scenarioResult
)

// scenario runs the scenario once per test binary (the first test that needs it carries its
// server and its step failures) and hands the collected result to every test; the result holds
// data only, so it outlives the server.
func scenario(t *testing.T) *scenarioResult {
	t.Helper()
	scenarioOnce.Do(func() { scenarioRes = runScenario(t) })
	if scenarioRes == nil {
		t.Fatal("the scenario did not finish (see the first test that ran it)")
	}
	return scenarioRes
}

// runScenario runs the whole scenario and collects every sink.
func runScenario(t *testing.T) *scenarioResult {
	t.Helper()
	idp := NewIDP(t, randomSecret())
	r := StartRun(t, IDPRunOptions(idp))
	ctx := t.Context()
	sc := NewScenario(ctx, r)
	sc.content()
	sc.files()
	sc.behavior()
	sc.runs()
	sc.commands()
	sc.components()
	sc.projectBinding()
	sc.people()
	sc.agents()
	sc.settings()
	sc.instance()
	sc.auth(idp)
	sc.apps()
	sc.incomingWebhooks()
	sc.hosts()
	sc.terminalSessions()
	sc.errorPaths()
	sc.gatewayOps()
	sc.FinishObservers()
	res := &scenarioResult{run: r, sc: sc}
	res.redacted = sc.RedactedEvent()
	cctx, cancel := context.WithTimeout(ctx, collectWait)
	defer cancel()
	audits := r.Collect(cctx)
	res.ipHMACs, res.alexIPRows = sc.AuditIPRows(cctx)
	var err error
	if res.secretFiles, err = r.RegisterSecretFiles(r.Server.SecretsDir); err != nil {
		t.Fatal(err)
	}
	if res.keyFiles, err = r.RegisterSecretFiles(r.Server.SigningKeyDir); err != nil {
		t.Fatal(err)
	}
	r.RegisterIdentities(cctx)
	t.Logf("canaries: content %d, admin_reason %d, secret %d, network %d, identity %d, manage_only %d; records: log %d, trace %d, metric %d, audit %d, error %d, foreign %d",
		r.Reg.Count(ClassContent), r.Reg.Count(ClassAdminReason), r.Reg.Count(ClassSecret), r.Reg.Count(ClassNetwork), r.Reg.Count(ClassIdentity),
		r.Reg.Count(ClassManageOnly), r.Sinks.Count(SinkLog), r.Sinks.Count(SinkTrace), r.Sinks.Count(SinkMetric), audits, r.Sinks.Count(SinkError), r.Sinks.Count(SinkForeign))
	return res
}

// TestSEC_T10_RateLimitedResponse: a server with the default rate limits answers canary-bearing
// logins (redeemEnrollmentCode with a canary code and device name) from one client address with
// 429 on rl.public_auth; no canary appears in any output, the 429 answers included. The login
// with an enrollment code is the cheap one of the class: a password login hashes with Argon2id,
// which takes minutes per burst under the race detector.
func TestSEC_T10_RateLimitedResponse(t *testing.T) {
	r := StartRun(t, RunOptions{Defaults: true})
	dev := r.Watch(r.Server.NewDevice(), "login burst")
	dev.ClientIP = r.IPv4
	limited := 0
	for i := 0; i < rateLimitAttempts && limited == 0; i++ {
		resp := dev.DoAnonymous(t.Context(), harness.Request{Method: http.MethodPost, Path: "/auth/enroll/redeem", Body: map[string]any{
			"enrollment_code": r.Reg.New(ClassSecret, "", "enrollment code (rate limit)"),
			"device":          dev.Registration(r.Reg.New(ClassContent, "", "device name (rate limit)")),
		}})
		if resp.Status == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited == 0 {
		t.Fatalf("no 429 after %d logins from one address", rateLimitAttempts)
	}
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	if _, err := r.RegisterSecretFiles(r.Server.SecretsDir); err != nil {
		t.Fatal(err)
	}
	r.RegisterIdentities(ctx)
	if f := Scan(r.Reg, r.Sinks, AllowRules{}); len(f) > 0 {
		t.Errorf("%d findings:\n%s", len(f), Report(f))
	}
}

// rateLimitAttempts bounds the logins of the rate limit test; the default burst of
// rl.public_auth is far below it.
const rateLimitAttempts = 100

// TestSEC_T10_DetectorFindsStdLoggerLine: a canary a library writes through the standard logger
// during a run is a finding in the log sink.
func TestSEC_T10_DetectorFindsStdLoggerLine(t *testing.T) {
	r := StartRun(t, RunOptions{})
	v := r.Reg.New(ClassSecret, "", "standard logger line")
	log.Printf("library message %s", v)
	ctx, cancel := context.WithTimeout(t.Context(), collectWait)
	defer cancel()
	r.Collect(ctx)
	got := findingsIn(Scan(r.Reg, r.Sinks, AllowRules{}), SinkLog)
	if len(got) != 1 || !strings.HasPrefix(got[0].Location, "std logger line") {
		t.Fatalf("log findings:\n%s\nwant exactly one in the standard logger lines", Report(got))
	}
}
