// SPDX-License-Identifier: Apache-2.0

//go:build platform && windows

package platform

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
)

// TestPlatform_ServiceAccountGroups (HD2 on Windows): the groups of the virtual account of the host
// service can be read, and a membership in Administrators shows. NetUserGetLocalGroups does not
// know a virtual account, so HD2 read no groups for it and refused every start with
// forbidden_group (run 45770587799).
func TestPlatform_ServiceAccountGroups(t *testing.T) {
	track(t)
	installed(t, "", userA)
	admin := func() bool {
		groups, err := hostos.WindowsUsers{}.Groups(install.HostAccountWindows)
		if err != nil {
			t.Fatalf("groups of the service account: %v", err)
		}
		return slices.ContainsFunc(groups, func(g hostos.Group) bool { return g.ID == hostos.SIDAdministrators })
	}
	if admin() {
		t.Fatal("the service account is an administrator after install")
	}
	addToGroup(t, install.HostAccountWindows, "Administrators")
	if !admin() {
		t.Error("the membership of the service account in Administrators does not show")
	}
	refusedWith(t, "forbidden_group")
}

// TestPlatform_SquattedStateDir: a person who is no administrator creates the state directory
// before install (C:\ProgramData lets every user create folders). install refuses it instead of
// making it the state directory of the service with the owner Administrators, and the folder
// keeps its owner.
func TestPlatform_SquattedStateDir(t *testing.T) {
	track(t)
	createPerson(t, personUser)
	t.Cleanup(func() { _ = os.RemoveAll(stateDir()) })
	if out, c := asUser(personUser, "cmd", "/c", "mkdir", stateDir()); c != 0 {
		t.Fatalf("mkdir as %s: %d %s", personUser, c, out)
	}
	writeConfig(t, "  session_users: ["+userA+"]\n")
	t.Cleanup(func() { uninstall(t, []string{userA}) })
	if out, c := host("install", "--drive-root-confirm", "yes"); c == 0 || !strings.Contains(out, "untrusted owner") {
		t.Errorf("install over a squatted state directory: %d\n%s", c, out)
	}
	if out := sh(t, "powershell", "-NoProfile", "-Command", "(Get-Acl -LiteralPath '"+stateDir()+"').Owner"); !strings.Contains(strings.ToLower(out), personUser) {
		t.Errorf("the squatted folder changed its owner: %s", out)
	}
}
