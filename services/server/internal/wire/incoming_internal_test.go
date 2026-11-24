// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
)

// TestInternalHooksValidatorOnlyWebhookPosts (M2-27): the chain of the internal webhook routes
// compiles the request validator of the two post operations only, not that of every enabled
// operation, so a worker role, which mounts no public API, does not pay the full compile before
// it is ready.
func TestInternalHooksValidatorOnlyWebhookPosts(t *testing.T) {
	t.Parallel()
	var want []string
	for _, op := range routes.Enabled() {
		if op.Action == actionWebhookInPost {
			want = append(want, op.ID)
		}
	}
	got := webhookPostOperationIDs()
	if len(want) != 2 || !slices.Equal(got, want) {
		t.Fatalf("validator operations %v, want the two webhook posts %v", got, want)
	}
	if _, err := internalHooksValidator(); err != nil {
		t.Fatal(err)
	}
}

// TestShutdownGateRefusesWebhooksDuringTheShutdown: the webhook routes of the internal listener,
// which closes last, answer 503 unavailable with Retry-After once the shutdown began, instead of
// writing while the log tail and the jobs are stopped and failing after the pool closed.
func TestShutdownGateRefusesWebhooksDuringTheShutdown(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	reached := 0
	h := shutdownGate(reg, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusAccepted)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/hooks/x", strings.NewReader("{}")))
	if rec.Code != http.StatusAccepted || reached != 1 {
		t.Fatalf("before the shutdown: %d, reached %d", rec.Code, reached)
	}
	reg.SetShuttingDown()
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/hooks/x", strings.NewReader("{}")))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "60" || reached != 1 ||
		!strings.Contains(rec.Body.String(), `"unavailable"`) {
		t.Fatalf("during the shutdown: %d %q %s, reached %d", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String(), reached)
	}
}
