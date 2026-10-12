// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// verdict is the expected outcome of a dial to an address.
type verdict int

const (
	public     verdict = iota // dialed with and without allow_private
	private                   // dialed only with allow_private
	alwaysDeny                // never dialed
)

// last returns the last address of a prefix.
func last(prefix string) netip.Addr {
	p := netip.MustParsePrefix(prefix)
	b := p.Addr().AsSlice()
	for i := p.Bits(); i < len(b)*8; i++ {
		b[i/8] |= 0x80 >> (i % 8)
	}
	a, _ := netip.AddrFromSlice(b)
	return a
}

// before returns the address right before a prefix, after the address right after it.
func before(prefix string) netip.Addr { return netip.MustParsePrefix(prefix).Addr().Prev() }

func after(prefix string) netip.Addr { return last(prefix).Next() }

// mapped returns the IPv4-mapped IPv6 form of an IPv4 address (::ffff:a.b.c.d).
func mapped(a netip.Addr) netip.Addr { return netip.AddrFrom16(a.As16()) }

// sect09Addresses holds one address per range of 19 SEC-NET-3 (design note 3 of the brief) and
// the boundaries right outside some ranges. Expected verdicts are written by hand from the
// chapter, not computed by the package.
func sect09Addresses() []struct {
	addr netip.Addr
	want verdict
} {
	type row = struct {
		addr netip.Addr
		want verdict
	}
	p := netip.MustParseAddr
	// The IPv4 metadata address inside 100.64.0.0/10, spelled as bytes: the generic deny-list
	// rule refuses a host address of that range as text.
	metadataV4 := netip.AddrFrom4([4]byte{100, 100, 100, 200})
	return []row{
		// Always blocked, IPv4.
		{p("0.0.0.0"), alwaysDeny},
		{p("0.0.0.1"), alwaysDeny},
		{p("169.254.169.254"), alwaysDeny},
		{p("169.254.0.1"), alwaysDeny},
		{p("224.0.0.1"), alwaysDeny},
		{p("239.255.255.250"), alwaysDeny},
		{p("240.0.0.1"), alwaysDeny},
		{p("255.255.255.255"), alwaysDeny},
		{p("192.0.0.1"), alwaysDeny},
		{p("198.18.0.1"), alwaysDeny},
		{p("198.19.255.254"), alwaysDeny},
		// Always blocked, IPv6 and IPv4 inside IPv6.
		{p("::"), alwaysDeny},
		{p("fe80::1"), alwaysDeny},
		{p("febf::1"), alwaysDeny},
		{p("ff02::1"), alwaysDeny},
		{p("ff0e::1"), alwaysDeny},
		{p("fec0::1"), alwaysDeny},
		{p("100::1"), alwaysDeny},
		{p("::7f00:1"), alwaysDeny},
		{p("::a9fe:a9fe"), alwaysDeny},
		{p("64:ff9b::a9fe:a9fe"), alwaysDeny},
		{p("64:ff9b::7f00:1"), alwaysDeny},
		{p("64:ff9b:1::1"), alwaysDeny},
		{p("2002:a9fe:a9fe::1"), alwaysDeny},
		{p("2002:7f00:1::1"), alwaysDeny},
		{p("2001::1"), alwaysDeny},
		{p("2001:0:ffff::1"), alwaysDeny},
		{mapped(p("169.254.169.254")), alwaysDeny},
		// Metadata addresses of the clouds inside private ranges (R-SE-10): IPv6 inside the
		// unique local range, IPv4 inside the shared address space.
		{p("fd00:ec2::254"), alwaysDeny},
		{metadataV4, alwaysDeny},
		{mapped(metadataV4), alwaysDeny},
		{p("fd00:ec2::253"), private},
		{metadataV4.Next(), private},
		{mapped(p("0.0.0.0")), alwaysDeny},
		// Private: only with allow_private.
		{p("127.0.0.1"), private},
		{p("127.255.255.254"), private},
		{p("::1"), private},
		{mapped(p("127.0.0.1")), private},
		{firstHost("10.0.0.0/8"), private},
		{firstHost("172.16.0.0/12"), private},
		{firstHost("192.168.0.0/16"), private},
		{firstHost("100.64.0.0/10"), private},
		{mapped(firstHost("10.0.0.0/8")), private},
		{p("fc00::1"), private},
		{p("fd00::1"), private},
		// Public: documentation ranges, and the addresses right before and after blocked and
		// private ranges, derived from the prefixes (white label: no real hosts as literals).
		{p("192.0.2.10"), public},
		{p("198.51.100.1"), public},
		{p("203.0.113.5"), public},
		{p("2001:db8::10"), public},
		{mapped(p("192.0.2.10")), public},
		{before("10.0.0.0/8"), public},
		{after("10.0.0.0/8"), public},
		{before("127.0.0.0/8"), public},
		{after("127.0.0.0/8"), public},
		{before("172.16.0.0/12"), public},
		{after("172.16.0.0/12"), public},
		{before("192.168.0.0/16"), public},
		{after("192.168.0.0/16"), public},
		{before("100.64.0.0/10"), public},
		{after("100.64.0.0/10"), public},
		{before("169.254.0.0/16"), public},
		{after("169.254.0.0/16"), public},
		{before("198.18.0.0/15"), public},
		{after("198.18.0.0/15"), public},
		{before("192.0.0.0/24"), public},
		{after("192.0.0.0/24"), public},
		{before("224.0.0.0/4"), public},
		{after("0.0.0.0/8"), public},
		{after("2001::/32"), public},
		{after("2002::/16"), public},
		{before("fc00::/7"), public},
		{after("fc00::/7"), public},
		{before("fe80::/10"), public},
		{after("64:ff9b::/96"), public},
		{after("64:ff9b:1::/48"), public},
		{after("100::/64"), public},
		{after("::/96"), public},
		// Last addresses of blocked and private ranges.
		{last("169.254.0.0/16"), alwaysDeny},
		{last("198.18.0.0/15"), alwaysDeny},
		{last("192.0.0.0/24"), alwaysDeny},
		{last("fec0::/10"), alwaysDeny},
		{last("2001::/32"), alwaysDeny},
		{last("10.0.0.0/8"), private},
		{last("172.16.0.0/12"), private},
		{last("192.168.0.0/16"), private},
		{last("100.64.0.0/10"), private},
		{last("fc00::/7"), private},
	}
}

// TestSECT09_BlockedRanges dials one address of every range of SEC-NET-3, once as an IP literal
// in the URL and once as the answer of a DNS name, with and without allow_private. A refused
// address ends with address_blocked before any dial; an allowed address reaches the dialer,
// which the test intercepts so that nothing leaves the host.
func TestSECT09_BlockedRanges(t *testing.T) {
	t.Parallel()
	for _, row := range sect09Addresses() {
		for _, allowPrivate := range []bool{false, true} {
			allowed := row.want == public || (row.want == private && allowPrivate)
			for _, viaDNS := range []bool{false, true} {
				name := row.addr.String()
				if allowPrivate {
					name += "/allow_private"
				}
				if viaDNS {
					name += "/dns"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					rule := egress.Rule{Host: row.addr.Unmap().String(), AllowPrivate: allowPrivate}
					target := "https://" + netip.AddrPortFrom(row.addr, 443).String() + "/"
					answers := map[string][]netip.Addr{}
					if viaDNS {
						rule.Host = "target.example.com"
						target = "https://target.example.com/"
						answers["target.example.com"] = []netip.Addr{row.addr}
					}
					h := newHarness(t, setup{rules: []egress.Rule{rule}, answers: answers, intercept: true})
					err := h.err(t, target)
					if allowed {
						if !errors.Is(err, errIntercepted) {
							t.Fatalf("want a dial, got %v", err)
						}
						want := netip.AddrPortFrom(row.addr.Unmap(), 443).String()
						if got := h.addresses(); len(got) != 1 || got[0] != want {
							t.Fatalf("dialed %v, want [%s]", got, want)
						}
						return
					}
					wantBlocked(t, err, egress.ReasonAddressBlocked)
					if n := h.dials.Load(); n != 0 {
						t.Fatalf("%d dials, want 0", n)
					}
				})
			}
		}
	}
}

func TestAllowedPublicAddresses(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"192.0.2.10", "198.51.100.1", "203.0.113.5", "2001:db8::10"} {
		a := netip.MustParseAddr(s)
		h := newHarness(t, setup{
			policy:    func(_ context.Context, _, host string, _ uint16) bool { return host == "api.example.com" },
			answers:   map[string][]netip.Addr{"api.example.com": {a}},
			intercept: true,
		})
		if err := h.err(t, "https://api.example.com/"); !errors.Is(err, errIntercepted) {
			t.Errorf("%s: want a dial, got %v", s, err)
		}
		if got := h.addresses(); len(got) != 1 || got[0] != netip.AddrPortFrom(a, 443).String() {
			t.Errorf("%s: dialed %v", s, got)
		}
	}
}

// TestDialsOnlyCheckedAddresses: a DNS answer that mixes blocked and public addresses leads to
// dials of the public ones only, in the order of the answer.
func TestDialsOnlyCheckedAddresses(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{
		policy: func(_ context.Context, _, host string, _ uint16) bool { return host == "mixed.example.com" },
		answers: map[string][]netip.Addr{"mixed.example.com": {
			firstHost("10.0.0.0/8"),
			netip.MustParseAddr("169.254.169.254"),
			netip.MustParseAddr("192.0.2.10"),
			netip.MustParseAddr("::1"),
			netip.MustParseAddr("2001:db8::10"),
		}},
		intercept: true,
	})
	if err := h.err(t, "https://mixed.example.com:8443/"); !errors.Is(err, errIntercepted) {
		t.Fatalf("want an intercepted dial, got %v", err)
	}
	got := h.addresses()
	want := []string{"192.0.2.10:8443", "[2001:db8::10]:8443"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("dialed %v, want %v", got, want)
	}
}
