// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows"
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
	sd, err := windows.GetSecurityInfo(l.(*pipeListener).next, windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION)
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
