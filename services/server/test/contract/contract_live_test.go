// SPDX-License-Identifier: Apache-2.0

//go:build contract_live

package contract

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// pushAppID is the only APNs topic of the server under test: registerPush of the harness
// (tools/contract/auth.py) registers apns targets of its macos devices for this app.
const pushAppID = "com.example.chat"

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
	fake, secretFiles := enableAPNs(t, cfg)
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: cfg, APNsFake: fake, SecretFiles: secretFiles})
	w := harness.BuildMatrixWorld(t, s)
	allgemein := w.Messages["Support/Allgemein"]
	if len(allgemein) == 0 {
		t.Fatal("the matrix world has no message in Support/Allgemein")
	}
	b, err := StartBroker(s, w.People["Alex"].PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	root := repoRoot(t)
	// A fixed seed (quality.contract.seed through make contract-test): every run draws the same
	// cases, so a failure repeats instead of passing on a rerun with another seed.
	seed, err := Seed(os.Getenv("CONTRACT_SEED")) //nolint:forbidigo // test input from make contract-test, not server configuration
	if err != nil {
		t.Fatal(err)
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
		// registerPush and getPushPreview (M3-22): the APNs topic, and a message of the owner in
		// Support/Allgemein that both passes may read (Alex through G-1).
		"BOTHY_CONTRACT_PUSH_APP_ID="+pushAppID,
		"BOTHY_CONTRACT_PREVIEW_MESSAGE="+allgemein[len(allgemein)-1],
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
	if n := len(fake.Requests()); n != 0 {
		t.Fatalf("%d requests reached Fake-APNs; egress must stay denied", n)
	}
}

// enableAPNs enables the APNs provider in cfg, so registerPush answers 201 (M3-22), and returns
// a Fake-APNs with the secret files of its key. The role all builds a real adapter for an enabled
// provider; Options.APNsFake points it at the Fake instead of Apple, and EgressDenyAll still
// refuses every outgoing request, so no notification leaves the process. FCM stays disabled.
func enableAPNs(t *testing.T, cfg map[string]string) (*apnstest.Server, map[string][]byte) {
	t.Helper()
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
	cfg["push.apns.team_id"], cfg["push.apns.key_id"], cfg["push.apns.topics"] = "EXAMPLETEAM", "EXAMPLEKEY", pushAppID
	cfg["push.apns.private_key_ref"] = "file:<secrets.dir>/apns/key.p8"
	return fake, map[string][]byte{"apns/key.p8": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})}
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
