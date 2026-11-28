// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestDigestRefusesWrongLength (srv-files-adapters-18): a sha256 column of another length than 32
// bytes is an internal error, never the zero digest; the mappings pass it on.
func TestDigestRefusesWrongLength(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, 31, 33} {
		if _, err := digest(make([]byte, n)); !errors.Is(err, errDigestLength) || errs.KindOf(err) != errs.KindInternal {
			t.Errorf("digest of %d bytes: err = %v, want an internal error wrapping errDigestLength", n, err)
		}
	}
	want := [32]byte{1, 2, 3}
	if got, err := digest(want[:]); err != nil || got != want {
		t.Errorf("digest of 32 bytes = %x, %v", got, err)
	}
	row := filled[sqlcgen.GetFileByIDRow](t)
	row.Sha256 = make([]byte, 31)
	if _, err := fileOf(row); !errors.Is(err, errDigestLength) {
		t.Errorf("fileOf with a short sha256: err = %v", err)
	}
	srow := filled[sqlcgen.GetSessionByIDRow](t)
	srow.DeclaredSha256 = nil
	if _, err := sessionOf(srow); !errors.Is(err, errDigestLength) {
		t.Errorf("sessionOf without a declared sha256: err = %v", err)
	}
	if _, err := partOf(sqlcgen.FileUploadPart{Sha256: make([]byte, 33)}); !errors.Is(err, errDigestLength) {
		t.Errorf("partOf with a long sha256: err = %v", err)
	}
}

// TestSessionRowCarriesTheFileRow: the session reads select every column of the file row shape
// (GetFileByID), with the same type, so sessionOf maps the file part through fileOf and a new
// column cannot be missing from one shape only. id and created_at of the file are file_id and
// file_created_at there.
func TestSessionRowCarriesTheFileRow(t *testing.T) {
	t.Parallel()
	session := reflect.TypeFor[sqlcgen.GetSessionByIDRow]()
	file := reflect.TypeFor[sqlcgen.GetFileByIDRow]()
	renamed := map[string]string{"ID": "FileID", "CreatedAt": "FileCreatedAt"}
	for i := range file.NumField() {
		f := file.Field(i)
		name := f.Name
		if r, ok := renamed[name]; ok {
			name = r
		}
		sf, ok := session.FieldByName(name)
		if !ok || sf.Type != f.Type {
			t.Errorf("the session row has no field %s of type %v", name, f.Type)
		}
	}
}

// TestMappingsFillEveryField: with every column set, fileOf and sessionOf leave no field of the
// records at its zero value, so no column is forgotten in a mapping (Thumbnails come from their
// own query).
func TestMappingsFillEveryField(t *testing.T) {
	t.Parallel()
	f, err := fileOf(filled[sqlcgen.GetFileByIDRow](t))
	if err != nil {
		t.Fatal(err)
	}
	noZeroField(t, "fileOf", reflect.ValueOf(f), "Thumbnails")
	s, err := sessionOf(filled[sqlcgen.GetSessionByIDRow](t))
	if err != nil {
		t.Fatal(err)
	}
	noZeroField(t, "sessionOf", reflect.ValueOf(s), "File")
	noZeroField(t, "sessionOf File", reflect.ValueOf(s.File), "Thumbnails")
}

// noZeroField reports every field of the struct v that holds its zero value, except skip.
func noZeroField(t *testing.T, what string, v reflect.Value, skip string) {
	t.Helper()
	for i := range v.NumField() {
		if name := v.Type().Field(i).Name; name != skip && v.Field(i).IsZero() {
			t.Errorf("%s leaves %s at its zero value", what, name)
		}
	}
}

// filled returns a row of type R whose every column holds a non-zero, non-NULL value.
func filled[R any](t *testing.T) R {
	t.Helper()
	var row R
	v := reflect.ValueOf(&row).Elem()
	for i := range v.NumField() {
		f := v.Field(i)
		switch p := f.Addr().Interface().(type) {
		case *string:
			*p = "x"
		case *int32:
			*p = 1
		case *int64:
			*p = 1
		case *bool:
			*p = true
		case *[]byte:
			*p = make([]byte, 32)
			(*p)[0] = 1
		case *pgtype.UUID:
			*p = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
		case *pgtype.Text:
			*p = pgtype.Text{String: "x", Valid: true}
		case *pgtype.Int4:
			*p = pgtype.Int4{Int32: 1, Valid: true}
		case *pgtype.Int8:
			*p = pgtype.Int8{Int64: 1, Valid: true}
		case *pgtype.Timestamptz:
			*p = pgtype.Timestamptz{Time: time.Unix(1, 0), Valid: true}
		default:
			t.Fatalf("filled: no value for field %s of type %s", v.Type().Field(i).Name, f.Type())
		}
	}
	return row
}
