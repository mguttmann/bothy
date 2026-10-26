// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/demo"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// demoAgentModule is the demo agent of the Python SDK (sdk/python/src/bothy_sdk/testing).
const demoAgentModule = "bothy_sdk.testing.demo_agent"

// TestDemoViews (brief M7-27 requirement 15, the integration test): bothy setup demo with an
// exchange directory on an empty instance, the demo agent of the Python SDK as the agents and the
// Fake-Host of the terminal tests as the demo host. After the command every view of 25 has data
// through its query (getStatsLive, getStatsTimeline, getStatsSlo, getStatsBackends), a watcher run
// is running with a live box, an approval card waits, the worker runs booked costs, and a
// terminal session opens on the demo host. The waits of the data set are shortened for the test;
// the command and the agents are the same code as in the stack.
func TestDemoViews(t *testing.T) {
	t.Parallel()
	ds := loadDataSet(t)
	ds.Values.WarmupPrompts = 6
	ds.Values.StepInterval = demo.Duration(20 * time.Millisecond)
	ds.Values.WatcherInterval = demo.Duration(200 * time.Millisecond)
	ds.Values.HeartbeatInterval = demo.Duration(time.Second)
	ds.Values.AgentWait = demo.Duration(testkit.Patience)
	ds.Values.HostWait = demo.Duration(testkit.Patience)
	conf := harness.RaisedRateLimits()
	conf["stats.flush_interval"] = "1s"
	s := harness.Start(t, harness.Options{EmptyInstance: true, Config: conf})
	exchange := filepath.Join(t.TempDir(), "exchange")
	if err := os.Mkdir(exchange, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx := testkit.Context(t)
	stack := &demoStack{t: t, s: s, dir: exchange}
	go stack.agents(ctx)
	go stack.host(ctx)

	var out bytes.Buffer
	res, err := demo.Run(ctx, demo.Options{DataSet: ds, Seed: 3, APIURL: s.ListenURL, PublicURL: s.PublicURL, Exchange: exchange, Pace: 0,
		FingerprintChars: 8, Bootstrap: bootstrapWith(s), Stdout: &out})
	if err != nil {
		t.Fatalf("demo.Run: %v (stack: %v)", err, stack.failure())
	}
	owner := signIn(t, s, strings.TrimSpace(out.String()))
	host := stack.connectHost()

	// Runs: every worker run of the warm-up completes with its costs, the watcher run is running
	// with a live box, the reviewer run waits with its approval card.
	testkit.Until(t, "the warm-up runs completed", func() bool { return completedRuns(t, s) >= ds.Values.WarmupPrompts })
	testkit.Until(t, "the costs of the warm-up booked", func() bool { return count(t, s, `SELECT count(*) FROM cost_entries`) >= ds.Values.WarmupPrompts })
	watch, release := res.LiveThreads["Watch the nightly build"], res.LiveThreads["Publish the 2.4 release notes"]
	testkit.Until(t, "the watcher run running with a live box", func() bool {
		return count(t, s, `SELECT count(*) FROM runs WHERE thread_id = $1 AND state = 'running' AND status_line <> ''`, watch) == 1
	})
	testkit.Until(t, "the reviewer run waiting for the approval", func() bool {
		return count(t, s, `SELECT count(*) FROM runs WHERE thread_id = $1 AND state = 'waiting_input'`, release) == 1
	})
	var msgs struct {
		Items []struct {
			Kind       string `json:"kind"`
			Components []any  `json:"components"`
		} `json:"items"`
	}
	harness.MustDo(t, owner, harness.Request{Method: http.MethodGet, Path: "/threads/" + release + "/messages"}, http.StatusOK).JSON(t, &msgs)
	cards := 0
	for _, m := range msgs.Items {
		if m.Kind == "card" && len(m.Components) > 0 {
			cards++
		}
	}
	if cards != 1 {
		t.Fatalf("approval cards in the release thread: %d, want 1", cards)
	}

	// The views of 25: each query has data.
	var live struct {
		Runs   []any `json:"runs"`
		Agents []any `json:"agents"`
	}
	harness.MustDo(t, owner, harness.Request{Method: http.MethodGet, Path: "/stats/live"}, http.StatusOK).JSON(t, &live)
	if len(live.Runs) == 0 || len(live.Agents) == 0 {
		t.Fatalf("getStatsLive: %d runs, %d agents", len(live.Runs), len(live.Agents))
	}
	now := time.Now().UTC()
	from, to := now.Truncate(time.Hour).Add(-time.Hour), now.Truncate(time.Minute).Add(2*time.Minute)
	q := url.Values{"from": {stamp(from)}, "to": {stamp(to)}}
	testkit.Until(t, "the timeline with buckets and runs", func() bool {
		var tl struct {
			Buckets []any `json:"buckets"`
			Runs    []any `json:"runs"`
		}
		harness.MustDo(t, owner, harness.Request{Method: http.MethodGet, Path: "/stats/timeline?" + q.Encode()}, http.StatusOK).JSON(t, &tl)
		return len(tl.Buckets) > 0 && len(tl.Runs) >= ds.Values.WarmupPrompts
	})
	var backends struct {
		Items []any `json:"items"`
	}
	harness.MustDo(t, owner, harness.Request{Method: http.MethodGet, Path: "/stats/backends?" + q.Encode()}, http.StatusOK).JSON(t, &backends)
	if len(backends.Items) == 0 {
		t.Fatal("getStatsBackends: no agent")
	}
	var slo struct {
		Items []struct {
			Total int64 `json:"total"`
		} `json:"items"`
	}
	harness.MustDo(t, owner, harness.Request{Method: http.MethodGet, Path: "/stats/slo"}, http.StatusOK).JSON(t, &slo)
	measured := 0
	for _, it := range slo.Items {
		if it.Total > 0 {
			measured++
		}
	}
	if len(slo.Items) == 0 || measured == 0 {
		t.Fatalf("getStatsSlo: %d items, %d with events", len(slo.Items), measured)
	}

	// The terminal: the person with a session user opens a session on the demo host.
	alex := personSession(t, s, owner, res.People["alex"])
	if err := alex.StepUp(ctx); err != nil {
		t.Fatal(err)
	}
	opened := alex.Do(ctx, harness.Request{Method: http.MethodPost, Path: "/hosts/" + res.HostID + "/terminal/sessions",
		Body: map[string]any{"profile": "shell", "cols": 120, "rows": 40, "client_ephemeral_key_hash": cek}})
	if opened.Status != http.StatusCreated {
		t.Fatalf("openTerminalSession: %d %s", opened.Status, opened.Body)
	}
	testkit.Until(t, "the session.open at the demo host", func() bool { return len(host.Opens()) == 1 })
}

// cek is a client_ephemeral_key_hash of the right form (the test opens no Noise stream).
var cek = base64.RawURLEncoding.EncodeToString(make([]byte, 32))

// stamp formats t as a request timestamp.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// completedRuns counts the completed runs of s.
func completedRuns(t *testing.T, s *harness.Server) int {
	t.Helper()
	return count(t, s, `SELECT count(*) FROM runs WHERE state = 'completed'`)
}

// count returns the integer result of query on s (assertions only).
func count(t *testing.T, s *harness.Server, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.AdminDB().QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// personSession signs the person userID in on a new device with an enrollment code the owner
// creates (the sessions of the command live only in its memory).
func personSession(t *testing.T, s *harness.Server, owner *harness.Client, userID string) *harness.Client {
	t.Helper()
	var code struct {
		Code string `json:"code"`
	}
	harness.MustDo(t, owner, harness.Request{Method: http.MethodPost, Path: "/users/" + userID + "/enrollment-codes"}, http.StatusCreated).JSON(t, &code)
	return signIn(t, s, code.Code)
}

// demoStack plays the stack of the overlay for the test: it starts the demo agent when
// agents.json appears and pairs the Fake-Host when host-pairing.json appears.
type demoStack struct {
	t   *testing.T
	s   *harness.Server
	dir string

	mu      sync.Mutex
	err     error
	hostKey hosttest.Key
	hostID  ids.ID
	report  map[string]any
}

// fail records the first failure of the stack.
func (d *demoStack) fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err == nil {
		d.err = err
	}
}

// failure returns the first failure of the stack.
func (d *demoStack) failure() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// await waits until name exists in the exchange directory or ctx ends.
func (d *demoStack) await(ctx context.Context, name string) bool {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(filepath.Join(d.dir, name)); err == nil {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
	}
}

// agents starts the demo agent of the Python SDK on the exchange directory; it ends with the test.
func (d *demoStack) agents(ctx context.Context) {
	if !d.await(ctx, demo.AgentsFile) {
		return
	}
	uv, err := exec.LookPath("uv")
	if err != nil {
		d.fail(errors.New("the demo test needs uv on PATH to run the demo agent of sdk/python"))
		return
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		d.fail(err)
		return
	}
	sdk := filepath.Join(root, "sdk", "python")
	cmd := exec.CommandContext(context.WithoutCancel(ctx), uv, "run", "--project", sdk, "--locked", "--quiet", "python", "-m", demoAgentModule, // #nosec G204 -- uv from PATH with constant arguments and repository paths
		"--exchange", d.dir, "--base-url", d.s.APIBase)
	cmd.Env = append(cmd.Environ(), "PYTHONPATH="+filepath.Join(sdk, "src"))
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stderr, &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		d.fail(err)
		return
	}
	d.t.Cleanup(func() {
		// uv runs Python as its child: stop the whole group of the process this test started.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		_ = cmd.Wait()
		if d.t.Failed() {
			d.t.Logf("demo agent output:\n%s", stderr.String())
		}
	})
}

// host pairs the Fake-Host with the code of host-pairing.json (pairHost, as bothy-host pair
// does), writes host-paired.json with the fingerprint and connects the host with a report of the
// session users of the data set.
func (d *demoStack) host(ctx context.Context) {
	if !d.await(ctx, demo.HostPairingFile) {
		return
	}
	b, err := os.ReadFile(filepath.Join(d.dir, demo.HostPairingFile))
	if err != nil {
		d.fail(err)
		return
	}
	var doc demo.HostPairingDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		d.fail(err)
		return
	}
	key, noise := fakehost.NewKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.JWK(), &jwk); err != nil {
		d.fail(err)
		return
	}
	users := make([]any, 0, len(doc.SessionUsers))
	for _, u := range doc.SessionUsers {
		users = append(users, map[string]any{"os_user": u, "workdirs": []string{"Demo project"}})
	}
	profiles := []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}
	resp := d.s.NewDevice().DoAnonymous(ctx, harness.Request{Method: http.MethodPost, Path: "/hosts/pair", Body: map[string]any{
		"pairing_code": doc.PairingCode, "noise_public_key": fakehost.B64(noise), "auth_public_jwk": jwk, "platform": "linux",
		"host_version": "1.0.0", "profiles": profiles, "session_users": users,
	}})
	if resp.Status != http.StatusCreated {
		d.fail(fmt.Errorf("pairHost: %d %s", resp.Status, resp.Body))
		return
	}
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	if err := json.Unmarshal(resp.Body, &paired); err != nil {
		d.fail(err)
		return
	}
	pb, err := json.Marshal(demo.HostPairedDoc{Fingerprint: paired.KeyFingerprint})
	if err != nil {
		d.fail(err)
		return
	}
	if err := os.WriteFile(filepath.Join(d.dir, demo.HostPairedFile), pb, 0o600); err != nil {
		d.fail(err)
		return
	}
	d.mu.Lock()
	d.hostKey, d.hostID, d.report = key, ids.MustParse(doc.HostID), hostReport(profiles, users)
	d.mu.Unlock()
}

// hostReport is a report of the Fake-Host whose doctor checks all pass.
func hostReport(profiles, users []any) map[string]any {
	var doctor []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": c, "ok": true})
	}
	return map[string]any{"profiles": profiles, "session_users": users, "doctor": doctor, "is_server_host": false,
		"require_local_confirm": false, "os_version": "Debian 13"}
}

// connectHost connects the paired Fake-Host (after the command confirmed it) and returns it.
func (d *demoStack) connectHost() *fakehost.Host {
	d.mu.Lock()
	key, id, report := d.hostKey, d.hostID, d.report
	d.mu.Unlock()
	return fakehost.Connect(d.t, d.s.PublicURL, id, key, report)
}
