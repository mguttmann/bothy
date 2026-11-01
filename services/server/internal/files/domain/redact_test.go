// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// fullFileUpdated is a stored file.updated v1 payload with every member of File set.
const fullFileUpdated = `{"file":{"id":"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b","sha256":"ab12","size_bytes":4711,` +
	`"mime_type":"image/png","original_name":"quarterly report.png","uploaded_by":"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f0001",` +
	`"purpose":"attachment","scope_channel_id":"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f0002","upload_state":"complete",` +
	`"scan_status":"clean","width":640,"height":480,"duration_ms":1200,"transcript_status":"done",` +
	`"transcript_language":"de","transcript_source":"server",` +
	`"thumbnails":[{"variant":"small","mime_type":"image/webp","width":64,"height":48}],"created_at":"2026-03-01T12:00:00.000Z"}}`

// TestRedactFileEvent: the redacted form keeps exactly the members of brief M3-08 design note 3,
// holds no null, is a fixed point, and other types, versions and malformed payloads fail.
func TestRedactFileEvent(t *testing.T) {
	out, err := domain.RedactFileEvent("file.updated", 1, json.RawMessage(fullFileUpdated))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		File map[string]any `json:"file"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"id": "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b", "size_bytes": float64(0), "mime_type": "application/octet-stream",
		"original_name": "", "uploaded_by": "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f0001", "purpose": "attachment",
		"upload_state": "complete", "scan_status": "clean", "transcript_status": "done", "thumbnails": []any{},
		"created_at": "2026-03-01T12:00:00.000Z",
	}
	if keys := slices.Sorted(maps.Keys(got.File)); !slices.Equal(keys, slices.Sorted(maps.Keys(want))) {
		t.Fatalf("members %v", keys)
	}
	for k, v := range want {
		if gv, _ := json.Marshal(got.File[k]); !bytes.Equal(gv, must(json.Marshal(v))) {
			t.Fatalf("%s = %s", k, gv)
		}
	}
	for _, leak := range []string{"quarterly", "ab12", "image/png", "0002", "server", `"de"`} {
		if strings.Contains(string(out), leak) {
			t.Fatalf("redacted payload holds %q: %s", leak, out)
		}
	}
	if testkit.HasJSONNull(out) {
		t.Fatalf("redacted payload holds JSON null: %s", out)
	}
	again, err := domain.RedactFileEvent("file.updated", 1, out)
	if err != nil || !bytes.Equal(again, out) {
		t.Fatalf("second application: %s, %v; want %s", again, err, out)
	}
	for name, tc := range map[string]struct {
		typ     string
		version int
		payload string
		want    error
	}{
		"other type":     {"file.created", 1, fullFileUpdated, domain.ErrRedactUnknown},
		"other version":  {"file.updated", 2, fullFileUpdated, domain.ErrRedactUnknown},
		"invalid json":   {"file.updated", 1, `{"file":`, domain.ErrRedactPayload},
		"no file member": {"file.updated", 1, `{"x":{}}`, domain.ErrRedactPayload},
		"missing id":     {"file.updated", 1, `{"file":{"uploaded_by":"u"}}`, domain.ErrRedactPayload},
	} {
		if _, err := domain.RedactFileEvent(tc.typ, tc.version, json.RawMessage(tc.payload)); !errors.Is(err, tc.want) {
			t.Fatalf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}
