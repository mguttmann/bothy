// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"errors"
	"net"
	"strconv"

	"golang.org/x/sys/unix"
)

// solLocal is SOL_LOCAL of macOS (sys/un.h), the level of LOCAL_PEERCRED and LOCAL_PEERPID.
const solLocal = 0

// peerOf returns the credentials of the peer of c (LOCAL_PEERCRED and LOCAL_PEERPID).
func peerOf(c *net.UnixConn) (Peer, error) {
	if c == nil {
		return Peer{}, errors.New("localchan: no connection")
	}
	raw, err := c.SyscallConn()
	if err != nil {
		return Peer{}, err
	}
	var cred *unix.Xucred
	var pid int
	var err1, err2 error
	if err := raw.Control(func(fd uintptr) {
		cred, err1 = unix.GetsockoptXucred(int(fd), solLocal, unix.LOCAL_PEERCRED)
		pid, err2 = unix.GetsockoptInt(int(fd), solLocal, unix.LOCAL_PEERPID)
	}); err != nil {
		return Peer{}, err
	}
	if err1 != nil || err2 != nil {
		return Peer{}, errors.New("localchan: peer credentials unavailable")
	}
	return Peer{UID: strconv.FormatUint(uint64(cred.Uid), 10), PID: pid}, nil
}
