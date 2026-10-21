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
