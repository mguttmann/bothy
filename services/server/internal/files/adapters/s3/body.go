// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
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

// errNoObjectLength is the error of an object answer that announces no length.
var errNoObjectLength = errors.New("object answer without a length")

// errNotTheRange is the error of an answer to a ranged GetObject that is not the range the
// request asked for.
var errNotTheRange = errors.New("object answer is not the range asked for")

// answerBodies wraps the HTTP client of the driver and bounds the body of every answer
// (19 SEC-NET-5, docs/decisions.d/AUD-srv-files-01.md, amending docs/DECISIONS.md M3-02/1, and
// docs/decisions.d/FIX-S3-READ-BOUND-1111.md):
//
//   - The body of a successful GET is the object or the range the caller asked for; the driver
//     sends no other GET (coreAPI). It is as long as the store announces, and net/http ends the
//     body at that length. No configuration value bounds it: an export package has no size limit
//     (20 5.2), a converted original may be larger than its upload, and file.max_upload_bytes
//     may have been lowered since an object was stored. An answer without a length has no bound
//     and is refused with errNoObjectLength. A caller that keeps what it reads in memory bounds
//     the read itself, with the stored size of its row or a configuration value.
//   - The answer to a GET with a range is that range: status 206 and a Content-Range that starts
//     at the first byte asked for, ends at or before the last one and has the length the answer
//     announces. Anything else, such as the whole object with status 200, is refused with
//     errNotTheRange, so a ranged read never yields other or more bytes than the caller asked for.
//   - Every other body (an error document, the result of a multipart call) is read into memory
//     by the SDK and is at most max bytes; a longer one fails with egress.ErrResponseTooLarge.
type answerBodies struct {
	next httpDoer
	// max is egress.max_response_bytes, the largest body of an answer that is not an object.
	max int64
}

// Do implements httpDoer.
func (a answerBodies) Do(req *http.Request) (*http.Response, error) {
	resp, err := a.next.Do(req)
	if err != nil {
		return resp, err
	}
	if req.Method == http.MethodGet && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if err := checkObjectAnswer(req, resp); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		return resp, nil
	}
	resp.Body = &cappedBody{rc: resp.Body, left: a.max}
	return resp, nil
}

// The byte range forms of RFC 9110 the driver sends and accepts: the request header Range
// "bytes=<first>-<last>" (Store.Get writes no other form) and the answer header Content-Range
// "bytes <first>-<last>/<length of the object or *>".
const (
	rangeUnit        = "bytes="
	contentRangeUnit = "bytes "
)

// checkObjectAnswer checks the successful answer resp to the GetObject request req: it announces
// a length (errNoObjectLength), and when req asks for a range it is that range (errNotTheRange).
func checkObjectAnswer(req *http.Request, resp *http.Response) error {
	if resp.ContentLength < 0 {
		return errNoObjectLength
	}
	asked := req.Header.Get("Range")
	if asked == "" {
		return nil
	}
	first, last, ok := askedRange(asked)
	gotFirst, gotLast, gotOK := answeredRange(resp.Header.Get("Content-Range"))
	if resp.StatusCode != http.StatusPartialContent || !ok || !gotOK ||
		gotFirst != first || gotLast > last || resp.ContentLength != gotLast-gotFirst+1 {
		return errNotTheRange
	}
	return nil
}

// askedRange parses the header Range of a request of the driver.
func askedRange(h string) (first, last int64, ok bool) {
	span, found := strings.CutPrefix(h, rangeUnit)
	if !found {
		return 0, 0, false
	}
	return byteSpan(span)
}

// answeredRange parses the header Content-Range of an answer; what follows the slash (the length
// of the object, or "*") is not read.
func answeredRange(h string) (first, last int64, ok bool) {
	v, found := strings.CutPrefix(h, contentRangeUnit)
	span, _, hasLength := strings.Cut(v, "/")
	if !found || !hasLength {
		return 0, 0, false
	}
	return byteSpan(span)
}

// byteSpan parses "<first>-<last>", two decimal numbers without sign with first <= last.
func byteSpan(s string) (first, last int64, ok bool) {
	a, b, found := strings.Cut(s, "-")
	if !found {
		return 0, 0, false
	}
	f, errFirst := strconv.ParseUint(a, 10, 63)
	l, errLast := strconv.ParseUint(b, 10, 63)
	if errFirst != nil || errLast != nil || f > l {
		return 0, 0, false
	}
	return int64(f), int64(l), true // #nosec G115 -- both numbers were parsed with 63 bits
}

// cappedBody fails a body that holds more than left bytes with egress.ErrResponseTooLarge, as
// the egress client does with its own limit: the bytes up to the limit come with the error, and
// the error stays.
type cappedBody struct {
	rc   io.ReadCloser
	left int64
	err  error
}

// Read implements io.Reader.
func (b *cappedBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	n, err := b.rc.Read(p)
	if int64(n) > b.left {
		n = int(b.left)
		b.left = 0
		b.err = egress.ErrResponseTooLarge
		return n, b.err
	}
	b.left -= int64(n)
	return n, err
}

// Close implements io.Closer.
func (b *cappedBody) Close() error { return b.rc.Close() }
