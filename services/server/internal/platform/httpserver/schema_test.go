// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestSchemaValidator (M1-64): a JSON value is checked against one named schema of the contract
// with the rules of the request validator (SEC-IN-1): unknown members, missing members, wrong
// types, formats and bounds are 400 validation_failed with the field pointer; a valid value
// passes; an unknown schema name is an error at construction and at use.
func TestSchemaValidator(t *testing.T) {
	t.Parallel()
	v, err := httpserver.NewSchemaValidator(httpservertest.Contract, []string{"ThingCreate"})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Validate("ThingCreate", []byte(`{"name":"abc","color":"red","nested":{"flag":true}}`)); err != nil {
		t.Fatalf("valid value: %v", err)
	}
	for name, c := range map[string]struct {
		body, pointer, code string
	}{
		"unknown member":        {`{"name":"abc","color":"red","zzz":1}`, "/zzz", "unknown_member"},
		"unknown nested member": {`{"name":"abc","color":"red","nested":{"x":1}}`, "/nested/x", "unknown_member"},
		"missing member":        {`{"name":"abc"}`, "/color", "required"},
		"wrong type":            {`{"name":1,"color":"red"}`, "/name", "invalid_type"},
		"bad format":            {`{"name":"abc","color":"red","owner_id":"x"}`, "/owner_id", "invalid_format"},
		"too large":             {`{"name":"abc","color":"red","size":11}`, "/size", "too_large"},
		"not an object":         {`[]`, "", "invalid_type"},
	} {
		err := v.Validate("ThingCreate", []byte(c.body))
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed {
			t.Errorf("%s: %v, want validation_failed", name, err)
			continue
		}
		found := false
		for _, f := range e.Fields {
			if f.Pointer == c.pointer && f.Code == c.code {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: fields %+v, want %s at %q", name, e.Fields, c.code, c.pointer)
		}
	}
	if err := v.Validate("Color", []byte(`"red"`)); err == nil {
		t.Error("a schema the validator was not built for passed")
	}
	if _, err := httpserver.NewSchemaValidator(httpservertest.Contract, []string{"Missing"}); err == nil {
		t.Error("an unknown schema name was accepted")
	}
}

// TestSchemaValidator_ConcurrentUse (M1-67): the gateway validates the frames of many connections
// with one validator at once; under -race, concurrent checks of the same schema must not race
// (libopenapi renders the schema while it validates).
func TestSchemaValidator_ConcurrentUse(t *testing.T) {
	t.Parallel()
	v, err := httpserver.NewSchemaValidator(httpservertest.Contract, []string{"ThingCreate"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				if err := v.Validate("ThingCreate", []byte(`{"name":"abc","color":"red"}`)); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
}
