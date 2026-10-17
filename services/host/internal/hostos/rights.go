// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"io/fs"
	"slices"
	"strings"
)

// Rights is a set of access rights in a form shared by the platforms. A folder's "create a file"
// and a file's "write data" share a bit, as on Windows (FILE_ADD_FILE equals FILE_WRITE_DATA).
type Rights uint32

// The rights of 24 2.2 point 6. RightWriteData is write (file) or add_file (folder),
// RightAppend is append (file) or add_subdirectory (folder).
const (
	RightRead Rights = 1 << iota
	RightExecute
	RightWriteData
	RightAppend
	RightDeleteChild
	RightDelete
	RightWriteAttr
	RightWriteEA
	RightWriteDAC
	RightWriteOwner

	// WriteRights are the rights that make a file or folder "writable" (24 2.2 point 6).
	WriteRights = RightWriteData | RightAppend | RightDeleteChild | RightDelete | RightWriteAttr |
		RightWriteEA | RightWriteDAC | RightWriteOwner
	// AllRights is every right; an unmappable entry grants it (fail closed).
	AllRights = RightRead | RightExecute | WriteRights
)

// Principal kinds of an ACE.
const (
	PrincipalUser     = "user"
	PrincipalGroup    = "group"
	PrincipalEveryone = "everyone"
	// PrincipalSID is a Windows SID, matched against the user SID and the token group SIDs.
	PrincipalSID = "sid"
)

// ACE is one access control entry in the shared form.
type ACE struct {
	// Allow is false for a deny entry; deny entries are ignored (an ignored deny only adds rights).
	Allow bool
	// Kind is one of the Principal constants; ID the uid, gid or SID.
	Kind, ID string
	// Rights are the rights the entry gives.
	Rights Rights
	// InheritOnly marks an entry that only children inherit; it gives the object itself nothing.
	InheritOnly bool
	// Unmappable marks an entry that could not be read, parsed or mapped: it counts as every right
	// for everyone.
	Unmappable bool
}

// FileSec is the security of one file system object.
type FileSec struct {
	IsDir, IsLink bool
	// OwnerID is the uid or SID of the owner; GroupID the gid of the group (Linux, macOS).
	OwnerID, GroupID string
	// Mode carries the permission bits and fs.ModeSetuid, fs.ModeSetgid (Linux, macOS).
	Mode fs.FileMode
	// ACL holds the entries of the POSIX ACL (Linux), the extended ACL (macOS) or the DACL (Windows).
	ACL []ACE
}

// Security reads the security of paths without following a final link.
type Security interface {
	// Lstat returns the security of path itself.
	Lstat(path string) (FileSec, error)
}

// rightsOf returns the rights fs gives s. Owning an object counts as every right, because the
// owner can change its rights.
func rightsOf(f FileSec, s Subject) Rights {
	if f.OwnerID != "" && f.OwnerID == s.ID {
		return AllRights
	}
	if f.IsLink && len(f.ACL) == 0 {
		// The mode of a symbolic link is not used by Linux and macOS; only its owner can replace it,
		// by owning it or by writing its folder, which is checked as the folder.
		return 0
	}
	var r Rights
	perm := f.Mode.Perm()
	if f.GroupID != "" && slices.Contains(s.GroupIDs, f.GroupID) {
		r |= modeRights(perm >> 3)
	}
	r |= modeRights(perm)
	for _, e := range f.ACL {
		switch {
		case e.Unmappable:
			r |= AllRights
		case !e.Allow || e.InheritOnly:
		case matches(e, s):
			r |= e.Rights
		}
	}
	return r
}

func matches(e ACE, s Subject) bool {
	switch e.Kind {
	case PrincipalEveryone:
		return true
	case PrincipalUser:
		return e.ID == s.ID
	case PrincipalGroup:
		return slices.Contains(s.GroupIDs, e.ID)
	case PrincipalSID:
		return e.ID == s.ID || slices.Contains(s.GroupIDs, e.ID)
	}
	return true // an unknown kind is unmappable
}

// modeRights maps the low three bits of a mode (rwx) to rights; w is every write right of a
// folder entry (create, append, delete children).
func modeRights(bits fs.FileMode) Rights {
	var r Rights
	if bits&4 != 0 {
		r |= RightRead
	}
	if bits&2 != 0 {
		r |= RightWriteData | RightAppend | RightDeleteChild
	}
	if bits&1 != 0 {
		r |= RightExecute
	}
	return r
}

// Writable reports whether s can write f in the sense of 24 2.2 point 6: the owner, the mode bit w
// of its group or of others, or any write right of an ACL entry for it, one of its groups or
// everyone.
func Writable(f FileSec, s Subject) bool { return rightsOf(f, s)&WriteRights != 0 }

// Readable reports whether s can read or traverse f (the place of a session user gives no other
// session user read or execute access, HD11).
func Readable(f FileSec, s Subject) bool { return rightsOf(f, s)&(RightRead|RightExecute) != 0 }

// Windows SIDs of the trusted owners (24 2.2 point 6).
const (
	SIDSystem         = "S-1-5-18"
	SIDAdministrators = "S-1-5-32-544"
	SIDTrustedInstall = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	SIDEveryone       = "S-1-1-0"
	SIDAuthenticated  = "S-1-5-11"
	SIDUsers          = "S-1-5-32-545"
	SIDOwnerRights    = "S-1-3-4"
	SIDInteractive    = "S-1-5-4"
)

// TrustedOwner reports whether the owner of f is a trusted owner of the platform goos: root on
// Linux and macOS, SYSTEM, Administrators or NT SERVICE\TrustedInstaller on Windows.
func TrustedOwner(goos string, f FileSec) bool {
	if goos == "windows" {
		return f.OwnerID == SIDSystem || f.OwnerID == SIDAdministrators || f.OwnerID == SIDTrustedInstall
	}
	return f.OwnerID == "0"
}

// Windows access masks (file-access-rights-constants, generic rights).
const (
	winReadData        = 0x1
	winWriteData       = 0x2
	winAppendData      = 0x4
	winWriteEA         = 0x10
	winExecute         = 0x20
	winDeleteChild     = 0x40
	winWriteAttributes = 0x100
	winDelete          = 0x10000
	winWriteDAC        = 0x40000
	winWriteOwner      = 0x80000
	winGenericAll      = 0x10000000
	winGenericExecute  = 0x20000000
	winGenericWrite    = 0x40000000
	winGenericRead     = 0x80000000
)

// WindowsRights maps a Windows access mask to rights. GENERIC_WRITE counts with the rights of
// FILE_GENERIC_WRITE, GENERIC_ALL as every right.
func WindowsRights(mask uint32) Rights {
	if mask&winGenericAll != 0 {
		return AllRights
	}
	if mask&winGenericWrite != 0 {
		mask |= winWriteData | winAppendData | winWriteEA | winWriteAttributes
	}
	if mask&winGenericRead != 0 {
		mask |= winReadData
	}
	if mask&winGenericExecute != 0 {
		mask |= winExecute
	}
	var r Rights
	for bit, right := range map[uint32]Rights{
		winReadData: RightRead, winExecute: RightExecute, winWriteData: RightWriteData, winAppendData: RightAppend,
		winDeleteChild: RightDeleteChild, winDelete: RightDelete, winWriteAttributes: RightWriteAttr,
		winWriteEA: RightWriteEA, winWriteDAC: RightWriteDAC, winWriteOwner: RightWriteOwner,
	} {
		if mask&bit != 0 {
			r |= right
		}
	}
	return r
}

// MacRights maps the rights of a macOS ACL entry (chmod(1) names) to rights. ok is false when a
// name is unknown; the caller then treats the entry as unmappable.
func MacRights(names []string) (r Rights, ok bool) {
	table := map[string]Rights{
		"read": RightRead, "list": RightRead, "readattr": RightRead, "readextattr": RightRead, "readsecurity": RightRead,
		"execute": RightExecute, "search": RightExecute,
		"write": RightWriteData, "add_file": RightWriteData, "append": RightAppend, "add_subdirectory": RightAppend,
		"delete_child": RightDeleteChild, "delete": RightDelete, "writeattr": RightWriteAttr, "writeextattr": RightWriteEA,
		"writesecurity": RightWriteDAC, "chown": RightWriteOwner,
		// Inheritance flags carry no right of their own.
		"file_inherit": 0, "directory_inherit": 0, "limit_inherit": 0, "only_inherit": 0, "inherited": 0,
	}
	for _, n := range names {
		v, known := table[strings.TrimSpace(n)]
		if !known {
			return AllRights, false
		}
		r |= v
	}
	return r, true
}
