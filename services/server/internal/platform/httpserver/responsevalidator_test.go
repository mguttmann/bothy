// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestResponseValidator: a body that matches the response schema of the contract passes; a
// missing required member, a JSON null and an undeclared status fail (M1-31).
func TestResponseValidator(t *testing.T) {
	t.Parallel()
	rv, err := httpservertest.NewResponseValidator(rest.Spec, []string{"getUser"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://chat.example.com/api/v1/users/0192f4a0-7b1c-7f89-8d6b-6960453241bc", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp := func(status int, body string) *httpservertest.Response {
		h := http.Header{}
		h.Set("Content-Type", "application/json")
		return &httpservertest.Response{Response: &http.Response{StatusCode: status, Header: h, Request: req}, Body: []byte(body)}
	}
	good := `{"id":"0192f4a0-7b1c-7f89-8d6b-6960453241bc","handle":"sam","display_name":"Sam","status":"active","is_owner":false,"created_at":"2026-10-10T09:30:00.000Z"}`
	if err := rv.Validate(req, resp(http.StatusOK, good)); err != nil {
		t.Fatalf("valid body: %v", err)
	}
	for name, r := range map[string]*httpservertest.Response{
		"missing member": resp(http.StatusOK, strings.Replace(good, `"is_owner":false,`, "", 1)),
		"null":           resp(http.StatusOK, strings.Replace(good, `"handle":"sam"`, `"handle":null`, 1)),
		"wrong enum":     resp(http.StatusOK, strings.Replace(good, `"active"`, `"gone"`, 1)),
		"undeclared":     resp(http.StatusTeapot, good),
	} {
		if err := rv.Validate(req, r); err == nil {
			t.Errorf("%s: no finding", name)
		}
	}
}
