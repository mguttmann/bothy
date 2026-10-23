// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestRelayUnpaired_Sides: the counter takes only the catalogue values client and host.
func TestRelayUnpaired_Sides(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.RelayUnpaired(ctx, domain.SideClient)
	m.RelayUnpaired(ctx, domain.SideClient)
	m.RelayUnpaired(ctx, domain.SideHost)
	got := metricstest.Find(t, reader, "bothy.terminal.relay.unpaired")
	for side, want := range map[string]float64{"client": 2, "host": 1} {
		if n := metricstest.Sum(got, map[string]string{"bothy.terminal.side": side}); n != want {
			t.Errorf("%s = %v, want %v", side, n, want)
		}
	}
}

// TestRelayUnpairedAttributeSet (M2-47 requirement 17): the only attribute of the counter is
// bothy.terminal.side, with exactly the values client and host.
func TestRelayUnpairedAttributeSet(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.RelayUnpaired(ctx, domain.SideClient)
	m.RelayUnpaired(ctx, domain.SideHost)
	values := map[string]bool{}
	for _, p := range metricstest.Points(metricstest.Find(t, reader, "bothy.terminal.relay.unpaired")) {
		if len(p.Attrs) != 1 {
			t.Fatalf("attributes %v, want only bothy.terminal.side", p.Attrs)
		}
		side, ok := p.Attrs["bothy.terminal.side"]
		if !ok {
			t.Fatalf("attributes %v", p.Attrs)
		}
		values[side] = true
	}
	if len(values) != 2 || !values["client"] || !values["host"] {
		t.Fatalf("values %v, want client and host", values)
	}
}

// TestTerminalMetrics_ClosedAttributes (M2-46 Design note 12, 24 section 10.4): the registry check
// of M1-68 passes for the four instruments of 4.3a, every attribute has a closed value set and none
// is an ID of a person or a host name; recording every allowed value keeps exactly these
// attributes.
func TestTerminalMetrics_ClosedAttributes(t *testing.T) {
	metricstest.Strict(t)
	specs := []metrics.Spec{metrics.TerminalRelayUnpaired, metrics.TerminalSessionsActive, metrics.TerminalSessionsClosed, metrics.TerminalHostsConnected}
	if errs := metrics.Check(specs); len(errs) != 0 {
		t.Fatalf("registry check: %v", errs)
	}
	for _, s := range specs {
		for _, a := range s.Attrs {
			if len(a.Values) == 0 || metrics.PersonOrIDKey(a.Key) {
				t.Errorf("%s: attribute %s is not a closed set without IDs", s.Name, a.Key)
			}
		}
	}
	reasons := map[string]bool{}
	for _, r := range domain.CloseReasons() {
		reasons[string(r)] = true
	}
	closed := metrics.TerminalSessionsClosed.Attrs[0].Values
	if len(closed) != len(reasons) {
		t.Fatalf("bothy.reason has %d values, the close reasons are %d", len(closed), len(reasons))
	}
	for _, v := range closed {
		if !reasons[v] {
			t.Errorf("bothy.reason %q is no close reason", v)
		}
	}
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, r := range domain.CloseReasons() {
		m.SessionClosed(ctx, r)
	}
	for _, st := range []string{domain.SessionOpening, domain.SessionActive, domain.SessionDetached} {
		m.SessionsActive(ctx, st, 1)
	}
	m.SessionsActive(ctx, domain.SessionActive, -1)
	m.HostsConnected(ctx, 1)
	m.HostsConnected(ctx, -1)
	m.HostsConnected(ctx, 1)
	for name, key := range map[string]string{"bothy.terminal.sessions.closed": "bothy.reason", "bothy.terminal.sessions.active": "bothy.terminal.state"} {
		for _, p := range metricstest.Points(metricstest.Find(t, reader, name)) {
			if _, ok := p.Attrs[key]; !ok || len(p.Attrs) != 1 {
				t.Errorf("%s: attributes %v, want only %s", name, p.Attrs, key)
			}
		}
	}
	if n := metricstest.Sum(metricstest.Find(t, reader, "bothy.terminal.sessions.active"), map[string]string{"bothy.terminal.state": "active"}); n != 0 {
		t.Errorf("active sessions = %v, want 0", n)
	}
	hosts := metricstest.Find(t, reader, "bothy.terminal.hosts.connected")
	for _, p := range metricstest.Points(hosts) {
		if len(p.Attrs) != 0 {
			t.Errorf("hosts.connected attributes %v", p.Attrs)
		}
	}
	if n := metricstest.Sum(hosts, nil); n != 1 {
		t.Errorf("hosts.connected = %v, want 1", n)
	}
}
