// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// emptyHeaderSpec has one operation with an optional header whose only value is "1", like the
// parameter ConsoleRequest of docs/api/openapi.yaml (M6-01).
const emptyHeaderSpec = `openapi: 3.1.0
info: {title: t, version: "1"}
servers: [{url: /api/v1}]
paths:
  /logout:
    post:
      operationId: logout
      parameters:
        - name: X-Console-Request
          in: header
          required: false
          schema: {type: string, enum: ["1"]}
      responses:
        "204": {description: ok}
`

// TestValidate_EmptyEnumHeaderRejected (D-M6-01-13): an optional header that is present with an
// empty value is checked like any other value, so an empty X-Console-Request is 400 invalid_enum
// as "2" is; the validator library treats an empty header as absent.
func TestValidate_EmptyEnumHeaderRejected(t *testing.T) {
	t.Parallel()
	v, err := httpserver.NewValidator([]byte(emptyHeaderSpec), []string{"logout"})
	if err != nil {
		t.Fatal(err)
	}
	op := httpserver.Operation{ID: "logout"}
	for _, tc := range []struct {
		name   string
		values []string
		ok     bool
	}{
		{"absent", nil, true},
		{"one", []string{"1"}, true},
		{"other", []string{"2"}, false},
		{"empty", []string{""}, false},
	} {
		r, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/logout", nil)
		for _, val := range tc.values {
			r.Header.Add("X-Console-Request", val)
		}
		err := v.Validate(r, op)
		if tc.ok {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		var e *errs.Error
		if !errors.As(err, &e) || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 ||
			e.Fields[0].Pointer != "#header/X-Console-Request" || e.Fields[0].Code != "invalid_enum" {
			t.Errorf("%s: %#v", tc.name, err)
		}
	}
}
