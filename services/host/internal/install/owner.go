// SPDX-License-Identifier: Apache-2.0

package install

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// OwnerTarget describes the account of "bothy-host install --owner-account" (ADR 0073, D-OA-2):
// the existing account of the instance owner that runs the command itself, without
// administrator rights.
type OwnerTarget struct {
	GOOS string
	// Account, UID and Home are the name, uid and home folder of the account.
	Account, UID, Home string
	// Running is the path of the running binary that install copies.
	Running string
	// ConfigExists tells that the configuration file exists (install never overwrites it).
	ConfigExists bool
	// SignedWithRuntime tells that the binary already carries a signature with the runtime flag
	// (macOS).
	SignedWithRuntime bool
}

// OwnerFiles are the places of the owner account mode under the account's home.
type OwnerFiles struct {
	ProgramDir, Binary, ConfigDir, Config, StateDir, RunDir string
}

// Definition is a service definition file and its content.
type Definition struct {
	Path, Content string
}

// OwnerPaths returns the places of the owner account mode for t and its service definition:
// macOS "<home>/Library/Application Support/bothy-host" with a LaunchAgent, Linux the XDG folders
// with a systemd user unit.
func OwnerPaths(t OwnerTarget) (OwnerFiles, Definition) {
	var f OwnerFiles
	var def Definition
	switch t.GOOS {
	case hostos.Darwin:
		base := filepath.Join(t.Home, "Library", "Application Support", "bothy-host")
		f = OwnerFiles{ProgramDir: filepath.Join(base, "bin"), ConfigDir: base, StateDir: filepath.Join(base, "state")}
		def.Path = filepath.Join(t.Home, "Library", "LaunchAgents", hostLabel+".plist")
	default:
		f = OwnerFiles{ProgramDir: filepath.Join(t.Home, ".local", "lib", "bothy-host"), ConfigDir: filepath.Join(t.Home, ".config", "bothy-host"),
			StateDir: filepath.Join(t.Home, ".local", "state", "bothy-host")}
		def.Path = filepath.Join(t.Home, ".config", "systemd", "user", hostLabel+".service")
	}
	f.Binary = filepath.Join(f.ProgramDir, "bothy-host")
	f.Config = filepath.Join(f.ConfigDir, "config.yaml")
	f.RunDir = filepath.Join(f.StateDir, "run")
	data := map[string]string{keyLabel: hostLabel, keyBinary: f.Binary, keyConfig: f.Config}
	if t.GOOS == hostos.Darwin {
		def.Content = render("owner.plist.tmpl", data)
	} else {
		def.Content = render("owner.service.tmpl", data)
	}
	return f, def
}

// errOwnerRoot refuses root as the owner account: the mode is an existing person account.
var errOwnerRoot = errors.New("install: the owner account must not be root; run the command as the account itself")

// OwnerAccount returns the plan of "bothy-host install --owner-account": every step runs as the
// account, owns only its own files under its home and starts programs by absolute path from
// /usr/bin and /bin. Errors: ErrUnsupported on Windows and any other platform (D-OA-3), and root.
func OwnerAccount(t OwnerTarget) ([]Step, error) {
	if t.GOOS != hostos.Darwin && t.GOOS != hostos.Linux {
		return nil, ErrUnsupported
	}
	if t.UID == "0" || t.Account == "" || t.Account == ownerRoot {
		return nil, errOwnerRoot
	}
	f, def := OwnerPaths(t)
	o := t.Account
	steps := []Step{
		{Kind: KindMkdir, Path: f.ConfigDir, Mode: 0o700, Owner: o},
		{Kind: KindMkdir, Path: f.ProgramDir, Mode: 0o755, Owner: o},
		{Kind: KindCopy, Source: t.Running, Path: f.Binary, Mode: 0o755, Owner: o},
	}
	if t.GOOS == hostos.Darwin && !t.SignedWithRuntime {
		steps = append(steps, run("/usr/bin/codesign", "--force", "--sign", "-", "--options", "runtime", f.Binary))
	}
	steps = append(steps,
		Step{Kind: KindMkdir, Path: f.StateDir, Mode: 0o700, Owner: o},
		Step{Kind: KindMkdir, Path: f.RunDir, Mode: 0o700, Owner: o},
	)
	if !t.ConfigExists {
		var b strings.Builder
		b.WriteString("# Configuration of the host service in the owner account mode (docs/spec/24-terminal.md 2.4).\nhost_service:\n")
		b.WriteString("  owner_account: " + o + "\n  session_users: [" + o + "]\n  state_dir: \"" + f.StateDir + "\"\n")
		steps = append(steps, Step{Kind: KindWrite, Path: f.Config, Mode: 0o600, Owner: o, Content: b.String()})
	}
	steps = append(steps,
		Step{Kind: KindMkdir, Path: filepath.Dir(def.Path), Mode: 0o755, Owner: o},
		Step{Kind: KindWrite, Path: def.Path, Mode: 0o644, Owner: o, Content: def.Content},
	)
	if t.GOOS == hostos.Darwin {
		domain := "gui/" + t.UID
		return append(steps,
			Step{Kind: KindRunTolerant, Argv: []string{launchctl, "bootout", domain + "/" + hostLabel}},
			run(launchctl, "bootstrap", domain, def.Path),
			run(launchctl, "kickstart", "-k", domain+"/"+hostLabel)), nil
	}
	return append(steps,
		run("/usr/bin/systemctl", "--user", "daemon-reload"),
		run("/usr/bin/systemctl", "--user", "enable", hostLabel+".service"),
		run("/usr/bin/systemctl", "--user", "restart", hostLabel+".service")), nil
}

// OwnerRestart returns the plan of "bothy-host restart" in the owner account mode, run as the
// account (also after a refused start).
func OwnerRestart(t OwnerTarget) ([]Step, error) {
	switch t.GOOS {
	case hostos.Darwin:
		return []Step{run(launchctl, "kickstart", "-k", "gui/"+t.UID+"/"+hostLabel)}, nil
	case hostos.Linux:
		return []Step{run("/usr/bin/systemctl", "--user", "reset-failed", hostLabel+".service"),
			run("/usr/bin/systemctl", "--user", "restart", hostLabel+".service")}, nil
	}
	return nil, ErrUnsupported
}
