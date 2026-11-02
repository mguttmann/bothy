// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strconv"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

// way is how pair and doctor reach the identity of the service user (D-M2-42-1).
type way int

const (
	// wayNone: the caller is neither the owner of the state directory nor an administrator.
	wayNone way = iota
	// wayDirect: the caller owns the state directory (the service user itself, and the tests).
	wayDirect
	// wayService: an administrator, through the admin endpoint of the running service.
	wayService
	// wayChild: an administrator with the service stopped (Linux and macOS: a child process with
	// the credentials of the service user; Windows: doctor-once).
	wayChild
)

// serviceAccount returns the name of the service user of the platform.
func (a *app) serviceAccount() string {
	if a.goos == hostos.Windows {
		return install.HostAccountWindows
	}
	return serviceUser
}

// identity decides the way for the state directory of c; conn is the open admin connection of
// wayService.
func (a *app) identity(ctx context.Context, c *config.Config) (way, *localchan.Conn) {
	if a.isOwnerCaller(c) {
		// The owner account mode (ADR 0073): the running service is the caller's own, so its admin
		// endpoint admits the caller; without it the caller works directly.
		if conn, err := a.dialAdmin(ctx, c.StateDir, a.callerID()); err == nil {
			return wayService, conn
		}
		return wayDirect, nil
	}
	users, sec, _ := a.platform(a.runner)
	if f, err := sec.Lstat(c.StateDir); err == nil && f.OwnerID == a.callerID() {
		return wayDirect, nil
	}
	if !a.isAdmin() {
		return wayNone, nil
	}
	acc, err := users.Lookup(a.serviceAccount())
	if err == nil {
		if conn, err := a.dialAdmin(ctx, c.StateDir, acc.ID); err == nil {
			return wayService, conn
		}
	}
	return wayChild, nil
}

// childIDs returns uid and gid of the service user for a child process.
func (a *app) childIDs() (int, int, error) {
	users, _, _ := a.platform(a.runner)
	acc, err := users.Lookup(serviceUser)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.Atoi(acc.ID)
	if err != nil {
		return 0, 0, err
	}
	gid := uid
	if gs, err := users.Groups(serviceUser); err == nil && len(gs) > 0 {
		if g, err := strconv.Atoi(gs[0].ID); err == nil {
			gid = g
		}
	}
	return uid, gid, nil
}
