// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package confirm

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
)

// Listen opens the Unix socket endpoint in its existing folder (created by the service manager or
// install; Listen never creates it) with mode 0666, so the agent of any local user can connect and
// the peer check decides. A stale socket of an earlier run is removed; any other file at endpoint
// stays and Listen fails.
func Listen(endpoint string) (net.Listener, error) {
	if fi, err := os.Lstat(endpoint); err == nil {
		if fi.Mode().Type() != fs.ModeSocket {
			return nil, errors.New("confirm: the endpoint path is taken by a file that is not a socket")
		}
		if err := os.Remove(endpoint); err != nil {
			return nil, fmt.Errorf("confirm: remove stale socket: %w", err)
		}
	}
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("confirm: endpoint: %w", err)
	}
	if err := os.Chmod(endpoint, 0o666); err != nil { //nolint:gosec // every local user may connect; the peer check decides (Design note 15 a)
		_ = l.Close()
		return nil, fmt.Errorf("confirm: endpoint: %w", err)
	}
	return l, nil
}

// dialEndpoint connects to the Unix socket endpoint.
func dialEndpoint(ctx context.Context, endpoint string) (net.Conn, error) {
	var d net.Dialer // nosemgrep: bothy-go-egress-only -- the Unix socket of the local confirmation on this host, never a network connection (Design note 15 a)
	c, err := d.DialContext(ctx, "unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("confirm: dial: %w", err)
	}
	return c, nil
}

// PeerOf returns the uid and process ID of the process at the other end of c, which must be a
// Unix socket connection (Linux SO_PEERCRED, macOS LOCAL_PEERCRED and LOCAL_PEERPID).
func PeerOf(c net.Conn) (Peer, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok || uc == nil {
		return Peer{}, errors.New("confirm: not a unix socket")
	}
	return peerCred(uc)
}
