// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// Compile-time check from the outside: the adapter is a provider of the push path.
var _ app.PushProvider = (*apns.Adapter)(nil)

func TestHTTP2AndSharedConnection(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	if w.a.Name() != "apns" {
		t.Fatalf("Name %q", w.a.Name())
	}
	// The first send opens the connection; then 49 sequential and 50 parallel sends share it.
	for range 50 {
		w.send(w.phone, w.hint())
	}
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, err := w.a.Send(t.Context(), w.phone, w.hint()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	reqs := w.prod.Requests()
	if len(reqs) != 100 {
		t.Fatalf("%d requests, want 100", len(reqs))
	}
	for _, r := range reqs {
		if r.Proto != "HTTP/2.0" {
			t.Fatalf("request over %s", r.Proto)
		}
	}
	if n := w.prod.Connections(); n != 1 {
		t.Fatalf("%d TLS connections, want 1", n)
	}
}

func TestEgressOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withoutRules())
	_, err := w.a.Send(t.Context(), w.phone, w.hint())
	if !errors.Is(err, egress.ErrBlocked) {
		t.Fatalf("without the egress rule: %v, want egress.ErrBlocked", err)
	}
	if n := len(w.prod.Requests()); n != 0 {
		t.Fatalf("%d requests reached the Fake", n)
	}
	ctx := context.Background()
	for _, c := range []struct {
		scheme, host string
		port         uint16
		want         bool
	}{
		{"https", "api.push.apple.com", 443, true},
		{"https", "api.sandbox.push.apple.com", 443, true},
		{"https", "push.apple.com", 443, false},
		{"https", "api.push.apple.com", 2197, false},
		{"http", "api.push.apple.com", 443, false},
		{"https", "api.push.apple.com.example.com", 443, false},
	} {
		if got := apns.AppleHosts(ctx, c.scheme, c.host, c.port); got != c.want {
			t.Errorf("%s://%s:%d: %v, want %v", c.scheme, c.host, c.port, got, c.want)
		}
	}
	d := w.deps()
	d.Endpoints = nil
	a, err := apns.New(ctx, w.cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	got := apns.BaseURLs(a)
	if len(got) != 2 || got["production"] != "https://api.push.apple.com" || got["sandbox"] != "https://api.sandbox.push.apple.com" {
		t.Fatalf("base URLs %v", got)
	}
}
