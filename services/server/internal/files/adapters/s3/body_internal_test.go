// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"errors"
	"io"
	"testing"
)

// lastByteFails yields its bytes one per read; the last one comes with err, and every read
// after that reports io.EOF, as io.Reader allows.
type lastByteFails struct {
	data []byte
	err  error
}

func (r *lastByteFails) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	if len(r.data) == 0 {
		return 1, r.err
	}
	return 1, nil
}

// TestExactBodyFailsOnAnErrorWithTheLastByte pins that a read error delivered together with
// the last byte stops the body, even when the source reports io.EOF afterwards: the request
// must never complete, so the driver stores nothing.
func TestExactBodyFailsOnAnErrorWithTheLastByte(t *testing.T) {
	boom := errors.New("source broke")
	body := newExactBody(&lastByteFails{data: []byte("abc"), err: boom}, 3)
	got, err := io.ReadAll(body)
	if !errors.Is(err, boom) {
		t.Fatalf("ReadAll = %q, %v; want the source error", got, err)
	}
	if len(got) == 3 {
		t.Fatalf("all bytes were handed out despite the error")
	}
}

// TestExactBodyAcceptsEOFWithTheLastByte keeps the allowed (1, io.EOF) form working.
func TestExactBodyAcceptsEOFWithTheLastByte(t *testing.T) {
	body := newExactBody(&lastByteFails{data: []byte("abc"), err: io.EOF}, 3)
	got, err := io.ReadAll(body)
	if err != nil || string(got) != "abc" {
		t.Fatalf("ReadAll = %q, %v; want abc", got, err)
	}
	if body.failed() {
		t.Fatalf("body marked as mismatch")
	}
}
