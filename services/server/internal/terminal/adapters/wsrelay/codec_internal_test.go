// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// restIdentify is the generated body of identify.
type restIdentify = rest.HostIdentify

// newSchemas compiles the schemas of the host ops.
func newSchemas() (SchemaChecker, error) {
	return httpserver.NewSchemaValidator(rest.Spec, SchemaNames())
}

// FuzzDecodeControlFrame (M2-43, PLAN section 5 "Security"): arbitrary frames give a frame or one
// of the two decode errors, never a panic; a decoded frame is a host op with its typed body.
func FuzzDecodeControlFrame(f *testing.F) {
	check, err := newSchemas()
	if err != nil {
		f.Fatal(err)
	}
	f.Add([]byte(`{"op":"heartbeat","d":{"active_sessions":0}}`))
	f.Add([]byte(`{"op":"identify","d":{"protocol_version":1,"host_id":"0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f","proof":"a.b.c","host_version":"1.0.0","live_sessions":[]}}`))
	f.Add([]byte(`{"op":"report","d":{"profiles":[],"session_users":[],"doctor":[],"is_server_host":false,"require_local_confirm":false}}`))
	f.Add([]byte(`{"op":"session.ended","d":{"session_id":"0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f","close_reason":"open_failed","close_detail":"pty_failed"}}`))
	f.Add([]byte(`{"op":"ready","d":{}}`))
	f.Add([]byte(`{"op":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		fr, err := decodeFrame(b, 32, check)
		if err != nil {
			if !errors.Is(err, errDecode) && !errors.Is(err, errUnknownOp) {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		if _, ok := hostOps[fr.op]; !ok || fr.d == nil {
			t.Fatalf("decoded %q without a host op body", fr.op)
		}
	})
}
