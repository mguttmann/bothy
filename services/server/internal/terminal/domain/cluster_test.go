// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestSignalCodec: each kind encodes and decodes to the same value; a payload with an unknown
// field, a missing ID, an extra ID for the kind, a non-UUID, null or an unknown terminal. kind
// fails; typing.started and other foreign kinds give ours false without error (M2-47 Design note 3).
func TestSignalCodec(t *testing.T) {
	t.Parallel()
	h, s, c := ids.New(), ids.New(), ids.New()
	for _, sig := range []domain.Signal{
		{Kind: domain.SignalSessionOpen, HostID: h, SessionID: s},
		{Kind: domain.SignalHostClose, HostID: h},
		{Kind: domain.SignalHostPresent, HostID: h, ConnID: c},
		{Kind: domain.SignalHostGone, HostID: h, ConnID: c},
		{Kind: domain.SignalSessionPaired, HostID: h, SessionID: s},
	} {
		b, err := domain.EncodeSignal(sig)
		if err != nil {
			t.Fatalf("%s: %v", sig.Kind, err)
		}
		got, ours, err := domain.DecodeSignal(b)
		if err != nil || !ours || got != sig {
			t.Fatalf("%s: %s decodes to %+v %v %v", sig.Kind, b, got, ours, err)
		}
	}
	if _, err := domain.EncodeSignal(domain.Signal{Kind: domain.SignalHostClose, HostID: h, SessionID: s}); err == nil {
		t.Fatal("an extra ID encodes")
	}
	if _, err := domain.EncodeSignal(domain.Signal{Kind: domain.SignalSessionOpen, HostID: h}); err == nil {
		t.Fatal("a missing ID encodes")
	}
	hs, ss := h.String(), s.String()
	bad := []string{
		`{"kind":"terminal.host_close","host_id":"` + hs + `","extra":1}`,
		`{"kind":"terminal.session_open","host_id":"` + hs + `"}`,
		`{"kind":"terminal.host_close","host_id":"` + hs + `","session_id":"` + ss + `"}`,
		`{"kind":"terminal.host_close","host_id":"not-a-uuid"}`,
		`{"kind":"terminal.host_close","host_id":null}`,
		`{"kind":"terminal.host_close","host_id":"00000000-0000-0000-0000-000000000000"}`,
		`{"kind":"terminal.unknown","host_id":"` + hs + `"}`,
		`{"kind":"terminal.host_close","host_id":"` + hs + `","host_id":"` + hs + `"}`,
		`{"kind":"terminal.host_close","host_id":"` + hs + `"} trailing`,
		`{"kind":null}`,
		`[]`,
		`null`,
		``,
		`{"kind":"terminal.host_close","host_id":"` + strings.ToUpper(hs) + `"}`,
	}
	for _, p := range bad {
		if _, ours, err := domain.DecodeSignal([]byte(p)); err == nil {
			t.Errorf("%q decodes (ours %v)", p, ours)
		}
	}
	for _, p := range []string{
		`{"kind":"typing.started","thread_id":"` + hs + `","principal_id":"` + ss + `"}`,
		`{"kind":"run.updated","anything":[1,2]}`,
	} {
		if _, ours, err := domain.DecodeSignal([]byte(p)); err != nil || ours {
			t.Errorf("%q: ours %v err %v", p, ours, err)
		}
	}
}

// FuzzDecodeSignal: any input gives an error, a foreign kind or a signal that encodes back to an
// equivalent payload; never a panic.
func FuzzDecodeSignal(f *testing.F) {
	h, s := ids.New().String(), ids.New().String()
	f.Add([]byte(`{"kind":"terminal.session_open","host_id":"` + h + `","session_id":"` + s + `"}`))
	f.Add([]byte(`{"kind":"terminal.host_present","host_id":"` + h + `","conn_id":"` + s + `"}`))
	f.Add([]byte(`{"kind":"typing.started"}`))
	f.Add([]byte(`{"kind":"terminal.host_close","host_id":null}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, b []byte) {
		sig, ours, err := domain.DecodeSignal(b)
		if err != nil || !ours {
			return
		}
		enc, err := domain.EncodeSignal(sig)
		if err != nil {
			t.Fatalf("a decoded signal does not encode: %v", err)
		}
		again, ours, err := domain.DecodeSignal(enc)
		if err != nil || !ours || again != sig {
			t.Fatalf("round trip differs: %+v %+v", sig, again)
		}
	})
}

// TestSignalPayloadBound: the longest signal stays below the protocol constant for the NOTIFY
// payload.
func TestSignalPayloadBound(t *testing.T) {
	t.Parallel()
	longest := 0
	full := ids.MustParse("ffffffff-ffff-7fff-bfff-ffffffffffff")
	for _, k := range []domain.SignalKind{domain.SignalSessionOpen, domain.SignalHostClose, domain.SignalHostPresent,
		domain.SignalHostGone, domain.SignalSessionPaired} {
		sig := domain.Signal{Kind: k, HostID: full}
		switch k {
		case domain.SignalSessionOpen, domain.SignalSessionPaired:
			sig.SessionID = full
		case domain.SignalHostPresent, domain.SignalHostGone:
			sig.ConnID = full
		}
		b, err := domain.EncodeSignal(sig)
		if err != nil {
			t.Fatal(err)
		}
		longest = max(longest, len(b))
	}
	if longest >= domain.NotifyPayloadMax {
		t.Fatalf("longest signal %d bytes, limit %d", longest, domain.NotifyPayloadMax)
	}
}

// TestOlderConnection: UUIDv7 IDs of two instants in both orders; equal IDs are not older.
func TestOlderConnection(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	gen := ids.NewGenerator(clk, rand.Reader)
	a := gen.New()
	clk.Advance(time.Millisecond)
	b := gen.New()
	if !domain.OlderConnection(a, b) || domain.OlderConnection(b, a) || domain.OlderConnection(a, a) {
		t.Fatal("order of connection IDs")
	}
}

// TestPresenceView: an entry counts at exactly the window after its last Seen and not one tick
// later; Gone of one conn_id keeps the other; an unknown host is not connected.
func TestPresenceView(t *testing.T) {
	t.Parallel()
	window := 20 * time.Second * 3
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	h, c1, c2 := ids.New(), ids.New(), ids.New()
	v := domain.NewPresenceView()
	if v.Connected(h, t0, window) {
		t.Fatal("unknown host connected")
	}
	v.Seen(h, c1, t0)
	if !v.Connected(h, t0.Add(window), window) || v.Connected(h, t0.Add(window+time.Nanosecond), window) {
		t.Fatal("window boundary")
	}
	v.Seen(h, c2, t0.Add(time.Second))
	v.Gone(h, c1)
	if !v.Connected(h, t0.Add(window), window) {
		t.Fatal("Gone of one connection removed the other")
	}
	v.Gone(h, c2)
	if v.Connected(h, t0, window) {
		t.Fatal("connected after both are gone")
	}
}

// TestProbationEnd: twice the probe timeout after the first announcement.
func TestProbationEnd(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	if got := domain.ProbationEnd(t0, 5*time.Second); !got.Equal(t0.Add(10 * time.Second)) {
		t.Fatalf("probation end %v", got)
	}
}

// TestLoneSideOutcome: the rules of Design note 8.
func TestLoneSideOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		side  domain.Side
		state string
		want  domain.LoneOutcome
	}{
		{domain.SideClient, domain.SessionOpening, domain.LoneOutcome{Timer: true, Code: 4102, End: true, Side: domain.SideClient}},
		{domain.SideHost, domain.SessionOpening, domain.LoneOutcome{Timer: true, Code: 4102, End: true, Side: domain.SideHost}},
		{domain.SideClient, domain.SessionDetached, domain.LoneOutcome{Timer: true, Code: 4102, Side: domain.SideClient}},
		{domain.SideClient, domain.SessionActive, domain.LoneOutcome{Timer: true, Code: 4102, Side: domain.SideClient}},
		{domain.SideHost, domain.SessionDetached, domain.LoneOutcome{}},
		{domain.SideHost, domain.SessionActive, domain.LoneOutcome{}},
		{domain.SideClient, domain.SessionClosed, domain.LoneOutcome{}},
	}
	for _, c := range cases {
		if got := domain.LoneSideOutcome(c.side, c.state); got != c.want {
			t.Errorf("%s in %s: %+v, want %+v", c.side, c.state, got, c.want)
		}
	}
}

// TestLoneSideDeadline: one millisecond before the acceptance time plus terminal.open_timeout the
// side is not yet unpaired, at that instant it is.
func TestLoneSideDeadline(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	open := 20 * time.Second
	d := domain.LoneDeadline(t0, open)
	if domain.LoneExpired(d, d.Add(-time.Millisecond)) || !domain.LoneExpired(d, d) {
		t.Fatal("deadline boundary")
	}
}

// TestHostCloseCode: the close code of a control connection after terminal.host_close, by the
// state read after the signal (the codes of M2-43's single-node path).
func TestHostCloseCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		state  domain.State
		reason string
		want   domain.CloseCode
	}{
		{domain.StateActive, "", 0},
		{domain.StateConfirming, "", 0},
		{domain.StatePairing, "", domain.CodeHostUnpaired},
		{domain.StateDisabled, "owner", domain.CodeHostDisabled},
		{domain.StateDisabled, domain.DisabledDuplicateConnection, domain.CodeHostDuplicate},
		{domain.StateRetired, "", domain.CodeHostDisabled},
	}
	for _, c := range cases {
		if got := domain.HostCloseCode(c.state, c.reason); got != c.want {
			t.Errorf("%s %s: %d, want %d", c.state, c.reason, got, c.want)
		}
	}
	if !bytes.Equal([]byte(domain.SignalPrefix), []byte("terminal.")) {
		t.Fatal("prefix")
	}
}
