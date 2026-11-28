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

// TestObjectStoreHasExactlyThePortOperations holds docs/spec/12-files.md "4.1 Port": exactly
// four operations, none for listing, copying, batch deletion or versioning, and the multipart
// operations of the S3 part strategy as the separate port MultipartStore; no presigned URL
// before the mode presigned is built (12 "3.3").
func TestObjectStoreHasExactlyThePortOperations(t *testing.T) {
	t.Parallel()
	for _, port := range []struct {
		typ  reflect.Type
		want []string
	}{
		{reflect.TypeFor[app.ObjectStore](), []string{"Delete", "Get", "Head", "Put"}},
		{reflect.TypeFor[app.MultipartStore](), []string{"AbortMultipart", "CompleteMultipart", "CreateMultipart", "UploadPart"}},
	} {
		var got []string
		for i := range port.typ.NumMethod() {
			got = append(got, port.typ.Method(i).Name)
		}
		slices.Sort(got)
		if !slices.Equal(got, port.want) {
			t.Fatalf("%s methods = %v, want %v", port.typ.Name(), got, port.want)
		}
		for _, m := range got {
			ft, _ := port.typ.MethodByName(m)
			if ft.Type.NumIn() == 0 || ft.Type.In(0).String() != "context.Context" {
				t.Errorf("%s.%s does not take a context first", port.typ.Name(), m)
			}
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
		app.ErrUploadNotFound, app.ErrInvalidPart}
	for i, a := range distinct {
		for j, b := range distinct {
			if i != j && errors.Is(a, b) {
				t.Errorf("%v is %v", a, b)
			}
		}
	}
}
