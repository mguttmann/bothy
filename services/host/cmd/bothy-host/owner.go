// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// The owner account mode (ADR 0073): the host service and its only session user are the existing
// account of the instance owner, installed and run without administrator rights.

// currentAccount returns the account of the calling process.
func currentAccount() (hostos.Account, error) {
	u, err := user.Current()
	if err != nil {
		return hostos.Account{}, err
	}
	return hostos.Account{Name: u.Username, ID: u.Uid, Home: u.HomeDir}, nil
}

// ownerTarget returns the install target of the caller's own account.
func (a *app) ownerTarget() (install.OwnerTarget, error) {
	if a.account == nil {
		return install.OwnerTarget{}, errors.New("the account of the caller is unknown")
	}
	acc, err := a.account()
	if err != nil {
		return install.OwnerTarget{}, err
	}
	exe, err := a.executable()
	if err != nil {
		return install.OwnerTarget{}, err
	}
	t := install.OwnerTarget{GOOS: a.goos, Account: acc.Name, UID: acc.ID, Home: acc.Home, Running: exe}
	if a.goos == hostos.Darwin {
		t.SignedWithRuntime = a.signedWithRuntime(exe)
	}
	return t, nil
}

// ownerConfigPath returns the configuration file of the owner account mode of the caller, "" when
// the account cannot be read.
func (a *app) ownerConfigPath() string {
	t, err := a.ownerTarget()
	if err != nil || (a.goos != hostos.Darwin && a.goos != hostos.Linux) {
		return ""
	}
	files, _ := install.OwnerPaths(t)
	return files.Config
}

// isOwnerCaller reports whether c is in the owner account mode and the caller is that account.
func (a *app) isOwnerCaller(c *config.Config) bool {
	if c == nil || c.OwnerAccount == "" || a.account == nil {
		return false
	}
	acc, err := a.account()
	return err == nil && acc.Name == c.OwnerAccount && acc.ID == a.callerID()
}

// installOwnerAccount is "install --owner-account": the per-user plan, as the account itself and
// never as root (D-OA-2, D-OA-3).
func installOwnerAccount(ctx context.Context, f installFlags, stdout, stderr io.Writer) int {
	switch {
	case sys.goos != hostos.Darwin && sys.goos != hostos.Linux:
		_, _ = fmt.Fprintln(stderr, "bothy-host install: --owner-account is not supported on this platform")
		return exitUsage
	case sys.isAdmin():
		_, _ = fmt.Fprintln(stderr, "bothy-host install: run --owner-account as the account itself, not as root")
		return exitUsage
	case len(f.users) > 0 || f.driveRoot != "":
		_, _ = fmt.Fprintln(stderr, "bothy-host install: --owner-account takes no --session-user and no --drive-root")
		return exitUsage
	}
	t, err := sys.ownerTarget()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
		return exitFailed
	}
	files, _ := install.OwnerPaths(t)
	if f.cfgPath != "" && f.cfgPath != files.Config {
		_, _ = fmt.Fprintln(stderr, "bothy-host install: --owner-account uses its own configuration file")
		return exitUsage
	}
	if c, err := config.Load(files.Config, sys.cfgPlatform()); err == nil {
		if c.OwnerAccount != t.Account {
			_, _ = fmt.Fprintln(stderr, "bothy-host install: the existing configuration names another host_service.owner_account")
			return exitUsage
		}
		t.ConfigExists = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
		return exitUsage
	}
	steps, err := install.OwnerAccount(t)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
		return exitUsage
	}
	if code := applyOrFail(ctx, stderr, "install", steps); code != exitOK {
		return code
	}
	_, _ = fmt.Fprintf(stdout, "install: done; next: %s pair --config %q --server <url> --code <code>, then %s doctor --config %q\n",
		files.Binary, files.Config, files.Binary, files.Config)
	return exitOK
}

// restartOwnerAccount restarts the per-user service of the owner account mode as the account.
func restartOwnerAccount(ctx context.Context, stdout, stderr io.Writer) int {
	t, err := sys.ownerTarget()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host restart: %v\n", err)
		return exitFailed
	}
	steps, err := install.OwnerRestart(t)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host restart: %v\n", err)
		return exitUsage
	}
	if code := applyOrFail(ctx, stderr, "restart", steps); code != 0 {
		return code
	}
	_, _ = fmt.Fprintln(stdout, "restart: done")
	return exitOK
}

// closeHelpers ends the child helpers of the owner account mode that env started (ADR 0073).
func closeHelpers(env doctor.Env) {
	if h, ok := env.Helpers.(startup.ChannelHelpers); ok {
		if k, ok := h.Services.(*startup.ChildHelpers); ok {
			k.Close()
		}
	}
}

// ownerHelperCommand returns the command of the child PTY helper of the owner account mode: this
// program with "pty-helper --owner-account", its environment and its standard error (D-OA-4).
func ownerHelperCommand(exe string) func(ctx context.Context, user string) *exec.Cmd {
	return func(ctx context.Context, name string) *exec.Cmd {
		c := exec.CommandContext(ctx, exe, "pty-helper", "--owner-account", "--session-user", name) //nolint:gosec // this program
		c.Env = os.Environ()
		c.Stderr = os.Stderr
		return c
	}
}
