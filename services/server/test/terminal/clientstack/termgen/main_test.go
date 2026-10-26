// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock advances only when the generator sleeps.
type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(d time.Duration) {
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
}

func TestRateWritesNumberedLinesOnTheSchedule(t *testing.T) {
	c := &fakeClock{now: time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)}
	var out bytes.Buffer
	if err := Rate(&out, c, 100*time.Millisecond, 3, 40); err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(out.String(), "\n")
	if got := lines[:4]; got[0] != "TERMGEN 000001 "+strings.Repeat("r", 25)+"\n" ||
		!strings.HasPrefix(got[1], "TERMGEN 000002 ") || !strings.HasPrefix(got[2], "TERMGEN 000003 ") || got[3] != doneLine {
		t.Fatalf("lines %q", lines)
	}
	for _, l := range lines[:3] {
		if len(l) != 41 {
			t.Fatalf("line %q has %d bytes, want 41", l, len(l))
		}
	}
	// The first line goes at once, the others one interval apart.
	if want := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond}; fmt.Sprint(c.sleeps) != fmt.Sprint(want) {
		t.Fatalf("sleeps %v, want %v", c.sleeps, want)
	}
}

// lateClock spends extra time on every write, as a loaded machine would.
type lateClock struct{ fakeClock }

type lateWriter struct {
	c    *lateClock
	late time.Duration
	buf  bytes.Buffer
}

func (w *lateWriter) Write(p []byte) (int, error) {
	w.c.now = w.c.now.Add(w.late)
	return w.buf.Write(p)
}

func TestRateKeepsTheAbsoluteSchedule(t *testing.T) {
	c := &lateClock{fakeClock{now: time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)}}
	w := &lateWriter{c: c, late: 30 * time.Millisecond}
	if err := Rate(w, c, 100*time.Millisecond, 3, 40); err != nil {
		t.Fatal(err)
	}
	// A write that took 30 ms shortens the next wait instead of shifting the schedule.
	if want := []time.Duration{70 * time.Millisecond, 70 * time.Millisecond}; fmt.Sprint(c.sleeps) != fmt.Sprint(want) {
		t.Fatalf("sleeps %v, want %v", c.sleeps, want)
	}
}

func TestChunkBodyHasTheExactSizeInWholeLines(t *testing.T) {
	for _, size := range []int{81, 100, 4096, 262144, 262145} {
		b := ChunkBody(3, size, 80)
		if len(b) != size {
			t.Fatalf("size %d: got %d bytes", size, len(b))
		}
		if b[len(b)-1] != '\n' {
			t.Fatalf("size %d: the chunk does not end with a line feed", size)
		}
		if !bytes.HasPrefix(b, []byte("BURST 03 000000 ")) {
			t.Fatalf("size %d: first line %q", size, b[:20])
		}
		if bytes.Contains(b, []byte("TERMGEN")) {
			t.Fatalf("size %d: the body carries a marker", size)
		}
	}
}

func TestBurstWritesOneChunkPerAcknowledgement(t *testing.T) {
	pr, pw := io.Pipe()
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- Burst(&out, bufio.NewReader(pr), 1000, 2, 80) }()

	// Nothing before the first acknowledgement.
	if out.Len() != 0 {
		t.Fatal("output before the first acknowledgement")
	}
	// Two chunks and the line that ends the generator: it never exits before the app drew the
	// last chunk.
	go func() {
		for range 3 {
			if _, err := pw.Write([]byte("\n")); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { _ = pw.Close() })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	want := string(ChunkBody(0, 1000, 80)) + EndLine(0, 1000) + string(ChunkBody(1, 1000, 80)) + EndLine(1, 1000) + doneLine
	if out.String() != want {
		t.Fatalf("output differs: %d bytes, want %d", out.Len(), len(want))
	}
	if EndLine(1, 1000) != "TERMGEN-END 1 1000\n" {
		t.Fatalf("end line %q", EndLine(1, 1000))
	}
}

func TestBurstWithoutAcknowledgementEnds(t *testing.T) {
	var out bytes.Buffer
	if err := Burst(&out, bufio.NewReader(strings.NewReader("\n")), 100, 2, 40); !errors.Is(err, errNoAck) {
		t.Fatalf("err %v, want errNoAck", err)
	}
	// Without the last line the generator does not end with TERMGEN-DONE.
	out.Reset()
	if err := Burst(&out, bufio.NewReader(strings.NewReader("\n")), 100, 1, 40); !errors.Is(err, errNoAck) {
		t.Fatalf("err %v, want errNoAck", err)
	}
	if strings.Contains(out.String(), doneLine) {
		t.Fatal("done before the last line")
	}
}

func TestRunChecksItsArguments(t *testing.T) {
	c := &fakeClock{}
	for _, args := range [][]string{
		nil, {"other"}, {"rate", "--lines", "0"}, {"rate", "--interval", "0s"}, {"rate", "--width", "3"},
		{"burst", "--chunks", "0"}, {"burst", "--chunk-bytes", "10"}, {"burst", "--bogus"},
	} {
		var stderr bytes.Buffer
		if rc := run(args, strings.NewReader(""), io.Discard, &stderr, c); rc != 2 {
			t.Fatalf("%v: exit %d, want 2", args, rc)
		}
	}
	var out bytes.Buffer
	if rc := run([]string{"rate", "--lines", "2", "--interval", "1ms"}, strings.NewReader(""), &out, io.Discard, c); rc != 0 {
		t.Fatalf("rate: exit %d", rc)
	}
	if !strings.HasSuffix(out.String(), doneLine) {
		t.Fatalf("rate output %q", out.String())
	}
	if rc := run([]string{"burst", "--chunks", "1"}, strings.NewReader(""), io.Discard, io.Discard, c); rc != 1 {
		t.Fatalf("burst without stdin: exit %d, want 1", rc)
	}
}

// syncBuffer is a bytes.Buffer for one writing goroutine and a reading test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
