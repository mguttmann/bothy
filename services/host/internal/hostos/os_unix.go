// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package hostos

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// probeTimeout bounds one probe command; it is a protocol constant of the probes, not a window
// of the product (sudo -n answers at once or fails).
const probeTimeout = probeTimeoutAll

func identity(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", ErrUnresolvable
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ErrUnresolvable
	}
	return fmt.Sprintf("%d:%d", st.Dev, st.Ino), nil
}

// unixSec reads owner, group, mode and the type of path.
func unixSec(path string) (FileSec, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return FileSec{}, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return FileSec{}, errors.New("hostos: no stat data")
	}
	m := fi.Mode()
	return FileSec{
		IsDir: m.IsDir(), IsLink: m&fs.ModeSymlink != 0,
		OwnerID: strconv.FormatUint(uint64(st.Uid), 10), GroupID: strconv.FormatUint(uint64(st.Gid), 10),
		Mode: m.Perm() | m&(fs.ModeSetuid|fs.ModeSetgid),
	}, nil
}

// UnixProbe is the Probe of Linux and macOS.
type UnixProbe struct {
	Runner Runner
}

// trueCandidates are the places of the program true; the first that exists is the command of the
// sudo probe.
var trueCandidates = []string{"/usr/bin/true", "/bin/true"}

// SudoWithoutPassword implements Probe: "sudo -n /usr/bin/true" by absolute paths with an empty
// environment. The command is absolute as well: without PATH sudo finds a bare "true" only
// through a secure_path of the sudoers file, which macOS does not set, so "sudo -n true" failed
// with "command not found" there and HD3 never saw a passwordless sudo.
func (p UnixProbe) SudoWithoutPassword(ctx context.Context) (bool, error) {
	if _, err := os.Stat("/usr/bin/sudo"); err != nil {
		return false, nil
	}
	cmd := trueCandidates[0]
	for _, c := range trueCandidates {
		if _, err := os.Stat(c); err == nil {
			cmd = c
			break
		}
	}
	out, err := p.Runner.Run(ctx, Cmd{Path: "/usr/bin/sudo", Args: []string{"-n", cmd}, Timeout: probeTimeout})
	if err != nil {
		return false, err
	}
	return out.ExitCode == 0 && !out.TimedOut, nil
}

// unixGroups runs "id -G" and "id -Gn" for name.
func unixGroups(r Runner, name string) ([]Group, error) {
	ctx := context.Background()
	ids, err := r.Run(ctx, Cmd{Path: "/usr/bin/id", Args: []string{"-G", name}, Timeout: probeTimeout})
	if err != nil || ids.ExitCode != 0 {
		return nil, fmt.Errorf("%w: groups", ErrUnknownUser)
	}
	names, err := r.Run(ctx, Cmd{Path: "/usr/bin/id", Args: []string{"-Gn", name}, Timeout: probeTimeout})
	if err != nil || names.ExitCode != 0 {
		return nil, fmt.Errorf("%w: groups", ErrUnknownUser)
	}
	return ParseIDGroups(ids.Stdout, names.Stdout)
}

func trimNL(b []byte) string { return strings.TrimSpace(string(b)) }
