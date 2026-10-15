// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestCheckActive_ObservedCredentials holds check_active for a principal whose authenticator
// read session, device and device key for this request (M1-34): the observed facts decide
// although the cached snapshot of the same rights version still holds the earlier ones, so a
// revocation works at the next request without a new rights version (AZ-P9 for sessions).
func TestCheckActive_ObservedCredentials(t *testing.T) {
	w := newWorld(t)
	read := func() (authorize.Action, authorize.Resource) {
		return authorize.ActionMessageRead, w.msg(w.allgemein, w.alex)
	}
	live := func(mod func(a *actor.Actor)) actor.Actor {
		a := w.user(w.alex)
		a.SessionActive, a.DeviceState, a.DeviceKeyActive = true, actor.DeviceActive, true
		mod(&a)
		return a
	}
	act, res := read()
	w.allow(t, live(func(*actor.Actor) {}), act, res)
	for _, c := range []struct {
		name   string
		mod    func(a *actor.Actor)
		o      authorize.Outcome
		reason string
	}{
		{"session revoked", func(a *actor.Actor) { a.SessionActive = false }, authorize.OutcomeUnauthenticated, "session_revoked"},
		{"device revoked", func(a *actor.Actor) { a.DeviceState = actor.DeviceRevoked }, authorize.OutcomeUnauthenticated, "device_inactive"},
		{"device unknown", func(a *actor.Actor) { a.DeviceState = actor.DeviceUnknown }, authorize.OutcomeUnauthenticated, "device_inactive"},
		{"key revoked", func(a *actor.Actor) { a.DeviceKeyActive = false }, authorize.OutcomeUnauthenticated, "device_inactive"},
		{"device pending", func(a *actor.Actor) { a.DeviceState = actor.DevicePending }, authorize.OutcomeForbidden, "device_pending"},
	} {
		act, res := read()
		w.expect(t, live(c.mod), act, res, c.o, c.reason)
	}
	// A device the source still reports pending was approved: the observed state counts.
	w.creds.PutDevice(w.devices[w.alex], fakeDevice(w.alex, authz.DevicePending))
	w.versions.Bump()
	act, res = read()
	w.allow(t, live(func(*actor.Actor) {}), act, res)
	// Without observed facts (DeviceState empty) the credential source decides as before.
	act, res = read()
	w.expect(t, w.user(w.alex), act, res, authorize.OutcomeForbidden, "device_pending")
}
