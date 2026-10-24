// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"bufio"
	"context"
	"errors"
	"io"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// ctxCheckEvery is how many runes pass between two checks of the context in the text adapter.
const ctxCheckEvery = 4096

// extractText is builtin-text (14 5.2 row 1, brief M3-15 design note 5): a UTF-8 byte order mark
// is removed, a UTF-16 byte order mark (little or big endian) switches to UTF-16, everything else
// is read as UTF-8 with invalid bytes replaced. No charset guessing. It stops reading when the
// builder is full.
func extractText(ctx context.Context, r io.Reader, b *domain.TextBuilder) error {
	br := bufio.NewReader(r)
	head, err := br.Peek(3)
	if err != nil && !errors.Is(err, io.EOF) {
		return readError(err)
	}
	switch {
	case len(head) >= 3 && head[0] == 0xEF && head[1] == 0xBB && head[2] == 0xBF:
		_, _ = br.Discard(3)
	case len(head) >= 2 && head[0] == 0xFF && head[1] == 0xFE:
		_, _ = br.Discard(2)
		return utf16Text(ctx, br, b, false)
	case len(head) >= 2 && head[0] == 0xFE && head[1] == 0xFF:
		_, _ = br.Discard(2)
		return utf16Text(ctx, br, b, true)
	}
	return readLoop(ctx, br, b, func() (bool, error) {
		r, _, err := br.ReadRune()
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		if err != nil {
			return false, readError(err)
		}
		b.WriteRune(r)
		return false, nil
	})
}

// readLoop calls step until it reports the end or an error, checks the context every
// ctxCheckEvery steps and stops when the builder is full.
func readLoop(ctx context.Context, br *bufio.Reader, b *domain.TextBuilder, step func() (bool, error)) error {
	for i := 0; ; i++ {
		if i%ctxCheckEvery == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
		if b.Full() {
			return markRest(br, b)
		}
		done, err := step()
		if done || err != nil {
			return err
		}
	}
}

// markRest sets the builder truncated when input is left after it filled up.
func markRest(br *bufio.Reader, b *domain.TextBuilder) error {
	if b.Truncated() {
		return nil
	}
	_, err := br.Peek(1)
	switch {
	case err == nil:
		b.MarkTruncated()
	case !errors.Is(err, io.EOF):
		return readError(err)
	}
	return nil
}

// utf16Text decodes UTF-16 code units of the given byte order; an unpaired surrogate and an odd
// last byte become U+FFFD.
func utf16Text(ctx context.Context, br *bufio.Reader, b *domain.TextBuilder, bigEndian bool) error {
	d := utf16Decoder{b: b, pending: -1}
	return readLoop(ctx, br, b, func() (bool, error) {
		var u [2]byte
		n, err := io.ReadFull(br, u[:])
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			d.flush(n == 1)
			return true, nil
		}
		if err != nil {
			return false, readError(err)
		}
		if bigEndian {
			d.unit(rune(u[1]) | rune(u[0])<<8)
		} else {
			d.unit(rune(u[0]) | rune(u[1])<<8)
		}
		return false, nil
	})
}

// utf16Decoder pairs surrogates; pending is a high surrogate waiting for its pair, -1 for none.
type utf16Decoder struct {
	b       *domain.TextBuilder
	pending rune
}

// unit takes one code unit.
func (d *utf16Decoder) unit(u rune) {
	if d.pending >= 0 {
		p := d.pending
		d.pending = -1
		if r := utf16.DecodeRune(p, u); r != utf8.RuneError {
			d.b.WriteRune(r)
			return
		}
		d.b.WriteRune(utf8.RuneError)
	}
	switch {
	case utf16.IsSurrogate(u) && u < 0xDC00:
		d.pending = u
	case utf16.IsSurrogate(u):
		d.b.WriteRune(utf8.RuneError)
	default:
		d.b.WriteRune(u)
	}
}

// flush ends the input: a waiting high surrogate and an odd last byte become U+FFFD.
func (d *utf16Decoder) flush(oddByte bool) {
	if d.pending >= 0 {
		d.b.WriteRune(utf8.RuneError)
	}
	if oddByte {
		d.b.WriteRune(utf8.RuneError)
	}
}
