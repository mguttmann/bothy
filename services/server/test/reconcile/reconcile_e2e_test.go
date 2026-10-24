// SPDX-License-Identifier: Apache-2.0

package reconcile_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestReconcileApp_EndToEnd (BR-10 end to end, the plan row "canary", design note 14): the real app
// as a subprocess against a fake cost API serving the fixture pages and the real server behind a
// recording proxy. The fixture has two days; W1 maps to A1 and A2, W2 to A4, W3 has no entry; on
// the first day A1 and A2 have runs with different estimates, one run with estimate 0 and one in
// EUR, A4 has one run and A3 (personal) has a run no workspace reaches; the second day has an
// amount of W1 but no run. The shares are literals computed by hand from the fixture. Afterwards a
// random admin key appears in no table, no recorded server traffic, no server log and no output of
// the app, while the fake received it in every request (the positive control).
func TestReconcileApp_EndToEnd(t *testing.T) {
	t.Parallel()
	uv, err := exec.LookPath("uv")
	if err != nil {
		t.Fatalf("uv is required for the end-to-end test of the reconciliation app: %v", err)
	}
	e := newEnv(t)
	_, token := e.installApp()

	// The first day of the fixture: W1 = 600.005 + 400.005 cents = 10000100 micro units over A1
	// (2000000) and A2 (1000000); W2 = 250.5 cents = 2505000 for A4; W3 = 50 cents unmapped.
	r1 := e.run(e.a1(), usd(2_000_000))
	r2 := e.run(e.a1(), usd(0))
	r3 := e.run(e.a2(), usd(1_000_000))
	r4 := e.run(e.a2(), report{model: "model-large", amount: 500_000, currency: "EUR", tokens: 100})
	r5 := e.run(e.a4(), usd(1_500_000))
	r6 := e.run(e.a3(), usd(700_000))
	runs := []string{r1, r2, r3, r4, r5, r6}
	// All runs on one day, so a pass across midnight cannot split them: noon of the day of the first
	// booking (test setup, as superuser).
	day, _ := e.day(r1)
	if _, err := e.db.Exec(context.Background(), `UPDATE cost_entries SET occurred_at = $1 WHERE run_id = ANY ($2::uuid[])`,
		day.Add(12*time.Hour), runs); err != nil {
		t.Fatal(err)
	}

	cost := newCostAPI(t, day.Sub(fixtureDay))
	proxy := newRecordingProxy(t, e.s.PublicURL)
	dir := t.TempDir()
	canary := "AdminKeyCanary" + randomAlnum(t, 40)
	writeSecret(t, filepath.Join(dir, "token"), token)
	writeSecret(t, filepath.Join(dir, "admin_key"), canary)
	config := filepath.Join(dir, "config.toml")
	cfg := `[reconciler]
server_url = "` + proxy.srv.URL + `"
cost_api_url = "` + cost.srv.URL + `"
token_ref = "file:` + filepath.Join(dir, "token") + `"
admin_key_ref = "file:` + filepath.Join(dir, "admin_key") + `"
min_request_interval = "10ms"
lookback = "3d"

[reconciler.workspace_map]
wrkspc_01e2ew1 = ["` + e.lotse.PrincipalID + `", "` + e.texter.PrincipalID + `"]
wrkspc_01e2ew2 = ["` + e.runner.PrincipalID + `"]
`
	if err := os.WriteFile(config, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	// --quiet keeps uv's own warnings (for example about VIRTUAL_ENV or hardlinks) off stderr,
	// which must carry only the app's JSON log lines.
	cmd := exec.CommandContext(ctx, uv, "--quiet", "run", "--directory", filepath.Join(repoRoot(t), "plugins", "cost-reconciler"), "--locked", "python", "-m", //nolint:gosec // G204: uv from PATH and paths of the test
		"cost_reconciler", "--config", config, "--once")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("the app ended with %d:\n%s", exit.ExitCode(), redact(stderr.String(), canary))
		}
		t.Fatal(err)
	}
	e.settle()

	// The shares: W1 10000100 over 2000000 and 1000000 is 6666733 and 3333366, the remainder 1 on
	// the larger estimate; W2 gets 2505000 whole. The zero, EUR and personal runs stay estimates.
	want := map[string]int64{r1: 6_666_734, r3: 3_333_366, r5: 2_505_000}
	for run, amount := range want {
		v := valid(e.entries(run))
		if len(v) != 1 || v[0].Source != "reconciled" || v[0].Estimate || v[0].Amount != amount {
			t.Fatalf("run %s: valid entries %+v, want one reconciled of %d", run, v, amount)
		}
		if c := e.runCostOf(run); c.Micros != amount || c.Estimate {
			t.Fatalf("run %s totals %+v", run, c)
		}
	}
	if want[r1]+want[r3] != 10_000_100 {
		t.Fatal("the shares of W1 do not sum to the fixture amount")
	}
	for _, run := range []string{r2, r4, r6} {
		for _, x := range e.entries(run) {
			if x.Source == "reconciled" || !x.Estimate {
				t.Fatalf("run %s was reconciled: %+v", run, x)
			}
		}
	}
	lines := logLines(t, stderr.String())
	if !hasLine(lines, map[string]string{"event": "not_assigned", "workspace_id": "wrkspc_01e2ew3", "reason": "no_mapping"}) ||
		!hasLine(lines, map[string]string{"event": "not_assigned", "workspace_id": "wrkspc_01e2ew1", "reason": "no_runs",
			"bucket_start": day.Add(-24 * time.Hour).Format("2006-01-02T15:04:05.000Z")}) {
		t.Fatalf("the not_assigned lines are missing:\n%s", redact(stderr.String(), canary))
	}

	// The canary: nowhere on the server's side, nowhere in the app's output.
	wantScanFinds(t, e.db, r1)
	if found := scanDatabase(t, e.db, canary); len(found) > 0 {
		t.Fatalf("the admin key is stored in %v", found)
	}
	traffic := proxy.got.all()
	if len(traffic) == 0 {
		t.Fatal("the proxy saw no request of the app")
	}
	for _, x := range traffic {
		if strings.Contains(x.Method+x.Path+x.Query+headerText(x.Header)+string(x.Body)+headerText(x.RespHeader)+string(x.RespBody), canary) {
			t.Fatalf("the admin key is in the server traffic of %s %s", x.Method, x.Path)
		}
	}
	for name, text := range map[string]string{"server log": e.logs.String(), "stdout": stdout.String(), "stderr": stderr.String()} {
		if strings.Contains(text, canary) {
			t.Fatalf("the admin key is in the %s", name)
		}
	}
	// Positive control: the fake received the key in the authentication header of every request.
	asked := cost.got.all()
	if len(asked) != 2 {
		t.Fatalf("the cost API got %d requests, want the two pages", len(asked))
	}
	for _, x := range asked {
		if x.Header.Get("x-api-key") != canary {
			t.Fatal("a request to the cost API lacks the admin key")
		}
	}
}

// writeSecret writes value to path with mode 0600.
func writeSecret(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

// redact replaces secret in s, so a failure message never prints it.
func redact(s, secret string) string { return strings.ReplaceAll(s, secret, "[canary]") }

// headerText renders h as text for the scan.
func headerText(h map[string][]string) string {
	var b strings.Builder
	for k, vs := range h {
		b.WriteString(k)
		for _, v := range vs {
			b.WriteString(v)
		}
	}
	return b.String()
}

// logLines decodes the JSON lines of the app's stderr.
func logLines(t *testing.T, s string) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("a log line of the app is no JSON object: %v", err)
		}
		out = append(out, m)
	}
	return out
}

// hasLine reports a line whose members include want.
func hasLine(lines []map[string]any, want map[string]string) bool {
	for _, l := range lines {
		ok := true
		for k, v := range want {
			if s, _ := l[k].(string); s != v {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
