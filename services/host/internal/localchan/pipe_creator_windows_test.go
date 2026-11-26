// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestListenPipeServesASecondConnection: a pipe with the helper DACL serves two connections one
// after the other; the second needs a further instance, which ListenPipe grants its creator
// (WithCreator). Before, the second admin or helper connection failed with "Access is denied".
func TestListenPipeServesASecondConnection(t *testing.T) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf(`\\.\pipe\bothy-localchan-second-test-%d`, os.Getpid())
	l, err := ListenPipe(path, HelperPipeSDDL(u.User.Sid.String()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
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
		c, err := dialPipe(ctx, path)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		if err := <-accepted; err != nil {
			t.Fatalf("accept %d: %v", i, err)
		}
		_ = c.Close()
	}
}
