// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"
	"strconv"
)

// DarwinUsers is the Users of macOS: dscacheutil and "id".
type DarwinUsers struct {
	Runner Runner
}

func (u DarwinUsers) accounts() ([]Account, error) {
	out, err := u.Runner.Run(context.Background(), Cmd{Path: "/usr/bin/dscacheutil", Args: []string{"-q", "user"}, Timeout: probeTimeout})
	if err != nil {
		return nil, err
	}
	accs := ParseDscache(out.Stdout)
	for i := range accs {
		uid, _ := strconv.Atoi(accs[i].ID)
		accs[i].Person = PersonUID("darwin", accs[i].Name, uid, 0, 0)
	}
	return accs, nil
}

// Lookup implements Users.
func (u DarwinUsers) Lookup(name string) (Account, error) {
	accs, err := u.accounts()
	if err != nil {
		return Account{}, err
	}
	return lookupIn(accs, name)
}

// Groups implements Users.
func (u DarwinUsers) Groups(name string) ([]Group, error) { return unixGroups(u.Runner, name) }

// Accounts implements Users.
func (u DarwinUsers) Accounts() ([]Account, error) { return u.accounts() }

// DarwinSecurity is the Security of macOS: owner, group, mode and the extended ACL parsed from
// "/bin/ls -led" (golang.org/x/sys/unix offers no ATTR_CMN_EXTENDED_SECURITY decoder).
type DarwinSecurity struct {
	Runner Runner
	Users  Users
}

// Lstat implements Security.
func (s DarwinSecurity) Lstat(path string) (FileSec, error) {
	f, err := unixSec(path)
	if err != nil || f.IsLink {
		return f, err
	}
	out, err := s.Runner.Run(context.Background(), Cmd{Path: "/bin/ls", Args: []string{"-led", path}, Timeout: probeTimeout})
	if err != nil || out.ExitCode != 0 {
		f.ACL = []ACE{{Unmappable: true}}
		return f, nil
	}
	f.ACL = ParseMacACL(out.Stdout, func(n string) (string, error) {
		a, err := s.Users.Lookup(n)
		return a.ID, err
	}, s.groupID)
	return f, nil
}

func (s DarwinSecurity) groupID(name string) (string, error) {
	out, err := s.Runner.Run(context.Background(), Cmd{Path: "/usr/bin/dscl", Args: []string{".", "-read", "/Groups/" + name, "PrimaryGroupID"}, Timeout: probeTimeout})
	if err != nil || out.ExitCode != 0 {
		return "", ErrUnknownUser
	}
	_, v, ok := cutField(trimNL(out.Stdout))
	if !ok {
		return "", ErrUnknownUser
	}
	return v, nil
}

func cutField(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			v := s[i+1:]
			for len(v) > 0 && v[0] == ' ' {
				v = v[1:]
			}
			return s[:i], v, v != ""
		}
	}
	return "", "", false
}

// PtraceScope implements Probe; macOS has no Yama.
func (p UnixProbe) PtraceScope() (int, error) { return -1, nil }

// Platform returns the Users, Security and Probe of macOS.
func Platform(r Runner) (Users, Security, Probe) {
	u := DarwinUsers{Runner: r}
	return u, DarwinSecurity{Runner: r, Users: u}, UnixProbe{Runner: r}
}
