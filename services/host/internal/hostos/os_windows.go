// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// fileIDInfo is FILE_ID_INFO: the volume serial number and the 128-bit file ID.
type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

func identity(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", ErrUnresolvable
	}
	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", ErrUnresolvable
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var info fileIDInfo
	if err := windows.GetFileInformationByHandleEx(h, windows.FileIdInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil { //nolint:gosec // fixed-size struct of the API
		return "", ErrUnresolvable
	}
	return fmt.Sprintf("%016x:%x", info.VolumeSerialNumber, info.FileID), nil
}

// Windows ACE types and flags.
const (
	aceAllowed     = 0
	aceDenied      = 1
	aceInheritOnly = 0x08
)

// WindowsSecurity is the Security of Windows: owner and DACL of the security descriptor.
type WindowsSecurity struct{}

// Lstat implements Security.
func (WindowsSecurity) Lstat(path string) (FileSec, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return FileSec{}, err
	}
	var f FileSec
	attrs, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path))
	if err == nil {
		f.IsDir = attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0
		f.IsLink = attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	if owner, _, err := sd.Owner(); err == nil && owner != nil {
		f.OwnerID = owner.String()
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		// No DACL means full access for everyone.
		f.ACL = []ACE{{Unmappable: true}}
		return f, nil
	}
	f.ACL = DACLEntries(dacl)
	return f, nil
}

// DACLEntries converts a DACL to ACEs; an entry of another type than allow or deny is unmappable.
func DACLEntries(dacl *windows.ACL) []ACE {
	var out []ACE
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			out = append(out, ACE{Unmappable: true})
			continue
		}
		t := ace.Header.AceType
		if t != aceAllowed && t != aceDenied {
			out = append(out, ACE{Unmappable: true})
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)) //nolint:gosec // layout of ACCESS_ALLOWED_ACE
		out = append(out, ACE{Allow: t == aceAllowed, Kind: PrincipalSID, ID: sid.String(), Rights: WindowsRights(uint32(ace.Mask)),
			InheritOnly: ace.Header.AceFlags&aceInheritOnly != 0})
	}
	return out
}

// WindowsUsers is the Users of Windows: account SIDs, the profiles of ProfileList and the groups
// of NetUserGetLocalGroups and NetUserGetGroups.
type WindowsUsers struct{}

const profileList = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`

// Accounts implements Users: every account with a profile under ProfileList is a person account
// (D-M2-42-6); the values Default and Public name no account and are not subkeys there.
func (WindowsUsers) Accounts() ([]Account, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, profileList, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, err
	}
	defer func() { _ = k.Close() }()
	sids, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}
	var out []Account
	for _, s := range sids {
		sub, err := registry.OpenKey(k, s, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		home, _, err := sub.GetStringValue("ProfileImagePath")
		_ = sub.Close()
		if err != nil {
			continue
		}
		name := s
		if sid, err := windows.StringToSid(s); err == nil {
			if acc, dom, _, err := sid.LookupAccount(""); err == nil {
				name = acc
				_ = dom
			}
		}
		out = append(out, Account{Name: name, ID: s, Home: home, Person: !strings.HasPrefix(s, "S-1-5-18") && !strings.HasPrefix(s, "S-1-5-19") && !strings.HasPrefix(s, "S-1-5-20")})
	}
	return out, nil
}

// Lookup implements Users.
func (u WindowsUsers) Lookup(name string) (Account, error) {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return Account{}, ErrUnknownUser
	}
	acc := Account{Name: name, ID: sid.String()}
	if accs, err := u.Accounts(); err == nil {
		for _, a := range accs {
			if a.ID == acc.ID {
				acc.Home, acc.Person = a.Home, a.Person
			}
		}
	}
	return acc, nil
}

var (
	netapi32                  = windows.NewLazySystemDLL("netapi32.dll")
	procNetUserGetLocalGroups = netapi32.NewProc("NetUserGetLocalGroups")
	procNetUserGetGroups      = netapi32.NewProc("NetUserGetGroups")
	procNetLocalGroupEnum     = netapi32.NewProc("NetLocalGroupEnum")
	procNetLocalGroupMembers  = netapi32.NewProc("NetLocalGroupGetMembers")
)

type groupInfo0 struct{ Name *uint16 }

// Groups implements Users: local and global groups of name plus the well-known groups that a token
// of an interactive or service logon of a local account can hold (Everyone, Authenticated Users,
// Users, INTERACTIVE, SERVICE, LOCAL, CONSOLE LOGON, This Organization, Local account). The list
// over-approximates on purpose: a right granted to any of them counts, so the checks fail closed.
func (WindowsUsers) Groups(name string) ([]Group, error) {
	out := []Group{
		{Name: "Everyone", ID: SIDEveryone}, {Name: "Authenticated Users", ID: SIDAuthenticated}, {Name: "Users", ID: SIDUsers},
		{Name: "INTERACTIVE", ID: SIDInteractive}, {Name: "SERVICE", ID: SIDService}, {Name: "LOCAL", ID: SIDLocal},
		{Name: "CONSOLE LOGON", ID: SIDConsoleLogon}, {Name: "This Organization", ID: SIDThisOrg}, {Name: "Local account", ID: SIDLocalAccount},
	}
	u, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, ErrUnknownUser
	}
	local, ok := netUserGroups(procNetUserGetLocalGroups, u)
	if !ok {
		// A virtual account (NT SERVICE\bothy-host, the account of the host service) is no user
		// of the account database, so NetUserGetLocalGroups does not know it; its local groups
		// are read by SID.
		extra, err := virtualAccountGroups(name)
		if err != nil {
			return nil, err
		}
		return append(out, extra...), nil
	}
	global, _ := netUserGroups(procNetUserGetGroups, u)
	return append(append(out, local...), global...), nil
}

// netUserGroups calls NetUserGetLocalGroups or NetUserGetGroups for the user u and returns the
// groups with their SIDs; false when the call failed.
func netUserGroups(proc *windows.LazyProc, u *uint16) ([]Group, bool) {
	var buf *byte
	var read, total uint32
	var r uintptr
	if proc == procNetUserGetLocalGroups {
		r, _, _ = proc.Call(0, uintptr(unsafe.Pointer(u)), 0, 1, uintptr(unsafe.Pointer(&buf)), 0xFFFFFFFF, uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total))) //nolint:gosec // documented signature
	} else {
		r, _, _ = proc.Call(0, uintptr(unsafe.Pointer(u)), 0, uintptr(unsafe.Pointer(&buf)), 0xFFFFFFFF, uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total))) //nolint:gosec // documented signature
	}
	if r != 0 {
		return nil, false
	}
	defer func() { _ = windows.NetApiBufferFree(buf) }()
	var out []Group
	for _, e := range unsafe.Slice((*groupInfo0)(unsafe.Pointer(buf)), read) { //nolint:gosec // array returned by the API
		gname := windows.UTF16PtrToString(e.Name)
		g := Group{Name: gname, ID: gname}
		if sid, _, _, err := windows.LookupSID("", gname); err == nil {
			g.ID = sid.String()
		}
		out = append(out, g)
	}
	return out, true
}

type localGroupMember0 struct{ SID *windows.SID }

// virtualAccountGroups returns the local groups that name is a member of, for an account that
// resolves to a SID but is no user of the local account database (a virtual service account). An
// unknown name or an unreadable group is an error, so HD2 fails closed. Without it HD2 read no
// groups for the account of the host service and reported forbidden_group on every Windows host
// (run 45770587799, docs/decisions.d/AUD-PKG-R2-06-host-platform-ci.md).
func virtualAccountGroups(name string) ([]Group, error) {
	sid, _, typ, err := windows.LookupSID("", name)
	if err != nil || typ == windows.SidTypeUser {
		return nil, fmt.Errorf("%w: groups", ErrUnknownUser)
	}
	var buf *byte
	var read, total uint32
	var resume uintptr
	r, _, _ := procNetLocalGroupEnum.Call(0, 0, uintptr(unsafe.Pointer(&buf)), 0xFFFFFFFF, uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&resume))) //nolint:gosec // documented signature
	if r != 0 {
		return nil, fmt.Errorf("%w: local groups %d", ErrUnknownUser, r)
	}
	defer func() { _ = windows.NetApiBufferFree(buf) }()
	var out []Group
	for _, g := range unsafe.Slice((*groupInfo0)(unsafe.Pointer(buf)), read) { //nolint:gosec // array returned by the API
		member, err := localGroupHas(g.Name, sid)
		if err != nil {
			return nil, err
		}
		if !member {
			continue
		}
		gname := windows.UTF16PtrToString(g.Name)
		group := Group{Name: gname, ID: gname}
		if gsid, _, _, err := windows.LookupSID("", gname); err == nil {
			group.ID = gsid.String()
		}
		out = append(out, group)
	}
	return out, nil
}

// localGroupHas reports whether sid is a direct member of the local group name.
func localGroupHas(name *uint16, sid *windows.SID) (bool, error) {
	var buf *byte
	var read, total uint32
	var resume uintptr
	r, _, _ := procNetLocalGroupMembers.Call(0, uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&buf)), 0xFFFFFFFF, uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&resume))) //nolint:gosec // documented signature
	if r != 0 {
		return false, fmt.Errorf("%w: members of a local group %d", ErrUnknownUser, r)
	}
	defer func() { _ = windows.NetApiBufferFree(buf) }()
	for _, m := range unsafe.Slice((*localGroupMember0)(unsafe.Pointer(buf)), read) { //nolint:gosec // array returned by the API
		if m.SID != nil && m.SID.Equals(sid) {
			return true, nil
		}
	}
	return false, nil
}

// WindowsProbe is the Probe of Windows: HD3 has no probe there (D-M2-42-6), Yama does not exist.
type WindowsProbe struct{}

// SudoWithoutPassword implements Probe; elevation needs the credentials of an administrator.
func (WindowsProbe) SudoWithoutPassword(context.Context) (bool, error) { return false, nil }

// PtraceScope implements Probe.
func (WindowsProbe) PtraceScope() (int, error) { return -1, nil }

// Platform returns the Users, Security and Probe of Windows.
func Platform(Runner) (Users, Security, Probe) {
	return WindowsUsers{}, WindowsSecurity{}, WindowsProbe{}
}
