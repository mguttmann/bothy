// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestMain migrates the pgtest templates like bothy migrate up. Without a Docker-compatible
// runtime the integration tests fail; they never skip (plan P-09).
func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// repoRoot returns the repository root above the test's working directory.
func repoRoot(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := RepoRoot(wd)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestGuide runs docs/guides/connect-agent.md against a fresh server: exit criterion 7 of M2.
func TestGuide(t *testing.T) {
	root := repoRoot(t)
	page, in, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(page, in); err != nil {
		t.Fatal(err)
	}
	env := StartEnv(t, root)
	began := time.Now()
	res, err := Run(t, env, page)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("guide: %d blocks in %d sections ran in %s with %d adapter processes", res.Blocks, len(res.Steps), time.Since(began).Round(time.Second), res.Adapters)
}

// TestGuide_DeviationFailsAtStep1 changes the answer of step 1 in the text: the run fails at
// that block with the JSON Pointer of the difference, and nothing after it runs.
func TestGuide_DeviationFailsAtStep1(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(root + "/" + PagePath) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		t.Fatal(err)
	}
	const original = `"billing_mode": "organization"`
	if strings.Count(string(raw), original) != 1 {
		t.Fatalf("the page holds %q %d times, want once (in step 1)", original, strings.Count(string(raw), original))
	}
	mutated := strings.Replace(string(raw), original, `"billing_mode": "personal"`, 1)
	page := mustParse(t, root, mutated)
	env := StartEnv(t, root)
	res, err := Run(t, env, page)
	if err == nil {
		t.Fatal("the changed page passed")
	}
	for _, want := range []string{"Step 1", `"Response 201"`, "/billing_mode"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want one naming %s", err, want)
		}
	}
	if res.Adapters != 0 {
		t.Fatalf("%d adapter processes started", res.Adapters)
	}
	if last := res.Last; last == nil || last.Step != 1 || last.OperationID != "createAgent" {
		t.Fatalf("the last block that ran is %v, want the createAgent of step 1", last)
	}
	var tokens int
	if err := env.Server.AdminDB().QueryRow(t.Context(), `SELECT count(*) FROM api_tokens`).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens != 0 {
		t.Fatalf("%d API tokens exist; step 2 ran after the failing block", tokens)
	}
}
