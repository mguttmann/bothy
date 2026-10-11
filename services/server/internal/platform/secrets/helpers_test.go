// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// newCanary returns a value no output may ever contain: EXAMPLE marks it as test data, the random
// part rules out a match with fixed text in a message.
func newCanary(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "EXAMPLE-canary-" + hex.EncodeToString(b)
}

// assertNoSecret fails when out contains the canary in any rendering a leak would take: the
// text itself, hex in both cases (verbs %x and %X), base64 (encoding/json of a []byte) and the
// decimal byte list that reflection prints for a []byte field.
func assertNoSecret(t *testing.T, label, out, canary string) {
	t.Helper()
	raw := []byte(canary)
	digits := make([]string, len(raw))
	for i, c := range raw {
		digits[i] = strconv.Itoa(int(c))
	}
	decimal := "[" + strings.Join(digits, " ") + "]"
	forms := map[string]string{
		"text":          canary,
		"hex":           hex.EncodeToString(raw),
		"HEX":           strings.ToUpper(hex.EncodeToString(raw)),
		"base64":        base64.StdEncoding.EncodeToString(raw),
		"decimal list":  decimal,
		"decimal bytes": strings.Trim(decimal, "[]"),
	}
	for name, form := range forms {
		if strings.Contains(out, form) {
			t.Errorf("%s: output contains the secret as %s: %q", label, name, out)
		}
	}
}

// fakeDriver is a Driver whose behaviour a test sets per call; it counts the calls so a test can
// tell a cache hit from a read.
type fakeDriver struct {
	scheme string
	calls  atomic.Int32
	get    func(ctx context.Context, call int32, ref secrets.Ref) (secrets.Value, secrets.Version, error)
}

func (f *fakeDriver) Scheme() string { return f.scheme }

func (f *fakeDriver) Get(ctx context.Context, ref secrets.Ref) (secrets.Value, secrets.Version, error) {
	call := f.calls.Add(1)
	return f.get(ctx, call, ref)
}

// countingHandler is a slog.Handler that only counts records, so a test can tell how many lines
// a code path would log.
type countingHandler struct {
	n atomic.Int32
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *countingHandler) Handle(context.Context, slog.Record) error {
	h.n.Add(1)
	return nil
}

func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *countingHandler) WithGroup(string) slog.Handler { return h }
