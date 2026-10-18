// SPDX-License-Identifier: Apache-2.0

package specschema_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/specschema"
)

// contract loads the real contracts once per test.
func contract(t *testing.T) *specschema.Contract {
	t.Helper()
	c, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Fixture frames built from the real schemas (GatewayHello, TypingStartedEvent,
// SessionRevokedEvent). The ids are UUIDv7 examples, the times RFC 3339 with milliseconds.
const (
	helloFrame = `{"op":"hello","d":{"heartbeat_interval_ms":30000,"resume_window_ms":300000,"protocol_versions":[1],` +
		`"dpop_nonce":"n1","server_time":"2026-01-01T00:00:00.000Z","sync_epoch":"0192f4a0-7b1c-7000-8a00-000000000001"}}`
	typingFrame = `{"op":"dispatch","d":{"id":"0192f4a0-7b1c-7000-8a00-000000000002","schema_version":1,` +
		`"occurred_at":"2026-01-01T00:00:00.000Z","scope":{"thread_id":"0192f4a0-7b1c-7000-8a00-000000000003"},` +
		`"resource":{"kind":"thread","id":"0192f4a0-7b1c-7000-8a00-000000000003"},"type":"typing.started",` +
		`"payload":{"thread_id":"0192f4a0-7b1c-7000-8a00-000000000003","principal_id":"0192f4a0-7b1c-7000-8a00-000000000004",` +
		`"expires_at":"2026-01-01T00:00:08.000Z"}}}`
	revokedFrame = `{"op":"dispatch","d":{"id":"0192f4a0-7b1c-7000-8a00-000000000005","seq":7,"permission_version":3,"schema_version":1,` +
		`"occurred_at":"2026-01-01T00:00:00.000Z","scope":{},"resource":{"kind":"auth_session","id":"0192f4a0-7b1c-7000-8a00-000000000006"},` +
		`"audience":"self","type":"session.revoked","payload":{"session_id":"0192f4a0-7b1c-7000-8a00-000000000006","reason":"logout"}}}`
)

// mustFail requires err to be a violation whose text contains want.
func mustFail(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no violation, want one about %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("violation %q, want it to mention %q", err, want)
	}
}

func TestLoad_RealContracts(t *testing.T) {
	t.Parallel()
	c := contract(t)
	msgs := c.Messages()
	if len(msgs) < 30 {
		t.Fatalf("%d gateway messages, want every op and event of the channel gateway", len(msgs))
	}
	for _, m := range msgs {
		if err := c.Compiles(m); err != nil {
			t.Errorf("message %s: %v", m, err)
		}
	}
	if got := c.CloseCodes()[4014]; got != "slow_consumer" {
		t.Errorf("close code 4014 is %q", got)
	}
	if len(c.Operations()) == 0 || len(c.EventMessages()) == 0 {
		t.Error("no operations or event messages")
	}
}

func TestLoad_UnknownRefFormFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "api"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "spec"), 0o750); err != nil {
		t.Fatal(err)
	}
	real, err := specschema.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"docs/api/openapi.yaml", "docs/api/asyncapi.yaml", "docs/spec/actions.yaml"} {
		b, err := os.ReadFile(filepath.Join(real, f)) // #nosec G304 -- the contract files of this repository
		if err != nil {
			t.Fatal(err)
		}
		if f == "docs/api/openapi.yaml" {
			// References in a form the bundle does not know; everything else stays real.
			b = bytes.ReplaceAll(b, []byte("$ref: '#/components/schemas/Uuid'"), []byte("$ref: 'other.yaml#/B'"))
		}
		if err := os.WriteFile(filepath.Join(root, f), b, 0o600); err != nil { // #nosec G703 -- a fixed name below t.TempDir
			t.Fatal(err)
		}
	}
	_, err = specschema.Load(root)
	mustFail(t, err, "other.yaml#/B")
}

func TestValidate_ValidHelloPasses(t *testing.T) {
	t.Parallel()
	c := contract(t)
	for name, f := range map[string]string{"hello": helloFrame, "typing.started": typingFrame, "session.revoked": revokedFrame} {
		if err := c.ValidateServerFrame([]byte(f)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestValidate_UndeclaredFieldFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(helloFrame, `"dpop_nonce"`, `"extra":1,"dpop_nonce"`, 1))), "/d")
	// An undeclared member in a nested payload of an allOf composition fails as well.
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(revokedFrame, `"reason":"logout"`, `"reason":"logout","x":true`, 1))), "/d/payload")
	// And one beside the envelope members of the composed event.
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(typingFrame, `"type":"typing.started"`, `"type":"typing.started","bogus":1`, 1))), "bogus")
}

func TestValidate_NullFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(helloFrame, `"dpop_nonce":"n1"`, `"dpop_nonce":null`, 1))), "/d/dpop_nonce")
}

func TestValidate_BigIntegerFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(helloFrame, `30000`, `9007199254740992`, 1))), "/d/heartbeat_interval_ms")
	if err := c.ValidateServerFrame([]byte(strings.Replace(helloFrame, `30000`, `9007199254740991`, 1))); err != nil {
		t.Errorf("2^53-1 must pass: %v", err)
	}
}

func TestValidate_DurableWithoutSeqFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(revokedFrame, `"seq":7,`, ``, 1))), "seq")
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(revokedFrame, `"permission_version":3,`, ``, 1))), "permission_version")
}

func TestValidate_EphemeralWithSeqFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(typingFrame, `"schema_version":1`, `"seq":5,"schema_version":1`, 1))), "seq")
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(typingFrame, `"schema_version":1`, `"permission_version":5,"schema_version":1`, 1))), "permission_version")
}

func TestValidate_ResourceVersionAgainstCatalogue(t *testing.T) {
	t.Parallel()
	c := contract(t)
	// session.revoked has resource_version false in the catalogue.
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(revokedFrame, `"kind":"auth_session",`, `"kind":"auth_session","version":2,`, 1))), "resource.version")
}

func TestValidate_UnknownEventTypeFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	mustFail(t, c.ValidateServerFrame([]byte(strings.Replace(typingFrame, `"type":"typing.started"`, `"type":"typing.unknown"`, 1))), "typing.unknown")
}

func TestValidate_ClientOpAsServerFrameFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	frame := `{"op":"heartbeat","d":{"last_seq":1}}`
	mustFail(t, c.ValidateServerFrame([]byte(frame)), "heartbeat")
	if err := c.ValidateClientFrame([]byte(frame)); err != nil {
		t.Errorf("heartbeat as client frame: %v", err)
	}
	mustFail(t, c.ValidateClientFrame([]byte(helloFrame)), "hello")
}

func TestValidate_OversizedFrameFails(t *testing.T) {
	t.Parallel()
	c := contract(t).WithMaxFrameBytes(len(helloFrame) - 1)
	mustFail(t, c.ValidateServerFrame([]byte(helloFrame)), "sync.ws_max_frame_bytes")
	if err := contract(t).WithMaxFrameBytes(len(helloFrame)).ValidateServerFrame([]byte(helloFrame)); err != nil {
		t.Errorf("frame at the limit: %v", err)
	}
}
