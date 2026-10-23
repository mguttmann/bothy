// SPDX-License-Identifier: Apache-2.0

// Package terminal holds the real-server tests of the terminal against the host service binary
// (component testsupport): bothy-host from services/host pairs itself with the HTTP chain of the
// terminal test world through a TLS front (M2-42).
package terminal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestMain migrates the pgtest templates like bothy migrate up (harness.Main): the full-server
// harness of the M2-46 tests needs the instance row the migration steps write.
func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// buildHost builds bothy-host from the host module next to this one.
func buildHost(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	module := filepath.Join(filepath.Dir(file), "..", "..", "..", "host")
	bin := filepath.Join(t.TempDir(), "bothy-host")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", bin, "./cmd/bothy-host") //nolint:gosec // the test builds the host service
	cmd.Dir = module                                                                      // the environment of the test (GOTOOLCHAIN, GOCACHE) carries over
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build bothy-host: %v\n%s", err, out)
	}
	return bin
}

// capture is a TLS front on 127.0.0.1 that records the request bodies of pairHost.
type capture struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (c *capture) front(t *testing.T, target string) *httptest.Server {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(u) }}
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.bodies = append(c.bodies, b)
		c.mu.Unlock()
		r.Body = io.NopCloser(bytes.NewReader(b))
		proxy.ServeHTTP(w, r)
	}))
}

func (c *capture) all() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(bytes.Join(c.bodies, []byte("\n")))
}

var fingerprintForm = regexp.MustCompile(`(?m)^fingerprint: ([0-9A-Z]{4}-){12}[0-9A-Z]{4}$`)

// TestHostPairAgainstServer (M2-42, TM-T19 host part): Robin, an owner, creates a host with
// step-up and issues a pairing code; bothy-host pair redeems it in the direct path (the test user
// owns the state directory) through a TLS front whose certificate goes to --ca-file. The host is
// confirming with the same fingerprint and the configured session users; the keys lie only in the
// state directory with mode 0600; a second pair with the same code is refused with 401 and keeps
// the keys.
func TestHostPairAgainstServer(t *testing.T) {
	bin := buildHost(t)
	w := terminalworld.NewEmpty(t)
	w.Robin = w.Person("robin", true)
	w.SQL(`INSERT INTO instance (instance_id, schema_version, sync_epoch) VALUES ($1, 1, $2) ON CONFLICT DO NOTHING`, ids.New().String(), ids.New().String())
	created := w.Want(w.Do("robin", http.MethodPost, terminalworld.HostsPath, `{"name":"Build server","reason":"pair test"}`, terminalworld.Key()...),
		http.StatusCreated, "")
	// createHost answers with a first code; issueHostPairingCode replaces it.
	host := struct{ ID string }{ID: created["host_id"].(string)}
	code := struct{ PairingCode string }{PairingCode: w.Want(w.Do("robin", http.MethodPost,
		terminalworld.HostPath(ids.MustParse(host.ID))+"/pairing-code", `{"reason":"pair test"}`, terminalworld.Key()...),
		http.StatusCreated, "")["pairing_code"].(string)}

	var cap capture
	front := cap.front(t, w.H.Server.URL)
	t.Cleanup(front.Close)
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: front.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte("host_service:\n  state_dir: "+stateDir+"\n  session_users: [bothy-s-alex, bothy-s-robin]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pair := func() (string, string, int) {
		cmd := exec.CommandContext(t.Context(), bin, "pair", "--server", front.URL, "--code", code.PairingCode, "--ca-file", caFile, "--config", cfg) //nolint:gosec // the built binary
		cmd.Env = []string{"GOTOOLCHAIN=local"}
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		exit := 0
		if ee, ok := err.(*exec.ExitError); ok { //nolint:errorlint // the exit status of the child
			exit = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return stdout.String(), stderr.String(), exit
	}
	stdout, stderr, exit := pair()
	if exit != 0 || !fingerprintForm.MatchString(stdout) || !strings.Contains(stdout, "host: "+host.ID) {
		t.Fatalf("pair: exit %d\n%s\n%s", exit, stdout, stderr)
	}
	fp := strings.TrimPrefix(fingerprintForm.FindString(stdout), "fingerprint: ")

	h := w.Want(w.Do("robin", http.MethodGet, terminalworld.HostPath(ids.MustParse(host.ID)), ""), http.StatusOK, "")
	var got struct {
		State                string `json:"state"`
		KeyFingerprint       string `json:"key_fingerprint"`
		ReportedSessionUsers []struct {
			OSUser string `json:"os_user"`
		} `json:"reported_session_users"`
	}
	raw, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, u := range got.ReportedSessionUsers {
		names = append(names, u.OSUser)
	}
	if got.State != "confirming" || got.KeyFingerprint != fp || strings.Join(names, ",") != "bothy-s-alex,bothy-s-robin" {
		t.Errorf("getHost: %+v (fingerprint %s)", got, fp)
	}

	keyFile := filepath.Join(stateDir, "keys")
	fi, err := os.Stat(keyFile)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("key file: %v %v", fi, err)
	}
	keys, err := os.ReadFile(keyFile) //nolint:gosec // the state directory of this test
	if err != nil {
		t.Fatal(err)
	}
	var k struct {
		Noise string `json:"noise_private"`
		Auth  string `json:"auth_seed"`
	}
	if err := jsonUnmarshal(keys, &k); err != nil || k.Noise == "" || k.Auth == "" {
		t.Fatalf("keys: %v", err)
	}
	seen := cap.all() + stdout + stderr
	for _, secret := range []string{k.Noise, k.Auth, stateDir, base64.StdEncoding.EncodeToString([]byte(k.Auth))} {
		if strings.Contains(seen, secret) {
			t.Error("a private key or the state path left the host")
		}
	}
	if strings.Contains(stdout+stderr, code.PairingCode) {
		t.Error("the pairing code in the output")
	}

	stdout, stderr, exit = pair()
	if exit != 1 || !strings.Contains(stderr, "pair: server refused: unauthenticated") || !strings.Contains(stderr, "ask an owner for a new pairing code") {
		t.Errorf("second pair: exit %d\n%s\n%s", exit, stdout, stderr)
	}
	again, _ := os.ReadFile(keyFile) //nolint:gosec // the state directory of this test
	if !bytes.Equal(again, keys) {
		t.Error("the keys of the first pairing changed")
	}
	if strings.Contains(stdout+stderr, code.PairingCode) {
		t.Error("the pairing code in the output")
	}
}
