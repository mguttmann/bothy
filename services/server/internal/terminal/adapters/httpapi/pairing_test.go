// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestPairingHandlers (M2-41): the three handlers through the HTTP chain: issueHostPairingCode
// answers 201 with a code, pairHost 201 with the fingerprint, confirmHost a mismatch with
// attempts_remaining and then 200 with the host and its ETag; the answers validate against the
// contract (the harness validates every response).
func TestPairingHandlers(t *testing.T) {
	t.Parallel()
	w := terminalworld.NewEmpty(t)
	w.Robin = w.Person("robin", true)
	w.SQL(`INSERT INTO instance (instance_id, schema_version, sync_epoch) VALUES ($1, 1, $2) ON CONFLICT DO NOTHING`, ids.New().String(), ids.New().String())
	m := w.Want(w.Do("robin", http.MethodPost, terminalworld.HostsPath, `{"name":"Build server","reason":"New"}`, terminalworld.Key()...), http.StatusCreated, "")
	id := ids.MustParse(m["host_id"].(string))
	code := w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/pairing-code", `{"reason":"Again"}`, terminalworld.Key()...),
		http.StatusCreated, "")["pairing_code"].(string)
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding
	body := `{"pairing_code":"` + code + `","noise_public_key":"` + enc.EncodeToString(x.PublicKey().Bytes()) +
		`","auth_public_jwk":{"kty":"OKP","crv":"Ed25519","x":"` + enc.EncodeToString(pub) + `"},"platform":"linux","os_version":"Debian 13",` +
		`"host_version":"1.0.0","profiles":[{"name":"shell","display_name":"Shell","kind":"shell"}],` +
		`"session_users":[{"os_user":"bothy-s-robin","workdirs":["Project A"]}]}`
	fp, _ := domain.ComputeFingerprint(x.PublicKey().Bytes(), pub)
	if got := w.Want(w.Do("", http.MethodPost, terminalworld.HostsPath+"/pair", body), http.StatusCreated, ""); got["key_fingerprint"] != fp {
		t.Fatalf("pair %v", got)
	}
	prefix := strings.ToLower(domain.CompactFingerprint(fp)[:8])
	wrong := "zzzzzzzz"
	if prefix == wrong {
		wrong = "yyyyyyyy"
	}
	confirm := func(p string) *httpservertest.Response {
		return w.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/confirm", `{"fingerprint_prefix":"`+p+
			`","llm_credentials":"organization","declared_server_host":false,"session_users":[{"user_id":"`+w.Robin.String()+
			`","os_user":"bothy-s-robin"}],"reason":"Confirm"}`, w.Keyed(id)...)
	}
	if pr := w.Want(confirm(wrong), http.StatusConflict, "fingerprint_mismatch"); pr["attempts_remaining"] != float64(terminalworld.HostConfig.FingerprintMaxAttempts-1) {
		t.Fatalf("mismatch %v", pr)
	}
	r := confirm(prefix)
	if got := w.Want(r, http.StatusOK, ""); got["state"] != "active" || r.Header.Get("ETag") == "" {
		t.Fatalf("confirm %v", got)
	}
}
