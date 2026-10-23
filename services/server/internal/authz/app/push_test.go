// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ownDevice is the resource of push.register: the device of p, owned by p (brief M3-22 design
// note 5 a).
func ownDevice(p actor.Actor) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceDevice, ID: p.DeviceID, OwnerID: p.ID}
}

// TestPushRegisterRequirement holds push.register (05 4.9 "self", brief M3-22 design note 6): the
// own device in a session is allowed; another person's device is not_found (not_self); a principal
// without a session (personal token, agent) is forbidden with step_up_not_possible after the
// visibility check; a pending device gets device_pending.
func TestPushRegisterRequirement(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionPushRegister, ownDevice(w.user(w.alex)))
	w.expect(t, w.user(w.alex), authorize.ActionPushRegister, ownDevice(w.user(w.sam)), authorize.OutcomeNotFound, "not_self")
	w.addToken(w.alex)
	token := actor.Actor{ID: w.alex, Kind: actor.KindUser, TokenID: w.tokens[w.alex]}
	w.expect(t, token, authorize.ActionPushRegister, ownDevice(token), authorize.OutcomeForbidden, "step_up_not_possible")
	agent := w.agent(w.lotse)
	w.expect(t, agent, authorize.ActionPushRegister, ownDevice(agent), authorize.OutcomeForbidden, "step_up_not_possible")
	w.creds.PutDevice(w.devices[w.robin], fake.Device{PrincipalID: w.robin, State: authz.DevicePending})
	w.versions.Bump()
	w.expect(t, w.user(w.robin), authorize.ActionPushRegister, ownDevice(w.user(w.robin)), authorize.OutcomeForbidden, "device_pending")
}

// TestPushUnregisterVisibility holds push.unregister on the kind push_registration (owner =
// user_id, brief M3-22 design note 6): its person is allowed, anybody else gets not_found.
func TestPushUnregisterVisibility(t *testing.T) {
	w := newWorld(t)
	reg := authorize.Resource{Kind: authorize.ResourcePushRegistration, ID: ids.New(), OwnerID: w.alex}
	w.allow(t, w.user(w.alex), authorize.ActionPushUnregister, reg)
	w.expect(t, w.user(w.sam), authorize.ActionPushUnregister, reg, authorize.OutcomeNotFound, "not_self")
	w.expect(t, w.user(w.owner), authorize.ActionPushUnregister, reg, authorize.OutcomeNotFound, "not_self")
}

// TestMessageReadAndPushDeliverForRequest holds the two checks of the preview fetch for a person in
// a request (brief M3-22 design note 8 g): a reader of the channel is allowed message.read on the
// message and push.deliver on its thread; Robin without read gets not_found for both.
func TestMessageReadAndPushDeliverForRequest(t *testing.T) {
	w := newWorld(t)
	msg := w.msg(w.allgemein, w.alex)
	w.allow(t, w.user(w.sam), authorize.ActionMessageRead, msg)
	w.allow(t, w.user(w.sam), authorize.ActionPushDeliver, w.thread(w.allgemein))
	w.expect(t, w.user(w.robin), authorize.ActionMessageRead, msg, authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.user(w.robin), authorize.ActionPushDeliver, w.thread(w.allgemein), authorize.OutcomeNotFound, "no_grant")
}
