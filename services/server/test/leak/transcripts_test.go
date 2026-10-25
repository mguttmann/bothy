// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// transcriptCases returns the comparisons of the two write routes of M3-09 with the file_id
// fixture (Sam's upload in Marketing/Kampagnen, which neither requester sees): Alex asking
// requestTranscription and Lotse sending submitTranscript, each against a random file ID. Both are
// content answers: 404 not_found (05 file.transcribe, file.transcript.submit; AZ-P4).
func (p *p4World) transcriptCases(t *testing.T) []pushCase {
	t.Helper()
	e, ok := extras.Load(p.w)
	if !ok {
		t.Fatal("leak: PrepareExtras did not run for this world")
	}
	file, random := e.(*Extras).FileID, ids.New().String()
	alex, lotse := p.w.People["Alex"], p.w.Agents["Lotse"]
	return []pushCase{
		{
			operationID: "requestTranscription", requester: "Alex", params: "file_id",
			pair: pair{a: "/files/" + file + "/transcribe", b: "/files/" + random + "/transcribe", foreign: []string{file}, random: []string{random}, content: true},
			send: func(t *testing.T, target string) harness.Response {
				t.Helper()
				h := http.Header{}
				h.Set("Idempotency-Key", ids.New().String())
				return alex.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: target, Header: h})
			},
		},
		{
			operationID: "submitTranscript", requester: "Lotse", params: "file_id",
			pair: pair{a: "/files/" + file + "/transcript", b: "/files/" + random + "/transcript", foreign: []string{file}, random: []string{random}, content: true},
			send: func(t *testing.T, target string) harness.Response {
				t.Helper()
				h := http.Header{}
				h.Set("If-Match", `"AAAAAAAAAAAAAAAAAAAAAA"`)
				return lotse.Do(t.Context(), harness.Request{Method: http.MethodPut, Path: target, Header: h, Body: map[string]any{"text": "hello"}})
			},
		},
	}
}
