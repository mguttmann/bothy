// SPDX-License-Identifier: Apache-2.0

package clamdtest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Script is what the fake answers. The zero value answers PONG and "stream: OK".
type Script struct {
	// PingReply is the reply to zPING without the NUL; "" means PONG.
	PingReply string
	// Reply is the reply to zINSTREAM without the NUL; "" means "stream: OK".
	Reply string
	// Raw, when set, is sent as the reply exactly as it is (no NUL appended).
	Raw []byte
	// CloseAfterBytes > 0 sends Reply as soon as that many data bytes arrived and closes the
	// connection without reading further, as clamd does at StreamMaxLength.
	CloseAfterBytes int64
	// Delay waits before every reply.
	Delay time.Duration
	// Stall never replies; the connection stays open until the client gives up.
	Stall bool
}

// Session is one connection the fake saw.
type Session struct {
	// Command is the command up to and without its NUL.
	Command string
	// Chunks are the lengths of the INSTREAM chunks before the terminator, in order.
	Chunks []int
	// Terminated reports the zero-length chunk.
	Terminated bool
	// Data are the streamed bytes.
	Data []byte
}

// Fake is a scripted clamd on a Unix socket. It is safe for concurrent use.
type Fake struct {
	dir  string
	ln   net.Listener
	wg   sync.WaitGroup
	mu   sync.Mutex
	scr  Script
	sess []Session
	// active and maxActive count the connections open at the same time.
	active, maxActive int
	conns             map[net.Conn]struct{}
}

// NewFake listens on a socket in a fresh temporary directory (short path, below the socket path
// limit of the platform). Close stops it and removes the directory.
func NewFake() (*Fake, error) {
	dir, err := os.MkdirTemp("", "clamd")
	if err != nil {
		return nil, err
	}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", filepath.Join(dir, "clamd.sock"))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	f := &Fake{dir: dir, ln: ln, conns: map[net.Conn]struct{}{}}
	f.wg.Add(1)
	go f.serve()
	return f, nil
}

// Address is the unix:// address of the fake.
func (f *Fake) Address() string { return "unix://" + filepath.Join(f.dir, "clamd.sock") }

// SetScript replaces the script for the next connections.
func (f *Fake) SetScript(s Script) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scr = s
}

// Sessions returns a copy of the sessions seen so far.
func (f *Fake) Sessions() []Session {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Session, len(f.sess))
	copy(out, f.sess)
	return out
}

// MaxActive returns the largest number of connections that were open at the same time.
func (f *Fake) MaxActive() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxActive
}

// Active returns the number of connections open now.
func (f *Fake) Active() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

// Close stops the listener, closes open connections, waits for the handlers and removes the
// directory.
func (f *Fake) Close() error {
	err := f.ln.Close()
	f.mu.Lock()
	for c := range f.conns {
		_ = c.Close()
	}
	f.mu.Unlock()
	f.wg.Wait()
	return errors.Join(err, os.RemoveAll(f.dir))
}

func (f *Fake) serve() {
	defer f.wg.Done()
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.active++
		f.maxActive = max(f.maxActive, f.active)
		f.conns[c] = struct{}{}
		scr := f.scr
		f.mu.Unlock()
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			h := &handler{f: f, c: c, scr: scr}
			h.handle()
			_ = c.Close()
			h.record()
			f.mu.Lock()
			delete(f.conns, c)
			f.mu.Unlock()
		}()
	}
}

// handler serves one connection. It records the session and ends the connection's count in
// active before it writes a reply, so a client that has its reply already sees its session in
// Sessions and never overlaps the next connection of its own slot in MaxActive. A connection
// without a reply is recorded after it closes.
type handler struct {
	f        *Fake
	c        net.Conn
	scr      Script
	s        Session
	recorded bool
}

// record appends the session and decrements active, once.
func (h *handler) record() {
	if h.recorded {
		return
	}
	h.recorded = true
	h.f.mu.Lock()
	h.f.active--
	h.f.sess = append(h.f.sess, h.s)
	h.f.mu.Unlock()
}

// handle reads one command and answers it by the script.
func (h *handler) handle() {
	cmd, err := readCommand(h.c)
	h.s.Command = cmd
	if err != nil {
		return
	}
	switch cmd {
	case "zPING":
		reply := h.scr.PingReply
		if reply == "" {
			reply = "PONG"
		}
		h.answer([]byte(reply + "\x00"))
	case "zINSTREAM":
		if !h.stream() {
			return
		}
		reply := h.scr.Reply
		if reply == "" {
			reply = "stream: OK"
		}
		h.answer([]byte(reply + "\x00"))
	}
}

// stream reads the chunks; false means the connection ended or the script closed early (the
// reply is sent already then).
func (h *handler) stream() bool {
	var lenBuf [4]byte
	for {
		if _, err := io.ReadFull(h.c, lenBuf[:]); err != nil {
			return false
		}
		n := binary.BigEndian.Uint32(lenBuf[:])
		if n == 0 {
			h.s.Terminated = true
			return true
		}
		chunk := make([]byte, n)
		if _, err := io.ReadFull(h.c, chunk); err != nil {
			return false
		}
		h.s.Chunks = append(h.s.Chunks, int(n))
		h.s.Data = append(h.s.Data, chunk...)
		if h.scr.CloseAfterBytes > 0 && int64(len(h.s.Data)) >= h.scr.CloseAfterBytes {
			reply := h.scr.Reply
			if reply == "" {
				reply = "INSTREAM size limit exceeded. ERROR"
			}
			h.answer([]byte(reply + "\x00"))
			return false
		}
	}
}

// answer sends reply (or the script's Raw) after the script's Delay, recording the session
// first; a stalling script waits until the client closes.
func (h *handler) answer(reply []byte) {
	if h.scr.Stall {
		_, _ = io.Copy(io.Discard, h.c)
		return
	}
	if h.scr.Delay > 0 {
		time.Sleep(h.scr.Delay)
	}
	if h.scr.Raw != nil {
		reply = h.scr.Raw
	}
	h.record()
	_, _ = h.c.Write(reply)
}

// readCommand reads up to the NUL of a z command; at most 64 bytes.
func readCommand(c net.Conn) (string, error) {
	var b bytes.Buffer
	one := make([]byte, 1)
	for b.Len() < 64 {
		if _, err := io.ReadFull(c, one); err != nil {
			return b.String(), err
		}
		if one[0] == 0 {
			return b.String(), nil
		}
		b.WriteByte(one[0])
	}
	return b.String(), errors.New("clamdtest: command too long")
}
