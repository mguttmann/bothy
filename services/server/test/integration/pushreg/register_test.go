// SPDX-License-Identifier: Apache-2.0

package pushreg_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestRegisterPush_CreatesRowAndReturnsTokenOnce: 13 6.4 and 13 13, brief M3-22 design notes 3
// and 5.
func TestRegisterPush_CreatesRowAndReturnsTokenOnce(t *testing.T) {
	w := newWorld(t, nil)
	tok := strings.ToUpper(apnsToken())
	key := http.Header{"Idempotency-Key": {ids.New().String()}}
	r := w.registerRaw(w.alex, map[string]any{"provider": "apns", "token": tok, "app_id": apnsTopic}, key)
	wantStatus(t, "registerPush", r, http.StatusCreated, "")
	var reg registered
	r.JSON(t, &reg)
	if reg.Environment != "production" || reg.Provider != "apns" || reg.AppID != apnsTopic || reg.DeviceID != w.alex.DeviceID {
		t.Errorf("answer %+v", reg)
	}
	if !strings.HasPrefix(reg.PreviewToken, "bothy_pvw_") {
		t.Errorf("preview token of length %d is not of kind pvw", len(reg.PreviewToken))
	}
	noToken(t, "the answer", r.Body, tok, strings.ToLower(tok))
	got, ok := w.row(reg.ID)
	if !ok {
		t.Fatal("no row")
	}
	if got.UserID != w.alex.PrincipalID || got.DeviceID != w.alex.DeviceID || got.SessionID != w.alex.SessionID || got.Disabled {
		t.Errorf("row binds user %v device %v session %v, disabled %v", got.UserID == w.alex.PrincipalID, got.DeviceID == w.alex.DeviceID,
			got.SessionID == w.alex.SessionID, got.Disabled)
	}
	if got.Token != strings.ToLower(tok) {
		t.Error("the apns token is not stored in lower case")
	}
	if !hmac.Equal(got.Hash, previewHash(t, w, reg.PreviewToken)) {
		t.Error("preview_token_hash is not the HMAC of the issued token")
	}

	replay := w.registerRaw(w.alex, map[string]any{"provider": "apns", "token": tok, "app_id": apnsTopic}, key)
	wantStatus(t, "replay", replay, http.StatusConflict, "conflict")
	if !strings.Contains(string(replay.Body), "one_time_secret") || replay.Header.Get("Idempotent-Replay") != "true" {
		t.Errorf("replay %s, Idempotent-Replay %q", replay.Body, replay.Header.Get("Idempotent-Replay"))
	}
	if n := w.count(`SELECT count(*) FROM push_registrations WHERE user_id = $1`, w.alex.PrincipalID); n != 1 {
		t.Errorf("%d rows after the replay, want 1", n)
	}
	var stored string
	if err := w.s.AdminDB().QueryRow(t.Context(), `SELECT coalesce(string_agg(coalesce(response_body::text, ''), ' '), '')
		FROM idempotency_keys WHERE principal_id = $1`, w.alex.PrincipalID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	noToken(t, "the idempotency rows", []byte(stored), reg.PreviewToken)
	ok200 := w.fetch(w.alex, reg.PreviewToken, w.post(w.alex, w.allgemeinMain, "hello"))
	wantStatus(t, "preview with the issued token", ok200, http.StatusOK, "")
}

// previewHash returns the HMAC-SHA-256 of token with the token pepper of the server's secrets
// directory (auth.token_pepper_ref, the derivation of api_tokens.token_hash).
func previewHash(t *testing.T, w *world, token string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(w.s.SecretsDir, "token_pepper"))
	if err != nil {
		t.Fatal(err)
	}
	// bothy secrets init writes the 32 bytes of the pepper as hex (internal/wire TokenPepper).
	pepper, err := hex.DecodeString(string(bytes.TrimSpace(raw)))
	if err != nil {
		t.Fatal(err)
	}
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(token))
	return m.Sum(nil)
}

// TestRegisterPush_RotatesPreviewToken: 13 6.4 "is rotated with every new registration", design
// note 5 d.
func TestRegisterPush_RotatesPreviewToken(t *testing.T) {
	w := newWorld(t, nil)
	msg := w.post(w.alex, w.allgemeinMain, "hello")
	first := w.register(w.alex, "apns", apnsToken(), apnsTopic)
	second := w.register(w.alex, "apns", apnsToken(), apnsTopic)
	if first.PreviewToken == second.PreviewToken {
		t.Fatal("the preview token was not rotated")
	}
	old, _ := w.row(first.ID)
	if !old.Disabled || old.DisabledReason != "token_invalid" {
		t.Errorf("old row disabled %v reason %q, want token_invalid", old.Disabled, old.DisabledReason)
	}
	wantStatus(t, "old token", w.fetch(w.alex, first.PreviewToken, msg), http.StatusUnauthorized, "unauthenticated")
	wantStatus(t, "new token", w.fetch(w.alex, second.PreviewToken, msg), http.StatusOK, "")
}

// TestRegisterPush_TakeoverOfKnownToken: 13 13, design note 5 c.
func TestRegisterPush_TakeoverOfKnownToken(t *testing.T) {
	w := newWorld(t, nil)
	w.android(w.alex)
	w.android(w.sam)
	x := fcmToken()
	alexReg := w.register(w.alex, "fcm", x, fcmApp)
	samReg := w.register(w.sam, "fcm", x, fcmApp)
	if n := w.count(`SELECT count(*) FROM push_registrations WHERE provider = 'fcm' AND token = $1 AND user_id = $2`, x, w.sam.PrincipalID); n != 1 {
		t.Fatalf("%d rows of the token for Sam, want 1", n)
	}
	if _, ok := w.row(alexReg.ID); ok {
		t.Error("Alex's row still exists after the takeover")
	}
	msg := w.post(w.alex, w.allgemeinMain, "hello")
	wantStatus(t, "Alex's old token", w.fetch(w.alex, alexReg.PreviewToken, msg), http.StatusUnauthorized, "unauthenticated")
	wantStatus(t, "Sam's token", w.fetch(w.sam, samReg.PreviewToken, msg), http.StatusOK, "")
}

// TestRegisterPush_ConcurrentSameToken: design note 5 g.
func TestRegisterPush_ConcurrentSameToken(t *testing.T) {
	var logs bytes.Buffer
	sw := &syncWriter{w: &logs}
	w := newWorld(t, nil, func(o *harness.Options) { o.LogWriter = sw })
	w.android(w.alex)
	w.android(w.sam)
	x := fcmToken()
	var wg sync.WaitGroup
	resps := make([]harness.Response, 2)
	for i, c := range []*harness.Client{w.alex, w.sam} {
		wg.Go(func() {
			resps[i] = c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/me/devices/current/push-registrations",
				Body: map[string]any{"provider": "fcm", "token": x, "app_id": fcmApp}})
		})
	}
	wg.Wait()
	msg := w.post(w.alex, w.allgemeinMain, "hello")
	working := 0
	for i, r := range resps {
		switch r.Status {
		case http.StatusCreated:
			var reg registered
			r.JSON(t, &reg)
			signer := []*harness.Client{w.alex, w.sam}[i]
			if w.fetch(signer, reg.PreviewToken, msg).Status == http.StatusOK {
				working++
			}
		case http.StatusConflict:
			if code(r.Body) != "conflict" {
				t.Errorf("409 with %s", r.Body)
			}
		default:
			sw.mu.Lock()
			for _, line := range strings.Split(logs.String(), "\n") {
				if strings.Contains(line, `"level":"ERROR"`) || strings.Contains(line, `"level":"WARN"`) {
					t.Log(line)
				}
			}
			sw.mu.Unlock()
			t.Fatalf("registration %d = %d %s", i, r.Status, r.Body)
		}
	}
	if n := w.count(`SELECT count(*) FROM push_registrations WHERE provider = 'fcm' AND token = $1`, x); n != 1 {
		t.Errorf("%d rows of the token, want 1", n)
	}
	if working != 1 {
		t.Errorf("%d preview tokens work, want exactly 1", working)
	}
}

// TestRegisterPush_Rules: one request per rule of design note 4 through HTTP; nothing is written.
func TestRegisterPush_Rules(t *testing.T) {
	w := newWorld(t, nil)
	long := strings.Repeat("a", 1025)
	for _, tc := range []struct {
		name, provider, token, env, app, pointer, code string
		android                                        bool
	}{
		{name: "wns", provider: "wns", token: "EXAMPLEtoken", app: "com.example.chat", pointer: "/provider", code: "not_enabled"},
		{name: "unifiedpush", provider: "unifiedpush", token: "EXAMPLEtoken", app: "com.example.chat", pointer: "/provider", code: "not_enabled"},
		{name: "fcm from macos", provider: "fcm", token: "EXAMPLEtoken", app: fcmApp, pointer: "/provider", code: "invalid"},
		{name: "apns from android", provider: "apns", token: apnsToken(), app: apnsTopic, pointer: "/provider", code: "invalid", android: true},
		{name: "fcm sandbox", provider: "fcm", token: "EXAMPLEtoken", env: "sandbox", app: fcmApp, pointer: "/environment", code: "invalid", android: true},
		{name: "apns not hex", provider: "apns", token: "g" + apnsToken()[1:], app: apnsTopic, pointer: "/token", code: "pattern_mismatch"},
		{name: "apns odd", provider: "apns", token: apnsToken()[1:], app: apnsTopic, pointer: "/token", code: "pattern_mismatch"},
		{name: "fcm space", provider: "fcm", token: "EXAMPLE token", app: fcmApp, pointer: "/token", code: "pattern_mismatch", android: true},
		{name: "token too long", provider: "fcm", token: long, app: fcmApp, pointer: "/token", code: "too_long", android: true},
		{name: "app too long", provider: "fcm", token: "EXAMPLEtoken", app: long, pointer: "/app_id", code: "too_long", android: true},
		{name: "foreign topic", provider: "apns", token: apnsToken(), app: "com.example.other", pointer: "/app_id", code: "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			platform := "macos"
			if tc.android {
				platform = "android"
			}
			if _, err := w.s.AdminDB().Exec(t.Context(), `UPDATE devices SET platform = $2 WHERE id = $1`, w.alex.DeviceID, platform); err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"provider": tc.provider, "token": tc.token, "app_id": tc.app}
			if tc.env != "" {
				body["environment"] = tc.env
			}
			r := w.registerRaw(w.alex, body, nil)
			wantStatus(t, "registerPush", r, http.StatusBadRequest, "validation_failed")
			if !strings.Contains(string(r.Body), `"`+tc.pointer+`"`) || !strings.Contains(string(r.Body), `"`+tc.code+`"`) {
				t.Errorf("answer %s, want %s at %s", r.Body, tc.code, tc.pointer)
			}
		})
	}
	if n := w.count(`SELECT count(*) FROM push_registrations`); n != 0 {
		t.Errorf("%d rows written, want 0", n)
	}

	cfg := pushConfig()
	cfg["push.apns.team_id"], cfg["push.apns.key_id"], cfg["push.apns.topics"], cfg["push.apns.private_key_ref"] = "", "", "", ""
	off := newWorld(t, cfg)
	r := off.registerRaw(off.alex, map[string]any{"provider": "apns", "token": apnsToken(), "app_id": apnsTopic}, nil)
	wantStatus(t, "apns not enabled", r, http.StatusForbidden, "feature_disabled")
	if !strings.Contains(string(r.Body), "push_provider_disabled") {
		t.Errorf("answer %s, want reason push_provider_disabled", r.Body)
	}
	if n := off.count(`SELECT count(*) FROM push_registrations`); n != 0 {
		t.Errorf("%d rows written, want 0", n)
	}
}

// TestRegisterPush_NeedsSessionAndActiveDevice: design note 6; an agent token has no session.
func TestRegisterPush_NeedsSessionAndActiveDevice(t *testing.T) {
	w := newWorld(t, nil)
	agent := w.s.NewAgent(t, harness.AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	r := agent.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/me/devices/current/push-registrations",
		Body: map[string]any{"provider": "apns", "token": apnsToken(), "app_id": apnsTopic}})
	if r.Status != http.StatusForbidden {
		t.Fatalf("agent token = %d %s, want 403", r.Status, r.Body)
	}
	if n := w.count(`SELECT count(*) FROM push_registrations`); n != 0 {
		t.Errorf("%d rows written, want 0", n)
	}
}

// TestUnregisterPush: design note 5 i; foreign and unknown give the same 404.
func TestUnregisterPush(t *testing.T) {
	w := newWorld(t, nil)
	msg := w.post(w.alex, w.allgemeinMain, "hello")
	reg := w.register(w.alex, "apns", apnsToken(), apnsTopic)
	foreign := w.sam.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: "/push-registrations/" + reg.ID})
	unknown := w.sam.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: "/push-registrations/" + ids.New().String()})
	wantStatus(t, "foreign", foreign, http.StatusNotFound, "not_found")
	wantStatus(t, "unknown", unknown, http.StatusNotFound, "not_found")
	if a, b := normalized(t, foreign), normalized(t, unknown); a != b {
		t.Errorf("foreign and unknown differ:\n%s\n%s", a, b)
	}
	if _, ok := w.row(reg.ID); !ok {
		t.Fatal("Sam removed Alex's registration")
	}
	del := w.alex.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: "/push-registrations/" + reg.ID})
	wantStatus(t, "unregister", del, http.StatusNoContent, "")
	if _, ok := w.row(reg.ID); ok {
		t.Error("the row still exists")
	}
	wantStatus(t, "old preview token", w.fetch(w.alex, reg.PreviewToken, msg), http.StatusUnauthorized, "unauthenticated")
	again := w.alex.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: "/push-registrations/" + reg.ID})
	wantStatus(t, "second unregister", again, http.StatusNotFound, "not_found")
	// Without credentials the answer is 401 for an existing and for an unknown registration alike.
	kept := w.register(w.alex, "apns", apnsToken(), apnsTopic)
	for _, id := range []string{kept.ID, ids.New().String()} {
		anon := w.alex.DoAnonymous(t.Context(), harness.Request{Method: http.MethodDelete, Path: "/push-registrations/" + id})
		wantStatus(t, "anonymous unregister", anon, http.StatusUnauthorized, "")
	}
	if _, ok := w.row(kept.ID); !ok {
		t.Error("an anonymous request removed a registration")
	}
}

// TestDevice_PushRegistrationsMember: design note 11.
func TestDevice_PushRegistrationsMember(t *testing.T) {
	w := newWorld(t, nil)
	tok := apnsToken()
	first := w.register(w.alex, "apns", tok, apnsTopic)
	second := w.register(w.alex, "apns", apnsToken(), apnsTopic)
	r := harness.MustDo(t, w.alex, harness.Request{Method: http.MethodGet, Path: "/me/devices"}, http.StatusOK)
	noToken(t, "listMyDevices", r.Body, tok, first.PreviewToken, second.PreviewToken)
	var list struct {
		Items []struct {
			ID                string `json:"id"`
			PushRegistrations []struct {
				ID             string `json:"id"`
				DisabledAt     string `json:"disabled_at"`
				DisabledReason string `json:"disabled_reason"`
			} `json:"push_registrations"`
		} `json:"items"`
	}
	r.JSON(t, &list)
	found := false
	for _, d := range list.Items {
		if d.ID != w.alex.DeviceID {
			continue
		}
		found = true
		if len(d.PushRegistrations) != 2 || d.PushRegistrations[0].ID != first.ID || d.PushRegistrations[1].ID != second.ID {
			t.Fatalf("registrations %+v", d.PushRegistrations)
		}
		if d.PushRegistrations[0].DisabledAt == "" || d.PushRegistrations[0].DisabledReason != "token_invalid" || d.PushRegistrations[1].DisabledAt != "" {
			t.Errorf("disabled members %+v", d.PushRegistrations)
		}
	}
	if !found {
		t.Fatal("listMyDevices lacks the device")
	}
	g := harness.MustDo(t, w.alex, harness.Request{Method: http.MethodGet, Path: "/devices/" + w.alex.DeviceID}, http.StatusOK)
	if !strings.Contains(string(g.Body), first.ID) || !strings.Contains(string(g.Body), second.ID) {
		t.Errorf("getDevice lacks the registrations: %s", g.Body)
	}
	s := harness.MustDo(t, w.sam, harness.Request{Method: http.MethodGet, Path: "/devices/" + w.sam.DeviceID}, http.StatusOK)
	if strings.Contains(string(s.Body), "push_registrations") {
		t.Errorf("a device without registration has the member: %s", s.Body)
	}
	other := w.sam.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/devices/" + w.alex.DeviceID})
	wantStatus(t, "another person's device", other, http.StatusNotFound, "")
}
