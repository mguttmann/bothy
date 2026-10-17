// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// OSOps are the Ops of Linux and macOS.
type OSOps struct {
	Runner hostos.Runner
	Users  hostos.Users
}

// ErrNoWindows is returned for a Windows step on another platform.
var ErrNoWindows = errors.New("install: Windows step on another platform")

// Run implements Ops.
func (o OSOps) Run(ctx context.Context, argv []string) error {
	out, err := o.Runner.Run(ctx, hostos.Cmd{Path: argv[0], Args: argv[1:]})
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return fmt.Errorf("exit code %d", out.ExitCode)
	}
	return nil
}

func (o OSOps) ids(owner string) (int, int, error) {
	if owner == "" || owner == ownerRoot {
		return 0, 0, nil
	}
	a, err := o.Users.Lookup(owner)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.Atoi(a.ID)
	if err != nil {
		return 0, 0, err
	}
	gid := uid
	if gs, err := o.Users.Groups(owner); err == nil && len(gs) > 0 {
		if g, err := strconv.Atoi(gs[0].ID); err == nil {
			gid = g
		}
	}
	return uid, gid, nil
}

func (o OSOps) own(path string, mode fs.FileMode, owner string) error {
	uid, gid, err := o.ids(owner)
	if err != nil {
		return err
	}
	if err := os.Lchown(path, uid, gid); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

// Mkdir implements Ops.
func (o OSOps) Mkdir(path string, mode fs.FileMode, owner string) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	return o.own(path, mode, owner)
}

// Write implements Ops.
func (o OSOps) Write(path string, content []byte, mode fs.FileMode, owner string) error {
	if old, err := os.ReadFile(path); err != nil || !bytes.Equal(old, content) { //nolint:gosec // a path of the plan
		if err := os.WriteFile(path, content, mode); err != nil {
			return err
		}
	}
	return o.own(path, mode, owner)
}

// Copy implements Ops: the binary is written to a temporary file next to dst and renamed, so a
// running program is replaced, not changed.
func (o OSOps) Copy(src, dst string, mode fs.FileMode, owner string) error {
	in, err := os.Open(src) //nolint:gosec // the running binary
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode) //nolint:gosec // a path of the plan
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := o.own(tmp, mode, owner); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Chmod implements Ops.
func (o OSOps) Chmod(path string, mode fs.FileMode, owner string) error {
	if owner == "" {
		return os.Chmod(path, mode)
	}
	return o.own(path, mode, owner)
}

// Remove implements Ops.
func (o OSOps) Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Home implements Ops.
func (o OSOps) Home(user string) (string, error) {
	a, err := o.Users.Lookup(user)
	if err != nil {
		return "", err
	}
	return a.Home, nil
}

// Windows implements Ops.
func (o OSOps) Windows(context.Context, Step) error { return ErrNoWindows }
