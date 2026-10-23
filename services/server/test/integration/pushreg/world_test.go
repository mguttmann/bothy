// SPDX-License-Identifier: Apache-2.0

// Package pushreg_test holds the integration tests of the push registration and the preview
// endpoint (brief M3-22, "Tests", Integration): the full server of test/harness (M1-71) in role
// api against PostgreSQL 18 through pgtest, real sessions and device keys, every request through
// the real HTTP chain with DPoP (RFC 9449). Role api builds no push adapter and reads no push
// secret; the APNs and FCM keys carry fixture values, so both providers count as enabled.
//
// The world (newWorld) is the part of 07 sections 10 and 11.1 these tests need: Alex with trigger
// on Support, the group Leser with Sam and read on Support/Allgemein, Marketing/Kampagnen, which
// Sam reads and Alex does not, Robin without a grant on Support. Each person is signed in on one
// device of the harness (platform macos, so apns fits); a test that needs an Android device
// changes the platform of a device as a fixture (android).
package pushreg_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// apnsTopic is the only topic of push.apns.topics in the world.
const apnsTopic = "com.example.chat"

// fcmApp is the app_id of the Android registrations of the world.
const fcmApp = "com.example.chat.android"

// world is one server with the people, places and grants of the package comment.
type world struct {
	t                  *testing.T
	s                  *harness.Server
	alex, sam, robin   *harness.Client
	leser              string
	support, marketing string
	allgemein          string
	allgemeinMain      string
	kampagnen          string
	kampagnenMain      string
	mu                 sync.Mutex
	nonce              string
	http               *http.Client
	cfg                map[string]string
}

// pushConfig returns the configuration of the world: raised rate limits and the push keys of the
// test configuration of the brief (APNs and FCM enabled, no adapter in role api).
func pushConfig() map[string]string {
	cfg := harness.RaisedRateLimits()
	cfg["push.apns.team_id"], cfg["push.apns.key_id"], cfg["push.apns.topics"] = "EXAMPLETEAM", "EXAMPLEKEY", apnsTopic
	// The reference only: role api builds no APNs adapter and never resolves it.
	cfg["push.apns.private_key_ref"] = "file:<secrets.dir>/apns/key.p8"
	cfg["push.fcm.project_id"] = "example-chat"
	return cfg
}

// newWorld starts a server in role api with cfg (pushConfig when nil) and builds the world.
func newWorld(t *testing.T, cfg map[string]string, opts ...func(*harness.Options)) *world {
	t.Helper()
	if cfg == nil {
		cfg = pushConfig()
	}
	o := harness.Options{Config: cfg, Role: wire.RoleAPI, EgressDenyAll: true}
	for _, f := range opts {
		f(&o)
	}
	s := harness.Start(t, o)
	w := &world{t: t, s: s, http: &http.Client{}, cfg: cfg}
	w.alex = s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	w.sam = s.NewUser(t, harness.UserSpec{Handle: "sam", DisplayName: "Sam", Email: "sam@example.com"})
	w.robin = s.NewUser(t, harness.UserSpec{Handle: "robin", DisplayName: "Robin", Email: "robin@example.com"})
	w.support = w.create(s.Owner, "/spaces", map[string]any{"name": "Support"})
	w.marketing = w.create(s.Owner, "/spaces", map[string]any{"name": "Marketing"})
	w.allgemein, w.allgemeinMain = w.channel(w.support, "Allgemein")
	w.kampagnen, w.kampagnenMain = w.channel(w.marketing, "Kampagnen")
	w.grant("user", w.alex.PrincipalID, "space", w.support, "trigger")
	w.leser = w.create(s.Owner, "/groups", map[string]any{"name": "Leser"})
	harness.MustDo(t, s.Owner, harness.Request{Method: http.MethodPost, Path: "/groups/" + w.leser + "/members",
		Body: map[string]any{"user_id": w.sam.PrincipalID}}, http.StatusCreated)
	w.grant("group", w.leser, "channel", w.allgemein, "read")
	w.grant("user", w.sam.PrincipalID, "channel", w.kampagnen, "read")
	return w
}

// create posts body to path as c, expects 201 and returns the id of the answer.
func (w *world) create(c *harness.Client, path string, body map[string]any) string {
	w.t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: path, Body: body}, http.StatusCreated).JSON(w.t, &out)
	return out.ID
}

// channel creates a text channel in space and returns its id and main thread.
func (w *world) channel(space, name string) (string, string) {
	w.t.Helper()
	var ch struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces/" + space + "/channels",
		Body: map[string]any{"name": name, "kind": "text"}}, http.StatusCreated).JSON(w.t, &ch)
	return ch.ID, ch.MainThreadID
}

// grant gives subject level on target as the owner.
func (w *world) grant(subjectKind, subject, targetKind, target, level string) {
	w.t.Helper()
	w.create(w.s.Owner, "/grants", map[string]any{
		"subject_kind": subjectKind, "subject_id": subject, "target_kind": targetKind, "target_id": target, "level": level, "reason": "test",
	})
}

// post writes content (with the mentioned principals) into thread as c and returns the message id.
func (w *world) post(c *harness.Client, thread, content string, mentions ...string) string {
	w.t.Helper()
	body := map[string]any{"client_msg_id": ids.New().String(), "content_md": content}
	if len(mentions) > 0 {
		body["mention_ids"] = mentions
	}
	return w.create(c, "/threads/"+thread+"/messages", body)
}

// setPreviewMode stores notify.preview_mode of c (putMySetting); "" deletes nothing and stores the
// JSON string value as given.
func (w *world) setPreviewMode(c *harness.Client, mode string) {
	w.t.Helper()
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPut, Path: "/me/settings/notify.preview_mode",
		Body: map[string]any{"value": mode}}, http.StatusOK)
}

// android makes the device of c an Android device (fixture: the harness enrolls macos devices).
func (w *world) android(c *harness.Client) {
	w.t.Helper()
	if _, err := w.s.AdminDB().Exec(w.t.Context(), `UPDATE devices SET platform = 'android' WHERE id = $1`, c.DeviceID); err != nil {
		w.t.Fatal(err)
	}
}

// registered is the answer of a successful registerPush.
type registered struct {
	ID           string `json:"id"`
	DeviceID     string `json:"device_id"`
	Provider     string `json:"provider"`
	Environment  string `json:"environment"`
	AppID        string `json:"app_id"`
	PreviewToken string `json:"preview_token"`
}

// registerRaw sends registerPush as c with body and the extra headers.
func (w *world) registerRaw(c *harness.Client, body map[string]any, header http.Header) harness.Response {
	w.t.Helper()
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/me/devices/current/push-registrations", Body: body, Header: header})
}

// register registers token of provider with appID for the device of c, expects 201 and returns
// the answer.
func (w *world) register(c *harness.Client, provider, token, appID string) registered {
	w.t.Helper()
	r := w.registerRaw(c, map[string]any{"provider": provider, "token": token, "app_id": appID}, nil)
	if r.Status != http.StatusCreated {
		w.t.Fatalf("registerPush = %d %s, want 201", r.Status, r.Body)
	}
	var out registered
	r.JSON(w.t, &out)
	return out
}

// apnsToken returns a fresh APNs device token: 64 lower-case hexadecimal digits.
func apnsToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// fcmToken returns a fresh FCM device token with the EXAMPLE marker.
func fcmToken() string { return "EXAMPLE:" + ids.New().String() }

// previewRequest is one getPushPreview request; the zero Auth sends the preview token as
// "DPoP <token>", Proof nil a valid proof of the device key of Signer.
type previewRequest struct {
	// Token is the preview token; Auth, when set, replaces the whole Authorization header (Values
	// sends several).
	Token  string
	Auth   []string
	NoAuth bool
	// Signer is the client whose device key signs the proof; ATH, when set, is the token the proof
	// binds instead of Token; NoATH leaves ath out; NoNonce sends no nonce; Nonce sends this one.
	Signer  *harness.Client
	ATH     string
	NoATH   bool
	NoNonce bool
	Nonce   string
	// Message and Kind are the query; Kind "" is mention; OmitKind leaves the parameter out.
	Message  string
	Kind     string
	OmitKind bool
}

// preview sends getPushPreview as described by p. Without NoNonce or Nonce it retries once on
// use_dpop_nonce with the nonce of the answer (the behaviour of the extension).
func (w *world) preview(p previewRequest) harness.Response {
	w.t.Helper()
	r := w.previewOnce(p)
	if !p.NoNonce && p.Nonce == "" && r.Status == http.StatusUnauthorized && code(r.Body) == "use_dpop_nonce" {
		r = w.previewOnce(p)
	}
	return r
}

// previewOnce sends one getPushPreview request.
func (w *world) previewOnce(p previewRequest) harness.Response {
	w.t.Helper()
	q := url.Values{"nid": {ids.New().String()}, "message_id": {p.Message}}
	switch {
	case p.OmitKind:
	case p.Kind == "":
		q.Set("kind", "mention")
	default:
		q.Set("kind", p.Kind)
	}
	target := w.s.APIBase + "/push/preview"
	req, err := http.NewRequestWithContext(w.t.Context(), http.MethodGet, target+"?"+q.Encode(), nil)
	if err != nil {
		w.t.Fatal(err)
	}
	switch {
	case p.NoAuth:
	case p.Auth != nil:
		req.Header["Authorization"] = p.Auth
	default:
		req.Header.Set("Authorization", "DPoP "+p.Token)
	}
	if p.Signer != nil {
		w.mu.Lock()
		nonce := w.nonce
		w.mu.Unlock()
		switch {
		case p.NoNonce:
			nonce = ""
		case p.Nonce != "":
			nonce = p.Nonce
		}
		ath := p.Token
		if p.ATH != "" {
			ath = p.ATH
		}
		if p.NoATH {
			ath = ""
		}
		req.Header.Set("DPoP", proof(w.t, p.Signer, http.MethodGet, target, nonce, ath))
	}
	resp, err := w.http.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		w.t.Fatal(err)
	}
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		w.mu.Lock()
		w.nonce = n
		w.mu.Unlock()
	}
	return harness.Response{Status: resp.StatusCode, Header: resp.Header, Body: b}
}

// fetch fetches the preview of message with token, signed by c, and returns the response.
func (w *world) fetch(c *harness.Client, token, message string) harness.Response {
	w.t.Helper()
	return w.preview(previewRequest{Token: token, Signer: c, Message: message})
}

// proof returns a DPoP proof of the device key of c for htm and htu with nonce and the ath of
// token (RFC 9449 section 4.2); "" leaves nonce or ath out.
func proof(t *testing.T, c *harness.Client, htm, htu, nonce, token string) string {
	t.Helper()
	var jwk struct {
		X, Y, D string
	}
	if err := json.Unmarshal(c.Key(), &jwk); err != nil {
		t.Fatal(err)
	}
	d, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := jose.JWKFromPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubRaw, err := json.Marshal(pub)
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{"jti": ids.New().String(), "htm": htm, "htu": htu, "iat": json.Number(strconv.FormatInt(time.Now().Unix(), 10))}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if token != "" {
		sum := sha256.Sum256([]byte(token))
		claims["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	s, err := jose.SignCompact(jose.ES256Signer{Key: key}, jose.Header{Typ: "dpop+jwt", JWK: pubRaw}, payload)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// syncWriter serializes the writes of the server log into w.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// Write implements io.Writer.
func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// code returns the problem code of body.
func code(body []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &p)
	return p.Code
}

// wantStatus fails the test unless r has status and, when c is not "", problem code c.
func wantStatus(t *testing.T, what string, r harness.Response, status int, c string) {
	t.Helper()
	if r.Status != status || c != "" && code(r.Body) != c {
		t.Fatalf("%s = %d %s, want %d %s", what, r.Status, r.Body, status, c)
	}
}

// normalized returns r without the members that differ per request (Correlation-Id, Date, the
// rate-limit headers, correlation_id and instance), to compare two 404 answers.
func normalized(t *testing.T, r harness.Response) string {
	t.Helper()
	h := r.Header.Clone()
	for _, k := range []string{"Correlation-Id", "Date", "Ratelimit", "Ratelimit-Policy", "Dpop-Nonce", "Content-Length"} {
		h.Del(k)
	}
	var body map[string]any
	if err := json.Unmarshal(r.Body, &body); err != nil {
		t.Fatalf("body: %v: %s", err, r.Body)
	}
	delete(body, "correlation_id")
	delete(body, "instance")
	hb, _ := json.Marshal(h)
	bb, _ := json.Marshal(body)
	return strconv.Itoa(r.Status) + " " + string(hb) + " " + string(bb)
}

// members returns the sorted member names of the JSON object body.
func members(t *testing.T, body []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body: %v: %s", err, body)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// sortStrings sorts s in place (insertion sort; the lists are short).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// row is the stored state of a registration.
type row struct {
	UserID, DeviceID, SessionID, Provider, Token, Environment, AppID string
	Hash                                                             []byte
	DisabledReason                                                   string
	Disabled                                                         bool
}

// row reads the registration id; ok is false when it does not exist.
func (w *world) row(id string) (row, bool) {
	w.t.Helper()
	var r row
	var reason *string
	err := w.s.AdminDB().QueryRow(w.t.Context(), `SELECT user_id::text, device_id::text, auth_session_id::text, provider, token, environment,
		app_id, coalesce(preview_token_hash, ''::bytea), disabled_reason, disabled_at IS NOT NULL FROM push_registrations WHERE id = $1`, id).Scan(
		&r.UserID, &r.DeviceID, &r.SessionID, &r.Provider, &r.Token, &r.Environment, &r.AppID, &r.Hash, &reason, &r.Disabled)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return row{}, false
		}
		w.t.Fatal(err)
	}
	if reason != nil {
		r.DisabledReason = *reason
	}
	return r, true
}

// count returns the integer result of query.
func (w *world) count(query string, args ...any) int64 {
	w.t.Helper()
	var n int64
	if err := w.s.AdminDB().QueryRow(w.t.Context(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// noToken fails the test when body holds one of the secrets (raw, base64 or hex).
func noToken(t *testing.T, where string, body []byte, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		for _, form := range []string{s, base64.StdEncoding.EncodeToString([]byte(s)), base64.RawURLEncoding.EncodeToString([]byte(s)),
			hex.EncodeToString([]byte(s))} {
			if form != "" && bytes.Contains(body, []byte(form)) {
				t.Errorf("%s holds a token (length %d)", where, len(s))
			}
		}
	}
}
