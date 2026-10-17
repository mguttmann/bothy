// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"io/fs"
	"testing"
)

var alex = Subject{Name: "bothy-s-alex", ID: "1001", GroupIDs: []string{"1001", "50"}}

func TestWritableUnixMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    FileSec
		want bool
	}{
		{"root 0755", FileSec{OwnerID: "0", GroupID: "0", Mode: 0o755}, false},
		{"owned by alex 0500", FileSec{OwnerID: "1001", GroupID: "0", Mode: 0o500}, true},
		{"group w of alex's group", FileSec{OwnerID: "0", GroupID: "50", Mode: 0o775}, true},
		{"group w of another group", FileSec{OwnerID: "0", GroupID: "60", Mode: 0o775}, false},
		{"others w", FileSec{OwnerID: "0", GroupID: "0", Mode: 0o757}, true},
		{"posix ACL named user w", FileSec{OwnerID: "0", Mode: 0o755, ACL: []ACE{{Allow: true, Kind: PrincipalUser, ID: "1001", Rights: RightWriteData}}}, true},
		{"posix ACL named group w", FileSec{OwnerID: "0", Mode: 0o755, ACL: []ACE{{Allow: true, Kind: PrincipalGroup, ID: "50", Rights: RightAppend}}}, true},
		{"posix ACL other user", FileSec{OwnerID: "0", Mode: 0o755, ACL: []ACE{{Allow: true, Kind: PrincipalUser, ID: "1002", Rights: WriteRights}}}, false},
		{"unmappable entry", FileSec{OwnerID: "0", Mode: 0o700, ACL: []ACE{{Unmappable: true}}}, true},
		{"deny entry ignored", FileSec{OwnerID: "0", Mode: 0o755, ACL: []ACE{{Allow: false, Kind: PrincipalEveryone, Rights: WriteRights}}}, false},
		{"unknown principal kind", FileSec{OwnerID: "0", Mode: 0o755, ACL: []ACE{{Allow: true, Kind: "weird", Rights: RightDelete}}}, true},
	} {
		if got := Writable(tc.f, alex); got != tc.want {
			t.Errorf("%s: Writable = %v, want %v", tc.name, got, tc.want)
		}
	}
	if !Readable(FileSec{OwnerID: "0", Mode: 0o704}, alex) || Readable(FileSec{OwnerID: "0", Mode: 0o700}, alex) {
		t.Error("Readable by others bit")
	}
	if !Readable(FileSec{OwnerID: "0", Mode: 0o701}, alex) {
		t.Error("execute counts as access")
	}
}

// TestWritableMacRights checks each right of chmod(1) that 24 2.2 point 6 names as its own row.
func TestWritableMacRights(t *testing.T) {
	for _, tc := range []struct {
		right string
		want  bool
	}{
		{"write", true}, {"append", true}, {"add_file", true}, {"add_subdirectory", true}, {"delete_child", true},
		{"delete", true}, {"writeattr", true}, {"writeextattr", true}, {"writesecurity", true}, {"chown", true},
		{"read", false}, {"list", false}, {"search", false}, {"execute", false}, {"readattr", false},
		{"readextattr", false}, {"readsecurity", false}, {"file_inherit", false},
	} {
		r, ok := MacRights([]string{tc.right})
		if !ok {
			t.Fatalf("%s unknown", tc.right)
		}
		for _, kind := range []ACE{{Kind: PrincipalUser, ID: "1001"}, {Kind: PrincipalGroup, ID: "50"}, {Kind: PrincipalEveryone}} {
			kind.Allow, kind.Rights = true, r
			f := FileSec{OwnerID: "0", Mode: 0o755, ACL: []ACE{kind}}
			if got := Writable(f, alex); got != tc.want {
				t.Errorf("%s for %s: Writable = %v", tc.right, kind.Kind, got)
			}
		}
	}
	if r, ok := MacRights([]string{"frobnicate"}); ok || r != AllRights {
		t.Error("unknown macOS right accepted")
	}
}

// TestWritableWindowsRights checks each Windows right of 24 2.2 point 6 and D-W-86 as its own row.
func TestWritableWindowsRights(t *testing.T) {
	for _, tc := range []struct {
		name string
		mask uint32
		want bool
	}{
		{"FILE_ADD_FILE/FILE_WRITE_DATA", winWriteData, true},
		{"FILE_ADD_SUBDIRECTORY/FILE_APPEND_DATA", winAppendData, true},
		{"FILE_DELETE_CHILD", winDeleteChild, true},
		{"FILE_WRITE_ATTRIBUTES", winWriteAttributes, true},
		{"FILE_WRITE_EA", winWriteEA, true},
		{"DELETE", winDelete, true},
		{"WRITE_DAC", winWriteDAC, true},
		{"WRITE_OWNER", winWriteOwner, true},
		{"GENERIC_WRITE", winGenericWrite, true},
		{"GENERIC_ALL", winGenericAll, true},
		{"FILE_READ_DATA", winReadData, false},
		{"FILE_EXECUTE", winExecute, false},
		{"GENERIC_READ", winGenericRead, false},
		{"GENERIC_EXECUTE", winGenericExecute, false},
	} {
		user := Subject{ID: "S-1-5-21-1-2-3-1001", GroupIDs: []string{SIDEveryone, SIDAuthenticated, SIDUsers}}
		for _, sid := range []string{user.ID, SIDEveryone, SIDAuthenticated, SIDUsers} {
			f := FileSec{OwnerID: SIDSystem, ACL: []ACE{{Allow: true, Kind: PrincipalSID, ID: sid, Rights: WindowsRights(tc.mask)}}}
			if got := Writable(f, user); got != tc.want {
				t.Errorf("%s for %s: Writable = %v", tc.name, sid, got)
			}
		}
		// An inherit-only entry gives the folder itself nothing.
		f := FileSec{OwnerID: SIDSystem, ACL: []ACE{{Allow: true, Kind: PrincipalSID, ID: SIDAuthenticated, Rights: WindowsRights(tc.mask), InheritOnly: true}}}
		if Writable(f, user) {
			t.Errorf("%s inherit-only counted", tc.name)
		}
	}
	if WindowsRights(winGenericAll) != AllRights || WindowsRights(winGenericRead)&RightRead == 0 {
		t.Error("generic expansion")
	}
}

func TestTrustedOwner(t *testing.T) {
	for _, tc := range []struct {
		goos, owner string
		want        bool
	}{
		{"linux", "0", true}, {"darwin", "0", true}, {"linux", "1000", false},
		{"windows", SIDSystem, true}, {"windows", SIDAdministrators, true}, {"windows", SIDTrustedInstall, true},
		{"windows", "S-1-5-21-1-2-3-500", false}, {"windows", "0", false},
	} {
		if got := TrustedOwner(tc.goos, FileSec{OwnerID: tc.owner}); got != tc.want {
			t.Errorf("%s %s: %v", tc.goos, tc.owner, got)
		}
	}
}

func TestModeRights(t *testing.T) {
	if modeRights(fs.FileMode(7)) != RightRead|RightExecute|RightWriteData|RightAppend|RightDeleteChild {
		t.Error("rwx")
	}
}
