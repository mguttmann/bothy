// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// validateProblem validates body against the schema Problem of the embedded contract.
func validateProblem(t *testing.T, body []byte) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	proxy, ok := model.Model.Components.Schemas.Get("Problem")
	if !ok {
		t.Fatal("the contract has no schema Problem")
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaBytes(proxy.Schema(), body); !valid {
		for _, p := range problems {
			t.Errorf("schema: %s %s", p.Message, p.Reason)
		}
		t.Fatalf("the body does not match Problem: %s", body)
	}
}

// TestProblem_I1ViolationCarriesConflicts (brief M1-29 test 14): an InvariantError gives 409 with
// code i1_violation, the hint and conflicts valid against the contract, and no null anywhere.
func TestProblem_I1ViolationCarriesConflicts(t *testing.T) {
	t.Parallel()
	agent, holder, sam := ids.New(), ids.New(), ids.New()
	err := fmt.Errorf("use case: %w", authz.NewI1Violation([]authz.I1Conflict{
		{AgentID: agent, CredentialHolderID: holder, PrincipalIDs: []ids.ID{sam}},
		{AgentID: ids.New()},
	}))
	rec := problemFor(t, err)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.Bytes()
	if bytes.Contains(body, []byte("null")) {
		t.Fatalf("null in %s", body)
	}
	var p struct {
		Code      string `json:"code"`
		Hint      string `json:"hint"`
		Conflicts []struct {
			Code               string   `json:"code"`
			AgentID            string   `json:"agent_id"`
			CredentialHolderID string   `json:"credential_holder_id"`
			PrincipalIDs       []string `json:"principal_ids"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != "i1_violation" || p.Hint != authz.HintI1 || len(p.Conflicts) != 2 {
		t.Fatalf("body %s", body)
	}
	for _, c := range p.Conflicts {
		if c.AgentID == agent.String() && (c.CredentialHolderID != holder.String() || len(c.PrincipalIDs) != 1 || c.PrincipalIDs[0] != sam.String()) {
			t.Fatalf("conflict %+v", c)
		}
	}
	validateProblem(t, body)
}

// extenderError is an error with extension members that try to override standard members and carry
// null values.
type extenderError struct{ ext map[string]any }

func (e extenderError) Error() string                     { return "extender" }
func (e extenderError) Unwrap() error                     { return errs.New(errs.CodeConflict, "detail") }
func (e extenderError) ProblemExtensions() map[string]any { return e.ext }

// TestProblem_ExtensionsNeverOverrideOrNull: extension members never replace a standard member
// (even one the body leaves out), nil values and nested nulls are dropped, and an internal error
// carries no extension.
func TestProblem_ExtensionsNeverOverrideOrNull(t *testing.T) {
	t.Parallel()
	rec := problemFor(t, extenderError{ext: map[string]any{
		"code": "hacked", "status": 200, "hint": "x", "current": map[string]any{}, "extra": "ok", "gone": nil,
		"nested": map[string]any{"a": nil, "b": []any{1, nil}},
	}})
	body := rec.Body.String()
	if rec.Code != http.StatusConflict || strings.Contains(body, "hacked") || strings.Contains(body, `"hint"`) ||
		strings.Contains(body, "null") || strings.Contains(body, `"current"`) || strings.Contains(body, `"gone"`) {
		t.Fatalf("%d %s", rec.Code, body)
	}
	if !strings.Contains(body, `"extra":"ok"`) || !strings.Contains(body, `"nested":{"b":[1]}`) {
		t.Fatalf("extensions missing: %s", body)
	}
	internal := problemFor(t, internalExtenderError{})
	if strings.Contains(internal.Body.String(), "extra") {
		t.Fatalf("an internal error with extensions: %s", internal.Body.String())
	}
}

// internalExtenderError is an internal error with an extension member.
type internalExtenderError struct{}

func (internalExtenderError) Error() string                     { return "internal" }
func (internalExtenderError) Unwrap() error                     { return errs.New(errs.CodeInternalError, "") }
func (internalExtenderError) ProblemExtensions() map[string]any { return map[string]any{"extra": "x"} }
