// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// cursorVersion is the format version of a cursor; a cursor of another version is invalid.
const cursorVersion = 1

// maxCursorLen bounds the text of a cursor, so a forged one cannot make the server decode much.
const maxCursorLen = 2048

// cursorFingerprintDomain separates the cursor fingerprint from any other SHA-256 use.
const cursorFingerprintDomain = "bothy/cursor/v1\x00"

// EncodeCursor returns the opaque cursor of one page position (06 "Pagination" item 2): base64url
// without padding of the version, the list name, the keyset values key as JSON, and a SHA-256
// fingerprint over version, list, binding and key. binding is the canonical encoding of the filter
// and sort of the request, including the caller's principal ID, which the caller builds; a cursor
// therefore works only for the same list, the same filters and the same principal. There is no
// signature: a client that forges keyset values only moves its position inside the same bound
// and filtered list. Every list uses this codec (PLAN M1-18).
//
// The error reports a list name that is empty or longer than 255 bytes, or a key that does not
// marshal to JSON or makes the cursor longer than 2048 characters.
//
// Example:
//
//	binding := []byte(principalID.String() + "\x00sort=created_at")
//	next, err := httpserver.EncodeCursor("threads", binding, threadKey{CreatedAt: t, ID: id})
func EncodeCursor(list string, binding []byte, key any) (string, error) {
	if list == "" || len(list) > 255 {
		return "", errors.New("httpserver: cursor list name must have 1 to 255 bytes")
	}
	k, err := json.Marshal(key)
	if err != nil {
		return "", fmt.Errorf("httpserver: cursor key: %w", err)
	}
	var b bytes.Buffer
	b.WriteByte(cursorVersion)
	b.WriteByte(byte(len(list))) //nolint:gosec // G115: len(list) is at most 255, checked above
	b.WriteString(list)
	b.Write(binary.AppendUvarint(nil, uint64(len(k))))
	b.Write(k)
	sum := cursorSum(list, binding, k)
	b.Write(sum[:])
	s := base64.RawURLEncoding.EncodeToString(b.Bytes())
	if len(s) > maxCursorLen {
		return "", errors.New("httpserver: cursor key too large")
	}
	return s, nil
}

// DecodeCursor checks the cursor s against list and binding and unmarshals its keyset values into
// key (a pointer). Every mismatch is 400 cursor_invalid: another version, another list, another
// binding or principal, a changed byte, trailing bytes, or a key that does not unmarshal into key
// without unknown members.
//
// Example:
//
//	var k threadKey
//	if err := httpserver.DecodeCursor("threads", binding, q.Cursor, &k); err != nil {
//		return err // 400 cursor_invalid
//	}
func DecodeCursor(list string, binding []byte, s string, key any) error {
	invalid := errs.New(errs.CodeCursorInvalid, "")
	if s == "" || len(s) > maxCursorLen {
		return invalid
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return invalid
	}
	k, ok := cursorKey(raw, list, binding)
	if !ok {
		return invalid
	}
	dec := json.NewDecoder(bytes.NewReader(k))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(key); err != nil || dec.More() {
		return invalid
	}
	return nil
}

// cursorKey parses a decoded cursor and returns its key bytes when version, list name, length,
// fingerprint and size all match; the fingerprint is compared in constant time.
func cursorKey(raw []byte, list string, binding []byte) ([]byte, bool) {
	r := bytes.NewReader(raw)
	if version, err := r.ReadByte(); err != nil || version != cursorVersion {
		return nil, false
	}
	if n, err := r.ReadByte(); err != nil || int(n) != len(list) {
		return nil, false
	}
	name := make([]byte, len(list))
	if _, err := io.ReadFull(r, name); err != nil || string(name) != list {
		return nil, false
	}
	size, err := binary.ReadUvarint(r)
	if err != nil || size > uint64(r.Len()) { //nolint:gosec // G115: Len of a bytes.Reader is never negative
		return nil, false
	}
	k := make([]byte, size)
	var got [sha256.Size]byte
	if _, err := io.ReadFull(r, k); err != nil {
		return nil, false
	}
	if _, err := io.ReadFull(r, got[:]); err != nil || r.Len() != 0 {
		return nil, false
	}
	want := cursorSum(list, binding, k)
	return k, subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// cursorSum is the fingerprint of a cursor: SHA-256 over a domain label and the length-prefixed
// list, binding and key, so no two different inputs share one encoding.
func cursorSum(list string, binding, key []byte) [sha256.Size]byte {
	h := sha256.New()
	h.Write([]byte(cursorFingerprintDomain))
	for _, part := range [][]byte{[]byte(list), binding, key} {
		h.Write(binary.AppendUvarint(nil, uint64(len(part))))
		h.Write(part)
	}
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}
