// SPDX-License-Identifier: Apache-2.0

//go:build sdkserver

package sdkserver_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// The waits of the end-to-end test: a start or restart builds a server (and at the first start
// the container and the world); every other operation answers at once.
const (
	slowOpWait = 5 * time.Minute
	opWait     = 2 * time.Minute
	exitWait   = 2 * time.Minute
)

// realServerTimings are the timing values the real-server tests of the Python SDK pass in the
// config of start (README.md "Timing values"); this test runs with them, so the server accepts
// them and a run starts within agent.invocation_token_ttl of its delivery.
var realServerTimings = map[string]string{
	"agent.invocation_token_ttl": "5s",
	"auth.clock_leeway":          "1s",
	"agent.ack_timeout":          "2s",
	"agent.ack_persist_interval": "100ms",
	"sync.heartbeat_interval":    "2s",
}

// e2eConsumer is the consumer name of the end-to-end test.
const e2eConsumer = "e2e"

// codeTokenInvalid is the problem code of a refused invocation token (09 4.5).
const codeTokenInvalid = "invocation_token_invalid"

// TestProtocolEndToEnd drives the program TestSDKServer through its line protocol, as the
// real-server tests of the Python SDK do: start with the thumbprint of an agent key of this
// test, trigger a run, read it, forge each token variant (the server refuses every one with
// reason invalid), restart with the cursor before the delivered event (the redelivery brings a
// newer token, the first one is superseded), start the run with the newer token (starts 1) and
// stop. Lotse's API token and every invocation token appear in no line of the program's output.
func TestProtocolEndToEnd(t *testing.T) {
	t.Parallel()
	a := newAgentClient(t)
	dir := t.TempDir()
	p := startProgram(t)

	p.wantError(t, map[string]any{"op": "trigger", "text": "too early"})
	ready := p.call(t, map[string]any{"op": "start", "agent_jkt": a.jkt, "dir": dir, "config": realServerTimings}, "ready", slowOpWait)
	for _, k := range []string{"origin", "instance_id", "agent_id", "other_agent_id", "token_file"} {
		if str(ready[k]) == "" {
			t.Fatalf("ready without %s: %v", k, ready)
		}
	}
	cfg, _ := ready["config"].(map[string]any)
	for k, v := range realServerTimings {
		if str(cfg[k]) != v {
			t.Fatalf("ready config %v, want %s = %s", cfg, k, v)
		}
	}
	a.origin, a.token = str(ready["origin"]), readSecret(t, str(ready["token_file"]))
	a.poll(t, e2eConsumer) // the first poll creates the consumer at head_seq (04 15.1 point 2)

	trig := p.call(t, map[string]any{"op": "trigger", "text": "Hello @Lotse, the export fails"}, "trigger", opWait)
	run := str(trig["run_id"])
	if str(trig["thread_id"]) == "" || str(trig["message_id"]) == "" {
		t.Fatalf("trigger answer %v", trig)
	}
	first := a.waitInvocation(t, e2eConsumer, run)
	p.wantRun(t, run, "queued", 0)

	forged := map[string]string{}
	for _, v := range []string{"bad_signature", "alg_hs256", "typ_access", "aud_other_agent"} {
		f := p.call(t, map[string]any{"op": "forge_token", "run_id": run, "variant": v}, "forge_token", opWait)
		forged[v] = readSecret(t, str(f["token_file"]))
		a.wantRefused(t, run, forged[v], "invalid", v)
	}
	p.wantError(t, map[string]any{"op": "forge_token", "run_id": run, "variant": "unknown"})
	p.wantError(t, map[string]any{"op": "no_such_op"})
	p.wantRun(t, run, "queued", 0)

	p.call(t, map[string]any{"op": "restart", "consumer": e2eConsumer, "acked_seq": first.seq - 1}, "restarted", slowOpWait)
	second := a.waitInvocation(t, e2eConsumer, run)
	if second.seq != first.seq || jtiOf(t, second.token) == jtiOf(t, first.token) {
		t.Fatalf("the redelivery has seq %d (was %d) and the same jti: %v", second.seq, first.seq, second.seq == first.seq)
	}
	a.wantRefused(t, run, first.token, "superseded", "the first token after the redelivery")
	if code, body := a.start(t, run, second.token); code != http.StatusOK {
		t.Fatalf("startRun with the redelivered token = %d %s", code, redact(body, a.secrets(first.token, second.token)))
	}
	p.wantRun(t, run, "running", 1)

	p.call(t, map[string]any{"op": "stop"}, "stop", opWait)
	p.waitExit(t)
	secrets := a.secrets(first.token, second.token)
	for _, tok := range forged {
		secrets = append(secrets, tok)
	}
	if p.output.leaks(secrets) {
		t.Fatal("a token is in the output of the program")
	}
}

// child is a running sdkserver program: the test binary itself, run with -test.run
// '^TestSDKServer$'.
type child struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	answers chan map[string]any
	output  *outputLog
	exited  chan error
}

// startProgram runs the test binary as the program and stops it, by its PID, at the end of t.
func startProgram(t *testing.T) *child {
	t.Helper()
	// os.Args[0] is this test binary; the program is its test TestSDKServer.
	cmd := exec.CommandContext(context.WithoutCancel(t.Context()), os.Args[0], "-test.run", "^TestSDKServer$") //nolint:gosec // G204: the own test binary with fixed arguments
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &child{cmd: cmd, stdin: stdin, answers: make(chan map[string]any, 16), output: &outputLog{}, exited: make(chan error, 1)}
	cmd.Stderr = p.output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	read := make(chan struct{})
	go func() {
		defer close(read)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			_, _ = p.output.Write([]byte(line + "\n"))
			if rest, ok := strings.CutPrefix(line, answerPrefix); ok {
				var m map[string]any
				if json.Unmarshal([]byte(rest), &m) == nil {
					p.answers <- m
				}
			}
		}
	}()
	go func() {
		<-read
		p.exited <- cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = stdin.Close()
		select {
		case <-p.exited:
		case <-time.After(exitWait):
			_ = cmd.Process.Kill()
			<-p.exited
		}
	})
	return p
}

// send writes one request line.
func (p *child) send(t *testing.T, req map[string]any) {
	t.Helper()
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.stdin.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
}

// answer waits for the next answer line.
func (p *child) answer(t *testing.T, op string, wait time.Duration) map[string]any {
	t.Helper()
	select {
	case a := <-p.answers:
		return a
	case err := <-p.exited:
		p.exited <- err
		t.Fatalf("the program exited (%v) before answering %s:\n%s", err, op, p.output.tail())
	case <-time.After(wait):
		t.Fatalf("no answer to %s within %s:\n%s", op, wait, p.output.tail())
	}
	return nil
}

// call sends req and requires the answer op want.
func (p *child) call(t *testing.T, req map[string]any, want string, wait time.Duration) map[string]any {
	t.Helper()
	p.send(t, req)
	a := p.answer(t, str(req["op"]), wait)
	if str(a["op"]) != want {
		t.Fatalf("%s answered %v, want op %s", req["op"], a, want)
	}
	return a
}

// wantError sends req and requires an error answer with a detail.
func (p *child) wantError(t *testing.T, req map[string]any) {
	t.Helper()
	a := p.call(t, req, "error", opWait)
	if str(a["detail"]) == "" {
		t.Fatalf("error answer without detail: %v", a)
	}
}

// wantRun reads the run and requires its state and the number of starts.
func (p *child) wantRun(t *testing.T, run, state string, starts int) {
	t.Helper()
	a := p.call(t, map[string]any{"op": "run", "run_id": run}, "run", opWait)
	n, _ := a["starts"].(float64)
	if str(a["state"]) != state || int(n) != starts || a["version"] == nil {
		t.Fatalf("run %v, want %s with %d starts", a, state, starts)
	}
}

// waitExit waits for the end of the program and requires exit code 0.
func (p *child) waitExit(t *testing.T) {
	t.Helper()
	_ = p.stdin.Close()
	select {
	case err := <-p.exited:
		p.exited <- err
		if err != nil {
			t.Fatalf("the program ended with %v:\n%s", err, p.output.tail())
		}
	case <-time.After(exitWait):
		t.Fatalf("the program did not end within %s", exitWait)
	}
}

// outputLog collects stdout and stderr of the program.
type outputLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write appends p.
func (o *outputLog) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.b.Write(p)
}

// leaks reports whether one of secrets occurs in the output.
func (o *outputLog) leaks(secrets []string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, s := range secrets {
		if s != "" && bytes.Contains(o.b.Bytes(), []byte(s)) {
			return true
		}
	}
	return false
}

// tail returns the end of the output for a failure message.
func (o *outputLog) tail() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	b := o.b.Bytes()
	if len(b) > 8192 {
		b = b[len(b)-8192:]
	}
	return string(b)
}

// agentClient is Lotse as an adapter: the API token of the program bound to the DPoP key of the
// test (RFC 9449), REST polling of a consumer, startRun.
type agentClient struct {
	key    *ecdsa.PrivateKey
	jwk    json.RawMessage
	jkt    string
	origin string
	token  string
	nonce  string
	http   *http.Client
}

// newAgentClient returns a client with a fresh P-256 key and its thumbprint.
func newAgentClient(t *testing.T) *agentClient {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := jose.JWKFromPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(jwk)
	if err != nil {
		t.Fatal(err)
	}
	return &agentClient{key: k, jwk: raw, jkt: jwk.Thumbprint(), http: &http.Client{Timeout: testkit.Patience}}
}

// secrets returns the API token and tokens.
func (a *agentClient) secrets(tokens ...string) []string { return append([]string{a.token}, tokens...) }

// do sends one request with a DPoP proof and the newest nonce, once more on use_dpop_nonce.
func (a *agentClient) do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	for range 2 {
		code, b := a.attempt(t, method, path, body)
		if code != http.StatusUnauthorized || problemOf(b).Code != "use_dpop_nonce" {
			return code, b
		}
	}
	return a.attempt(t, method, path, body)
}

// attempt sends one request.
func (a *agentClient) attempt(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	target := a.origin + "/api/v1" + path
	req, err := http.NewRequestWithContext(t.Context(), method, target, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	htu, _, _ := strings.Cut(target, "?")
	req.Header.Set("Authorization", "DPoP "+a.token)
	req.Header.Set("DPoP", a.proof(t, method, htu))
	resp, err := a.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		a.nonce = n
	}
	return resp.StatusCode, b
}

// proof returns a DPoP proof for htm and htu with the newest nonce, bound to the API token.
func (a *agentClient) proof(t *testing.T, htm, htu string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(a.token))
	claims := map[string]any{"jti": ids.New().String(), "htm": htm, "htu": htu, "iat": json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		"ath": base64.RawURLEncoding.EncodeToString(sum[:])}
	if a.nonce != "" {
		claims["nonce"] = a.nonce
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	s, err := jose.SignCompact(jose.ES256Signer{Key: a.key}, jose.Header{Typ: "dpop+jwt", JWK: a.jwk}, payload)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// delivered is one delivered agent.invocation.
type delivered struct {
	seq   int64
	token string
}

// poll reads one page of consumer (pollAgentEvents), acknowledges it (ackAgentEvents) and returns
// its events.
func (a *agentClient) poll(t *testing.T, consumer string) []map[string]any {
	t.Helper()
	code, b := a.do(t, http.MethodGet, "/agents/me/consumers/"+consumer+"/events?limit=100", nil)
	if code != http.StatusOK {
		t.Fatalf("pollAgentEvents = %d %s", code, problemOf(b).Code)
	}
	var page struct {
		Events     []map[string]any `json:"events"`
		NextCursor int64            `json:"next_cursor"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Events) > 0 {
		if code, b := a.do(t, http.MethodPost, "/agents/me/consumers/"+consumer+"/ack", map[string]any{"seq": page.NextCursor}); code != http.StatusOK {
			t.Fatalf("ackAgentEvents = %d %s", code, problemOf(b).Code)
		}
	}
	return page.Events
}

// waitInvocation polls consumer until the agent.invocation of run arrives.
func (a *agentClient) waitInvocation(t *testing.T, consumer, run string) delivered {
	t.Helper()
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		for _, ev := range a.poll(t, consumer) {
			res, _ := ev["resource"].(map[string]any)
			if str(ev["type"]) == "agent.invocation" && str(res["id"]) == run && str(ev["invocation_token"]) != "" {
				seq, _ := ev["seq"].(float64)
				return delivered{seq: int64(seq), token: str(ev["invocation_token"])}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("no agent.invocation of run %s within %s", run, testkit.Patience)
	return delivered{}
}

// start sends startRun with token, the configured billing mode and tool class chat.
func (a *agentClient) start(t *testing.T, run, token string) (int, []byte) {
	t.Helper()
	return a.do(t, http.MethodPost, "/runs/"+run+"/start", map[string]any{"invocation_token": token, "billing_mode_used": "organization",
		"tool_class_used": "chat"})
}

// wantRefused requires 403 invocation_token_invalid with reason at startRun with token.
func (a *agentClient) wantRefused(t *testing.T, run, token, reason, label string) {
	t.Helper()
	code, b := a.start(t, run, token)
	if p := problemOf(b); code != http.StatusForbidden || p.Code != codeTokenInvalid || p.Reason != reason {
		t.Fatalf("%s: startRun = %d %s %s, want 403 %s %s", label, code, p.Code, p.Reason, codeTokenInvalid, reason)
	}
}

// problem is the part of a problem body the test reads.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// problemOf decodes a problem body; an empty problem when it is none.
func problemOf(b []byte) problem {
	var p problem
	_ = json.Unmarshal(b, &p)
	return p
}

// readSecret reads a token file of the program and requires mode 0600.
func readSecret(t *testing.T, path string) string {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("%s has mode %v, want 0600", path, fi.Mode().Perm())
	}
	b, err := os.ReadFile(path) // #nosec G304 -- a token file the program wrote into the temp dir of this test
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatalf("%s is empty", path)
	}
	return string(b)
}

// jtiOf returns the jti of a compact token without verifying it.
func jtiOf(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatal("not a compact JWS")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Jti string `json:"jti"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c.Jti
}

// str returns v as a string, or "".
func str(v any) string {
	s, _ := v.(string)
	return s
}
