// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"testing"

	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestAZP3_UnknownActionIs500 (05 rule 5, 07 AZ-P3): a handler that returns the error of
// Catalog.Spec for an unknown action answers 500 internal_error, and the body does not contain
// the action.
func TestAZP3_UnknownActionIs500(t *testing.T) {
	t.Parallel()
	const unknown = "no.such.action"
	cat := authzapp.NewCatalog(slog.New(&httpservertest.LogCapture{}))
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: getThing(),
		Handler: func(w http.ResponseWriter, r *http.Request) {
			if _, err := cat.Spec(authorize.Action(unknown)); err != nil {
				httpserver.WriteProblem(w, r, err)
				return
			}
			httpservertest.JSON(w, http.StatusOK, map[string]any{})
		}}}})
	resp := getOne(h)
	p := resp.Problem(t)
	if resp.StatusCode != http.StatusInternalServerError || p["code"] != "internal_error" {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if bytes.Contains(resp.Body, []byte(unknown)) || bytes.Contains(resp.Body, []byte("unknown_action")) {
		t.Fatalf("the body names the action or the cause: %s", resp.Body)
	}
}
