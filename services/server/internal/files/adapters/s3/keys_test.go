// SPDX-License-Identifier: Apache-2.0

package s3_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// objectNameCases are the table of TestObjectName and the seed corpus of FuzzObjectName.
var objectNameCases = []struct {
	prefix, key string
	want        string // "" means ErrInvalidKey
}{
	{"", "f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/orig-1", "f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/orig-1"},
	{"env-a", "f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/small", "env-a/f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/small"},
	{"tenant/prod", "f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/poster", "tenant/prod/f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/poster"},
	{"", "..", ""},
	{"", "../x", ""},
	{"", "a/../b", ""},
	{"", "/abs", ""},
	{"", "trailing/", ""},
	{"", "a//b", ""},
	{"", "Upper", ""},
	{"", "with space", ""},
	{"", "café", ""},
	{"", "", ""},
	{"env-a", "", ""},
	{"/env", "x", ""},
	{"env/", "x", ""},
	{"env//a", "x", ""},
	{"..", "x", ""},
	{"Env", "x", ""},
	{"env a", "x", ""},
}

// TestObjectName holds the key mapping of docs/DECISIONS.md M3-02/7: the key grammar of all
// drivers for key and prefix, joined with one "/".
func TestObjectName(t *testing.T) {
	t.Parallel()
	for _, c := range objectNameCases {
		got, err := s3.ObjectName(c.prefix, c.key)
		if c.want == "" {
			if !errors.Is(err, app.ErrInvalidKey) {
				t.Errorf("ObjectName(%q, %q) = %q, %v; want ErrInvalidKey", c.prefix, c.key, got, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ObjectName(%q, %q) = %q, %v; want %q", c.prefix, c.key, got, err, c.want)
		}
	}
	id := ids.New()
	keys := []string{}
	if k, err := domain.OriginalKey(id, 3); err == nil {
		keys = append(keys, k)
	}
	for _, v := range []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster} {
		k, err := domain.VariantKey(id, v)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	for _, k := range keys {
		if got, err := s3.ObjectName("", k); err != nil || got != k {
			t.Errorf("ObjectName of schema key %q = %q, %v", k, got, err)
		}
	}
}

// FuzzObjectName: for any prefix and key the result is an error or a name in the key grammar that
// starts with the prefix and has no empty or ".." segment.
func FuzzObjectName(f *testing.F) {
	for _, c := range objectNameCases {
		f.Add(c.prefix, c.key)
	}
	f.Fuzz(func(t *testing.T, prefix, key string) {
		got, err := s3.ObjectName(prefix, key)
		if err != nil {
			if !errors.Is(err, app.ErrInvalidKey) {
				t.Fatalf("error %v does not wrap ErrInvalidKey", err)
			}
			// Short keys occur in any text; a long one would be a repetition.
			if len(key) >= 16 && strings.Contains(err.Error(), key) {
				t.Fatalf("error %q repeats the key", err)
			}
			return
		}
		if verr := domain.ValidateKey(got); verr != nil {
			t.Fatalf("name %q outside the grammar: %v", got, verr)
		}
		if prefix != "" && !strings.HasPrefix(got, prefix+"/") {
			t.Fatalf("name %q does not start with prefix %q", got, prefix)
		}
		if !strings.HasSuffix(got, key) {
			t.Fatalf("name %q does not end with key %q", got, key)
		}
		for _, seg := range strings.Split(got, "/") {
			if seg == "" || seg == ".." || seg == "." {
				t.Fatalf("name %q has segment %q", got, seg)
			}
		}
	})
}
