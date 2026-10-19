// SPDX-License-Identifier: Apache-2.0

package fs_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// TestFreeBytes: the free space of a temporary root is positive.
func TestFreeBytes(t *testing.T) {
	s, err := fs.Open(fs.Config{Root: newRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var _ app.FreeSpace = s
	n, err := s.FreeBytes(context.Background())
	if err != nil || n <= 0 {
		t.Fatalf("FreeBytes = %d, %v", n, err)
	}
}

// failingAtEnd yields its bytes and then an error instead of io.EOF.
type failingAtEnd struct{ r io.Reader }

var errAtEnd = errors.New("digest mismatch at the end")

func (f failingAtEnd) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if errors.Is(err, io.EOF) {
		return n, errAtEnd
	}
	return n, err
}

// TestPutFailingReaderLeavesNothing is the precondition of brief M3-03 design note 9 g: Put with
// a reader that fails at its end, after all bytes of the size, leaves no object.
func TestPutFailingReaderLeavesNothing(t *testing.T) {
	s, err := fs.Open(fs.Config{Root: newRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	key := "uploads/0192f4a0-7b1c-7000-8a00-000000000501/1"
	if err := s.Put(ctx, key, failingAtEnd{strings.NewReader("abcd")}, 4); err == nil {
		t.Fatal("Put succeeded with a failing reader")
	}
	if _, err := s.Head(ctx, key); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("Head after a failed Put = %v, want ErrNotFound", err)
	}
}
