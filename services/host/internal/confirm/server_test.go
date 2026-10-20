// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	reqOpen   = Request{Kind: KindOpen, Profile: "shell", SessionUser: "bothy-s-alex"}
	reqAttach = Request{Kind: KindAttach, Profile: "claude", SessionUser: "bothy-s-sam"}
	robin     = Peer{UID: uidRobin, PID: 4242}
)

func TestConfirmOpenAndAttach(t *testing.T) {
	const wait = 5 * time.Second

	t.Run("allow starts", func(t *testing.T) {
		h := newHarness(t, testRule())
		d := &fixedDialog{allow: true}
		h.agent(robin, d.dialog, 1)
		if !h.s.Ask(context.Background(), reqOpen, wait) {
			t.Fatal("Ask = false after an allow")
		}
		if got := d.requests(); len(got) != 1 || got[0] != reqOpen {
			t.Fatalf("dialog saw %+v, want exactly %+v", got, reqOpen)
		}
	})

	t.Run("deny", func(t *testing.T) {
		h := newHarness(t, testRule())
		h.agent(robin, (&fixedDialog{allow: false}).dialog, 1)
		if h.s.Ask(context.Background(), reqOpen, wait) {
			t.Fatal("Ask = true after a denial")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		h := newHarness(t, testRule())
		b := newBlockingDialog()
		h.agent(robin, b.dialog, 1)
		start := time.Now()
		if h.s.Ask(context.Background(), reqOpen, 100*time.Millisecond) {
			t.Fatal("Ask = true without an answer")
		}
		if time.Since(start) > 3*time.Second {
			t.Fatal("Ask did not end at its timeout")
		}
		waitFor(t, b.cancelled, "the dialog to end after the timeout")
	})

	t.Run("ctx end", func(t *testing.T) {
		h := newHarness(t, testRule())
		b := newBlockingDialog()
		h.agent(robin, b.dialog, 1)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			waitFor(t, b.started, "the dialog")
			cancel()
		}()
		if h.s.Ask(ctx, reqOpen, wait) {
			t.Fatal("Ask = true after the end of ctx")
		}
	})

	t.Run("no agent", func(t *testing.T) {
		h := newHarness(t, testRule())
		start := time.Now()
		if h.s.Ask(context.Background(), reqOpen, wait) {
			t.Fatal("Ask = true without an agent")
		}
		if time.Since(start) > time.Second {
			t.Fatal("Ask without an agent waited for the timeout")
		}
	})

	t.Run("denied attach", func(t *testing.T) {
		h := newHarness(t, testRule())
		d := &fixedDialog{allow: false}
		h.agent(robin, d.dialog, 1)
		if h.s.Ask(context.Background(), reqAttach, wait) {
			t.Fatal("Ask = true for a denied attach")
		}
		if got := d.requests(); len(got) != 1 || got[0] != reqAttach {
			t.Fatalf("dialog saw %+v, want %+v", got, reqAttach)
		}
	})

	t.Run("first of two agents wins", func(t *testing.T) {
		h := newHarness(t, testRule())
		b := newBlockingDialog()
		h.agent(robin, b.dialog, 1)
		h.agent(Peer{UID: uidRobin, PID: 4243}, (&fixedDialog{allow: true}).dialog, 2)
		if !h.s.Ask(context.Background(), reqOpen, wait) {
			t.Fatal("Ask = false although one agent allowed")
		}
		waitFor(t, b.started, "the request at the second agent")
		waitFor(t, b.cancelled, "the cancel at the other agent")
	})

	t.Run("several requests at once", func(t *testing.T) {
		h := newHarness(t, testRule())
		h.agent(robin, (&fixedDialog{allow: true}).dialog, 1)
		results := make(chan bool, 8)
		for range 8 {
			go func() { results <- h.s.Ask(context.Background(), reqOpen, wait) }()
		}
		for range 8 {
			if !waitFor(t, results, "an answer") {
				t.Fatal("a concurrent request was not allowed")
			}
		}
	})
}

func TestConfirmPeerRefused(t *testing.T) {
	ctx := context.Background()
	t.Run("rule", func(t *testing.T) {
		consoleIs := func(uid string) Rule {
			r := testRule()
			r.ConsoleOwner = func(context.Context) (string, error) { return uid, nil }
			return r
		}
		cases := []struct {
			name string
			rule Rule
			peer Peer
			want bool
		}{
			{"console owner", testRule(), robin, true},
			{"service user", consoleIs(uidService), Peer{UID: uidService, PID: 1}, false},
			{"session user", consoleIs(uidAlex), Peer{UID: uidAlex, PID: 1}, false},
			{"former session user", consoleIs(uidFormer), Peer{UID: uidFormer, PID: 1}, false},
			{"without the console session", testRule(), Peer{UID: uidOther, PID: 1}, false},
			{"empty user", consoleIs(""), Peer{PID: 1}, false},
			{"console lookup fails", Rule{ServiceUID: uidService, Excluded: testRule().Excluded,
				ConsoleOwner: func(context.Context) (string, error) { return "", errors.New("no seat") }}, robin, false},
			{"no console lookup", Rule{ServiceUID: uidService, Excluded: testRule().Excluded}, robin, false},
			{"no exclusion list", Rule{ServiceUID: uidService, ConsoleOwner: testRule().ConsoleOwner}, robin, false},
			{"other console session", func() Rule {
				r := testRule()
				r.ConsoleSession = func(context.Context, Peer) (bool, error) { return false, nil }
				return r
			}(), robin, false},
			{"console session lookup fails", func() Rule {
				r := testRule()
				r.ConsoleSession = func(context.Context, Peer) (bool, error) { return true, errors.New("no session") }
				return r
			}(), robin, false},
			{"in the console session", func() Rule {
				r := testRule()
				r.ConsoleSession = func(_ context.Context, p Peer) (bool, error) { return p.Session == 3, nil }
				return r
			}(), Peer{UID: uidRobin, PID: 4242, Session: 3}, true},
		}
		for _, c := range cases {
			if got := c.rule.Allow(ctx, c.peer); got != c.want {
				t.Errorf("%s: Allow = %v, want %v", c.name, got, c.want)
			}
		}
	})

	t.Run("server closes without sending", func(t *testing.T) {
		h := newHarness(t, testRule())
		for _, p := range []Peer{{UID: uidService, PID: 1}, {UID: uidAlex, PID: 2}, {UID: uidFormer, PID: 3}, {UID: uidOther, PID: 4}} {
			c, err := h.l.dialAs(p)(ctx, "mem")
			if err != nil {
				t.Fatal(err)
			}
			_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := io.Copy(io.Discard, c)
			if n != 0 || err != nil {
				t.Fatalf("peer %s: read %d bytes, err %v; want a close without data", p.UID, n, err)
			}
			_ = c.Close()
		}
		if h.s.Agents() != 0 {
			t.Fatalf("agents = %d after refused peers", h.s.Agents())
		}
		if got := strings.Count(h.log.String(), "confirm_peer_refused"); got != 4 {
			t.Fatalf("confirm_peer_refused logged %d times, want 4:\n%s", got, h.log.String())
		}
		if h.s.Ask(ctx, reqOpen, time.Second) {
			t.Fatal("Ask = true with only refused peers")
		}
	})

	t.Run("rule checked again at each request", func(t *testing.T) {
		var mu sync.Mutex
		owner := uidRobin
		rule := testRule()
		rule.ConsoleOwner = func(context.Context) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			return owner, nil
		}
		h := newHarness(t, rule)
		d := &fixedDialog{allow: true}
		h.agent(robin, d.dialog, 1)
		mu.Lock()
		owner = uidOther // Robin left the console; the agent stays connected
		mu.Unlock()
		if h.s.Ask(ctx, reqOpen, time.Second) {
			t.Fatal("Ask = true from an agent whose user no longer owns the console")
		}
		if len(d.requests()) != 0 {
			t.Fatal("the request reached an agent that the rule refuses now")
		}
		h.waitAgents(0)
	})
}

// rawAgent connects as robin and speaks the protocol by hand.
func rawAgent(t *testing.T, h *harness) (net.Conn, *bufio.Reader) {
	t.Helper()
	c, err := h.l.dialAs(robin)(context.Background(), "mem")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	h.waitAgents(1)
	return c, bufio.NewReader(c)
}

func readMsg(t *testing.T, c net.Conn, r *bufio.Reader) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("not JSON: %q", line)
	}
	return m
}

func writeLine(t *testing.T, c net.Conn, s string) {
	t.Helper()
	_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(c, s+"\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestConfirmProtocolStrict(t *testing.T) {
	ctx := context.Background()

	t.Run("request shape and late answers", func(t *testing.T) {
		h := newHarness(t, testRule())
		c, r := rawAgent(t, h)
		res := make(chan bool, 1)
		go func() { res <- h.s.Ask(ctx, reqOpen, 5*time.Second) }()
		m := readMsg(t, c, r)
		if m["op"] != "confirm" || m["kind"] != KindOpen || m["profile"] != "shell" || m["session_user"] != "bothy-s-alex" ||
			m["timeout_ms"] != float64(5000) || len(m) != 6 {
			t.Fatalf("confirm message %v", m)
		}
		id, _ := m["id"].(string)
		if len(id) != 36 {
			t.Fatalf("id %q is not a uuid", id)
		}
		// An answer to an unknown id is ignored and the agent stays.
		writeLine(t, c, `{"op":"answer","id":"00000000-0000-4000-8000-000000000000","allow":true}`)
		writeLine(t, c, `{"op":"answer","id":"`+id+`","allow":true}`)
		if !waitFor(t, res, "the answer") {
			t.Fatal("Ask = false after allow")
		}
		// A second answer to the same id after the first is ignored as well.
		writeLine(t, c, `{"op":"answer","id":"`+id+`","allow":false}`)
		if h.s.Agents() != 1 {
			t.Fatal("an answer to an unknown id closed the agent")
		}
	})

	bad := map[string]string{
		"unknown member": `{"op":"answer","id":"x","allow":true,"extra":1}`,
		"unknown op":     `{"op":"hello","id":"x"}`,
		"null allow":     `{"op":"answer","id":"x","allow":null}`,
		"missing allow":  `{"op":"answer","id":"x"}`,
		"not json":       `{"op":`,
		"trailing data":  `{"op":"answer","id":"x","allow":true} {}`,
		"too long":       `{"op":"answer","id":"` + strings.Repeat("a", maxLine) + `","allow":true}`,
	}
	for name, line := range bad {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, testRule())
			c, _ := rawAgent(t, h)
			go func() {
				_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_, _ = io.WriteString(c, line+"\n")
			}()
			h.waitAgents(0)
		})
	}
}

func TestEndpoint(t *testing.T) {
	cases := map[string]string{
		"linux":   "/run/bothy-host-confirm/confirm.sock",
		"darwin":  "/Library/Application Support/bothy-host/confirm/confirm.sock",
		"windows": `\\.\pipe\bothy-host-confirm`,
	}
	for goos, want := range cases {
		if got := Endpoint(goos); got != want {
			t.Errorf("Endpoint(%s) = %q, want %q", goos, got, want)
		}
	}
}
