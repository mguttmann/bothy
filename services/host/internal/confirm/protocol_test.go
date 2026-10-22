// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"errors"
	"testing"
)

// TestDecodeServiceStrict (brief M2-45 Design note 15 b): the agent accepts from the service exactly
// a confirm or a cancel; a cancel with any member of a confirm or an answer, and every other op,
// is malformed and closes the connection.
func TestDecodeServiceStrict(t *testing.T) {
	m, err := decodeService([]byte(`{"op":"cancel","id":"r1"}`))
	if err != nil || m.op != opCancel || m.id != "r1" {
		t.Fatalf("cancel %+v %v", m, err)
	}
	m, err = decodeService([]byte(`{"op":"confirm","id":"r2","kind":"attach","profile":"shell","session_user":"bothy-s-alex","timeout_ms":5000}`))
	if err != nil || m.op != opConfirm || m.id != "r2" || m.timeoutMS != 5000 ||
		m.req != (Request{Kind: KindAttach, Profile: "shell", SessionUser: "bothy-s-alex"}) {
		t.Fatalf("confirm %+v %v", m, err)
	}
	for _, line := range []string{
		`{"op":"cancel","id":"r1","kind":"open"}`,
		`{"op":"cancel","id":"r1","profile":"shell"}`,
		`{"op":"cancel","id":"r1","session_user":"bothy-s-alex"}`,
		`{"op":"cancel","id":"r1","timeout_ms":5000}`,
		`{"op":"cancel","id":"r1","allow":true}`,
		`{"op":"answer","id":"r1","allow":true}`,
		`{"op":"ping","id":"r1"}`,
		`{"op":"","id":"r1"}`,
	} {
		if m, err := decodeService([]byte(line)); !errors.Is(err, errMalformed) || m != (serviceMsg{}) {
			t.Errorf("%s: %+v %v", line, m, err)
		}
	}
}
