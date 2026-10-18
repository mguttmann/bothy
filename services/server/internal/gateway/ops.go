// SPDX-License-Identifier: Apache-2.0

package gateway

import "github.com/OWNER/bothy/services/server/internal/kernel/authorize"

// Register declares the client ops the gateway accepts (M1-25, AZ-P2; brief M1-64): identify,
// resume, heartbeat and reauth, the section gateway_ops.enabled of api/operations.yaml. focus and
// typing follow with M1-66. internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	for _, op := range []authorize.Op{authorize.OpIdentify, authorize.OpResume, authorize.OpHeartbeat, authorize.OpReauth} {
		r.Op("gateway", op)
	}
}
