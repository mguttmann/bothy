// SPDX-License-Identifier: Apache-2.0

//go:build contract_live

package contract

import (
	"context"
	"crypto/rand"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestG7_SchemathesisAllEnabledOperations runs Schemathesis (tools/contract/live_contract.py)
// against the harness server with the matrix world: every enabled operation in an owner and a
// plain user pass, the stateful phase, SEC-T06 and the project checks. Egress is denied, and the
// rate limits are raised (23 R-QA-11: G7 checks conformance, M1-19 owns throttling). It fails when
// pytest exits non-zero; the output streams to the test log.
func TestG7_SchemathesisAllEnabledOperations(t *testing.T) {
	cfg := harness.RaisedRateLimits()
	// Every positive case of an upload operation opens a session (live_contract.py
	// _prepare_upload); the default of 20 open sessions per principal would turn later cases into
	// a 429 at preparation and leave them on the 404 of an unknown upload.
	cfg["file.upload_max_open_sessions"] = "100000"
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: cfg})
	w := harness.BuildMatrixWorld(t, s)
	b, err := StartBroker(s, w.People["Alex"].PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	root := repoRoot(t)
	seed := os.Getenv("CONTRACT_SEED") //nolint:forbidigo // test input from make contract-test, not server configuration
	if seed == "" {
		n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
		if err != nil {
			t.Fatal(err)
		}
		seed = n.String()
	}
	examples := os.Getenv("CONTRACT_MAX_EXAMPLES") //nolint:forbidigo // test input from make contract-test
	if examples == "" {
		examples = "100"
	}
	t.Logf("CONTRACT_SEED=%s CONTRACT_MAX_EXAMPLES=%s", seed, examples)
	args := []string{"run", "--project", "tools", "--locked", "pytest", "-p", "no:cacheprovider", "tools/contract/live_contract.py"}
	// CONTRACT_PYTEST_K narrows a reproduction to the pytest expression, for example "getMe".
	if extra := os.Getenv("CONTRACT_PYTEST_K"); extra != "" { //nolint:forbidigo // test input of a narrowed reproduction
		args = append(args, "-k", extra)
	}
	cmd := exec.CommandContext(context.Background(), "uv", args...) //nolint:gosec // G702: fixed program, arguments from the test
	cmd.Dir = root
	// uv needs PATH and HOME of the caller; the server itself reads nothing from it.
	cmd.Env = append(os.Environ(), //nolint:forbidigo // environment of the uv subprocess, not server configuration
		"BOTHY_CONTRACT_BASE_URL="+s.APIBase,
		"BOTHY_CONTRACT_BROKER_URL="+b.URL(),
		"BOTHY_CONTRACT_OPERATIONS="+filepath.Join(root, "services", "server", "api", "operations.yaml"),
		"BOTHY_CONTRACT_SPEC="+filepath.Join(root, "docs", "api", "openapi.yaml"),
		// The channel where Alex uploads, so the upload operations reach a real session (M3-03).
		"BOTHY_CONTRACT_UPLOAD_CHANNEL="+w.Channels["Support/Allgemein"],
		"CONTRACT_MAX_EXAMPLES="+examples,
		"CONTRACT_SEED="+seed,
		"BOTHY_CONTRACT_STEP_UP_MAX_AGE_S="+seconds(t, s.ConfigValue("auth.step_up_max_age")),
		"BOTHY_CONTRACT_ACCESS_TOKEN_TTL_S="+seconds(t, s.ConfigValue("auth.access_token_ttl")),
		"BOTHY_CONTRACT_NONCE_TTL_S="+seconds(t, s.ConfigValue("auth.dpop_nonce_ttl")),
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Schemathesis run failed (%v); reproduce with CONTRACT_SEED=%s", err, seed)
	}
}

// seconds returns a duration text as whole seconds.
func seconds(t *testing.T, value string) string {
	t.Helper()
	d, err := time.ParseDuration(value)
	if err != nil {
		t.Fatalf("duration %q: %v", value, err)
	}
	return strconv.Itoa(int(d.Seconds()))
}

// repoRoot returns the directory above the working directory that holds docs/api/openapi.yaml.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docs", "api", "openapi.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("docs/api/openapi.yaml not found above the working directory")
		}
		dir = parent
	}
}
