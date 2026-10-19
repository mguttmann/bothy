// SPDX-License-Identifier: Apache-2.0

package pairing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/fingerprint"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/state"
)

const canaryCode = "bothy_hpc_EXAMPLECANARY000000000000000000000000000000_EXAMPL"

type server struct {
	mu     sync.Mutex
	bodies [][]byte
	answer func(req map[string]any) (int, any)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.bodies = append(s.bodies, b)
	s.mu.Unlock()
	var req map[string]any
	_ = json.Unmarshal(b, &req)
	if r.URL.Path != "/api/v1/hosts/pair" || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	status, body := s.answer(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// accept answers 201 with the fingerprint of the received keys.
func accept(req map[string]any) (int, any) {
	n, _ := base64.RawURLEncoding.DecodeString(req["noise_public_key"].(string))
	jwk := req["auth_public_jwk"].(map[string]any)
	a, _ := base64.RawURLEncoding.DecodeString(jwk["x"].(string))
	fp, _ := fingerprint.Compute(n, a)
	return http.StatusCreated, map[string]string{"host_id": "0192f4a0-7b1c-7a11-8c3e-4f5a6b7c8d9e", "key_fingerprint": fp, "instance_id": "0192f4a0-7b1c-7000-8a00-000000000001"}
}

func setup(t *testing.T, answer func(map[string]any) (int, any)) (*server, Options) {
	t.Helper()
	s := &server{answer: answer}
	ts := httptest.NewTLSServer(s)
	t.Cleanup(ts.Close)
	st := state.Open(filepath.Join(t.TempDir(), "state"), state.Plain{})
	if err := st.Ensure(); err != nil {
		t.Fatal(err)
	}
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	return s, Options{ServerURL: ts.URL, Code: canaryCode, CAPEM: ca, State: st, Platform: "linux", HostVersion: "0.0.0-dev", Random: rand.Reader,
		Lists: Lists{Profiles: []doctor.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}}, SessionUsers: []doctor.ReportedSessionUser{{OSUser: "bothy-s-alex", Workdirs: []string{}}}}}
}

// listsAfterHD7 runs doctor over a world where one folder is under A and B: it drops from both.
func listsAfterHD7(t *testing.T) Lists {
	t.Helper()
	w := hostostest.New("linux")
	w.AddUser("bothy-host", "999", "/var/lib/bothy-host", false)
	w.AddUser("bothy-s-alex", "1001", "/home/bothy-s-alex", true)
	w.AddUser("bothy-s-sam", "1002", "/home/bothy-s-sam", true)
	w.Dir("/srv/shared", "0", 0o755).ID = "dev:1"
	w.Dir("/home/bothy-s-alex/own", "1001", 0o700)
	cfg := config.Defaults(config.Platform{GOOS: "linux"})
	cfg.SessionUsers = []string{"bothy-s-alex", "bothy-s-sam"}
	cfg.ClaudeWorkdirs = []config.WorkdirList{
		{SessionUser: "bothy-s-alex", Dirs: []config.Workdir{{DisplayName: "Own", Path: "/home/bothy-s-alex/own"}, {DisplayName: "Shared", Path: "/srv/shared"}}},
		{SessionUser: "bothy-s-sam", Dirs: []config.Workdir{{DisplayName: "Shared", Path: "/srv/shared"}}},
	}
	h := ids{"bothy-s-alex": {"dev:2", "dev:1"}, "bothy-s-sam": {"dev:1"}}
	r := doctor.RunService(context.Background(), doctor.Env{GOOS: "linux", Config: &cfg, ServiceUser: "bothy-host", Users: w.Users(), Security: w.Security(),
		Resolver: w.As("bothy-host"), Files: w.As("bothy-host"), Probe: w.As("bothy-host").Probe(), Helpers: h, PeerCheckProven: true, Declaration: state.DeclPersonal})
	return Lists{Profiles: r.Profiles, SessionUsers: r.SessionUsers}
}

type ids map[string][]string

func (i ids) Check(_ context.Context, u string, _ doctor.Request) (doctor.HelperResult, error) {
	return doctor.HelperResult{WorkdirIDs: i[u]}, nil
}
func (ids) Scan(context.Context, string, doctor.Request) (string, error) { return "", nil }

func TestPairRequestBody(t *testing.T) {
	s, o := setup(t, accept)
	o.Lists = listsAfterHD7(t)
	if _, err := Pair(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	if err := json.Unmarshal(s.bodies[0], &req); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"pairing_code": true, "noise_public_key": true, "auth_public_jwk": true, "platform": true, "os_version": true, "host_version": true, "profiles": true, "session_users": true}
	for k := range req {
		if !allowed[k] {
			t.Errorf("member %q", k)
		}
	}
	jwk := req["auth_public_jwk"].(map[string]any)
	if len(jwk) != 3 || jwk["kty"] != "OKP" || jwk["crv"] != "Ed25519" || jwk["d"] != nil {
		t.Errorf("jwk %v", jwk)
	}
	if strings.Contains(string(s.bodies[0]), "/") {
		t.Errorf("a path in the request: %s", s.bodies[0])
	}
	users := req["session_users"].([]any)
	alex := users[0].(map[string]any)["workdirs"].([]any)
	sam := users[1].(map[string]any)["workdirs"].([]any)
	if len(alex) != 1 || alex[0] != "Own" || len(sam) != 0 {
		t.Errorf("lists after HD7: %v %v", alex, sam)
	}
}

func TestPairRequiresHTTPS(t *testing.T) {
	_, o := setup(t, accept)
	for _, u := range []string{"http://chat.example.com", "chat.example.com", "https://", "https://user@chat.example.com", "://x"} {
		o.ServerURL = u
		if _, err := Pair(context.Background(), o); !errors.Is(err, ErrNotHTTPS) || err.Error() != "pair: the server URL must use https" {
			t.Errorf("%s: %v", u, err)
		}
	}
}

func TestPairStoresKeysOnlyAfterAcceptance(t *testing.T) {
	_, o := setup(t, func(map[string]any) (int, any) {
		return http.StatusBadRequest, map[string]any{"code": "validation_failed", "errors": []map[string]string{{"code": "too_long", "pointer": "/session_users/0/os_user"}}}
	})
	if _, err := Pair(context.Background(), o); err == nil {
		t.Fatal("refusal accepted")
	}
	if _, err := os.Stat(o.State.File(state.KeysFile)); !errors.Is(err, os.ErrNotExist) {
		t.Error("keys stored after a refusal")
	}
	_, o = setup(t, accept)
	res, err := Pair(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	k, err := o.State.ReadKeys()
	if err != nil {
		t.Fatal(err)
	}
	fp, _ := fingerprint.Compute(noisePublic(t, k), k.AuthPrivate.Public().(ed25519.PublicKey))
	p, _ := o.State.ReadPairing()
	if res.Fingerprint != fp || p.Fingerprint != fp || p.HostID == "" || !strings.HasPrefix(p.ServerURL, "https://") {
		t.Errorf("stored %+v", p)
	}
	fi, _ := os.Stat(o.State.File(state.KeysFile))
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
}

func noisePublic(t *testing.T, k state.Keys) []byte {
	t.Helper()
	kp, err := noiseFromPrivate(k.NoisePrivate)
	if err != nil {
		t.Fatal(err)
	}
	return kp
}

func TestPairFingerprintMismatch(t *testing.T) {
	_, o := setup(t, func(req map[string]any) (int, any) {
		return http.StatusCreated, map[string]string{"host_id": "h", "key_fingerprint": "0000-0000", "instance_id": "i"}
	})
	if _, err := Pair(context.Background(), o); !errors.Is(err, ErrFingerprintMismatch) {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(o.State.File(state.KeysFile)); !errors.Is(err, os.ErrNotExist) {
		t.Error("keys stored after a mismatch")
	}
	_, o = setup(t, func(map[string]any) (int, any) { return http.StatusCreated, "x" })
	if _, err := Pair(context.Background(), o); err == nil {
		t.Error("unreadable answer")
	}
}

func TestPairServerRefusal(t *testing.T) {
	_, o := setup(t, func(map[string]any) (int, any) {
		return http.StatusBadRequest, map[string]any{"code": "validation_failed", "errors": []map[string]string{{"code": "duplicate", "pointer": "/session_users/1/os_user"}}}
	})
	_, err := Pair(context.Background(), o)
	var re *RefusedError
	if !errors.As(err, &re) || err.Error() != "pair: server refused: validation_failed\n  duplicate at /session_users/1/os_user" {
		t.Errorf("%v", err)
	}
	_, o = setup(t, func(map[string]any) (int, any) {
		return http.StatusUnauthorized, map[string]any{"code": "unauthenticated"}
	})
	if _, err := Pair(context.Background(), o); err == nil || err.Error() != "pair: server refused: unauthenticated\npair: ask an owner for a new pairing code" {
		t.Errorf("%v", err)
	}
	_, o = setup(t, func(map[string]any) (int, any) {
		return http.StatusConflict, map[string]any{"code": "conflict", "reason": "host_key_in_use"}
	})
	if _, err := Pair(context.Background(), o); err == nil || err.Error() != "pair: server refused: conflict (host_key_in_use)" {
		t.Errorf("%v", err)
	}
	_, o = setup(t, func(map[string]any) (int, any) { return http.StatusBadGateway, "x" })
	if _, err := Pair(context.Background(), o); err == nil || !strings.Contains(err.Error(), "http_502") {
		t.Errorf("%v", err)
	}
	// Unreachable and an untrusted certificate.
	_, o = setup(t, accept)
	o.CAPEM = nil
	if _, err := Pair(context.Background(), o); err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("untrusted: %v", err)
	}
	o.CAPEM = []byte("not pem")
	if _, err := Pair(context.Background(), o); !errors.Is(err, ErrCAFile) {
		t.Errorf("CA: %v", err)
	}
}

func TestPairNeverPrintsCodeOrKeys(t *testing.T) {
	var outputs []string
	for _, answer := range []func(map[string]any) (int, any){
		accept,
		func(map[string]any) (int, any) {
			return http.StatusUnauthorized, map[string]any{"code": "unauthenticated"}
		},
		func(map[string]any) (int, any) {
			return http.StatusCreated, map[string]string{"host_id": "h", "key_fingerprint": "X"}
		},
	} {
		_, o := setup(t, answer)
		res, err := Pair(context.Background(), o)
		outputs = append(outputs, res.Fingerprint+res.HostID)
		if err != nil {
			outputs = append(outputs, err.Error())
		}
		if k, err := o.State.ReadKeys(); err == nil {
			for _, out := range outputs {
				if strings.Contains(out, base64.RawURLEncoding.EncodeToString(k.NoisePrivate[:])) || bytes.Contains([]byte(out), k.AuthPrivate.Seed()) {
					t.Error("a private key in the output")
				}
			}
		}
	}
	for _, out := range outputs {
		if strings.Contains(out, "CANARY") || strings.Contains(out, "bothy_hpc_") {
			t.Errorf("the code in %q", out)
		}
	}
	if b, err := ReadCA(""); b != nil || err != nil {
		t.Error("empty CA path")
	}
	if _, err := ReadCA(filepath.Join(t.TempDir(), "none.pem")); err == nil {
		t.Error("missing CA file")
	}
}

// TestPairStoresServerCA (M2-43 red team): the CA of --ca-file is stored with the pairing, so the
// control connection of run trusts the same server; a pairing without one removes an old CA.
func TestPairStoresServerCA(t *testing.T) {
	_, o := setup(t, accept)
	if _, err := Pair(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	got, err := o.State.ReadServerCA()
	if err != nil || string(got) != string(o.CAPEM) {
		t.Fatalf("stored CA %q %v", got, err)
	}
	if err := o.State.WriteServerCA(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := o.State.ReadServerCA(); err != nil || got != nil {
		t.Fatalf("removed CA %q %v", got, err)
	}
}
