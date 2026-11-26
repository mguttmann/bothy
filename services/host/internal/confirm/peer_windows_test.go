// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestPeerOfReadsTheAgentOnARealPipe: the service reads the user of an agent from a real pipe
// connection before the agent has written anything, as Server.admit does.
func TestPeerOfReadsTheAgentOnARealPipe(t *testing.T) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	endpoint := fmt.Sprintf(`\\.\pipe\bothy-confirm-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	// The DACL of Listen grants interactive users; the account of a CI runner may be none, so the
	// test grants its own account.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + u.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := &pipeListener{path: endpoint, sa: &windows.SecurityAttributes{SecurityDescriptor: sd}, closed: ev}
	l.sa.Length = uint32(unsafe.Sizeof(*l.sa))
	if l.next, err = l.instance(true); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	type result struct {
		p   Peer
		err error
	}
	got := make(chan result, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			got <- result{err: err}
			return
		}
		defer func() { _ = c.Close() }()
		p, err := PeerOf(c)
		got <- result{p, err}
	}()
	c, err := dialEndpoint(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	r := <-got
	if r.err != nil || r.p.UID != u.User.Sid.String() || r.p.PID != os.Getpid() {
		t.Errorf("PeerOf: %+v %v", r.p, r.err)
	}
}
