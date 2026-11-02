// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// listFlag collects a repeated flag.
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("bothy-host "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// installFlags are the flags of install.
type installFlags struct {
	users                       listFlag
	driveRoot, confirm, cfgPath string
	ownerAccount                bool
}

func parseInstall(args []string, stderr io.Writer) (installFlags, bool) {
	var f installFlags
	fl := newFlags("install", stderr)
	fl.Var(&f.users, "session-user", "a session user to create (repeatable)")
	fl.StringVar(&f.driveRoot, "drive-root", "", "only the drive root step for this drive (Windows)")
	fl.StringVar(&f.confirm, "drive-root-confirm", "", "answer the drive root prompt: yes or no")
	fl.StringVar(&f.cfgPath, "config", "", "path of the configuration file")
	fl.BoolVar(&f.ownerAccount, "owner-account", false, "install for the calling account itself, without administrator rights (ADR 0073)")
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 || (f.confirm != "" && f.confirm != "yes" && f.confirm != "no") {
		return f, false
	}
	return f, true
}

// cmdInstall is "install [--session-user <name>] [--drive-root <drive>] [--drive-root-confirm
// yes|no] [--config <path>]" or "install --owner-account" (ADR 0073).
func cmdInstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	f, ok := parseInstall(args, stderr)
	if !ok {
		return exitUsage
	}
	if f.ownerAccount {
		return installOwnerAccount(ctx, f, stdout, stderr)
	}
	if !sys.isAdmin() {
		return adminRequired(stderr, "install")
	}
	if f.cfgPath == "" {
		f.cfgPath = config.DefaultPath(sys.cfgPlatform())
	}
	c, exists, err := loadOrDefault(f.cfgPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
		return exitUsage
	}
	if f.driveRoot != "" {
		if sys.goos != hostos.Windows {
			_, _ = fmt.Fprintln(stderr, "bothy-host install: --drive-root exists only on Windows")
			return exitUsage
		}
		return applyOrFail(ctx, stderr, "install", install.DriveRoot(f.driveRoot, askDriveRoot(stdout, f.driveRoot, f.confirm)))
	}
	if code := installHost(ctx, c, exists, f.users, stderr); code != exitOK {
		return code
	}
	if sys.goos == hostos.Windows {
		for _, d := range driveRoots(c) {
			if code := applyOrFail(ctx, stderr, "install", install.DriveRoot(d, askDriveRoot(stdout, d, f.confirm))); code != exitOK {
				return code
			}
		}
	}
	_, _ = fmt.Fprintln(stdout, "install: done; run bothy-host doctor to check the host")
	return exitOK
}

// installHost applies the plan for the configured session users plus extra and adds extra to an
// existing configuration file.
func installHost(ctx context.Context, c *config.Config, exists bool, extra []string, stderr io.Writer) int {
	all, added, code := withNewSessionUsers(c, extra, stderr)
	if code != exitOK {
		return code
	}
	tg, err := sys.target(c, exists)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
		return exitFailed
	}
	steps, err := install.Install(tg, all)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
		return exitUsage
	}
	// The names go into an existing file before the plan restarts the service, which reads
	// host_service.session_users only at start.
	if exists {
		for _, u := range added {
			if err := config.AddSessionUser(c.Path, u); err != nil {
				_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
				return exitFailed
			}
		}
	}
	return applyOrFail(ctx, stderr, "install", steps)
}

// withNewSessionUsers returns the configured session users plus the new names of extra, and the
// new names alone; a new name passes the checks of session-user add (D-W-67, T12) or the command
// fails with the exit code of session-user add.
func withNewSessionUsers(c *config.Config, extra []string, stderr io.Writer) (all, added []string, code int) {
	bindings, err := state.Open(c.StateDir, sys.protector).ReadBindings()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host install: %v\n", err)
		return nil, nil, exitFailed
	}
	all = slices.Clone(c.SessionUsers)
	for _, u := range extra {
		if slices.Contains(all, u) {
			continue
		}
		if err := install.CheckNewSessionUser(u, all, bindings); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			var bound *install.BoundError
			if errors.As(err, &bound) {
				return nil, nil, exitFailed
			}
			return nil, nil, exitUsage
		}
		all = append(all, u)
		added = append(added, u)
	}
	return all, added, exitOK
}

// loadOrDefault loads the file at path, or the defaults when it does not exist yet.
func loadOrDefault(path string) (*config.Config, bool, error) {
	c, err := config.Load(path, sys.cfgPlatform())
	if errors.Is(err, fs.ErrNotExist) {
		d := config.Defaults(sys.cfgPlatform())
		d.Path = path
		return &d, false, nil
	}
	return c, err == nil, err
}

// driveRoots returns the system drive and the drive of every configured place (Windows).
func driveRoots(c *config.Config) []string {
	roots := []string{strings.ToUpper(sys.getenv("SystemDrive"))}
	places := []string{c.ClaudeBin}
	for _, l := range c.ClaudeWorkdirs {
		for _, w := range l.Dirs {
			places = append(places, w.Path)
		}
	}
	for _, p := range places {
		if v := strings.ToUpper(filepath.VolumeName(p)); len(v) == 2 && !slices.Contains(roots, v) {
			roots = append(roots, v)
		}
	}
	return slices.DeleteFunc(roots, func(s string) bool { return s == "" })
}

// askDriveRoot answers the drive root prompt: the flag, else the terminal, else no (D-M2-42-2).
func askDriveRoot(stdout io.Writer, drive, flagAnswer string) bool {
	switch flagAnswer {
	case "yes":
		return true
	case "no":
		return false
	}
	if !sys.interactive() {
		_, _ = fmt.Fprintf(stdout, "install: no terminal; the entry Authenticated Users:(AD) at the root of %s stays (--drive-root-confirm yes removes it)\n", drive)
		return false
	}
	_, _ = fmt.Fprintf(stdout, "Remove the entry Authenticated Users:(AD) from the root of %s, so no person can create folders there? [y/N] ", drive)
	line, _ := bufio.NewReader(sys.stdin).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(line), "y") || strings.EqualFold(strings.TrimSpace(line), "yes")
}

func applyOrFail(ctx context.Context, stderr io.Writer, cmd string, steps []install.Step) int {
	if err := sys.apply(ctx, steps); err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host %s: %v\n", cmd, err)
		return exitFailed
	}
	return exitOK
}
