// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

func TestParseRef(t *testing.T) {
	t.Parallel()
	const dir = "/run/bothy/secrets"
	valid := []struct {
		raw, dir string
		want     secrets.Ref
	}{
		{"file:/run/bothy/secrets/db_app_password", dir, secrets.Ref{Scheme: "file", Path: "/run/bothy/secrets/db_app_password"}},
		{"file:<secrets.dir>/x", dir, secrets.Ref{Scheme: "file", Path: "/run/bothy/secrets/x"}},
		{"file:<secrets.dir>/a<secrets.dir>", dir, secrets.Ref{Scheme: "file", Path: "/run/bothy/secrets/a/run/bothy/secrets"}},
		{"file:/run/bothy/secrets/a:b", dir, secrets.Ref{Scheme: "file", Path: "/run/bothy/secrets/a:b"}},
		{"env:BOTHY_SECRET_X", dir, secrets.Ref{Scheme: "env", Path: "BOTHY_SECRET_X"}},
		{"env:_X9", "", secrets.Ref{Scheme: "env", Path: "_X9"}},
	}
	for _, tc := range valid {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			got, err := secrets.ParseRef(tc.raw, tc.dir)
			if err != nil {
				t.Fatalf("ParseRef: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ParseRef = %#v, want %#v", got, tc.want)
			}
			if !strings.Contains(tc.raw, "<secrets.dir>") && got.String() != tc.raw {
				t.Fatalf("String() = %q, want the input %q back", got.String(), tc.raw)
			}
		})
	}

	invalid := []struct {
		raw, dir string
		want     error
	}{
		{"file:relative", dir, secrets.ErrInvalidRef},
		{"file:/a/../b", dir, secrets.ErrInvalidRef},
		{"file:/a/./b", dir, secrets.ErrInvalidRef},
		{"file:/a//b", dir, secrets.ErrInvalidRef},
		{"file:/a/", dir, secrets.ErrInvalidRef},
		{"file:/a\x00b", dir, secrets.ErrInvalidRef},
		{"file:", dir, secrets.ErrInvalidRef},
		{"file:<secrets.dir>/x", "", secrets.ErrInvalidRef},
		{"file:<secrets.dir>/x", "run/bothy", secrets.ErrInvalidRef},
		{"file:<secrets.dir>/x", "/run/bothy/", secrets.ErrInvalidRef},
		{"env:lower", dir, secrets.ErrInvalidRef},
		{"env:1ABC", dir, secrets.ErrInvalidRef},
		{"env:", dir, secrets.ErrInvalidRef},
		{"azkv://v/s", dir, secrets.ErrUnsupportedScheme},
		{"awssm://r/s", dir, secrets.ErrUnsupportedScheme},
		{"foo:bar", dir, secrets.ErrInvalidRef},
		{"nocolon", dir, secrets.ErrInvalidRef},
		{":x", dir, secrets.ErrInvalidRef},
		{"", dir, secrets.ErrInvalidRef},
	}
	for _, tc := range invalid {
		t.Run(fmt.Sprintf("%q/%q", tc.raw, tc.dir), func(t *testing.T) {
			t.Parallel()
			got, err := secrets.ParseRef(tc.raw, tc.dir)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ParseRef error = %v, want %v", err, tc.want)
			}
			if got != (secrets.Ref{}) {
				t.Fatalf("ParseRef returned %#v with an error, want the zero Ref", got)
			}
			var re *secrets.RefError
			if !errors.As(err, &re) {
				t.Fatalf("error %T is not a *RefError", err)
			}
			if re.Ref != "" {
				t.Fatalf("RefError.Ref = %q, want empty for an unparsed reference", re.Ref)
			}
			msg := err.Error()
			if tc.raw != "" && strings.Contains(msg, tc.raw) {
				t.Fatalf("message %q quotes the reference", msg)
			}
			if _, rest, _ := strings.Cut(tc.raw, ":"); rest != "" && strings.Contains(msg, rest) {
				t.Fatalf("message %q contains the text after the scheme %q", msg, rest)
			}
		})
	}
}

// TestParseRefRejectsTheRootAsSecretsDir: like NewProvider and the loader, ParseRef refuses / as
// secrets.dir with the secrets.dir detail instead of building file://x.
func TestParseRefRejectsTheRootAsSecretsDir(t *testing.T) {
	t.Parallel()
	_, err := secrets.ParseRef("file:<secrets.dir>/x", "/")
	if !errors.Is(err, secrets.ErrInvalidRef) {
		t.Fatalf("ParseRef error = %v, want ErrInvalidRef", err)
	}
	if !strings.Contains(err.Error(), "<secrets.dir> must be an absolute clean path") {
		t.Fatalf("message %q does not name secrets.dir", err.Error())
	}
}

func TestRefErrorMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  *secrets.RefError
		want string
	}{
		{
			"ref and detail",
			&secrets.RefError{Ref: "file:/run/bothy/secrets/x", Err: secrets.ErrNotFound, Detail: "file does not exist"},
			"secrets: file:/run/bothy/secrets/x: secret not found: file does not exist",
		},
		{
			"ref without detail",
			&secrets.RefError{Ref: "env:BOTHY_SECRET_X", Err: secrets.ErrNotFound},
			"secrets: env:BOTHY_SECRET_X: secret not found",
		},
		{
			"detail without ref",
			&secrets.RefError{Err: secrets.ErrInvalidRef, Detail: "must have the form <scheme>:<path>"},
			"secrets: invalid reference: must have the form <scheme>:<path>",
		},
		{
			"control character in the ref",
			&secrets.RefError{Ref: "file:/run/a\nb", Err: secrets.ErrInsecureFile, Detail: "not a regular file"},
			`secrets: "file:/run/a\nb": insecure secret file: not a regular file`,
		},
		{
			"no sentinel",
			&secrets.RefError{Ref: "env:BOTHY_SECRET_X"},
			"secrets: env:BOTHY_SECRET_X: error",
		},
		{
			"leading space in the ref",
			&secrets.RefError{Ref: "file:/run/ a", Err: secrets.ErrReadFailed},
			`secrets: file:/run/ a: secret could not be read`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
			if tc.err.Err != nil && !errors.Is(tc.err, tc.err.Err) {
				t.Fatalf("errors.Is(err, %v) = false, want true through Unwrap", tc.err.Err)
			}
		})
	}
}

func TestErrorCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{secrets.ErrInvalidRef, "invalid_ref"},
		{&secrets.RefError{Err: secrets.ErrUnsupportedScheme}, "unsupported_scheme"},
		{fmt.Errorf("wrapped: %w", &secrets.RefError{Ref: "env:X", Err: secrets.ErrNotFound}), "not_found"},
		{&secrets.RefError{Ref: "file:/x", Err: secrets.ErrInsecureFile}, "insecure_file"},
		{secrets.ErrReadFailed, "read_failed"},
		{errors.New("some driver failure"), "read_failed"},
		{context.Canceled, "read_failed"},
	}
	for _, tc := range tests {
		if got := secrets.ErrorCode(tc.err); got != tc.want {
			t.Errorf("ErrorCode(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
