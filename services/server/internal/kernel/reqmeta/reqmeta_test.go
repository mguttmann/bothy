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
