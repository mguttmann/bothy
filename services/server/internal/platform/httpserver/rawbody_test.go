// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestRawJSONBody: the handler sees the JSON body exactly as received (member order and white
// space included); outside the chain there is none.
func TestRawJSONBody(t *testing.T) {
	t.Parallel()
	op := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	var got string
	var ok bool
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			var b []byte
			b, ok = httpserver.RawJSONBody(r.Context())
			got = string(b)
			w.WriteHeader(http.StatusCreated)
		})}}})
	body := `{ "color":"red",  "name":"ab" }`
	if r := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", body)); r.StatusCode != http.StatusCreated {
		t.Fatalf("status %d %s", r.StatusCode, r.Body)
	}
	if !ok || got != body {
		t.Fatalf("raw body %q %v, want %q", got, ok, body)
	}
	if b, ok := httpserver.RawJSONBody(context.Background()); ok || b != nil {
		t.Fatal("a context outside the chain has a body")
	}
}
