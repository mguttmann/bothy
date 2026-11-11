// SPDX-License-Identifier: Apache-2.0

package app

import "io"

// limitedObject is the reader the use cases put between an object and a consumer that keeps
// what it reads in memory (docs/decisions.d/FIX-S3-READ-BOUND-1111.md). The length of an object
// is whatever the storage answers: neither driver bounds it, and the object may have been
// replaced behind the server. So such a consumer never reads the object directly: it reads at
// most left bytes, the stored size of the row or a named configuration value, and gets the
// error over when the object has more. At most one byte beyond the limit is taken from r, and
// after over nothing more is read.
type limitedObject struct {
	r io.Reader
	// left is the number of bytes still handed out, never negative.
	left int64
	// over is the error of an object with more bytes than the limit.
	over error
	err  error
}

// limitObject returns the reader of r that hands out at most limit bytes and fails with over
// when r has more; a negative limit counts as 0.
func limitObject(r io.Reader, limit int64, over error) *limitedObject {
	return &limitedObject{r: r, left: max(limit, 0), over: over}
}

// Read implements io.Reader.
func (l *limitedObject) Read(p []byte) (int, error) {
	if l.err != nil {
		return 0, l.err
	}
	// One byte beyond the limit shows that the object has more; the comparison cannot overflow.
	if int64(len(p))-1 > l.left {
		p = p[:l.left+1]
	}
	n, err := l.r.Read(p)
	if int64(n) > l.left {
		n = int(l.left)
		l.left, l.err = 0, l.over
		return n, l.err
	}
	l.left -= int64(n)
	return n, err
}
