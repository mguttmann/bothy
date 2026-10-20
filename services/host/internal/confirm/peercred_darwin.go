// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"errors"
	"net"
	"strconv"

	"golang.org/x/sys/unix"
)

// solLocal is SOL_LOCAL of macOS (sys/un.h), the level of LOCAL_PEERCRED and LOCAL_PEERPID.
const solLocal = 0

// peerCred returns the credentials of the peer of c (LOCAL_PEERCRED and LOCAL_PEERPID). A copy of
// peerOf in internal/localchan/peercred_darwin.go, which this package may not import.
func peerCred(c *net.UnixConn) (Peer, error) {
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
		return Peer{}, errors.New("confirm: peer credentials unavailable")
	}
	return Peer{UID: strconv.FormatUint(uint64(cred.Uid), 10), PID: pid}, nil
}
