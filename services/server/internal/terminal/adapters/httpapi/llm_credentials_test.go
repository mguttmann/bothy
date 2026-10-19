// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestSetHostLlmCredentials_Handler (M2-40): the strict-server method maps the body, If-Match and
// the answer of setHostLlmCredentials: Robin, the owner, declares organization on R3 and gets 200
// with the raised version and its ETag; a body without reason is 400 validation_failed with the
// pointer /reason; a missing If-Match is 428. The rules themselves are proved in terminal/app.
func TestSetHostLlmCredentials_Handler(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	path := terminalworld.HostPath(w.R3) + "/llm-credentials"
	v := w.HostVersion(w.R3)
	res := w.Do("robin", http.MethodPut, path, `{"llm_credentials":"organization","reason":"Team key"}`, w.Keyed(w.R3)...)
	if etag := res.Header.Get("ETag"); etag == "" {
		t.Fatal("no ETag")
	}
	m := w.Want(res, http.StatusOK, "")
	if m["llm_credentials"] != "organization" || m["version"] != float64(v+1) {
		t.Fatalf("answer %v", m)
	}
	if _, ptr := terminalworld.Field(w.Want(w.Do("robin", http.MethodPut, path, `{"llm_credentials":"none"}`, w.Keyed(w.R3)...),
		http.StatusBadRequest, "validation_failed")); ptr != "/reason" {
		t.Fatalf("pointer %q", ptr)
	}
	w.Want(w.Do("robin", http.MethodPut, path, `{"llm_credentials":"none","reason":"Back"}`, terminalworld.Key()...),
		http.StatusPreconditionRequired, "precondition_required")
}
