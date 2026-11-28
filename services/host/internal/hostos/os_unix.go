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

// trueBins are the places of the program true, the first that exists is used.
var trueBins = []string{"/usr/bin/true", "/bin/true"}

// SudoWithoutPassword implements Probe: "sudo -n true" with sudo and true by absolute path and an
// empty environment. Without PATH sudo finds a bare "true" only through secure_path, which macOS
// does not set, so the bare name made every answer "no" there (docs/decisions.d/V1-13.md).
func (p UnixProbe) SudoWithoutPassword(ctx context.Context) (bool, error) {
	if _, err := os.Stat("/usr/bin/sudo"); err != nil {
		return false, nil
	}
	trueBin := trueBins[0]
	for _, b := range trueBins {
		if _, err := os.Stat(b); err == nil {
			trueBin = b
			break
		}
	}
	out, err := p.Runner.Run(ctx, Cmd{Path: "/usr/bin/sudo", Args: []string{"-n", "--", trueBin}, Timeout: probeTimeout})
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
