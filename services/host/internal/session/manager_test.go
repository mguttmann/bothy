// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// TestNewDefaults: a manager without a clock or a logger uses the real clock and the default
// logger.
func TestNewDefaults(t *testing.T) {
	m := New(Deps{})
	t.Cleanup(m.Shutdown)
	if m.d.Clock == nil || m.d.Log == nil {
		t.Fatalf("clock %v, log %v", m.d.Clock, m.d.Log)
	}
	if d := time.Since(m.d.Clock.Now()); d < 0 || d > testkit.Patience {
		t.Errorf("the default clock is %v off", d)
	}
}

// TestOpenAfterShutdown: a session.open after the stop of the manager dials nothing.
func TestOpenAfterShutdown(t *testing.T) {
	m := quietManager(t)
	m.Shutdown()
	n := newNotes()
	m.Open(context.Background(), control.SessionOpen{SessionID: sessID, DataPath: control.DataPathPrefix + sessID + "/stream",
		ExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)}, n)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.notifier != nil || len(m.conns) != 0 {
		t.Errorf("a stopped manager took the session.open: notifier %v, conns %d", m.notifier, len(m.conns))
	}
}

// TestServeRefusesUnusableOpen: a session.open with an unusable expiry, data path or ID, or whose
// data connection cannot be opened, leaves no data connection.
func TestServeRefusesUnusableOpen(t *testing.T) {
	ctx := testkit.Context(t)
	dials := make(chan struct{}, 8)
	url := wsServer(t, func(context.Context, *websocket.Conn) { dials <- struct{}{} })
	base := strings.TrimSuffix(url, "/data")
	expires := time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)
	good := control.SessionOpen{SessionID: sessID, DataPath: control.DataPathPrefix + sessID + "/stream", ExpiresAt: expires}
	rows := []struct {
		name    string
		open    func(o control.SessionOpen) control.SessionOpen
		hostID  string
		dataURL func(string) (string, error)
	}{
		{"expiry not RFC 3339", func(o control.SessionOpen) control.SessionOpen { o.ExpiresAt = "tomorrow"; return o }, hostID, nil},
		{"data path of another session", func(o control.SessionOpen) control.SessionOpen {
			o.DataPath = control.DataPathPrefix + otherID + "/stream"
			return o
		}, hostID, nil},
		{"session ID not a UUID", func(o control.SessionOpen) control.SessionOpen {
			o.SessionID, o.DataPath = "session-1", control.DataPathPrefix+"session-1/stream"
			return o
		}, hostID, nil},
		{"host ID not a UUID", func(o control.SessionOpen) control.SessionOpen { return o }, "host-1", nil},
		{"data URL refused", func(o control.SessionOpen) control.SessionOpen { return o }, hostID,
			func(string) (string, error) { return "", control.ErrDataPath }},
		{"data connection refused", func(o control.SessionOpen) control.SessionOpen { return o }, hostID,
			func(p string) (string, error) { return base + "/elsewhere" + p, nil }},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			du := r.dataURL
			if du == nil {
				du = func(string) (string, error) { return url, nil }
			}
			m := New(Deps{HostID: r.hostID, HTTP: func() *http.Client { return http.DefaultClient }, DataURL: du,
				Log: slog.New(slog.DiscardHandler)})
			t.Cleanup(m.Shutdown)
			m.serve(ctx, r.open(good), newNotes(), time.Now())
			m.mu.Lock()
			conns := len(m.conns)
			m.mu.Unlock()
			if conns != 0 || len(dials) != 0 {
				t.Errorf("conns %d, dials %d", conns, len(dials))
			}
		})
	}
}

// TestLinkDialOutcomes: a failed peer check is the HD10 event of the gate (helper_untrusted), any
// other dial error is pty_failed; a working link is reused until it breaks.
func TestLinkDialOutcomes(t *testing.T) {
	ctx := testkit.Context(t)
	gr := &gateRunner{res: passing(), lightUser: map[string]int{}}
	g := NewGate(gr, clocktest.New(now), time.Minute, nil, nil)
	h := newFakeHelper(t)
	m := New(Deps{Gate: g, Dial: h.dial, Log: slog.New(slog.DiscardHandler)})
	t.Cleanup(m.Shutdown)

	h.dialErr = errors.New("connection refused")
	if l, d := m.link(ctx, alexOSU); l != nil || d != DetailPTYFailed {
		t.Errorf("dial error: %v %q", l, d)
	}
	if g.Current().Host != "" {
		t.Error("a dial error recorded a peer mismatch")
	}
	h.dialErr = fmt.Errorf("open: %w", doctor.ErrPeerMismatch)
	if l, d := m.link(ctx, alexOSU); l != nil || d != DetailHelperUntrusted {
		t.Errorf("peer mismatch: %v %q", l, d)
	}
	if p := g.Current(); p.Host != DetailHelperUntrusted || !p.Ends {
		t.Errorf("policy after a peer mismatch %+v", p)
	}

	h.dialErr = nil
	l1, d := m.link(ctx, alexOSU)
	if l1 == nil || d != "" {
		t.Fatalf("dial: %v %q", l1, d)
	}
	if l2, _ := m.link(ctx, alexOSU); l2 != l1 {
		t.Error("a working link was not reused")
	}
	_ = h.helperEnd().Close()
	waitDone(t, ctx, l1)
	if l3, _ := m.link(ctx, alexOSU); l3 == nil || l3 == l1 {
		t.Error("a broken link was reused")
	}
	if n := h.dialCount(); n != 4 {
		t.Errorf("%d dials", n)
	}
}

// TestAPIKeyRead: claude_api_key is read for one start without the line end; a missing reader, a
// read error or an empty file give no key.
func TestAPIKeyRead(t *testing.T) {
	rows := []struct {
		name string
		read func() ([]byte, error)
		key  string
		ok   bool
	}{
		{"no reader", nil, "", false},
		{"read error", func() ([]byte, error) { return nil, errors.New("permission denied") }, "", false},
		{"empty", func() ([]byte, error) { return []byte(" \r\n"), nil }, "", false},
		{"trailing line end", func() ([]byte, error) { return []byte("EXAMPLE-KEY \r\n"), nil }, "EXAMPLE-KEY", true},
	}
	for _, r := range rows {
		m := &Manager{d: Deps{APIKey: r.read}}
		if key, ok := m.apiKey(); key != r.key || ok != r.ok {
			t.Errorf("%s: %q %v", r.name, key, ok)
		}
	}
}

// TestStartRequest: pty.start carries the program of a command profile from the configuration,
// and the working directory and, with the declaration organization, the API key of a claude
// profile; what cannot be resolved refuses the start.
func TestStartRequest(t *testing.T) {
	cfg := config.Defaults(config.Platform{GOOS: "linux"})
	cfg.Profiles = append(cfg.Profiles, config.Profile{Name: "echo", Kind: config.KindCommand, Program: "/bin/echo", Args: []string{"hello"}})
	key := func() ([]byte, error) { return []byte("EXAMPLE-KEY\n"), nil }
	m := &Manager{d: Deps{Config: &cfg, APIKey: key}}
	zero, one := 0, 1
	claims := func(prf string, wdi *int) Claims { return Claims{Sid: sessID, Prf: prf, Osu: alexOSU, Wdi: wdi} }
	org := pipeResult()
	personal := pipeResult()
	personal.Declaration = state.DeclPersonal

	if _, d := m.startRequest(claims("shell", nil), config.KindShell, nil, 80, 24); d != DetailPTYFailed {
		t.Errorf("no result: %q", d)
	}
	req, d := m.startRequest(claims("echo", nil), config.KindCommand, org, 80, 24)
	if d != "" || req.Program != "/bin/echo" || !slices.Equal(req.Args, []string{"hello"}) || req.Cols != 80 || req.Rows != 24 {
		t.Errorf("command: %q %+v", d, req)
	}
	if _, d := m.startRequest(claims("ghost", nil), config.KindCommand, org, 80, 24); d != DetailProfileUnknown {
		t.Errorf("command missing from the configuration: %q", d)
	}
	for _, wdi := range []*int{nil, &one} {
		if _, d := m.startRequest(claims("claude", wdi), config.KindClaude, org, 80, 24); d != DetailProfileUnknown {
			t.Errorf("claude with wdi %v: %q", wdi, d)
		}
	}
	req, d = m.startRequest(claims("claude", &zero), config.KindClaude, org, 80, 24)
	if d != "" || req.Workdir != "/srv/project" || req.WorkdirIndex == nil || *req.WorkdirIndex != 0 || req.APIKey != "EXAMPLE-KEY" {
		t.Errorf("claude with organization: %q %+v", d, req.Workdir)
	}
	req, d = m.startRequest(claims("claude", &zero), config.KindClaude, personal, 80, 24)
	if d != "" || req.APIKey != "" {
		t.Errorf("claude with personal: %q, key set %v", d, req.APIKey != "")
	}
	m.d.APIKey = nil
	if _, d := m.startRequest(claims("claude", &zero), config.KindClaude, org, 80, 24); d != DetailPTYFailed {
		t.Errorf("claude with organization without a key: %q", d)
	}
}
