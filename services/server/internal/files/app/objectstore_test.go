// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// TestObjectStoreHasExactlyThePortOperations holds docs/spec/12-dateien.md "4.1 Port": exactly
// nine operations, none for listing, copying, batch deletion or versioning.
func TestObjectStoreHasExactlyThePortOperations(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[app.ObjectStore]()
	var got []string
	for i := range typ.NumMethod() {
		got = append(got, typ.Method(i).Name)
	}
	want := []string{
		"AbortMultipart", "CompleteMultipart", "CreateMultipart", "Delete", "Get", "Head",
		"PresignUploadPart", "Put", "UploadPart",
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("ObjectStore methods = %v, want %v", got, want)
	}
	for _, m := range got {
		ft, _ := typ.MethodByName(m)
		if ft.Type.NumIn() == 0 || ft.Type.In(0).String() != "context.Context" {
			t.Errorf("%s does not take a context first", m)
		}
	}
}

// TestErrInvalidKeyIsTheDomainError: callers test one value, whichever layer refused the key.
func TestErrInvalidKeyIsTheDomainError(t *testing.T) {
	t.Parallel()
	if !errors.Is(app.ErrInvalidKey, domain.ErrInvalidKey) || !errors.Is(domain.ErrInvalidKey, app.ErrInvalidKey) {
		t.Fatal("app.ErrInvalidKey is not domain.ErrInvalidKey")
	}
	distinct := []error{app.ErrInvalidKey, app.ErrNotFound, app.ErrInvalidRange, app.ErrSizeMismatch,
		app.ErrUploadNotFound, app.ErrInvalidPart, app.ErrNotSupported}
	for i, a := range distinct {
		for j, b := range distinct {
			if i != j && errors.Is(a, b) {
				t.Errorf("%v is %v", a, b)
			}
		}
	}
}
