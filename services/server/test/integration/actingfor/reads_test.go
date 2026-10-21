// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// scopeLog records the operations of the requests that called Scope of the server's Authorizer
// (harness.Options.ScopeObserver); calls outside a request (jobs, deliveries) have no operation
// and are left out.
type scopeLog struct {
	mu  sync.Mutex
	ops map[string]bool
}

// observe is the ScopeObserver of the server.
func (l *scopeLog) observe(ctx context.Context, _ authorize.Action) {
	op, ok := httpserver.OperationFrom(ctx)
	if !ok {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ops == nil {
		l.ops = map[string]bool{}
	}
	l.ops[op.ID] = true
}

// take returns the recorded operations and starts a new record.
func (l *scopeLog) take() map[string]bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.ops
	l.ops = nil
	return out
}

// readsWorld is the world of the read cases: Alex's run R in Support/Allgemein, started by Lotse,
// whose delivered token T binds Lotse's requests; the forum post of the owner in
// Engineering/Projekt-A with its first message.
type readsWorld struct {
	*env
	r             run
	post, postMsg string
	supportMsg    string
	scopes        *scopeLog
	sinks         *harness.TelemetrySinks
	alexAccess    string
	tamperedT     string
	autocompleteT string
	texterToken   string
}

// TestAZM44_AZM45_ReadsInRun runs the read cases of brief M2-14 on one server: AZ-M44, AZ-M45,
// the unread counters, the consumer of pollAgentEvents, the invalid header values, every scoped
// list and, last, the token check over everything before. Telemetry sinks are set, so the test
// never runs in parallel with another harness start.
func TestAZM44_AZM45_ReadsInRun(t *testing.T) {
	scopes := &scopeLog{}
	sinks := harness.NewTelemetrySinks()
	e := newEnv(t, harness.Options{Telemetry: sinks, ScopeObserver: scopes.observe}, nil)
	w := &readsWorld{env: e, scopes: scopes, sinks: sinks}
	w.r = e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Bitte zusammenfassen")
	w.post = e.w.Threads["Engineering/Projekt-A"]
	w.postMsg = e.text(`SELECT id::text FROM messages WHERE thread_id = $1 AND kind <> 'run' ORDER BY created_at, id LIMIT 1`, w.post)
	w.supportMsg = e.w.Messages["Support/Allgemein"][0]

	t.Run("TestAZM44_EndToEnd", func(t *testing.T) { w.azm44(e.in(t)) })
	t.Run("TestAZM45_EndToEnd", func(t *testing.T) { w.azm45(e.in(t)) })
	t.Run("TestSR9_UnreadCounters", func(t *testing.T) { w.unreadCounters(e.in(t)) })
	t.Run("TestPollAgentEvents_HeaderDoesNotNarrowConsumer", func(t *testing.T) { w.pollUnnarrowed(e.in(t)) })
	t.Run("TestSR9_InvalidHeader", func(t *testing.T) { w.invalidHeader(e.in(t)) })
	t.Run("TestSR9_EveryScopedList", func(t *testing.T) { w.everyScopedList(e.in(t)) })
	t.Run("TestSR9_TokenNeverLogged", func(t *testing.T) { w.tokenNeverLogged(e.in(t)) })
	t.Run("Contract_NoNullInSuccessBodies", func(t *testing.T) { e.in(t).noNullInSuccess() })
}

// azm44 (AZ-M44): getMessage of the Projekt-A post with T answers 404 exactly like a random
// UUIDv7, and one deny entry message.read with acting_for_insufficient names Alex and the run.
func (w *readsWorld) azm44(e *env) {
	got := e.do(e.lotse, http.MethodGet, "/messages/"+w.postMsg, nil, w.r.token)
	unknown := e.do(e.lotse, http.MethodGet, "/messages/"+ids.New().String(), nil, w.r.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "getMessage of the post with T")
	e.sameAnswer(got, unknown, "getMessage of the post with T")
	ds := e.denials("message.read", w.postMsg, e.lotse.PrincipalID, "acting_for_insufficient")
	if len(ds) != 1 {
		e.t.Fatalf("%d deny entries of the post, want 1", len(ds))
	}
	d := ds[0]
	if d.reason != "acting_for_insufficient" || d.details["acting_for_user_id"] != e.alex.PrincipalID || d.details["run_id"] != w.r.id {
		e.t.Fatalf("deny entry %s %v, want acting_for_insufficient for Alex and run %s", d.reason, d.details, w.r.id)
	}
}

// azm45 (AZ-M45, hint H1): without the header Lotse reads the post with its own rights, and
// explainChannelAccess of Projekt-A by the owner lists Alex with level none and a source via_agent
// that names Lotse.
func (w *readsWorld) azm45(e *env) {
	m := e.ok(e.do(e.lotse, http.MethodGet, "/messages/"+w.postMsg, nil), http.StatusOK, "getMessage of the post without the header")
	if m["id"] != w.postMsg {
		e.t.Fatalf("getMessage returned %v", m["id"])
	}
	access := e.ok(e.must(e.owner, http.MethodGet, "/channels/"+e.w.Channels["Engineering/Projekt-A"]+"/access", nil, http.StatusOK),
		http.StatusOK, "explainChannelAccess")
	entries, _ := access["entries"].([]any)
	for _, raw := range entries {
		en, _ := raw.(map[string]any)
		p, _ := en["principal"].(map[string]any)
		if p["id"] != e.alex.PrincipalID {
			continue
		}
		if en["level"] != "none" {
			e.t.Fatalf("Alex has level %v on Projekt-A, want none", en["level"])
		}
		sources, _ := en["sources"].([]any)
		for _, s := range sources {
			if src, _ := s.(map[string]any); src["kind"] == "via_agent" && src["agent_id"] == e.lotse.PrincipalID {
				return
			}
		}
		e.t.Fatalf("Alex's sources %v name no via_agent of Lotse", sources)
	}
	e.t.Fatalf("explainChannelAccess lists no entry of Alex: %v", entries)
}

// unreadCounters (brief "TestSR9_UnreadCounters"): getSnapshot of Lotse without the header shows
// Engineering in spaces, channels and active_runs; with T none of its IDs appears in spaces,
// channels, active_runs or unread_threads. The merged snapshot computes no unread threads for an
// agent (agents keep no read states), so unread_threads is empty both ways.
func (w *readsWorld) unreadCounters(e *env) {
	eng := e.engineeringIDs()
	member := func(snap map[string]any, name string) []byte {
		b, _ := json.Marshal(snap[name])
		return b
	}
	plain := e.ok(e.do(e.lotse, http.MethodGet, "/sync/snapshot", nil), http.StatusOK, "getSnapshot")
	for _, name := range []string{"spaces", "channels", "active_runs"} {
		if firstID(member(plain, name), eng) == "" {
			e.t.Fatalf("getSnapshot without the header lists nothing of Engineering in %s", name)
		}
	}
	if u, _ := plain["unread_threads"].([]any); len(u) != 0 {
		e.t.Fatalf("unread_threads of an agent without the header: %v", u)
	}
	bound := e.ok(e.do(e.lotse, http.MethodGet, "/sync/snapshot", nil, w.r.token), http.StatusOK, "getSnapshot with T")
	for _, name := range []string{"spaces", "channels", "active_runs", "unread_threads"} {
		if id := firstID(member(bound, name), eng); id != "" {
			e.t.Fatalf("getSnapshot with T lists %s of Engineering in %s", id, name)
		}
	}
	if firstID(member(bound, "channels"), []string{e.w.Channels["Support/Allgemein"]}) == "" {
		e.t.Fatal("getSnapshot with T lists Support/Allgemein no longer; the intersection is empty")
	}
}

// pollUnnarrowed (07 8.1, brief design note 6): a message.created in Projekt-A reaches Lotse, which
// reads Engineering, through pollAgentEvents with T exactly as without the header: two consumers,
// the same event IDs.
func (w *readsWorld) pollUnnarrowed(e *env) {
	e.page(e.lotse, "plain")
	e.page(e.lotse, "bound", w.r.token)
	msg := e.post(e.owner, w.post, "Neuer Beitrag in Projekt-A")
	collect := func(name string, tokens ...string) []string {
		var got []string
		seenMsg := false
		e.eventually("message.created of the post on "+name, func() bool {
			for _, ev := range e.page(e.lotse, name, tokens...).Events {
				got = append(got, ev.ID)
				if ev.Type == "message.created" && ev.Resource.ID == msg {
					seenMsg = true
				}
			}
			return seenMsg
		})
		return got
	}
	plain := collect("plain")
	bound := collect("bound", w.r.token)
	// The consumers started one request apart; compare from the first event both received.
	if i := slices.Index(plain, firstCommon(plain, bound)); i > 0 {
		plain = plain[i:]
	}
	if i := slices.Index(bound, firstCommon(bound, plain)); i > 0 {
		bound = bound[i:]
	}
	if !slices.Equal(plain, bound) {
		e.t.Fatalf("the consumer with T got %v, without %v", bound, plain)
	}
}

// firstCommon returns the first element of a that b also holds, or "".
func firstCommon(a, b []string) string {
	for _, x := range a {
		if slices.Contains(b, x) {
			return x
		}
	}
	return ""
}

// invalidHeader (brief "TestSR9_InvalidHeader"): every value that names no run of Lotse answers
// getMessage with 403 invocation_token_invalid reason invalid, also Alex's own session with T; T
// without Authorization answers 401 unauthenticated; the deny entries name no resource (the
// status is decided before the path, design note 4b) and carry token_verdict invalid, no
// acting_for_user_id and no token.
func (w *readsWorld) invalidHeader(e *env) {
	w.tamperedT = tamper(w.r.token)
	texterRun := e.newRun(e.owner, e.texter, "Marketing/Kampagnen", "Texte entwerfen")
	w.texterToken = texterRun.token
	w.alexAccess, _ = e.alex.Tokens()
	w.autocompleteT = w.forgeAutocomplete(e)
	e.remember(w.tamperedT, w.alexAccess, w.autocompleteT)
	path := "/messages/" + w.supportMsg
	for _, c := range []struct {
		name   string
		tokens []string
	}{
		{"a changed signature byte", []string{w.tamperedT}},
		{"Texter's token", []string{w.texterToken}},
		{"an access token of Alex", []string{w.alexAccess}},
		{"a token with trigger_kind autocomplete", []string{w.autocompleteT}},
		{"two header values", []string{w.r.token, w.r.token}},
		{"an empty value", []string{""}},
	} {
		e.wantProblem(e.do(e.lotse, http.MethodGet, path, nil, c.tokens...), http.StatusForbidden, "invocation_token_invalid", "invalid",
			"getMessage with "+c.name)
	}
	e.wantProblem(e.do(e.alex, http.MethodGet, path, nil, w.r.token), http.StatusForbidden, "invocation_token_invalid", "invalid",
		"Alex's session with T")
	req, err := http.NewRequestWithContext(e.t.Context(), http.MethodGet, e.s.APIBase+path, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set(headerName, w.r.token)
	resp, err := e.plain.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	plain := harness.Response{Status: resp.StatusCode, Header: resp.Header, Body: body}
	e.seen.observe(harness.Request{Method: http.MethodGet, Path: path}, plain)
	e.wantProblem(plain, http.StatusUnauthorized, "unauthenticated", "", "T without Authorization")
	for _, who := range []string{e.lotse.PrincipalID, e.alex.PrincipalID} {
		for _, d := range e.denials("message.read", "", who, "invocation_token_invalid") {
			if d.details["token_verdict"] != "invalid" {
				e.t.Fatalf("deny entry %s %v, want invocation_token_invalid with token_verdict invalid", d.reason, d.details)
			}
			if _, has := d.details["acting_for_user_id"]; has {
				e.t.Fatalf("an invalid binding names a person: %v", d.details)
			}
		}
	}
}

// tamper returns tok with one changed character in the middle of its signature segment.
func tamper(tok string) string {
	i := strings.LastIndexByte(tok, '.') + (len(tok)-strings.LastIndexByte(tok, '.'))/2
	c := byte('A')
	if tok[i] == 'A' {
		c = 'B'
	}
	return tok[:i] + string(c) + tok[i+1:]
}

// forgeAutocomplete signs the claims of T with trigger_kind autocomplete, without run_id and with
// a fresh jti, under the header of T with the server's own invocation key from its key store
// (secrets.signing_key_dir, a PKCS#8 PEM file per key, ADR 0066): a token whose signature,
// issuer and audience are valid but that names no run (05 section 5; brief design note 3b).
func (w *readsWorld) forgeAutocomplete(e *env) string {
	parts := strings.Split(w.r.token, ".")
	rawHead, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		e.t.Fatal(err)
	}
	var head struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(rawHead, &head); err != nil || head.Kid == "" {
		e.t.Fatalf("the header of T has no kid: %v", err)
	}
	pemBytes, err := os.ReadFile(filepath.Join(e.s.SigningKeyDir, "invocation-"+head.Kid+".pem"))
	if err != nil {
		e.t.Fatalf("the invocation key of T is not in the key store: %v", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		e.t.Fatal("the key file is no PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		e.t.Fatal(err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		e.t.Fatal("the invocation key is no Ed25519 key")
	}
	c := claims(e.t, w.r.token)
	delete(c, "run_id")
	c["trigger_kind"] = "autocomplete"
	c["jti"] = ids.New().String()
	payload, err := json.Marshal(c)
	if err != nil {
		e.t.Fatal(err)
	}
	in := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload)
	return in + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(in)))
}

// tokenNeverLogged (brief "TestSR9_TokenNeverLogged"): no token the cases above sent or received,
// nor its payload or signature segment, is in a log line, an exported span or OTLP log record, a
// problem body or a row of audit_events.
func (w *readsWorld) tokenNeverLogged(e *env) {
	if err := e.s.FlushTelemetry(e.t.Context()); err != nil {
		e.t.Fatal(err)
	}
	tokens := e.secrets()
	e.noTokenLeak(tokens)
	var telemetry []string
	for _, s := range w.sinks.Spans.GetSpans() {
		telemetry = append(telemetry, s.Name, s.Status.Description)
		for _, a := range s.Attributes {
			telemetry = append(telemetry, string(a.Key)+"="+a.Value.String())
		}
		for _, ev := range s.Events {
			telemetry = append(telemetry, ev.Name)
			for _, a := range ev.Attributes {
				telemetry = append(telemetry, a.Value.String())
			}
		}
	}
	for _, r := range w.sinks.Logs.Records() {
		telemetry = append(telemetry, r.Body().String())
		r.WalkAttributes(func(kv attribute.KeyValue) bool {
			telemetry = append(telemetry, string(kv.Key)+"="+kv.Value.String())
			return true
		})
	}
	if len(telemetry) == 0 {
		e.t.Fatal("the telemetry sinks hold nothing; the check would be vacuous")
	}
	all := strings.Join(telemetry, "\n")
	for i, tok := range tokens {
		for j, part := range tokenParts(tok) {
			if strings.Contains(all, part) {
				e.t.Fatalf("token %d part %d is in the telemetry", i, j)
			}
		}
	}
}
