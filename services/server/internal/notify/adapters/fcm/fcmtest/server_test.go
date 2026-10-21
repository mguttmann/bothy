// SPDX-License-Identifier: Apache-2.0

package fcmtest_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
)

const (
	project = "example-chat"
	email   = "push@example-chat.example.com"
	kid     = "EXAMPLEKEY0001"
	canary  = "CANARY-fcmtest-message"
	// sam is Sam's fixture device token.
	sam = "sam-EXAMPLE-token:APA91b"
	// robin is Robin's fixture device token.
	robin = "robin-EXAMPLE-token:APA91b"
)

// fake starts a Fake with a fresh key and returns it with the key.
func fake(t *testing.T, lifetime time.Duration) (*fcmtest.Server, *rsa.PrivateKey) {
	t.Helper()
	key := newKey(t)
	srv, err := fcmtest.Start(fcmtest.Options{ProjectID: project, ClientEmail: email, KeyID: kid, PublicKey: &key.PublicKey, Lifetime: lifetime, ErrorMessage: canary})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv, key
}

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// claims returns the payload members of a valid assertion for srv.
func claims(srv *fcmtest.Server) map[string]any {
	iat := time.Now().Unix()
	return map[string]any{"iss": email, "scope": fcmtest.Scope, "aud": srv.TokenURL(), "iat": iat, "exp": iat + 3600}
}

// assertion signs payload with signer under header.
func assertion(t *testing.T, signer jose.Signer, h jose.Header, payload map[string]any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	a, err := jose.SignCompact(signer, h, b)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// client returns an HTTP/1.1 client that trusts the Fake.
func client(srv *fcmtest.Server) *http.Client {
	return &http.Client{Transport: &http.Transport{ // nosemgrep: bothy-go-egress-only -- a test client of the in-process Fake on loopback
		TLSClientConfig: &tls.Config{RootCAs: srv.RootCAs(), MinVersion: tls.VersionTLS12},
	}}
}

// do sends one request and returns status, header and decoded JSON body (nil when empty).
func do(t *testing.T, srv *fcmtest.Server, method, target, contentType, bearer string, body []byte) (int, http.Header, map[string]any) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client(srv).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("body is not a JSON object: %q", raw)
		}
	}
	return resp.StatusCode, resp.Header, m
}

const formType = "application/x-www-form-urlencoded"

// form returns the token request form for a.
func form(a string) []byte {
	return []byte(url.Values{"grant_type": {fcmtest.GrantType}, "assertion": {a}}.Encode())
}

// token obtains an access token with a valid assertion.
func token(t *testing.T, srv *fcmtest.Server, key *rsa.PrivateKey) string {
	t.Helper()
	a := assertion(t, jose.RS256Signer{Key: key}, jose.Header{Typ: "JWT", Kid: kid}, claims(srv))
	status, _, body := do(t, srv, http.MethodPost, srv.TokenURL(), formType, "", form(a))
	tok, _ := body["access_token"].(string)
	if status != http.StatusOK || tok == "" {
		t.Fatalf("token request: %d %v", status, body)
	}
	return tok
}

// sendURL is the send endpoint of srv.
func sendURL(srv *fcmtest.Server) string {
	return srv.URL() + "/v1/projects/" + project + "/messages:send"
}

// message returns a send body for the device token tok.
func message(tok string) []byte {
	return []byte(`{"message":{"token":` + strconv.Quote(tok) + `,"data":{"k":"message"}}}`)
}

// rpcError returns error.code and error.status of an FCM error body.
func rpcError(t *testing.T, body map[string]any) (int, string) {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object: %v", body)
	}
	code, _ := e["code"].(float64)
	status, _ := e["status"].(string)
	return int(code), status
}

// TestFakeVerifiesAssertion: the token endpoint issues a token only for a valid form and a valid
// assertion; everything else gets 400 invalid_grant, and every POST counts.
func TestFakeVerifiesAssertion(t *testing.T) {
	t.Parallel()
	srv, key := fake(t, 0)
	rs := jose.RS256Signer{Key: key}
	good := jose.Header{Typ: "JWT", Kid: kid}
	valid := assertion(t, rs, good, claims(srv))

	status, _, body := do(t, srv, http.MethodPost, srv.TokenURL(), formType, "", form(valid))
	if status != http.StatusOK || body["token_type"] != "Bearer" || body["expires_in"] != float64(3600) {
		t.Fatalf("valid assertion: %d %v", status, body)
	}
	if tok, _ := body["access_token"].(string); tok == "" {
		t.Fatalf("no access_token: %v", body)
	}

	now := time.Now().Unix()
	with := func(k string, v any) map[string]any {
		c := claims(srv)
		c["iat"], c["exp"] = now, now+3600
		if v == nil {
			delete(c, k)
		} else {
			c[k] = v
		}
		return c
	}
	bad := map[string]string{
		"other key":         assertion(t, jose.RS256Signer{Key: newKey(t)}, good, claims(srv)),
		"wrong kid":         assertion(t, rs, jose.Header{Typ: "JWT", Kid: "OTHEREXAMPLE"}, claims(srv)),
		"no kid":            assertion(t, rs, jose.Header{Typ: "JWT"}, claims(srv)),
		"no typ":            assertion(t, rs, jose.Header{Kid: kid}, claims(srv)),
		"wrong typ":         assertion(t, rs, jose.Header{Typ: "dpop+jwt", Kid: kid}, claims(srv)),
		"extra header":      assertion(t, rs, jose.Header{Typ: "JWT", Kid: kid, X5tS256: "EXAMPLE"}, claims(srv)),
		"PS256":             assertion(t, jose.PS256Signer{Key: key}, good, claims(srv)),
		"wrong iss":         assertion(t, rs, good, with("iss", "other@example-chat.example.com")),
		"wrong aud":         assertion(t, rs, good, with("aud", "https://oauth2.googleapis.com/token")),
		"wrong scope":       assertion(t, rs, good, with("scope", "https://www.googleapis.com/auth/cloud-platform")),
		"lifetime over 1h":  assertion(t, rs, good, with("exp", now+3601)),
		"exp equals iat":    assertion(t, rs, good, with("exp", now)),
		"missing exp":       assertion(t, rs, good, with("exp", nil)),
		"missing scope":     assertion(t, rs, good, with("scope", nil)),
		"extra claim":       assertion(t, rs, good, with("sub", email)),
		"exp not a number":  assertion(t, rs, good, with("exp", strconv.FormatInt(now+60, 10))),
		"tampered":          valid[:len(valid)-4] + "AAAA",
		"not a JWS":         "not-a-jws",
		"empty":             "",
		"two segments only": valid[:strings.LastIndex(valid, ".")],
	}
	for name, a := range bad {
		t.Run(name, func(t *testing.T) {
			status, _, body := do(t, srv, http.MethodPost, srv.TokenURL(), formType, "", form(a))
			if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
				t.Fatalf("got %d %v, want 400 invalid_grant", status, body)
			}
		})
	}

	forms := map[string]struct {
		contentType string
		body        []byte
	}{
		"json content type": {"application/json", form(valid)},
		"no content type":   {"", form(valid)},
		"wrong grant_type":  {formType, []byte(url.Values{"grant_type": {"client_credentials"}, "assertion": {valid}}.Encode())},
		"extra field":       {formType, []byte(url.Values{"grant_type": {fcmtest.GrantType}, "assertion": {valid}, "scope": {fcmtest.Scope}}.Encode())},
		"assertion twice":   {formType, []byte(url.Values{"grant_type": {fcmtest.GrantType}, "assertion": {valid, valid}}.Encode())},
		"no assertion":      {formType, []byte(url.Values{"grant_type": {fcmtest.GrantType}}.Encode())},
	}
	for name, f := range forms {
		t.Run(name, func(t *testing.T) {
			status, _, body := do(t, srv, http.MethodPost, srv.TokenURL(), f.contentType, "", f.body)
			if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
				t.Fatalf("got %d %v, want 400 invalid_grant", status, body)
			}
		})
	}
	if status, _, _ := do(t, srv, http.MethodGet, srv.TokenURL(), "", "", nil); status != http.StatusMethodNotAllowed {
		t.Fatalf("GET /token: %d, want 405", status)
	}
	if got, want := srv.TokenRequests(), 1+len(bad)+len(forms); got != want {
		t.Fatalf("TokenRequests = %d, want %d (every POST, refused ones included)", got, want)
	}

	// A key rotation: the old key is refused, the new one accepted.
	next := newKey(t)
	srv.SetPublicKey(&next.PublicKey)
	if status, _, _ := do(t, srv, http.MethodPost, srv.TokenURL(), formType, "", form(assertion(t, rs, good, claims(srv)))); status != http.StatusBadRequest {
		t.Fatalf("old key after SetPublicKey: %d, want 400", status)
	}
	token(t, srv, next)

	// Lifetime sets expires_in.
	short, skey := fake(t, 90*time.Second)
	a := assertion(t, jose.RS256Signer{Key: skey}, good, claims(short))
	if _, _, body := do(t, short, http.MethodPost, short.TokenURL(), formType, "", form(a)); body["expires_in"] != float64(90) {
		t.Fatalf("expires_in = %v, want 90", body["expires_in"])
	}
}

// TestFakeRequiresPostPathAndBearer: the send endpoint answers 405, 404, 401 and 400 before it
// records a request, and RevokeTokens revokes only the tokens issued so far.
func TestFakeRequiresPostPathAndBearer(t *testing.T) {
	t.Parallel()
	srv, key := fake(t, 0)
	tok := token(t, srv, key)

	if status, _, _ := do(t, srv, http.MethodGet, sendURL(srv), "", tok, nil); status != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d, want 405", status)
	}
	status, _, body := do(t, srv, http.MethodPost, srv.URL()+"/v1/projects/other-project/messages:send", "application/json", tok, message(sam))
	if code, rpc := rpcError(t, body); status != http.StatusNotFound || code != 404 || rpc != "NOT_FOUND" {
		t.Fatalf("other project: %d %v", status, body)
	}
	if e := body["error"].(map[string]any); e["details"] != nil || e["message"] != canary {
		t.Fatalf("404 must carry no FcmError and the ErrorMessage: %v", e)
	}
	for name, bearer := range map[string]string{"no bearer": "", "unknown bearer": "ya29.unknown"} {
		status, _, body := do(t, srv, http.MethodPost, sendURL(srv), "application/json", bearer, message(sam))
		if _, rpc := rpcError(t, body); status != http.StatusUnauthorized || rpc != "UNAUTHENTICATED" {
			t.Fatalf("%s: %d %v", name, status, body)
		}
	}
	for _, b := range []string{``, `not json`, `[1]`, `{}`, `{"message":null}`, `{"message":"x"}`, `{"message":[1]}`} {
		status, _, body := do(t, srv, http.MethodPost, sendURL(srv), "application/json", tok, []byte(b))
		if _, rpc := rpcError(t, body); status != http.StatusBadRequest || rpc != "INVALID_ARGUMENT" {
			t.Fatalf("body %q: %d %v", b, status, body)
		}
	}
	if n := len(srv.Requests()); n != 0 {
		t.Fatalf("refused requests were recorded: %d", n)
	}

	status, _, body = do(t, srv, http.MethodPost, sendURL(srv), "application/json", tok, message(sam))
	if name, _ := body["name"].(string); status != http.StatusOK || !strings.HasPrefix(name, "projects/"+project+"/messages/") {
		t.Fatalf("valid send: %d %v", status, body)
	}
	reqs := srv.Requests()
	if len(reqs) != 1 {
		t.Fatalf("Requests = %d, want 1", len(reqs))
	}
	r := reqs[0]
	if r.Proto != "HTTP/1.1" || r.Method != http.MethodPost || r.Path != "/v1/projects/"+project+"/messages:send" || r.Bearer != tok || !bytes.Equal(r.Body, message(sam)) || r.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("recorded request: %+v", r)
	}

	srv.RevokeTokens()
	status, _, body = do(t, srv, http.MethodPost, sendURL(srv), "application/json", tok, message(sam))
	if _, rpc := rpcError(t, body); status != http.StatusUnauthorized || rpc != "UNAUTHENTICATED" {
		t.Fatalf("revoked bearer: %d %v", status, body)
	}
	fresh := token(t, srv, key)
	if status, _, _ := do(t, srv, http.MethodPost, sendURL(srv), "application/json", fresh, message(sam)); status != http.StatusOK {
		t.Fatalf("token issued after RevokeTokens: %d, want 200", status)
	}
	if n := len(srv.Requests()); n != 2 {
		t.Fatalf("Requests = %d, want 2", n)
	}
}

// TestFakeScriptOrder: scripted send answers come in order per device token, then 200, with the
// error JSON of Design note 6; scripted token answers likewise.
func TestFakeScriptOrder(t *testing.T) {
	t.Parallel()
	srv, key := fake(t, 0)

	srv.ScriptToken(fcmtest.TokenResponse{Status: 400, Error: "invalid_grant"}, fcmtest.TokenResponse{Status: 503}, fcmtest.TokenResponse{Status: 200})
	valid := assertion(t, jose.RS256Signer{Key: key}, jose.Header{Typ: "JWT", Kid: kid}, claims(srv))
	// A bad assertion uses no scripted answer.
	if status, _, _ := do(t, srv, http.MethodPost, srv.TokenURL(), formType, "", form("bad")); status != http.StatusBadRequest {
		t.Fatalf("bad assertion: %d", status)
	}
	for i, want := range []struct {
		status int
		err    string
	}{{400, "invalid_grant"}, {503, ""}, {200, ""}, {200, ""}} {
		status, _, body := do(t, srv, http.MethodPost, srv.TokenURL(), formType, "", form(valid))
		if status != want.status {
			t.Fatalf("token answer %d: %d %v, want %d", i, status, body, want.status)
		}
		if want.status == 200 {
			if tok, _ := body["access_token"].(string); tok == "" {
				t.Fatalf("token answer %d: no access_token: %v", i, body)
			}
		} else if e, ok := body["error"].(string); !ok || e != want.err {
			t.Fatalf("token answer %d: %v, want error %q", i, body, want.err)
		}
	}
	if n := srv.TokenRequests(); n != 5 {
		t.Fatalf("TokenRequests = %d, want 5", n)
	}

	tok := token(t, srv, key)
	srv.Script(sam,
		fcmtest.Response{Status: 503, RPCStatus: "UNAVAILABLE", RetryAfter: "30"},
		fcmtest.Response{Status: 404, RPCStatus: "NOT_FOUND", ErrorCode: "UNREGISTERED"},
		fcmtest.Response{Status: 400, RPCStatus: "INVALID_ARGUMENT", ErrorCode: "INVALID_ARGUMENT", TokenField: true},
		fcmtest.Response{Status: 200, RetryAfter: "Wed, 21 Oct 2026 07:28:00 GMT"},
	)
	type detail = map[string]any
	fcmErr := func(code string) detail {
		return detail{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": code}
	}
	tokenViolation := detail{"@type": "type.googleapis.com/google.rpc.BadRequest", "fieldViolations": []any{
		detail{"field": "message.token", "description": "The registration token is not a valid FCM registration token"},
	}}
	for i, want := range []struct {
		status     int
		rpc        string
		details    []any
		retryAfter string
	}{
		{503, "UNAVAILABLE", nil, "30"},
		{404, "NOT_FOUND", []any{fcmErr("UNREGISTERED")}, ""},
		{400, "INVALID_ARGUMENT", []any{fcmErr("INVALID_ARGUMENT"), tokenViolation}, ""},
		{200, "", nil, "Wed, 21 Oct 2026 07:28:00 GMT"},
		{200, "", nil, ""},
	} {
		// Robin's token has no script and is not affected by Sam's.
		if status, _, _ := do(t, srv, http.MethodPost, sendURL(srv), "application/json", tok, message(robin)); status != http.StatusOK {
			t.Fatalf("robin before answer %d: %d", i, status)
		}
		status, header, body := do(t, srv, http.MethodPost, sendURL(srv), "application/json", tok, message(sam))
		if status != want.status || header.Get("Retry-After") != want.retryAfter {
			t.Fatalf("answer %d: %d Retry-After %q, want %d %q", i, status, header.Get("Retry-After"), want.status, want.retryAfter)
		}
		if want.status == 200 {
			if _, ok := body["name"].(string); !ok || body["error"] != nil {
				t.Fatalf("answer %d: %v", i, body)
			}
			continue
		}
		wantBody := map[string]any{"error": map[string]any{"code": float64(want.status), "status": want.rpc, "message": canary}}
		if want.details != nil {
			wantBody["error"].(map[string]any)["details"] = want.details
		}
		got, _ := json.Marshal(body)
		exp, _ := json.Marshal(wantBody)
		if !bytes.Equal(got, exp) {
			t.Fatalf("answer %d body:\n got %s\nwant %s", i, got, exp)
		}
	}
	if n := len(srv.Requests()); n != 10 {
		t.Fatalf("Requests = %d, want 10", n)
	}
}

// TestFakeNotInBinary: the server binary never depends on Fake-FCM.
func TestFakeNotInBinary(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../../../../cmd/bothy").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "/internal/kernel/jose") {
		t.Fatalf("go list output does not look like the dependencies of cmd/bothy:\n%.400s", out)
	}
	if strings.Contains(string(out), "/internal/notify/adapters/fcm/fcmtest") {
		t.Fatal("cmd/bothy depends on internal/notify/adapters/fcm/fcmtest")
	}
}
