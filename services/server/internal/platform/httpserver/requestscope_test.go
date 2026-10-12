// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestRequestScopeInstalledPerRequest: the chain installs one request memo of kernel/authorize
// per request at step 1 (02 section 9.1), so the rights version is read once per request (M1-28).
func TestRequestScopeInstalledPerRequest(t *testing.T) {
	t.Parallel()
	c := newChain(t, "getThing")
	var memos []*authorize.RequestMemo
	op := getThing()
	mux := http.NewServeMux()
	mux.Handle(op.Pattern, c.Wrap(op, httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
		m, ok := authorize.RequestMemoFrom(r.Context())
		if !ok {
			t.Error("no request memo in the handler")
		}
		memos = append(memos, m)
		httpservertest.JSON(w, http.StatusOK, map[string]any{"ok": true})
	})))
	h := c.Public(mux)
	for range 2 {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/things/"+thingID, nil)
		req.Header.Set("X-Mode", "fast")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
	}
	if len(memos) != 2 || memos[0] == nil || memos[0] == memos[1] {
		t.Fatalf("want one fresh memo per request, got %v", memos)
	}
}
