// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

// shortReader hands out n bytes of crypto/rand and then fails, so each read of the generator
// can be made to fail in turn.
type shortReader struct{ n int }

var errShortRead = errors.New("random source exhausted")

func (r *shortReader) Read(p []byte) (int, error) {
	if r.n <= 0 {
		return 0, errShortRead
	}
	if len(p) > r.n {
		p = p[:r.n]
	}
	r.n -= len(p)
	return rand.Read(p)
}

// TestPGBackRestTLSFailsWhenTheRandomSourceFails: whichever read of the random source fails,
// the generator returns an error and no partial set of files.
func TestPGBackRestTLSFailsWhenTheRandomSourceFails(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	failed := 0
	for n := 0; n < 4096; n += 8 {
		files, err := generatePGBackRestTLS("pgbackrest_tls/", &shortReader{n: n}, now)
		if err == nil {
			break
		}
		failed++
		if files != nil {
			t.Fatalf("after %d random bytes: error %v with %d files", n, err, len(files))
		}
	}
	if failed == 0 {
		t.Fatal("no read of the random source failed the generator")
	}
}
