// SPDX-License-Identifier: Apache-2.0

package canary

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

// The APNs configuration of the scenario server (13 17): the fixture team and key of Fake-APNs
// and the one topic the push step registers for.
const (
	apnsTeamID  = "EXAMPLETEAM"
	apnsKeyID   = "EXAMPLEKEY"
	apnsTopic   = "com.example.chat"
	apnsKeyFile = "apns/key.p8"
)

// PushRunOptions enables the push provider apns on the scenario server over o: the keys
// push.apns.*, a generated P-256 key as the secret file apns/key.p8 and the APNs adapter pointed at
// a Fake-APNs that lives until the test ends (harness.Options.APNsFake). The scenario server runs
// in role all, which builds the adapter of every enabled provider; a real FCM adapter would send
// to Google, and the harness has a fake only for APNs, so the push step registers with apns and
// leaves fcm disabled (its registration answers 403 push_provider_disabled).
func PushRunOptions(t testing.TB, o RunOptions) RunOptions {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	fake, err := apnstest.Start(apnstest.Options{TeamID: apnsTeamID, KeyID: apnsKeyID, PublicKey: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fake.Close)
	cfg := map[string]string{}
	for k, v := range o.Config {
		cfg[k] = v
	}
	cfg["push.apns.team_id"], cfg["push.apns.key_id"], cfg["push.apns.topics"] = apnsTeamID, apnsKeyID, apnsTopic
	cfg["push.apns.private_key_ref"] = "file:<secrets.dir>/" + apnsKeyFile
	o.Config = cfg
	prev := o.Harness
	o.Harness = func(h *harness.Options) {
		if prev != nil {
			prev(h)
		}
		h.APNsFake = fake
		files := map[string][]byte{}
		for k, v := range h.SecretFiles {
			files[k] = v
		}
		files[apnsKeyFile] = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		h.SecretFiles = files
	}
	return o
}

// push drives the push registration and the preview endpoint (M3-22, brief design note 15):
// Alex pairs a phone (platform ios), registers it with provider fcm and a device token that
// carries a canary (403 push_provider_disabled), then with apns and a random device token
// (registered as a secret; an apns token is hexadecimal, so it cannot carry a cnry word), replays
// the registration with the same Idempotency-Key (409 one_time_secret), fetches a preview with
// the issued preview token and a DPoP proof of the phone's key, reads listMyDevices, sends
// "Authorization: DPoP bothy_pvw_<canary>" to the preview endpoint (401), unregisters and fetches
// once more with the now worthless token (401). The preview token is a secret: the registerPush
// answer that issues it and the getPushPreview requests that carry it are the only places it may
// be, and no sink captures success bodies or requests.
func (sc *Scenario) push() {
	sc.T.Helper()
	alex := sc.W.People["Alex"]
	phone := sc.device("alex push phone")
	reg := phone.Registration(sc.Reg.New(ClassContent, "", "device name (push phone)"))
	reg["platform"] = "ios"
	b := obj(sc.anon(phone, req{method: http.MethodPost, path: "/auth/pairing/begin", body: map[string]any{"device": reg}}, created...))
	sc.post(alex, "/me/devices/pairings", map[string]any{"pairing_code": str(b, "pairing_code")}, created...)
	if !sc.adopt(phone, sc.anon(phone, req{method: http.MethodPost, path: "/auth/pairing/complete", body: map[string]any{"pairing_id": str(b, "pairing_id")}}, created...)) {
		return
	}
	sc.approve(phone)

	sc.post(phone, "/me/devices/current/push-registrations", map[string]any{
		"provider": "fcm", "token": "fcm:" + sc.Reg.New(ClassSecret, "", "push device token (fcm)"), "app_id": apnsTopic,
	}, http.StatusForbidden)
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		sc.T.Fatal(err)
	}
	token := hex.EncodeToString(raw)
	sc.Reg.Register(token, ClassSecret, "", "push device token (apns)")
	key := header("Idempotency-Key", newID())
	body := map[string]any{"provider": "apns", "token": token, "app_id": apnsTopic}
	issued := obj(sc.call(phone, req{method: http.MethodPost, path: "/me/devices/current/push-registrations", body: body, header: key}, http.StatusCreated))
	regID, preview := str(issued, "id"), str(issued, "preview_token")
	if regID == "" || preview == "" {
		sc.T.Errorf("registerPush answered no id or preview token")
		return
	}
	sc.Reg.Register(preview, ClassSecret, "", "preview token")
	sc.call(phone, req{method: http.MethodPost, path: "/me/devices/current/push-registrations", body: body, header: key}, http.StatusConflict)

	msg := sc.W.Messages["Support/Allgemein"][0]
	q := "/push/preview?" + url.Values{"nid": {ids.New().String()}, "message_id": {msg}, "kind": {"message"}}.Encode()
	sc.preview(phone, preview, q, http.StatusOK)
	sc.get(phone, "/me/devices", ok...)
	sc.preview(phone, "bothy_pvw_"+sc.Reg.New(ClassSecret, "", "forged preview token"), q, http.StatusUnauthorized)
	sc.del(phone, "/push-registrations/"+regID, http.StatusNoContent)
	sc.preview(phone, preview, q, http.StatusUnauthorized)
}

// preview sends getPushPreview with path (query included) as the device c: "Authorization: DPoP
// <token>" and a DPoP proof of c's key bound to token (ath, RFC 9449), once more with the server's
// nonce after use_dpop_nonce. Every proof is registered as a secret. It reports a final status
// other than want.
func (sc *Scenario) preview(c *harness.Client, token, path string, want int) {
	sc.T.Helper()
	htu := sc.Server.APIBase + "/push/preview"
	nonce := ""
	for attempt := 0; attempt < 2; attempt++ {
		proof, err := previewProof(c.Key(), htu, nonce, token)
		if err != nil {
			sc.T.Errorf("preview proof: %v", err)
			return
		}
		sc.Reg.Register(proof, ClassSecret, "", "DPoP proof (preview)")
		r := sc.raw(http.MethodGet, path, header("Authorization", "DPoP "+token, "DPoP", proof), nil, "alex push phone")
		if n := r.Header.Get("DPoP-Nonce"); r.Status == http.StatusUnauthorized && str(obj(r), "code") == "use_dpop_nonce" && n != "" && nonce == "" {
			nonce = n
			continue
		}
		if r.Status != want {
			sc.T.Errorf("GET getPushPreview = %d %s, want %d", r.Status, clip(r.Body), want)
		}
		return
	}
	sc.T.Errorf("GET getPushPreview: use_dpop_nonce twice")
}

// previewProof returns a DPoP proof (RFC 9449) of the private JWK priv for GET htu with nonce
// (none when empty) and ath = base64url(SHA-256(token)), as the notification extension of a
// device sends it with the preview token.
func previewProof(priv json.RawMessage, htu, nonce, token string) (string, error) {
	var k struct {
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
		D   string `json:"d"`
	}
	if err := json.Unmarshal(priv, &k); err != nil {
		return "", err
	}
	d, err := base64.RawURLEncoding.DecodeString(k.D)
	if err != nil {
		return "", err
	}
	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
	if err != nil {
		return "", err
	}
	pub, err := json.Marshal(map[string]string{"kty": "EC", "crv": k.Crv, "x": k.X, "y": k.Y})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(token))
	claims := map[string]any{"jti": ids.New().String(), "htm": http.MethodGet, "htu": htu, "iat": json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		"ath": base64.RawURLEncoding.EncodeToString(sum[:])}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return jose.SignCompact(jose.ES256Signer{Key: key}, jose.Header{Typ: "dpop+jwt", JWK: pub}, payload)
}
