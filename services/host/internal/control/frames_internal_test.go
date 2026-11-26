// SPDX-License-Identifier: Apache-2.0

package control

import (
	"errors"
	"testing"
)

// TestDecodeServerFrameRefusesMalformed (M2-43 Design note 12 a): every server frame that breaks
// the strict decoding is ErrFrame, which closes the connection with 4002; a valid ready decodes.
func TestDecodeServerFrameRefusesMalformed(t *testing.T) {
	t.Parallel()
	for name, frame := range map[string]string{
		"no envelope":    `[]`,
		"no op":          `{"d":{}}`,
		"no body":        `{"op":"ready"}`,
		"unknown member": `{"op":"ready","d":{},"x":1}`,
		"trailing data":  `{"op":"ready","d":{}} {}`,
		"client op":      `{"op":"identify","d":{}}`,
		"body no object": `{"op":"ready","d":[1]}`,
		"missing member": `{"op":"ready","d":{"host_id":"h","state":"active"}}`,
		"null member":    `{"op":"ready","d":{"host_id":"h","state":"active","llm_credentials":null}}`,
		"unknown in d":   `{"op":"ready","d":{"host_id":"h","state":"active","llm_credentials":"personal","x":1}}`,
		"wrong type":     `{"op":"ready","d":{"host_id":1,"state":"active","llm_credentials":"personal"}}`,
	} {
		if _, _, err := decodeServerFrame([]byte(frame)); !errors.Is(err, ErrFrame) {
			t.Errorf("%s: %v, want ErrFrame", name, err)
		}
	}
	op, v, err := decodeServerFrame([]byte(`{"op":"ready","d":{"host_id":"h","state":"active","llm_credentials":"personal"}}`))
	if r, ok := v.(*ReadyFrame); err != nil || op != opReady || !ok || r.LLMCredentials != "personal" {
		t.Errorf("valid ready: %q %+v %v", op, v, err)
	}
}

// TestComputerServerFramesDecodeStrictly: computer.open, computer.close and computer.disable decode
// strictly into their types (docs/spec/27-computer-use.md 12.3); a missing member, an unknown
// member and a null are refused.
func TestComputerServerFramesDecodeStrictly(t *testing.T) {
	t.Parallel()
	open := `{"op":"computer.open","d":{"session_id":"s","run_id":"r","agent_id":"a","agent_name":"Builder","user_id":"u",` +
		`"data_path":"/api/v1/hosts/computer-sessions/s/stream","expires_at":"2026-11-26T10:00:00Z","settings":{"image_max_edge":1280,` +
		`"jpeg_quality":80,"settle_ms":500,"auto_screenshot":true,"wait_max_ms":60000,"key_repeat_max":50,"audit_apps_max":32,"status_interval_ms":2000}}}`
	op, body, err := decodeServerFrame([]byte(open))
	if o, ok := body.(*ComputerOpen); err != nil || op != opComputerOpen || !ok || o.Settings.ImageMaxEdge != 1280 || !o.Settings.AutoScreenshot {
		t.Fatalf("computer.open: %q %+v %v", op, body, err)
	}
	if _, body, err := decodeServerFrame([]byte(`{"op":"computer.close","d":{"session_id":"s","reason":"run_ended"}}`)); err != nil || body.(*ComputerClose).Reason != "run_ended" {
		t.Errorf("computer.close: %v", err)
	}
	if _, body, err := decodeServerFrame([]byte(`{"op":"computer.disable","d":{"generation":4}}`)); err != nil || body.(*ComputerDisable).Generation != 4 {
		t.Errorf("computer.disable: %v", err)
	}
	for _, bad := range []string{
		`{"op":"computer.close","d":{"session_id":"s"}}`,
		`{"op":"computer.close","d":{"session_id":"s","reason":"done","x":1}}`,
		`{"op":"computer.disable","d":{"generation":null}}`,
		`{"op":"computer.open","d":{"session_id":"s"}}`,
	} {
		if _, _, err := decodeServerFrame([]byte(bad)); err == nil {
			t.Errorf("%s decoded", bad)
		}
	}
}
