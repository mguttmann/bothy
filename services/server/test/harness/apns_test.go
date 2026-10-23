// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestStartWithAPNsFake (brief M3-24 design note 3 a): with Options.APNsFake and the keys
// push.apns.*, the wired APNs adapter of the server reaches Fake-APNs. Alex mentions Sam in a
// channel both can read; Sam holds an apns registration and the level mentions; exactly one
// request reaches the Fake over HTTP/2, a mention of that message with a badge.
func TestStartWithAPNsFake(t *testing.T) {
	t.Parallel()
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
	cfg := RaisedRateLimits()
	cfg["push.apns.team_id"], cfg["push.apns.key_id"], cfg["push.apns.topics"] = "EXAMPLETEAM", "EXAMPLEKEY", "com.example.chat"
	cfg["push.apns.private_key_ref"] = "file:<secrets.dir>/apns/key.p8"
	s := Start(t, Options{Config: cfg, APNsFake: fake,
		SecretFiles: map[string][]byte{"apns/key.p8": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})}})

	alex := s.NewUser(t, UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	sam := s.NewUser(t, UserSpec{Handle: "sam", DisplayName: "Sam", Email: "sam@example.com"})
	var space struct {
		ID string `json:"id"`
	}
	MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Team"}}, http.StatusCreated).JSON(t, &space)
	var ch struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces/" + space.ID + "/channels", Body: map[string]any{"name": "general", "kind": "text"}},
		http.StatusCreated).JSON(t, &ch)
	for who, level := range map[*Client]string{alex: "write", sam: "read"} {
		MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
			"subject_kind": "user", "subject_id": who.PrincipalID, "target_kind": "channel", "target_id": ch.ID, "level": level, "reason": "test",
		}}, http.StatusCreated)
	}
	MustDo(t, sam, Request{Method: http.MethodPut, Path: "/me/notification-preferences/channel/" + ch.ID, Body: map[string]any{"level": "mentions"}}, http.StatusOK)
	tok := make([]byte, 32)
	_, _ = rand.Read(tok)
	if _, err := s.AdminDB().Exec(t.Context(), `INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, environment, app_id)
		SELECT $1, $2, $3, (SELECT id FROM auth_sessions WHERE device_id = $3 ORDER BY created_at DESC, id DESC LIMIT 1), 'apns', $4, 'production', 'com.example.chat'`,
		ids.New().String(), sam.PrincipalID, sam.DeviceID, hex.EncodeToString(tok)); err != nil {
		t.Fatal(err)
	}

	var msg struct {
		ID string `json:"id"`
	}
	MustDo(t, alex, Request{Method: http.MethodPost, Path: "/threads/" + ch.MainThreadID + "/messages", Body: map[string]any{
		"client_msg_id": ids.New().String(), "content_md": "@sam please look", "mention_ids": []string{sam.PrincipalID},
	}}, http.StatusCreated).JSON(t, &msg)
	testkit.Until(t, "a request at Fake-APNs", func() bool { return len(fake.Requests()) > 0 })
	reqs := fake.Requests()
	if len(reqs) != 1 {
		t.Fatalf("%d requests at Fake-APNs, want 1", len(reqs))
	}
	r := reqs[0]
	var body struct {
		APS struct {
			Badge *int64 `json:"badge"`
		} `json:"aps"`
		B struct {
			K string `json:"k"`
			M string `json:"m"`
			T string `json:"t"`
		} `json:"b"`
	}
	if err := json.Unmarshal(r.Body, &body); err != nil {
		t.Fatal(err)
	}
	if r.Proto != "HTTP/2.0" || r.Token != hex.EncodeToString(tok) || r.AcceptedAt.IsZero() {
		t.Errorf("request proto %q, token of Sam %v, accepted %v", r.Proto, r.Token == hex.EncodeToString(tok), !r.AcceptedAt.IsZero())
	}
	if body.B.K != "mention" || body.B.M != msg.ID || body.B.T != ch.MainThreadID || body.APS.Badge == nil || *body.APS.Badge < 1 {
		t.Errorf("payload k %q m %q (want %q) t %q (want %q) badge %v", body.B.K, body.B.M, msg.ID, body.B.T, ch.MainThreadID, body.APS.Badge)
	}
}
