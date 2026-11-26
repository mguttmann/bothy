// SPDX-License-Identifier: Apache-2.0

package localchan

// CreatorRights are the rights a server needs on its own pipe to create a further instance:
// FILE_CREATE_PIPE_INSTANCE alone is not enough, because CreateNamedPipe of a further instance
// is checked against the access its open mode asks for, which for PIPE_ACCESS_DUPLEX is
// FILE_GENERIC_READ and FILE_GENERIC_WRITE (FILE_GENERIC_WRITE contains FILE_APPEND_DATA, which
// on a pipe is FILE_CREATE_PIPE_INSTANCE). No WRITE_DAC, WRITE_OWNER or DELETE.
const CreatorRights = "0x12019f"

// WithCreator returns the SDDL of a pipe with one more entry that gives the account sid, the
// account that creates the pipe, CreatorRights. The OWNER RIGHTS entry (ownerCreatesInstances)
// grants FILE_CREATE_PIPE_INSTANCE only, so every server of these pipes failed to create its
// second instance with "Access is denied" (the event log of the helper, run 45842510971): the
// helper ended with the first connection of the host service, and the host service saw
// helper_unreachable (AUD-PKG-R2-06/8). A client of the creator's own account still gets nothing:
// the server checks the token of every client. It lives in this untagged file so its form is
// tested on every build host.
func WithCreator(sddl, sid string) string {
	if sid == "" {
		return sddl
	}
	return sddl + "(A;;" + CreatorRights + ";;;" + sid + ")"
}
