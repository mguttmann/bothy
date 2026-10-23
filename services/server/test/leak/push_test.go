// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The push routes of AZ-P4 (brief M3-22 design note 15): unregisterPush is a DELETE and
// getPushPreview carries its message ID in the query, so SelectRoutes (GET with a uuid path
// parameter) does not select them; TestAZ_P4_RouteCoverage compares them with pushCases.

// pushTopic is the only APNs topic of the P4 server and the app_id of its registrations.
const pushTopic = "com.example.chat"

// pushForeignChannel is the channel of the matrix world whose message Sam previews: Sam reads
// Marketing (G-2) and Support/Allgemein (G-4 through Leser), not Engineering (05 message.read).
const pushForeignChannel = "Engineering/Werkzeuge"

// withPush enables the provider apns on the P4 server (push.apns.*, so registerPush answers 201).
// The server runs in role all, which builds the real APNs adapter: its endpoints point at a
// Fake-APNs with a fresh P-256 key, and EgressDenyAll of the P4 server still refuses every
// outgoing request, so no request leaves the test.
func withPush(t *testing.T, opts harness.Options) harness.Options {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	fake, err := apnstest.Start(apnstest.Options{TeamID: "EXAMPLETEAM", KeyID: "EXAMPLEKEY", PublicKey: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fake.Close)
	opts.Config["push.apns.team_id"], opts.Config["push.apns.key_id"], opts.Config["push.apns.topics"] = "EXAMPLETEAM", "EXAMPLEKEY", pushTopic
	opts.Config["push.apns.private_key_ref"] = "file:<secrets.dir>/apns/key.p8"
	opts.APNsFake = fake
	opts.SecretFiles = map[string][]byte{"apns/key.p8": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})}
	return opts
}

// pushCase is one comparison of a push route: send answers request A (pair.a, the foreign ID)
// and request B (pair.b, a fresh random UUIDv7).
type pushCase struct {
	operationID, requester, params string
	pair                           pair
	send                           func(t *testing.T, target string) harness.Response
}

// pushCases registers an APNs target for Alex and for Sam (registerPush) and returns the two
// comparisons of design note 15: Sam unregistering Alex's registration against a random
// registration ID, and Sam's valid preview token for a message of pushForeignChannel against a
// random message ID. Both are content answers: 404 not_found (05 push.unregister, message.read).
func (p *p4World) pushCases(t *testing.T) []pushCase {
	t.Helper()
	alex, sam := p.w.People["Alex"], p.w.People["Sam"]
	alexReg, _ := p.register(t, alex)
	_, samToken := p.register(t, sam)
	randomReg := ids.New().String()
	message, randomMessage := p.w.Messages[pushForeignChannel][0], ids.New().String()
	nid := ids.New().String()
	query := func(m string) string {
		return "/push/preview?" + url.Values{"nid": {nid}, "message_id": {m}, "kind": {"mention"}}.Encode()
	}
	var nonce string
	return []pushCase{
		{
			operationID: "unregisterPush", requester: "Sam", params: "registration_id of Alex",
			pair: pair{a: "/push-registrations/" + alexReg, b: "/push-registrations/" + randomReg,
				foreign: []string{alexReg}, random: []string{randomReg}, content: true},
			send: func(t *testing.T, target string) harness.Response {
				t.Helper()
				r := sam.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: target})
				if r.Status == 0 {
					t.Fatalf("DELETE %s: %s", target, r.Body)
				}
				return r
			},
		},
		{
			operationID: "getPushPreview", requester: "Sam", params: "message_id (query) with Sam's preview token",
			pair: pair{a: query(message), b: query(randomMessage), foreign: []string{message}, random: []string{randomMessage}, content: true},
			send: func(t *testing.T, target string) harness.Response {
				t.Helper()
				return p.preview(t, sam, samToken, target, &nonce)
			},
		},
	}
}

// register registers a fresh APNs token of c's device (platform macos) and returns the
// registration ID and its preview token.
func (p *p4World) register(t *testing.T, c *harness.Client) (id, previewToken string) {
	t.Helper()
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		t.Fatal(err)
	}
	var out struct {
		ID           string `json:"id"`
		PreviewToken string `json:"preview_token"`
	}
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/me/devices/current/push-registrations", Body: map[string]any{
		"provider": "apns", "token": hex.EncodeToString(tok), "app_id": pushTopic,
	}}, http.StatusCreated).JSON(t, &out)
	if out.ID == "" || out.PreviewToken == "" {
		t.Fatal("leak: registerPush answered without id or preview_token")
	}
	return out.ID, out.PreviewToken
}

// preview sends getPushPreview for target (path and query relative to the API base) with
// "Authorization: DPoP <token>" and a proof of the device key of c bound to token (ath). It sends
// the last nonce first and retries once with the nonce of a use_dpop_nonce answer.
func (p *p4World) preview(t *testing.T, c *harness.Client, token, target string, nonce *string) harness.Response {
	t.Helper()
	r := p.previewOnce(t, c, token, target, nonce)
	if r.Status == http.StatusUnauthorized && problemCode(r.Body) == "use_dpop_nonce" {
		r = p.previewOnce(t, c, token, target, nonce)
	}
	return r
}

// previewOnce sends one getPushPreview request and keeps the DPoP-Nonce of the answer in nonce.
func (p *p4World) previewOnce(t *testing.T, c *harness.Client, token, target string, nonce *string) harness.Response {
	t.Helper()
	u, err := url.Parse(p.s.APIBase + target)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	htu := *u
	htu.RawQuery = ""
	req.Header.Set("Authorization", "DPoP "+token)
	req.Header.Set("DPoP", previewProof(t, c, http.MethodGet, htu.String(), *nonce, token))
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		*nonce = n
	}
	return harness.Response{Status: resp.StatusCode, Header: resp.Header, Body: body, Elapsed: time.Since(start)}
}

// previewProof returns a DPoP proof (RFC 9449) of the device key of c for htm and htu with nonce
// (left out when "") and ath = base64url(SHA-256(token)), the binding getPushPreview checks.
func previewProof(t *testing.T, c *harness.Client, htm, htu, nonce, token string) string {
	t.Helper()
	var jwk struct {
		D string `json:"d"`
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
	sum := sha256.Sum256([]byte(token))
	claims := map[string]any{"jti": ids.New().String(), "htm": htm, "htu": htu, "iat": json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		"ath": base64.RawURLEncoding.EncodeToString(sum[:])}
	if nonce != "" {
		claims["nonce"] = nonce
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
