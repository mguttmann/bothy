// SPDX-License-Identifier: Apache-2.0

package gateway

import "github.com/OWNER/bothy/services/server/internal/kernel/authorize"

// Register declares the client ops the gateway accepts (M1-25, AZ-P2; briefs M1-64 and M1-66):
// identify, resume, heartbeat, reauth, focus, typing.start and typing.stop, the section
// gateway_ops.enabled of api/operations.yaml, and the ephemeral type it emits itself,
// typing.started. internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	for _, op := range []authorize.Op{
		authorize.OpIdentify, authorize.OpResume, authorize.OpHeartbeat, authorize.OpReauth,
		authorize.OpFocus, authorize.OpTypingStart, authorize.OpTypingStop,
	} {
		r.Op("gateway", op)
	}
	r.Event("gateway", authorize.EventTypingStarted)
}
