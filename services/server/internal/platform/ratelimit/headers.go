// SPDX-License-Identifier: Apache-2.0

package ratelimit

import "strconv"

// Header names of draft-ietf-httpapi-ratelimit-headers-11 (06 "Rate-Limits" point 2, R-API-11).
const (
	HeaderPolicy = "RateLimit-Policy"
	HeaderState  = "RateLimit"
)

// windowSeconds is w: the policy describes the per-minute quota.
const windowSeconds = 60

// HeaderValues returns the values of RateLimit-Policy and RateLimit for class after decision d:
// `"<class>";q=<per_minute>;w=60` and `"<class>";r=<whole tokens>;t=<seconds until full>`.
//
// Example:
//
//	policy, state := ratelimit.HeaderValues("rl.write", d) // "rl.write";q=120;w=60 and "rl.write";r=29;t=1
func HeaderValues(class Class, d Decision) (policy, state string) {
	name := strconv.Quote(string(class))
	policy = name + ";q=" + strconv.Itoa(d.Limits.PerMinute) + ";w=" + strconv.Itoa(windowSeconds)
	state = name + ";r=" + strconv.Itoa(d.Remaining) + ";t=" + strconv.Itoa(d.ResetSeconds)
	return policy, state
}
