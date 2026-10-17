// SPDX-License-Identifier: Apache-2.0

package install

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// Layout of a binary ACL and its ACEs (MS-DTYP 2.4.4, 2.4.5).
const (
	aclHeaderLen        = 8
	aceHeaderLen        = 4
	aceAccessAllowed    = 0x00
	accessAppendData    = 0x00000004 // FILE_APPEND_DATA = FILE_ADD_SUBDIRECTORY, the (AD) of icacls
	aceInheritanceFlags = 0x1f       // OI, CI, NP, IO and INHERITED
)

// sidAuthenticatedUsers is S-1-5-11 in binary form: revision 1, one sub-authority, authority 5,
// sub-authority 11.
var sidAuthenticatedUsers = []byte{1, 1, 0, 0, 0, 0, 0, 5, 11, 0, 0, 0}

// ErrACLFormat is returned for a binary ACL that cannot be parsed; nothing is written then.
var ErrACLFormat = errors.New("install: unreadable ACL")

// RemoveAuthenticatedUsersAD returns acl without the access-allowed entry for Authenticated
// Users (S-1-5-11) with exactly the right (AD) and no inheritance flags (24 2.2 point 6,
// D-W-87). Every other entry stays in its order with its flags, also the inheritable
// (OI)(CI)(IO)(M) entry. removed tells whether the entry was there; without it acl is returned
// unchanged. The control bits of the security descriptor are not part of the ACL and stay with the
// caller.
func RemoveAuthenticatedUsersAD(acl []byte) (out []byte, removed bool, err error) {
	if len(acl) < aclHeaderLen {
		return nil, false, ErrACLFormat
	}
	size := int(binary.LittleEndian.Uint16(acl[2:]))
	count := int(binary.LittleEndian.Uint16(acl[4:]))
	if size < aclHeaderLen || size > len(acl) {
		return nil, false, ErrACLFormat
	}
	var kept [][]byte
	off := aclHeaderLen
	for i := 0; i < count; i++ {
		if off+aceHeaderLen > size {
			return nil, false, ErrACLFormat
		}
		aceSize := int(binary.LittleEndian.Uint16(acl[off+2:]))
		if aceSize < aceHeaderLen || off+aceSize > size {
			return nil, false, ErrACLFormat
		}
		ace := acl[off : off+aceSize]
		if !removed && isAuthenticatedUsersAD(ace) {
			removed = true
		} else {
			kept = append(kept, ace)
		}
		off += aceSize
	}
	if !removed {
		return append([]byte(nil), acl[:size]...), false, nil
	}
	body := bytes.Join(kept, nil)
	out = make([]byte, aclHeaderLen, aclHeaderLen+len(body))
	copy(out, acl[:aclHeaderLen])
	binary.LittleEndian.PutUint16(out[2:], uint16(aclHeaderLen+len(body))) //nolint:gosec // smaller than the input size
	binary.LittleEndian.PutUint16(out[4:], uint16(len(kept)))              //nolint:gosec // smaller than the input count
	return append(out, body...), true, nil
}

func isAuthenticatedUsersAD(ace []byte) bool {
	if len(ace) != aceHeaderLen+4+len(sidAuthenticatedUsers) || ace[0] != aceAccessAllowed || ace[1]&aceInheritanceFlags != 0 {
		return false
	}
	return binary.LittleEndian.Uint32(ace[4:]) == accessAppendData && bytes.Equal(ace[8:], sidAuthenticatedUsers)
}
