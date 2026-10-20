// SPDX-License-Identifier: Apache-2.0

package session

import "bytes"

// Ring is the scrollback buffer of one session: the newest PTY output up to
// host_service.scrollback_bytes, in memory only (24 6.2); never a file, never logged. It is not
// safe for concurrent use; the session holds its lock.
type Ring struct {
	buf     []byte
	start   int // index of the oldest byte
	n       int // bytes held
	wrapped bool
}

// NewRing returns a ring of size bytes (host_service.scrollback_bytes).
func NewRing(size int) *Ring {
	if size < 0 {
		size = 0
	}
	return &Ring{buf: make([]byte, size)}
}

// Write appends p, dropping the oldest bytes when the ring is full.
func (r *Ring) Write(p []byte) {
	size := len(r.buf)
	if size == 0 || len(p) == 0 {
		return
	}
	if len(p) >= size {
		r.wrapped = r.wrapped || r.n > 0 || len(p) > size
		copy(r.buf, p[len(p)-size:])
		r.start, r.n = 0, size
		return
	}
	for _, b := range p {
		end := (r.start + r.n) % size
		r.buf[end] = b
		if r.n < size {
			r.n++
		} else {
			r.start = (r.start + 1) % size
			r.wrapped = true
		}
	}
}

// Snapshot returns a copy of the held bytes in order. When the ring has dropped bytes, the copy
// starts after its first line feed, so no half line, half UTF-8 sequence or half escape sequence
// opens the replay; without a line feed in a wrapped ring it is empty.
func (r *Ring) Snapshot() []byte {
	out := make([]byte, 0, r.n)
	size := len(r.buf)
	for i := range r.n {
		out = append(out, r.buf[(r.start+i)%size])
	}
	if !r.wrapped {
		return out
	}
	i := bytes.IndexByte(out, '\n')
	if i < 0 {
		return []byte{}
	}
	return out[i+1:]
}

// Reset drops every byte (the session ended).
func (r *Ring) Reset() {
	clear(r.buf)
	r.start, r.n, r.wrapped = 0, 0, false
}
