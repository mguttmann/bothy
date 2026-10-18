// SPDX-License-Identifier: Apache-2.0

package instance

import "github.com/OWNER/bothy/services/server/internal/kernel/authorize"

// Register declares what the instance module emits (M1-25, docs/spec/05-aktionen.md section 6):
// the durable event instance.updated of the settings, the branding and the feature flags.
// internal/wire lists it in moduleRegistrations, so the register check of gate G9 (M1-67) and the
// catalogue check at start see it.
func Register(r *authorize.Registry) {
	r.Event("instance", authorize.EventInstanceUpdated)
}
