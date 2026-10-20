// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/device"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// deadlineMargin is kept free before t.Deadline() for the removal of a case (a test constant).
const deadlineMargin = 30 * time.Second

// streamWait bounds the waits of stream steps that expect an effect of the host (a test constant).
const streamWait = 60 * time.Second

// Run runs every case of sc as a parallel subtest <scenario>/<case> in its own world.
func Run(t *testing.T, env *Env, sc *Scenario) {
	t.Helper()
	if sc.Profile != env.ProfileName {
		t.Fatalf("%s: profile %s, but the stack of this package runs %s", sc.ID, sc.Profile, env.ProfileName)
	}
	t.Run(sc.ID, func(t *testing.T) {
		for _, c := range sc.Cases {
			t.Run(c.Name, func(t *testing.T) {
				if !sc.Sequential {
					t.Parallel()
				}
				RunCase(t, env, sc, c)
			})
		}
	})
}

// RunCase builds the world of c and runs its steps in order.
func RunCase(t *testing.T, env *Env, sc *Scenario, c Case) {
	t.Helper()
	w := newInstance(t, env, sc, c)
	w.recordBaselines()
	for i, s := range c.Steps {
		if msgs := w.step(i, s); len(msgs) > 0 {
			w.diagnose()
			t.Fatal(Failure(sc.ID, c.Name, i, msgs))
		}
	}
}

// Failure renders a failed step: scenario, case and step index with the mismatches, which name
// expected and observed codes, states and counts only.
func Failure(scenario, caseName string, step int, mismatches []string) string {
	return fmt.Sprintf("%s/%s step %d: %s", scenario, caseName, step, strings.Join(mismatches, "; "))
}

// deadline returns the context of one wait: until t.Deadline() minus the margin, at most limit.
func (w *instance) deadline(limit time.Duration) (context.Context, context.CancelFunc) {
	end := time.Now().Add(limit)
	if d, ok := w.t.Deadline(); ok && d.Add(-deadlineMargin).Before(end) {
		end = d.Add(-deadlineMargin)
	}
	return context.WithDeadline(w.ctx, end)
}

// step runs step i and returns its mismatches.
func (w *instance) step(i int, s Step) []string {
	kind, err := s.Kind()
	if err != nil {
		return []string{err.Error()}
	}
	switch kind {
	case KindCall:
		return w.call(s)
	case KindDevice:
		return w.deviceStep(s)
	case KindHost:
		return w.hostStep(s)
	case KindServer:
		return w.serverStep(s)
	case KindWait:
		return w.wait(s)
	case KindCheck:
		return w.check(s)
	case KindParallel:
		return w.parallel(i, s)
	}
	return []string{"unknown kind " + kind}
}

// parallel runs the steps of s at once and joins their mismatches.
func (w *instance) parallel(i int, s Step) []string {
	var mu sync.Mutex
	var out []string
	var wg sync.WaitGroup
	for j, p := range s.Parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if msgs := w.step(i, p); len(msgs) > 0 {
				mu.Lock()
				out = append(out, fmt.Sprintf("parallel %d: %s", j, strings.Join(msgs, "; ")))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if s.Expect != nil && len(s.Expect.Statuses) > 0 {
		var got []int
		w.mu.Lock()
		for _, p := range s.Parallel {
			if st, ok := w.statuses[p.Session]; ok {
				got = append(got, st)
			}
		}
		w.mu.Unlock()
		out = append(out, CompareStatuses(*s.Expect, got)...)
	}
	return out
}

// refRE matches ${name}, ${name.field} and ${marker:name}.
var refRE = regexp.MustCompile(`\$\{([a-z0-9_:+-]+)(?:\.([a-z_]+))?\}`)

// expand replaces the references of s by the values of the instance.
func (w *instance) expand(s string) string {
	return refRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := refRE.FindStringSubmatch(m)
		v, err := w.lookup(sub[1], sub[2])
		if err != nil {
			w.t.Fatalf("%s/%s: %v", w.sc.ID, w.c.Name, err)
		}
		return v
	})
}

// expandAny expands every string inside v (maps, lists and scalars of YAML).
func (w *instance) expandAny(v any) any {
	switch x := v.(type) {
	case string:
		return w.expand(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[w.expand(k)] = w.expandAny(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = w.expandAny(e)
		}
		return out
	}
	return v
}

// lookup returns the value of a reference.
func (w *instance) lookup(name, field string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if c, ok := strings.CutPrefix(name, "canary:"); ok {
		return w.canaryValue(c), nil
	}
	if strings.HasPrefix(name, "marker") {
		if v, ok := w.markers[name]; ok {
			return v, nil
		}
		n, err := rand.Int(rand.Reader, big.NewInt(9_000_000))
		if err != nil {
			return "", err
		}
		v := fmt.Sprint(1_000_000 + n.Int64())
		w.markers[name] = v
		return v, nil
	}
	if strings.HasPrefix(name, "now+") {
		d, err := ParseDuration(strings.TrimPrefix(name, "now+"))
		if err != nil {
			return "", err
		}
		return time.Now().Add(d).UTC().Format("2006-01-02T15:04:05.000Z"), nil
	}
	if field == "" {
		if v, ok := w.keep[name]; ok {
			return v, nil
		}
		return "", fmt.Errorf("unknown reference ${%s}", name)
	}
	if c, ok := w.devices[name]; ok {
		switch field {
		case "id":
			return c.PrincipalID, nil
		case "device":
			return c.DeviceID, nil
		case "session":
			return c.SessionID, nil
		}
	}
	for _, m := range []map[string]*harness.Client{w.agents, w.apps} {
		if c, ok := m[name]; ok && field == "id" {
			return c.PrincipalID, nil
		}
	}
	if h, ok := w.hosts[name]; ok {
		switch field {
		case "pairing_code":
			return h.pairingCode, nil
		case "bad_code":
			return badChecksum(h.pairingCode), nil
		case "fp_prefix", "wrong_prefix":
			p := strings.ReplaceAll(h.fingerprint, "-", "")
			n, err := w.env.Profile.Int("terminal.fingerprint_confirm_chars")
			if err != nil {
				n = 8
			}
			if len(p) < n {
				return "", fmt.Errorf("host %s has no fingerprint yet", name)
			}
			if field == "wrong_prefix" {
				return wrongPrefix(p[:n]), nil
			}
			return p[:n], nil
		}
	}
	if r, ok := strings.CutPrefix(name, "random:"); ok {
		if v, ok := w.keep["random:"+r]; ok {
			return v, nil
		}
		v := randomUUID()
		w.keep["random:"+r] = v
		return v, nil
	}
	if field == "id" {
		if h, ok := w.hosts[name]; ok {
			return h.id, nil
		}
		if f, ok := w.fakes[name]; ok {
			return f.id, nil
		}
		for _, m := range []map[string]string{w.groups, w.grants} {
			if v, ok := m[name]; ok {
				return v, nil
			}
		}
		if s, ok := w.sessions[name]; ok {
			return s.id, nil
		}
	}
	return "", fmt.Errorf("unknown reference ${%s.%s}", name, field)
}

// badChecksum returns the pairing code with its last character changed, so its checksum fails
// (TM-T20); the code stays in memory.
func badChecksum(code string) string {
	if code == "" {
		return code
	}
	last := code[len(code)-1]
	repl := byte('A')
	if last == 'A' {
		repl = 'B'
	}
	return code[:len(code)-1] + string(repl)
}

// wrongPrefix returns a prefix of the same length whose every character differs from p
// (TM-T19); the alphabet of the fingerprint is Crockford base32.
func wrongPrefix(p string) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	out := []byte(p)
	for i := range out {
		j := strings.IndexByte(alphabet, out[i])
		out[i] = alphabet[(j+1+len(alphabet))%len(alphabet)]
	}
	return string(out)
}

// randomUUID returns a random UUID (version 4) that names no object of the stack.
func randomUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// client returns the client of a principal name (person, device, agent or app).
func (w *instance) client(name string) (*harness.Client, error) {
	if c, ok := w.devices[name]; ok {
		return c, nil
	}
	if c, ok := w.agents[name]; ok {
		return c, nil
	}
	if c, ok := w.apps[name]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("unknown principal %s", name)
}

// observedCall is what an expectation of a call compares.
type observedCall struct {
	Status int
	Body   []byte
}

// call runs a call step: as a principal, an operation of the API with the literal body.
func (w *instance) call(s Step) []string {
	c, err := w.client(s.As)
	if err != nil {
		return []string{err.Error()}
	}
	op, ok := w.env.Ops[s.Call]
	if !ok {
		return []string{"unknown operation " + s.Call}
	}
	path := op.Path
	for k, v := range s.Params {
		path = strings.ReplaceAll(path, "{"+k+"}", w.expand(v))
	}
	if strings.Contains(path, "{") {
		return []string{fmt.Sprintf("%s: a path parameter is missing", s.Call)}
	}
	req := harness.Request{Method: op.Method, Path: path}
	if s.With != nil {
		req.Body = w.expandAny(s.With)
	}
	if s.IfMatch != "" {
		get := c.Do(w.ctx, harness.Request{Method: http.MethodGet, Path: w.expand(s.IfMatch)})
		if get.Status != http.StatusOK {
			return []string{fmt.Sprintf("the read for If-Match answered %d", get.Status)}
		}
		req.Header = http.Header{"If-Match": []string{get.Header.Get("ETag")}}
	}
	if s.StepUp {
		if err := c.StepUp(w.ctx); err != nil {
			return []string{fmt.Sprintf("step-up before %s: %v", s.Call, err)}
		}
	}
	resp := c.Do(w.ctx, req)
	// A host raises its version with every report, so a read for If-Match can be stale by the
	// time of the write; the conductor reads again (a few times) before it compares.
	for attempt := 0; s.IfMatch != "" && resp.Status == http.StatusPreconditionFailed && attempt < confirmRetries; attempt++ {
		get := c.Do(w.ctx, harness.Request{Method: http.MethodGet, Path: w.expand(s.IfMatch)})
		req.Header = http.Header{"If-Match": []string{get.Header.Get("ETag")}}
		resp = c.Do(w.ctx, req)
	}
	msgs := CompareCall(*s.Expect, observedCall{Status: resp.Status, Body: resp.Body}, w.expandAny)
	if len(msgs) == 0 {
		w.keepValues(s.Keep, resp.Body)
	}
	return msgs
}

// keepValues binds the names of keep to the values at their JSON pointers in body.
func (w *instance) keepValues(keep map[string]string, body []byte) {
	if len(keep) == 0 {
		return
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		w.t.Fatalf("%s/%s: keep: the answer is no JSON", w.sc.ID, w.c.Name)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for name, ptr := range keep {
		v, ok := Pointer(doc, ptr)
		if !ok {
			w.t.Fatalf("%s/%s: keep %s: %s is not in the answer", w.sc.ID, w.c.Name, name, ptr)
		}
		w.keep[name] = fmt.Sprint(v)
	}
}

// session returns the session of a name.
func (w *instance) session(name string) (*sessInst, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s, ok := w.sessions[name]
	if !ok {
		return nil, fmt.Errorf("unknown session %s", name)
	}
	return s, nil
}

// lastConn returns the newest connection of a session.
func (s *sessInst) lastConn() (*device.Conn, error) {
	if len(s.conns) == 0 {
		return nil, errors.New("the session has no connection")
	}
	return s.conns[len(s.conns)-1], nil
}

// grant is the decoded TerminalSessionGrant.
type grant struct {
	Session struct {
		ID     string `json:"id"`
		HostID string `json:"host_id"`
	} `json:"session"`
	SessionToken   string    `json:"session_token"`
	TokenExpiresAt time.Time `json:"session_token_expires_at"`
	StreamPath     string    `json:"stream_path"`
	NoisePublicKey string    `json:"noise_public_key"`
}

// deviceStep runs a step of a device.
func (w *instance) deviceStep(s Step) []string {
	c, err := w.client(s.Device)
	if err != nil {
		return []string{err.Error()}
	}
	switch s.Do {
	case "enroll":
		return nil
	case "step_up":
		if err := c.StepUp(w.ctx); err != nil {
			return []string{fmt.Sprintf("step-up: %v", err)}
		}
		return nil
	case "open", "attach":
		return w.openOrAttach(c, s)
	}
	ses, err := w.session(s.Session)
	if err != nil {
		return []string{err.Error()}
	}
	conn, err := ses.lastConn()
	if err != nil {
		return []string{err.Error()}
	}
	if s.Do == "connect" && s.Device != ses.connDevice[len(ses.connDevice)-1] {
		// another principal dials the stream of the session (TM-T17): a connection of its own
		other, err := device.NewConn(w.env.Sidecar)
		if err != nil {
			return []string{err.Error()}
		}
		w.t.Cleanup(other.Close)
		other.StreamPath, other.Token = conn.StreamPath, conn.Token
		w.mu.Lock()
		ses.conns = append(ses.conns, other)
		ses.connDevice = append(ses.connDevice, s.Device)
		w.mu.Unlock()
		conn = other
	}
	switch s.Do {
	case "connect":
		return w.connect(c, s, conn)
	case "handshake":
		return w.handshake(s, ses, conn)
	case "start":
		return sendOrFail(conn.SendFrame(w.ctx, device.StartFrame(120, 40)), "start")
	case "send":
		return sendOrFail(conn.SendFrame(w.ctx, device.DataFrame([]byte(w.expand(s.Text)))), "send")
	case "expect_output":
		ctx, cancel := w.deadline(streamWait)
		defer cancel()
		if err := conn.WaitOutput(ctx, []byte(w.expand(s.Expect.Output))); err != nil {
			code, reason := conn.Stream.CloseCode(context.Background())
			return []string{fmt.Sprintf("expected output not seen (stream close %d %s)", code, reason)}
		}
		return w.expectReplay(s, conn)
	case "replay_message1":
		from := 0
		if s.From != nil {
			from = *s.From
		}
		if from >= len(ses.conns) {
			return []string{fmt.Sprintf("the session has no connection %d", from)}
		}
		if budget := time.Since(ses.conns[from].Opened); budget > TMT03Budget {
			return []string{fmt.Sprintf("INVALID: the replay came %s after the opening, beyond %s (the token may have expired)", budget.Round(time.Second), TMT03Budget)}
		}
		if err := conn.SendRaw(w.ctx, ses.conns[from].Message1); err != nil {
			return []string{fmt.Sprintf("replay: %v", err)}
		}
		return w.expectClose(s, conn)
	case "cut":
		return sendOrFail(conn.Stream.Cut(), "cut")
	case "close":
		return sendOrFail(conn.SendFrame(w.ctx, device.CloseFrame()), "close")
	case "wait_close":
		return w.expectClose(s, conn)
	}
	return []string{"unknown device op " + s.Do}
}

// sendOrFail turns a send error into a mismatch.
func sendOrFail(err error, what string) []string {
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", what, err)}
	}
	return nil
}

// openOrAttach runs openTerminalSession or attachTerminalSession with the hash of a fresh ephemeral
// key of the sidecar and records the grant under the session name.
func (w *instance) openOrAttach(c *harness.Client, s Step) []string {
	conn, err := device.NewConn(w.env.Sidecar)
	if err != nil {
		return []string{err.Error()}
	}
	w.t.Cleanup(conn.Close)
	body := map[string]any{"cols": 120, "rows": 40, "client_ephemeral_key_hash": conn.CEK}
	var path string
	var prior *sessInst
	if s.Do == "open" {
		body["profile"] = orDefault(s.Profile, "shell")
		if s.WorkdirIndex != nil {
			body["workdir_index"] = *s.WorkdirIndex
		}
		path = "/hosts/" + w.hostID(s.On) + "/terminal/sessions"
	} else {
		ses, err := w.session(s.Session)
		if err != nil {
			return []string{err.Error()}
		}
		prior = ses
		path = "/terminal/sessions/" + ses.id + "/attach"
	}
	resp := c.Do(w.ctx, harness.Request{Method: http.MethodPost, Path: path, Body: body})
	w.mu.Lock()
	w.statuses[s.Session] = resp.Status
	w.mu.Unlock()
	var msgs []string
	if s.Expect != nil {
		msgs = CompareCall(*s.Expect, observedCall{Status: resp.Status, Body: resp.Body}, w.expandAny)
	}
	if resp.Status != http.StatusCreated && resp.Status != http.StatusOK {
		return msgs
	}
	var g grant
	if err := json.Unmarshal(resp.Body, &g); err != nil {
		return append(msgs, "the grant is no JSON")
	}
	conn.Opened = time.Now()
	w.mu.Lock()
	defer w.mu.Unlock()
	ses := prior
	if ses == nil {
		if s.Session == "" {
			return append(msgs, "an open step needs a session name")
		}
		ses = &sessInst{id: g.Session.ID, host: s.On, device: s.Device}
		w.sessions[s.Session] = ses
	}
	ses.token, ses.streamPath, ses.expires = g.SessionToken, g.StreamPath, g.TokenExpiresAt
	conn.Token, conn.StreamPath = g.SessionToken, g.StreamPath
	ses.conns = append(ses.conns, conn)
	ses.connDevice = append(ses.connDevice, s.Device)
	return msgs
}

// connect opens the device stream of the newest connection.
func (w *instance) connect(c *harness.Client, s Step, conn *device.Conn) []string {
	// An agent or app has no device; its identify names a random one (TM-T17), so the frame is
	// well formed and only the kind of the token decides.
	deviceID := c.DeviceID
	if deviceID == "" {
		deviceID = randomUUID()
	}
	st, err := device.Dial(w.ctx, device.DialParams{Dialer: w.env.Dialer, Origin: stack.InstanceOrigin, StreamPath: conn.StreamPath, Prover: c, DeviceID: deviceID})
	if err != nil {
		return []string{fmt.Sprintf("connect: %v", err)}
	}
	conn.Stream = st
	if s.Expect.Close != nil {
		return w.expectClose(s, conn)
	}
	return nil
}

// handshake runs the Noise handshake of the newest connection with the token of s.Token.
func (w *instance) handshake(s Step, ses *sessInst, conn *device.Conn) []string {
	token := []byte(conn.Token)
	if s.Token != nil {
		t, msgs := w.tokenOf(*s.Token, conn.Token)
		if len(msgs) > 0 {
			return msgs
		}
		token = t
	}
	if s.Ephemeral == "other" {
		if err := conn.ReplaceSidecar(w.env.Sidecar); err != nil {
			return []string{err.Error()}
		}
	}
	key, err := w.noiseKey(ses.host)
	if err != nil {
		return []string{err.Error()}
	}
	ctx, cancel := w.deadline(streamWait)
	defer cancel()
	err = conn.Handshake(ctx, key, ses.id, w.hostID(ses.host), token)
	if s.Expect.Close != nil {
		if err == nil {
			// message 2 came: the host accepted a token the scenario expects to be refused
			return []string{fmt.Sprintf("the handshake completed, want close %d %s", s.Expect.Close.Code, s.Expect.Close.Reason)}
		}
		return w.expectClose(s, conn)
	}
	if err != nil {
		code, reason := conn.Stream.CloseCode(context.Background())
		return []string{fmt.Sprintf("handshake failed: %v (stream close %d %s)", err, code, reason)}
	}
	return nil
}

// noiseKey returns the static Noise key of a host or Fake-Host.
func (w *instance) noiseKey(host string) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if h, ok := w.hosts[host]; ok {
		return h.noiseKey, nil
	}
	if f, ok := w.fakes[host]; ok {
		return f.noiseKey, nil
	}
	return nil, fmt.Errorf("unknown host %s", host)
}

// expectClose waits for the close of the stream of conn and compares it.
func (w *instance) expectClose(s Step, conn *device.Conn) []string {
	ctx, cancel := w.deadline(streamWait)
	defer cancel()
	code, reason := conn.Stream.CloseCode(ctx)
	if s.Expect == nil || s.Expect.Close == nil {
		return []string{"the step expects no close"}
	}
	return CompareClose(*s.Expect.Close, code, reason)
}

// expectReplay compares replay_begin and replay_end of an attach.
func (w *instance) expectReplay(s Step, conn *device.Conn) []string {
	if s.Expect.Replay == nil {
		return nil
	}
	// replay_end can follow the last replayed output by a moment.
	return w.poll(func() []string {
		begin, end := conn.Replay()
		return CompareReplay(*s.Expect.Replay, begin, end)
	})
}

// wait passes the window of the configuration keys of s: the sum of their profile values, then
// the next step polls.
func (w *instance) wait(s Step) []string {
	keys, err := s.WaitKeys()
	if err != nil {
		return []string{err.Error()}
	}
	var total time.Duration
	for _, k := range keys {
		d, err := w.env.Profile.Duration(k)
		if err != nil {
			return []string{err.Error()}
		}
		total += d
	}
	select {
	case <-w.ctx.Done():
		return []string{"the test ended during a wait"}
	case <-time.After(total + time.Second):
	}
	return nil
}

// b64url decodes base64url without padding.
func b64url(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// problemCode returns the code of a Problem body, for world failures.
func problemCode(body []byte) string {
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(body, &p) != nil {
		return ""
	}
	return strings.TrimSpace(p.Code + " " + p.Reason)
}

// diagnose writes the journal of the bothy-host units of every host container of the case,
// filtered to codes, into the test output (Design note 3: on a failure, codes only).
func (w *instance) diagnose() {
	w.mu.Lock()
	hosts := make(map[string]*stack.Host, len(w.hosts))
	for name, h := range w.hosts {
		if h.container != nil {
			hosts[name] = h.container
		}
	}
	w.mu.Unlock()
	for name, h := range hosts {
		j, err := h.Journal(context.WithoutCancel(w.ctx))
		if err != nil {
			continue
		}
		var b strings.Builder
		stack.WriteCodes(&b, j)
		w.t.Logf("journal of %s (codes only):\n%s", name, b.String())
	}
}
