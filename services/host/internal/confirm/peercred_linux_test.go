// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"net"
	"path/filepath"
	"testing"
)

// PeerOf reports an error instead of credentials for a connection without a socket and for a
// closed one.
func TestPeerOfWithoutSocket(t *testing.T) {
	if p, err := PeerOf(&net.UnixConn{}); err == nil {
		t.Fatalf("PeerOf of an unconnected value = %+v without an error", p)
	}

	l, err := Listen(filepath.Join(shortDir(t), "confirm.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		if c, err := l.Accept(); err == nil {
			accepted <- c
		}
	}()
	c, err := dialEndpoint(context.Background(), l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	sc := waitFor(t, accepted, "the accept")
	_ = sc.Close()
	if p, err := PeerOf(sc); err == nil {
		t.Fatalf("PeerOf of a closed connection = %+v without an error", p)
	}
}
