// SPDX-License-Identifier: Apache-2.0

package hmackey_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/hmackey"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// source returns a fixed value or error for every reference.
type source struct {
	value string
	err   error
}

func (s source) Resolve(context.Context, string) (secrets.Value, secrets.Version, error) {
	if s.err != nil {
		return secrets.Value{}, "", s.err
	}
	return secrets.NewValue([]byte(s.value)), "v1", nil
}

// TestMasterKey: the hex value of generator key decodes to the key; a short or non-hex value and
// a resolution error fail without repeating the value.
func TestMasterKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	key, err := hmackey.New(source{value: strings.Repeat("a5", 32) + "\n"}, "file:<secrets.dir>/webhook_hmac_master_key").Key(ctx)
	if err != nil || !bytes.Equal(key, bytes.Repeat([]byte{0xa5}, 32)) {
		t.Fatalf("Key = %x %v", key, err)
	}
	for _, bad := range []string{strings.Repeat("a5", 16), "zz" + strings.Repeat("a5", 31), ""} {
		m := hmackey.New(source{value: bad}, "file:x")
		if err := m.Check(ctx); err == nil || (bad != "" && strings.Contains(err.Error(), bad)) {
			t.Fatalf("value %q: %v", bad, err)
		}
	}
	missing := errors.New("missing")
	if err := hmackey.New(source{err: missing}, "file:x").Check(ctx); !errors.Is(err, missing) {
		t.Fatalf("resolution error %v", err)
	}
}
