// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

func TestEnvDriver(t *testing.T) {
	t.Parallel()
	canary := newCanary(t)
	other := newCanary(t)
	environ := []string{
		"BOTHY_SECRET_X=" + canary,
		"BOTHY_SECRET_EMPTY=",
		"BOTHY_SECRET_DUP=" + canary + "-first",
		"BOTHY_SECRET_DUP=" + other,
		"BOTHY_SECRET_EQ=" + canary + "=tail",
		"NO_EQUALS_SIGN",
		"PATH=/usr/bin",
	}
	d := secrets.NewEnvDriver(environ)
	if d.Scheme() != "env" {
		t.Fatalf("Scheme = %q, want env", d.Scheme())
	}
	// The driver reads a snapshot: a later change of the caller's slice changes nothing.
	environ[0] = "BOTHY_SECRET_X=" + other
	ctx := context.Background()
	get := func(name string) (secrets.Value, secrets.Version, error) {
		return d.Get(ctx, secrets.Ref{Scheme: "env", Path: name})
	}

	for name, want := range map[string]string{
		"BOTHY_SECRET_X":   canary,
		"BOTHY_SECRET_DUP": canary + "-first",
		"BOTHY_SECRET_EQ":  canary + "=tail",
	} {
		v, ver, err := get(name)
		if err != nil {
			t.Fatalf("Get %s: %v", name, err)
		}
		if string(v.Bytes()) != want {
			t.Errorf("Get %s returned the wrong value (length %d, want %d)", name, v.Len(), len(want))
		}
		if len(ver) != 16 {
			t.Errorf("Get %s: version %q is not 16 hex characters", name, ver)
		}
	}

	for _, name := range []string{"BOTHY_SECRET_UNSET", "BOTHY_SECRET_EMPTY", "NO_EQUALS_SIGN"} {
		v, ver, err := get(name)
		if !errors.Is(err, secrets.ErrNotFound) {
			t.Errorf("Get %s: error = %v, want ErrNotFound", name, err)
			continue
		}
		if code := secrets.ErrorCode(err); code != "not_found" {
			t.Errorf("Get %s: ErrorCode = %q, want not_found", name, code)
		}
		if !strings.Contains(err.Error(), "env:"+name) {
			t.Errorf("Get %s: message %q does not name the reference", name, err)
		}
		if v.Len() != 0 || ver != "" {
			t.Errorf("Get %s returned a value with the error", name)
		}
		assertNoSecret(t, "error", err.Error(), canary)
		assertNoSecret(t, "error", err.Error(), other)
	}

	for _, tc := range []struct {
		ref  secrets.Ref
		want error
	}{
		{secrets.Ref{Scheme: "env", Path: "lower"}, secrets.ErrInvalidRef},
		{secrets.Ref{Scheme: "env"}, secrets.ErrInvalidRef},
		{secrets.Ref{Scheme: "file", Path: "/run/bothy/secrets/x"}, secrets.ErrUnsupportedScheme},
	} {
		if _, _, err := d.Get(ctx, tc.ref); !errors.Is(err, tc.want) {
			t.Errorf("Get(%#v) error = %v, want %v", tc.ref, err, tc.want)
		}
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := d.Get(canceled, secrets.Ref{Scheme: "env", Path: "BOTHY_SECRET_X"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get with a canceled context: error = %v, want context.Canceled", err)
	}
}
