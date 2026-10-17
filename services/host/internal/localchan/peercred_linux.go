// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"errors"
	"net"
	"strconv"

	"golang.org/x/sys/unix"
)

// peerOf returns the credentials of the peer of c (SO_PEERCRED).
func peerOf(c *net.UnixConn) (Peer, error) {
	if c == nil {
		return Peer{}, errors.New("localchan: no connection")
	}
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
