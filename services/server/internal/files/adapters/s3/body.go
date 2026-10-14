// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"errors"
	"io"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// errBodyLength is the read error exactBody returns when the source yields another number of
// bytes than announced; the request then fails before S3 has a complete body.
var errBodyLength = errors.New("body length differs from the size")

// maxEmptyReads bounds the reads of zero bytes without error that exactBody accepts in a row
// before it treats the source as broken (io.Reader allows them but discourages them).
const maxEmptyReads = 100

// exactBody streams exactly size bytes of src and never buffers more than one byte
// (docs/DECISIONS.md M3-02/8). It holds back the last byte until src reports the end: a longer
// source fails before the last byte leaves, a shorter one fails before ContentLength is reached,
// so S3 never receives a complete request and stores nothing. It hides every method of src but
// Read, so the SDK cannot seek or hash the body before sending.
type exactBody struct {
	src  io.Reader
	left int64 // bytes still to hand out

	mu       sync.Mutex
	mismatch bool
}

func newExactBody(src io.Reader, size int64) *exactBody {
	if src == nil {
		src = eofReader{}
	}
	return &exactBody{src: src, left: size}
}

// failed reports whether the source had another length than size.
func (b *exactBody) failed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mismatch
}

func (b *exactBody) fail() (int, error) {
	b.mu.Lock()
	b.mismatch = true
	b.mu.Unlock()
	return 0, errBodyLength
}

// Read implements io.Reader.
func (b *exactBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	switch {
	case b.left == 0:
		return b.atEnd()
	case b.left > 1:
		want := int64(len(p))
		if want > b.left-1 {
			want = b.left - 1
		}
		n, err := b.src.Read(p[:want])
		b.left -= int64(n)
		if errors.Is(err, io.EOF) {
			return b.fail()
		}
		if err != nil {
			return n, err
		}
		return n, nil
	}
	// The last byte: read it, then prove the end of the source before handing it out.
	var one [1]byte
	if err := b.readOne(one[:]); err != nil {
		return 0, err
	}
	if _, err := b.atEnd(); !errors.Is(err, io.EOF) {
		return 0, err
	}
	b.left = 0
	p[0] = one[0]
	return 1, nil
}

// readOne fills buf with one byte of the source or fails.
func (b *exactBody) readOne(buf []byte) error {
	for range maxEmptyReads {
		n, err := b.src.Read(buf)
		if n == 1 {
			return nil
		}
		if errors.Is(err, io.EOF) {
			_, ferr := b.fail()
			return ferr
		}
		if err != nil {
			return err
		}
	}
	return io.ErrNoProgress
}

// atEnd returns io.EOF when the source has no more bytes, else the length error.
func (b *exactBody) atEnd() (int, error) {
	var probe [1]byte
	for range maxEmptyReads {
		n, err := b.src.Read(probe[:])
		if n > 0 {
			return b.fail()
		}
		if errors.Is(err, io.EOF) {
			return 0, io.EOF
		}
		if err != nil {
			return 0, err
		}
	}
	return 0, io.ErrNoProgress
}

// eofReader is an empty body.
type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }

// sizeError returns app.ErrSizeMismatch for op when the body had another length, else nil.
func (b *exactBody) sizeError(op, key string) error {
	if b.failed() {
		return refused(op, key, app.ErrSizeMismatch)
	}
	return nil
}
