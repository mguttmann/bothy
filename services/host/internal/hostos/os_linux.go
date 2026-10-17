// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"errors"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// LinuxUsers is the Users of Linux: /etc/passwd, /etc/login.defs and "id".
type LinuxUsers struct {
	Runner Runner
}

func (u LinuxUsers) accounts() ([]Account, error) {
	b, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defs, _ := os.ReadFile("/etc/login.defs") // missing: the documented defaults
	uidMin, uidMax := ParseLoginDefs(defs)
	accs := ParsePasswd(b)
	for i := range accs {
		uid, _ := strconv.Atoi(accs[i].ID)
		accs[i].Person = PersonUID("linux", accs[i].Name, uid, uidMin, uidMax)
	}
	return accs, nil
}

// Lookup implements Users.
func (u LinuxUsers) Lookup(name string) (Account, error) {
	accs, err := u.accounts()
	if err != nil {
		return Account{}, err
	}
	return lookupIn(accs, name)
}

// Groups implements Users.
func (u LinuxUsers) Groups(name string) ([]Group, error) { return unixGroups(u.Runner, name) }

// Accounts implements Users.
func (u LinuxUsers) Accounts() ([]Account, error) { return u.accounts() }

// LinuxSecurity is the Security of Linux: owner, group, mode and the POSIX ACL.
type LinuxSecurity struct{}

// Lstat implements Security.
func (LinuxSecurity) Lstat(path string) (FileSec, error) {
	f, err := unixSec(path)
	if err != nil || f.IsLink {
		return f, err
	}
	buf := make([]byte, 4096)
	n, err := unix.Getxattr(path, "system.posix_acl_access", buf)
	switch {
	case errors.Is(err, unix.ENODATA) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP):
		return f, nil
	case err != nil:
		f.ACL = []ACE{{Unmappable: true}}
		return f, nil
	}
	aces, perr := ParsePosixACL(buf[:n])
	if perr != nil {
		aces = []ACE{{Unmappable: true}}
	}
	f.ACL = aces
	return f, nil
}

// PtraceScope implements Probe: kernel.yama.ptrace_scope, 0 when Yama is absent.
func (p UnixProbe) PtraceScope() (int, error) {
	b, err := os.ReadFile("/proc/sys/kernel/yama/ptrace_scope")
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(trimNL(b))
}

// Platform returns the Users, Security and Probe of Linux.
func Platform(r Runner) (Users, Security, Probe) {
	return LinuxUsers{Runner: r}, LinuxSecurity{}, UnixProbe{Runner: r}
}
