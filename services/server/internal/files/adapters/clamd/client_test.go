// SPDX-License-Identifier: Apache-2.0

package clamd_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// newFake starts a scripted clamd for t.
func newFake(t *testing.T) *clamdtest.Fake {
	t.Helper()
	f, err := clamdtest.NewFake()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// newClient is a client of the fake with chunk bytes chunk and parallelism par.
func newClient(t *testing.T, addr string, chunk, par int) *clamd.Client {
	t.Helper()
	c, err := clamd.New(clamd.Config{Address: addr, ChunkBytes: chunk, Timeout: 5 * time.Second, MaxConcurrent: par})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewRejectsNonUnixAddress(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"tcp://clamav:3310", "http://clamav.example.com:3310", "unix://relative/path", "unix://", "unix:///run/../x.sock", "/run/clamav/clamd.sock"} {
		_, err := clamd.New(clamd.Config{Address: addr, ChunkBytes: 1, Timeout: time.Second, MaxConcurrent: 1})
		if err == nil {
			t.Errorf("%q accepted", addr)
			continue
		}
		if !strings.Contains(err.Error(), "file.scan.clamd_address") || strings.Contains(err.Error(), strings.TrimPrefix(addr, "unix://")) && addr != "unix://" {
			t.Errorf("%q: error %q must name the key and not the value", addr, err)
		}
	}
	if _, err := clamd.New(clamd.Config{Address: "unix:///run/clamav/clamd.sock", ChunkBytes: 1, Timeout: time.Second, MaxConcurrent: 1}); err != nil {
		t.Errorf("unix:///run/clamav/clamd.sock: %v", err)
	}
}

// chunkReader returns its data in reads of the given sizes, and zero-length reads in between.
type chunkReader struct {
	data  []byte
	sizes []int
	i     int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	r.i++
	if r.i%2 == 0 {
		return 0, nil // an empty read sends nothing
	}
	n := min(len(p), len(r.data), r.sizes[(r.i/2)%len(r.sizes)])
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func TestInstreamFraming(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	const chunk = 16
	c := newClient(t, f.Address(), chunk, 1)
	data := bytes.Repeat([]byte("abcdefghij"), 10)
	v, err := c.Scan(t.Context(), &chunkReader{data: data, sizes: []int{3, 50, 1}})
	if err != nil || v.Kind != domain.VerdictClean {
		t.Fatalf("Scan = %+v, %v", v, err)
	}
	s := f.Sessions()
	if len(s) != 1 || s[0].Command != "zINSTREAM" || !s[0].Terminated || !bytes.Equal(s[0].Data, data) {
		t.Fatalf("session %+v", s)
	}
	for i, n := range s[0].Chunks {
		if n < 1 || n > chunk {
			t.Errorf("chunk %d has %d bytes, want 1 to %d", i, n, chunk)
		}
	}
}

func TestPing(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	c := newClient(t, f.Address(), 16, 1)
	if err := c.Ping(t.Context()); err != nil {
		t.Fatalf("PONG: %v", err)
	}
	f.SetScript(clamdtest.Script{PingReply: "PANG"})
	if err := c.Ping(t.Context()); !errors.Is(err, clamd.ErrNotPong) {
		t.Fatalf("PANG: %v", err)
	}
	f.SetScript(clamdtest.Script{Raw: []byte{}})
	if err := c.Ping(t.Context()); err == nil {
		t.Fatal("empty reply accepted")
	}
	if err := newClient(t, "unix:///nonexistent/clamd.sock", 16, 1).Ping(t.Context()); err == nil {
		t.Fatal("ping of a missing socket succeeded")
	}
}

func TestScanReadsReplyAfterWriteError(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	f.SetScript(clamdtest.Script{CloseAfterBytes: 256})
	c := newClient(t, f.Address(), 64, 1)
	v, err := c.Scan(t.Context(), bytes.NewReader(bytes.Repeat([]byte("x"), 8<<20)))
	if err != nil {
		t.Fatalf("Scan: %v, want the verdict of the reply", err)
	}
	if v.Kind != domain.VerdictError || v.Reason != domain.ReasonScanStreamLimitExceeded {
		t.Fatalf("verdict %+v", v)
	}
}

func TestScanTimeout(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	f.SetScript(clamdtest.Script{Stall: true})
	c := newClient(t, f.Address(), 16, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err := c.Scan(ctx, strings.NewReader("hello"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Scan = %v, want the deadline", err)
	}
}

// TestStallScans holds a scan without an answer while PING still answers: the stream reaches
// the fake in full, the scan ends only with its context, and clamd stays healthy meanwhile.
func TestStallScans(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	f.SetScript(clamdtest.Script{StallScans: true})
	c := newClient(t, f.Address(), 16, 2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.Scan(ctx, strings.NewReader("hello"))
		done <- err
	}()
	testkit.Until(t, "the scan's connection", func() bool { return f.Active() == 1 })
	if err := c.Ping(t.Context()); err != nil {
		t.Fatalf("Ping during a stalled scan: %v", err)
	}
	select {
	case err := <-done:
		t.Fatalf("the stalled scan ended with %v", err)
	default:
	}
	cancel()
	if err := testkit.Receive(t, "the scan's end", done); !errors.Is(err, context.Canceled) {
		t.Fatalf("Scan = %v, want the cancellation", err)
	}
	testkit.Until(t, "the scan's session", func() bool { return len(f.Sessions()) == 2 })
	for _, s := range f.Sessions() {
		if s.Command == "zINSTREAM" && (!s.Terminated || string(s.Data) != "hello") {
			t.Fatalf("scan session %+v, want the whole stream", s)
		}
	}
}

func TestScanMaxConcurrent(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	f.SetScript(clamdtest.Script{Delay: 150 * time.Millisecond})
	const par = 2
	c := newClient(t, f.Address(), 16, par)
	var wg sync.WaitGroup
	for range par + 1 {
		wg.Go(func() {
			if _, err := c.Scan(t.Context(), strings.NewReader("hello")); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if m := f.MaxActive(); m > par {
		t.Fatalf("%d connections at once, want at most %d", m, par)
	}
	if n := len(f.Sessions()); n != par+1 {
		t.Fatalf("%d sessions, want %d", n, par+1)
	}
	// A waiting scan ends with its context.
	f.SetScript(clamdtest.Script{Stall: true})
	c1 := newClient(t, f.Address(), 16, 1)
	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})
	go func() {
		close(started)
		_, _ = c1.Scan(ctx, strings.NewReader("a"))
	}()
	<-started
	// The fake counts the connection once the first scan holds the only slot.
	testkit.Until(t, "the first scan holds the slot", func() bool { return f.Active() == 1 })
	ctx2, cancel2 := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel2()
	if _, err := c1.Scan(ctx2, strings.NewReader("b")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting scan = %v, want the deadline", err)
	}
	cancel()
}

func TestReplyBound(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	f.SetScript(clamdtest.Script{Raw: []byte("stream: " + strings.Repeat("A", 40) + " FOUND\x00")})
	c := newClient(t, f.Address(), 16, 1)
	if _, err := c.Scan(t.Context(), strings.NewReader("x")); !errors.Is(err, domain.ErrClamdProtocol) {
		t.Fatalf("long reply = %v, want a protocol error", err)
	}
	f.SetScript(clamdtest.Script{Raw: []byte("stream: OK\x00")})
	if v, err := c.Scan(t.Context(), strings.NewReader("x")); err != nil || v.Kind != domain.VerdictClean {
		t.Fatalf("short reply = %+v, %v", v, err)
	}
}

// TestClientLogsNoSignature: the adapter has no logger and no tracer, so nothing but the verdict
// carries the signature; the errors of the client name steps, never reply text.
func TestClientLogsNoSignature(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	const sig = "Canary.Signature-4711"
	f.SetScript(clamdtest.Script{Reply: "stream: " + sig + " FOUND"})
	c := newClient(t, f.Address(), 64, 1)
	v, err := c.Scan(t.Context(), strings.NewReader("x"))
	if err != nil || v.Kind != domain.VerdictInfected || v.Signature != sig {
		t.Fatalf("verdict %+v, %v", v, err)
	}
	f.SetScript(clamdtest.Script{Raw: []byte("weird " + sig + " text\x00")})
	_, err = c.Scan(t.Context(), strings.NewReader("x"))
	if err == nil || strings.Contains(err.Error(), sig) {
		t.Fatalf("protocol error %v must not carry the reply", err)
	}
}
