// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"encoding/binary"
	"errors"
	"testing"
)

func posixACL(entries ...[3]uint32) []byte {
	b := binary.LittleEndian.AppendUint32(nil, 2)
	for _, e := range entries {
		b = binary.LittleEndian.AppendUint16(b, uint16(e[0])) //nolint:gosec // small test values
		b = binary.LittleEndian.AppendUint16(b, uint16(e[1])) //nolint:gosec // small test values
		b = binary.LittleEndian.AppendUint32(b, e[2])
	}
	return b
}

func TestParsePosixACL(t *testing.T) {
	aces, err := ParsePosixACL(posixACL([3]uint32{aclUserObj, 7, 0}, [3]uint32{aclUser, 6, 1001}, [3]uint32{aclGroup, 2, 50},
		[3]uint32{aclMask, 5, 0}, [3]uint32{aclOther, 5, 0}, [3]uint32{0x40, 7, 0}))
	if err != nil {
		t.Fatal(err)
	}
	if len(aces) != 3 || aces[0].Kind != PrincipalUser || aces[0].ID != "1001" || aces[0].Rights&RightWriteData == 0 ||
		aces[1].Kind != PrincipalGroup || aces[1].ID != "50" || !aces[2].Unmappable {
		t.Errorf("aces %+v", aces)
	}
	for _, bad := range [][]byte{nil, {1, 0, 0, 0}, append(posixACL(), 1)} {
		if _, err := ParsePosixACL(bad); !errors.Is(err, ErrACLFormat) {
			t.Errorf("%v accepted", bad)
		}
	}
}

func TestParseMacACL(t *testing.T) {
	out := []byte(`drwxr-xr-x+ 3 root  wheel  96 Oct 15 09:00 /Library/x
 0: user:bothy-s-alex allow add_file,delete_child
 1: group:staff allow list,search,file_inherit,directory_inherit,only_inherit
 2: everyone deny delete
 3: user:ghost allow write
 4: group:everyone allow frobnicate
 garbage line
`)
	users := func(n string) (string, error) {
		if n == "bothy-s-alex" {
			return "1001", nil
		}
		return "", ErrUnknownUser
	}
	groups := func(n string) (string, error) { return "20", nil }
	aces := ParseMacACL(out, users, groups)
	if len(aces) != 6 {
		t.Fatalf("aces %+v", aces)
	}
	if aces[0].Kind != PrincipalUser || aces[0].ID != "1001" || aces[0].Rights != RightWriteData|RightDeleteChild || !aces[0].Allow {
		t.Errorf("0: %+v", aces[0])
	}
	if aces[1].Kind != PrincipalGroup || !aces[1].InheritOnly {
		t.Errorf("1: %+v", aces[1])
	}
	if aces[2].Kind != PrincipalEveryone || aces[2].Allow {
		t.Errorf("2: %+v", aces[2])
	}
	if !aces[3].Unmappable || !aces[4].Unmappable || !aces[5].Unmappable {
		t.Errorf("unmappable entries %+v", aces[3:])
	}
}

func TestParseAccounts(t *testing.T) {
	accs := ParsePasswd([]byte("root:x:0:0:root:/root:/bin/bash\n#c\nbad\ndaemon:x:1:1::/usr/sbin:/usr/sbin/nologin\nx:x:y:1::/h:/s\n"))
	if len(accs) != 2 || accs[1].Name != "daemon" || accs[1].Home != "/usr/sbin" || accs[0].Shell != "/bin/bash" {
		t.Errorf("passwd %+v", accs)
	}
	ds := ParseDscache([]byte("name: _www\npassword: *\nuid: 70\ngid: 70\ndir: /Library/WebServer\nshell: /usr/bin/false\n\nname: alex\nuid: 501\ndir: /Volumes/Home/alex\nnoise\n\n\nname: half\n"))
	if len(ds) != 2 || ds[0].ID != "70" || ds[1].Home != "/Volumes/Home/alex" || ds[0].Shell != "/usr/bin/false" {
		t.Errorf("dscache %+v", ds)
	}
	g, err := ParseIDGroups([]byte("1001 27\n"), []byte("alex sudo\n"))
	if err != nil || len(g) != 2 || g[1] != (Group{Name: "sudo", ID: "27"}) {
		t.Errorf("groups %+v %v", g, err)
	}
	if _, err := ParseIDGroups([]byte("1 2"), []byte("a")); err == nil {
		t.Error("length mismatch accepted")
	}
}

func TestLoginDefs(t *testing.T) {
	min, max := ParseLoginDefs([]byte("# comment\nUID_MIN 2000\nUID_MAX\t3000\nGID_MIN 5\nUID_MAX x\n"))
	if min != 2000 || max != 3000 {
		t.Errorf("%d %d", min, max)
	}
	min, max = ParseLoginDefs(nil)
	if min != DefaultUIDMin || max != DefaultUIDMax {
		t.Errorf("defaults %d %d", min, max)
	}
	for _, tc := range []struct {
		goos, name string
		uid        int
		want       bool
	}{
		{"linux", "daemon", 1, false}, {"linux", "alex", 1000, true}, {"linux", "big", 70000, false}, {"linux", "nobody", 1500, false},
		{"darwin", "_www", 70, false}, {"darwin", "alex", 501, true}, {"darwin", "nobody", -2, false},
	} {
		if got := PersonUID(tc.goos, tc.name, tc.uid, DefaultUIDMin, DefaultUIDMax); got != tc.want {
			t.Errorf("%+v: %v", tc, got)
		}
	}
}
