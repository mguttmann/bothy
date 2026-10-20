// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"testing"
	"time"
)

// startAgent runs RunAgent against l with dialog d and returns the service end of its first
// connection.
func startAgent(t *testing.T, l *memListener, d Dialog) (net.Conn, *bufio.Reader) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunAgent(ctx, AgentOptions{Endpoint: "mem", Dialog: d, Log: slog.New(slog.DiscardHandler), Dial: l.dialAs(robin)})
	}()
	t.Cleanup(func() {
		cancel()
		if err := waitFor(t, done, "RunAgent to return"); err != nil {
			t.Errorf("RunAgent = %v after the end of ctx", err)
		}
	})
	c := accept(t, l)
	return c, bufio.NewReader(c)
}

func accept(t *testing.T, l *memListener) net.Conn {
	t.Helper()
	ch := make(chan net.Conn, 1)
	go func() {
		c, err := l.Accept()
		if err == nil {
			ch <- c
		}
	}()
	c := waitFor(t, ch, "the agent to connect")
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestAgentAnswersAndIgnoresMalformed(t *testing.T) {
	l := newMemListener()
	t.Cleanup(func() { _ = l.Close() })
	d := &fixedDialog{allow: true}
	c, r := startAgent(t, l, d.dialog)
	for _, line := range []string{
		`{"op":"confirm","id":"a","kind":"open","profile":"shell","session_user":"bothy-s-alex","timeout_ms":1000,"x":1}`,
		`{"op":"confirm","id":"b","kind":"delete","profile":"shell","session_user":"bothy-s-alex","timeout_ms":1000}`,
		`{"op":"confirm","id":"c","kind":"open","profile":"shell","session_user":"bothy-s-alex","timeout_ms":0}`,
		`{"op":"confirm","id":"","kind":"open","profile":"shell","session_user":"bothy-s-alex","timeout_ms":1000}`,
		`{"op":"confirm","id":"d","kind":"open","profile":"","session_user":"bothy-s-alex","timeout_ms":1000}`,
		`{"op":"confirm","id":"e","kind":null,"profile":"shell","session_user":"bothy-s-alex","timeout_ms":1000}`,
		`{"op":"shutdown"}`,
		`not json`,
	} {
		writeLine(t, c, line)
	}
	writeLine(t, c, `{"op":"confirm","id":"f","kind":"attach","profile":"claude","session_user":"bothy-s-sam","timeout_ms":1000}`)
	m := readMsg(t, c, r)
	if m["op"] != "answer" || m["id"] != "f" || m["allow"] != true || len(m) != 3 {
		t.Fatalf("answer %v", m)
	}
	if got := d.requests(); len(got) != 1 || got[0] != reqAttach {
		t.Fatalf("dialog saw %+v, want only %+v", got, reqAttach)
	}
}

func TestAgentCancelEndsDialog(t *testing.T) {
	l := newMemListener()
	t.Cleanup(func() { _ = l.Close() })
	b := newBlockingDialog()
	c, r := startAgent(t, l, b.dialog)
	writeLine(t, c, `{"op":"confirm","id":"g","kind":"open","profile":"shell","session_user":"bothy-s-alex","timeout_ms":60000}`)
	waitFor(t, b.started, "the dialog")
	writeLine(t, c, `{"op":"cancel","id":"g"}`)
	waitFor(t, b.cancelled, "the dialog to end on cancel")
	// No answer follows a cancel: the next line the service reads answers the next request.
	d2 := `{"op":"confirm","id":"h","kind":"open","profile":"shell","session_user":"bothy-s-alex","timeout_ms":1}`
	writeLine(t, c, d2)
	waitFor(t, b.started, "the second dialog")
	m := readMsg(t, c, r)
	if m["id"] != "h" || m["allow"] != false {
		t.Fatalf("answer %v, want a denial of h after its timeout", m)
	}
}

func TestAgentReconnects(t *testing.T) {
	l := newMemListener()
	t.Cleanup(func() { _ = l.Close() })
	b := newBlockingDialog()
	c, _ := startAgent(t, l, b.dialog)
	writeLine(t, c, `{"op":"confirm","id":"i","kind":"open","profile":"shell","session_user":"bothy-s-alex","timeout_ms":60000}`)
	waitFor(t, b.started, "the dialog")
	_ = c.Close()
	waitFor(t, b.cancelled, "the dialog to end with the connection")
	accept(t, l)
}

func TestDialogText(t *testing.T) {
	cases := map[Request]string{
		reqOpen:   "Open a terminal session with the profile shell as the session user bothy-s-alex?",
		reqAttach: "Attach to the terminal session with the profile claude of the session user bothy-s-sam?",
	}
	for r, want := range cases {
		if got := dialogText(r); got != want {
			t.Errorf("dialogText(%+v) = %q, want %q", r, got, want)
		}
	}
}

func TestDialogSeconds(t *testing.T) {
	for d, want := range map[time.Duration]int{0: 1, time.Millisecond: 1, time.Second: 1, 1500 * time.Millisecond: 2, time.Minute: 60} {
		if got := dialogSeconds(d); got != want {
			t.Errorf("dialogSeconds(%v) = %d, want %d", d, got, want)
		}
	}
}
