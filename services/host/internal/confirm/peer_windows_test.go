// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/winpipe"
)

// TestListenSetsTheTokenUserAsPipeOwner holds the owner rule of docs/spec/27-computer-use.md 9.4:
// the pipe's owner is the account of the process that created it (LocalSystem or the service
// account), never the token's default owner, because the computer helper trusts the pipe by it.
func TestListenSetsTheTokenUserAsPipeOwner(t *testing.T) {
	endpoint := fmt.Sprintf(`\\.\pipe\bothy-confirm-owner-test-%d`, os.Getpid())
	l, err := Listen(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	sd, err := windows.GetSecurityInfo(l.(*winpipe.Listener).Waiting(), windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		t.Fatal(err)
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if !owner.Equals(u.User.Sid) {
		t.Fatalf("pipe owner %s, want the token user %s", owner, u.User.Sid)
	}
}

// TestListenServesASecondConnection: the second Accept creates a further instance of the pipe,
// which needs read and write for its creator besides FILE_CREATE_PIPE_INSTANCE; before, the
// second helper connection to the computer pipe failed with "Access is denied".
func TestListenServesASecondConnection(t *testing.T) {
	endpoint := fmt.Sprintf(`\\.\pipe\bothy-confirm-second-test-%d`, os.Getpid())
	l, err := Listen(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	ctx := testkit.Context(t)
	accepted := make(chan error, 2)
	go func() {
		for range 2 {
			c, err := l.Accept()
			if err == nil {
				_ = c.Close()
			}
			accepted <- err
		}
	}()
	for i := range 2 {
		c, err := dialEndpoint(ctx, endpoint)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		if err := <-accepted; err != nil {
			t.Fatalf("accept %d: %v", i, err)
		}
		_ = c.Close()
	}
}
