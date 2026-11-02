// SPDX-License-Identifier: Apache-2.0

package install

import (
	"errors"
	"strings"
	"testing"
)

func ownerTarget(goos string) OwnerTarget {
	home := "/home/alex"
	if goos == "darwin" {
		home = "/" + "Users/alex"
	}
	return OwnerTarget{GOOS: goos, Account: "alex", UID: "501", Home: home, Running: "/tmp/download/bothy-host"}
}

// TestInstallOwnerAccount: the plan of install --owner-account (ADR 0073, D-OA-2) needs no
// administrator rights: no new user or group, no system service, nothing owned by root, nothing
// outside the account's home, every program by absolute path.
func TestInstallOwnerAccount(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		tg := ownerTarget(goos)
		steps, err := OwnerAccount(tg)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "owner_install_"+goos, lines(steps))
		for _, s := range steps {
			text := s.String()
			for _, banned := range []string{"dscl", "useradd", "createhomedir", "/Library/LaunchDaemons", "/etc/systemd", "/opt/", "system/", "sudo"} {
				if strings.Contains(text, banned) {
					t.Errorf("%s: step needs administrator rights: %s", goos, text)
				}
			}
			switch s.Kind {
			case KindRun, KindRunTolerant:
				if !strings.HasPrefix(s.Argv[0], "/usr/bin/") && !strings.HasPrefix(s.Argv[0], "/bin/") {
					t.Errorf("%s: program outside the fixed list: %v", goos, s.Argv)
				}
			case KindMkdir, KindWrite, KindCopy:
				if s.Owner != tg.Account || !strings.HasPrefix(s.Path, tg.Home+"/") {
					t.Errorf("%s: %s is not the account's own", goos, text)
				}
			default:
				t.Errorf("%s: unexpected step %s", goos, text)
			}
		}
	}
	if _, def := OwnerPaths(ownerTarget("darwin")); !strings.HasSuffix(def.Path, "/Library/LaunchAgents/bothy-host.plist") {
		t.Errorf("darwin definition %s", def.Path)
	}
	if _, def := OwnerPaths(ownerTarget("linux")); def.Path != "/home/alex/.config/systemd/user/bothy-host.service" {
		t.Errorf("linux definition %s", def.Path)
	}
}

func TestInstallOwnerAccountDefinitions(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		_, def := OwnerPaths(ownerTarget(goos))
		golden(t, "owner_definition_"+goos, def.Content)
		if strings.Contains(def.Content, "UserName") || strings.Contains(def.Content, "User=") {
			t.Errorf("%s: a per-user definition names a user", goos)
		}
	}
}

func TestInstallOwnerAccountRefused(t *testing.T) {
	if _, err := OwnerAccount(ownerTarget("windows")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("windows: %v", err)
	}
	tg := ownerTarget("linux")
	tg.UID = "0"
	if _, err := OwnerAccount(tg); err == nil {
		t.Error("root as the owner account")
	}
}

func TestRestartOwnerAccount(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		steps, err := OwnerRestart(ownerTarget(goos))
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "owner_restart_"+goos, lines(steps))
	}
	if _, err := OwnerRestart(ownerTarget("windows")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("windows: %v", err)
	}
}

// TestInstallOwnerAccountKeepsConfig: an existing configuration file is never overwritten.
func TestInstallOwnerAccountKeepsConfig(t *testing.T) {
	tg := ownerTarget("linux")
	tg.ConfigExists = true
	steps, _ := OwnerAccount(tg)
	paths, _ := OwnerPaths(tg)
	for _, s := range steps {
		if s.Kind == KindWrite && s.Path == paths.Config {
			t.Fatal("the configuration file is overwritten")
		}
	}
}
