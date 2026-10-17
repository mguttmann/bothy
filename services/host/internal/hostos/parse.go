// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Tags of the POSIX ACL extended attribute system.posix_acl_access (Linux).
const (
	aclUserObj  = 0x01
	aclUser     = 0x02
	aclGroupObj = 0x04
	aclGroup    = 0x08
	aclMask     = 0x10
	aclOther    = 0x20
	aclVersion  = 2
)

// ErrACLFormat is returned for an ACL that cannot be parsed; callers treat it as unmappable.
var ErrACLFormat = errors.New("hostos: unreadable ACL")

// ParsePosixACL decodes the value of system.posix_acl_access: a little-endian version 2 header and
// entries of tag, permission and ID. Named users and groups become ACEs; the owner, owning group,
// mask and other entries are reflected by the mode and ignored. The mask is not applied, so a
// named entry counts with its full rights (fail closed).
func ParsePosixACL(b []byte) ([]ACE, error) {
	if len(b) < 4 || binary.LittleEndian.Uint32(b) != aclVersion || (len(b)-4)%8 != 0 {
		return nil, ErrACLFormat
	}
	var out []ACE
	for off := 4; off < len(b); off += 8 {
		tag := binary.LittleEndian.Uint16(b[off:])
		perm := binary.LittleEndian.Uint16(b[off+2:])
		id := binary.LittleEndian.Uint32(b[off+4:])
		switch tag {
		case aclUser, aclGroup:
			kind := PrincipalUser
			if tag == aclGroup {
				kind = PrincipalGroup
			}
			out = append(out, ACE{Allow: true, Kind: kind, ID: strconv.FormatUint(uint64(id), 10), Rights: modeRights(fsMode(perm))})
		case aclUserObj, aclGroupObj, aclMask, aclOther:
		default:
			out = append(out, ACE{Unmappable: true})
		}
	}
	return out, nil
}

// macACLLine matches one entry of "ls -le": " 0: user:alex allow add_file,delete_child".
var macACLLine = regexp.MustCompile(`^\s*\d+:\s+(?:(user|group):(\S+)|(everyone))\s+(allow|deny)\s+(\S+)\s*$`)

// ParseMacACL parses the entries of the output of "/bin/ls -led <path>" (macOS). userID and
// groupID map names to IDs; an entry whose name or rights cannot be mapped, or a line that does
// not parse, becomes an unmappable ACE.
func ParseMacACL(out []byte, userID, groupID func(string) (string, error)) []ACE {
	var aces []ACE
	sc := bufio.NewScanner(bytes.NewReader(out))
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			continue // the ls line of the object itself
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		aces = append(aces, macACE(line, userID, groupID))
	}
	return aces
}

// macACE parses one entry line of "ls -le"; anything it cannot map is unmappable.
func macACE(line string, userID, groupID func(string) (string, error)) ACE {
	m := macACLLine.FindStringSubmatch(line)
	if m == nil {
		return ACE{Unmappable: true}
	}
	names := strings.Split(m[5], ",")
	rights, ok := MacRights(names)
	e := ACE{Allow: m[4] == "allow", Rights: rights, InheritOnly: slices.Contains(names, "only_inherit")}
	var err error
	switch {
	case m[3] != "":
		e.Kind = PrincipalEveryone
	case m[1] == "user":
		e.Kind = PrincipalUser
		e.ID, err = userID(m[2])
	default:
		e.Kind = PrincipalGroup
		e.ID, err = groupID(m[2])
	}
	if !ok || err != nil {
		return ACE{Unmappable: true}
	}
	return e
}

// ParsePasswd parses /etc/passwd lines into accounts (name, uid, home); malformed lines are
// skipped.
func ParsePasswd(b []byte) []Account {
	var out []Account
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Split(sc.Text(), ":")
		if len(f) < 7 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if _, err := strconv.Atoi(f[2]); err != nil {
			continue
		}
		out = append(out, Account{Name: f[0], ID: f[2], Home: f[5], Shell: f[6]})
	}
	return out
}

// ParseDscache parses the output of "dscacheutil -q user" (macOS): blocks of "name:", "uid:" and
// "dir:" lines separated by blank lines.
func ParseDscache(b []byte) []Account {
	var out []Account
	var cur Account
	flush := func() {
		if cur.Name != "" && cur.ID != "" {
			out = append(out, cur)
		}
		cur = Account{}
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "name":
			cur.Name = v
		case "uid":
			cur.ID = v
		case "dir":
			cur.Home = v
		case "shell":
			cur.Shell = v
		}
	}
	flush()
	return out
}

// ParseIDGroups zips the outputs of "id -G <name>" and "id -Gn <name>" into groups. Lists of
// different lengths are an error.
func ParseIDGroups(ids, names []byte) ([]Group, error) {
	i, n := strings.Fields(string(ids)), strings.Fields(string(names))
	if len(i) != len(n) {
		return nil, errors.New("hostos: id output differs in length")
	}
	out := make([]Group, len(i))
	for k := range i {
		out[k] = Group{ID: i[k], Name: n[k]}
	}
	return out, nil
}
