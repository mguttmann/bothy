// SPDX-License-Identifier: Apache-2.0

//go:build platform && windows

package platform

import (
	"slices"
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
