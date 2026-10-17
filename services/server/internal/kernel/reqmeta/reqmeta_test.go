// SPDX-License-Identifier: Apache-2.0

package reqmeta_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

func TestCorrelationIDRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got := reqmeta.CorrelationID(ctx); got != "" {
		t.Fatalf("empty context: CorrelationID = %q, want empty", got)
	}
	const id = "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"
	ctx = reqmeta.WithCorrelationID(ctx, id)
	if got := reqmeta.CorrelationID(ctx); got != id {
		t.Fatalf("CorrelationID = %q, want %q", got, id)
	}
}

func TestClientIPRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if ip, ok := reqmeta.ClientIP(ctx); ok || ip.IsValid() {
		t.Fatalf("empty context: ClientIP = %v, %v; want invalid, false", ip, ok)
	}
	want := netip.MustParseAddr("192.0.2.1")
	ctx = reqmeta.WithClientIP(ctx, want)
	if ip, ok := reqmeta.ClientIP(ctx); !ok || ip != want {
		t.Fatalf("ClientIP = %v, %v; want %v, true", ip, ok, want)
	}
}

func TestInvalidClientIPIsNotStored(t *testing.T) {
	t.Parallel()
	ctx := reqmeta.WithClientIP(context.Background(), netip.Addr{})
	if _, ok := reqmeta.ClientIP(ctx); ok {
		t.Fatal("an invalid address must read back as absent")
	}
}

// TestProofRoundTrip: the DPoP facts of a request travel to the handlers of the login and refresh
// endpoints (M1-34).
func TestProofRoundTrip(t *testing.T) {
	t.Parallel()
	if _, ok := reqmeta.ProofOf(context.Background()); ok {
		t.Fatal("empty context has a proof")
	}
	in := reqmeta.Proof{Method: "POST", HTU: "https://chat.example.com/api/v1/auth/token/refresh", Values: []string{"a", "b"}}
	ctx := reqmeta.WithProof(context.Background(), in)
	in.Values[0] = "changed"
	got, ok := reqmeta.ProofOf(ctx)
	if !ok || got.Method != "POST" || got.HTU != in.HTU || len(got.Values) != 2 || got.Values[0] != "a" {
		t.Fatalf("ProofOf = %+v, %v", got, ok)
	}
}

// TestResponseHeader: SetResponseHeader reaches the setter of the chain and is a no-op without
// one.
func TestResponseHeader(t *testing.T) {
	t.Parallel()
	reqmeta.SetResponseHeader(context.Background(), "DPoP-Nonce", "n")
	got := map[string]string{}
	ctx := reqmeta.WithResponseHeaders(context.Background(), func(k, v string) { got[k] = v })
	reqmeta.SetResponseHeader(ctx, "DPoP-Nonce", "n1")
	if got["DPoP-Nonce"] != "n1" {
		t.Fatalf("headers %v", got)
	}
}

func TestTransportRoundTrip(t *testing.T) {
	t.Parallel()
	if _, ok := reqmeta.TransportOf(context.Background()); ok {
		t.Fatal("empty context has a transport")
	}
	want := reqmeta.Transport{
		Peer: netip.MustParseAddr("127.0.0.1"), Action: "auth.login.complete",
		Header: map[string][]string{"Tailscale-User-Login": {"alex@example.com"}},
	}
	got, ok := reqmeta.TransportOf(reqmeta.WithTransport(context.Background(), want))
	if !ok || got.Peer != want.Peer || got.Action != want.Action {
		t.Fatalf("TransportOf = %+v, %v", got, ok)
	}
	if v := got.Values("tailscale-user-login"); len(v) != 1 || v[0] != "alex@example.com" {
		t.Fatalf("Values without regard to case = %q", v)
	}
	if v := got.Values("X-Other"); v != nil {
		t.Fatalf("Values of a missing header = %q", v)
	}
}
