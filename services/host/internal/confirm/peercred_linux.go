// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"net"
	"strconv"

	"golang.org/x/sys/unix"
)

// peerCred returns the credentials of the peer of c (SO_PEERCRED). A copy of peerOf in
// internal/localchan/peercred_linux.go, which this package may not import.
func peerCred(c *net.UnixConn) (Peer, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return Peer{}, err
	}
	var cred *unix.Ucred
	var serr error
	if err := raw.Control(func(fd uintptr) {
		cred, serr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return Peer{}, err
	}
	if serr != nil {
		return Peer{}, serr
	}
	return Peer{UID: strconv.FormatUint(uint64(cred.Uid), 10), PID: int(cred.Pid)}, nil
}
