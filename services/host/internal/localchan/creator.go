// SPDX-License-Identifier: Apache-2.0

package localchan

// WithCreator returns the SDDL of a pipe with one more entry that lets the account sid, the
// account that creates the pipe, create further instances of it (FILE_CREATE_PIPE_INSTANCE)
// and nothing else. The OWNER RIGHTS entry of the DACLs (ownerCreatesInstances) grants the same
// to the owner of the pipe, but a server of these pipes failed to create its second instance on
// Windows, so every helper ended with its first connection and the host service saw
// helper_unreachable (AUD-PKG-R2-06/8); naming the account does not depend on which owner the
// pipe got. It lives in this untagged file so its form is tested on every build host.
func WithCreator(sddl, sid string) string {
	if sid == "" {
		return sddl
	}
	return sddl + "(A;;0x4;;;" + sid + ")"
}
