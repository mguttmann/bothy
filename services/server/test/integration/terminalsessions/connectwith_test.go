// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// TestFakeHostConnectWith (M2-48, the manipulated host service of TM-T11 behind the TLS front of
// the suite): fakehost.ConnectWith dials the control connection and every data connection through
// the dialer of its network (hosttest.WithTransport), and a session pairs over it as over
// Connect.
func TestFakeHostConnectWith(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.h1.Close()
	var dials atomic.Int32
	var nd net.Dialer
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dials.Add(1)
		return nd.DialContext(ctx, network, addr)
	}
	h := fakehost.ConnectWith(t, w.s.PublicURL, fakehost.Network{Dial: dial}, w.r1, w.h1Key, report(reportedUsers(), false))
	h.Default(fakehost.Script{Kind: fakehost.Accept})
	if dials.Load() == 0 {
		t.Fatal("the control connection did not dial through the network")
	}
	before := dials.Load()
	g := w.opened(w.alex, w.r1, openBody("shell"))
	if !strings.Contains(g.StreamPath, g.Session.ID) {
		t.Fatalf("stream path %q", g.StreamPath)
	}
	w.eventually("the data connection of the session through the network", func() bool { return dials.Load() > before })
}
