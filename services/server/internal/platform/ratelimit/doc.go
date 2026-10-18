// SPDX-License-Identifier: Apache-2.0

// Package ratelimit is the rate limiter of the server (docs/spec/05-aktionen.md "Rate-Limit-Klassen",
// docs/spec/06-api.md "Rate-Limits"): one token bucket per class and key, kept in the process
// (R-AR-12; with several api nodes each node limits on its own).
//
// A Limiter holds the buckets in a fixed number of shards with a hard cap on the number of tracked
// keys (ratelimit.max_tracked_keys), and a sweeper (Run) removes full buckets every
// ratelimit.sweep_interval: a full bucket decides exactly like a missing one, so memory stays
// bounded however many distinct keys arrive. The REST chain uses it through Install, which fills
// httpserver.StageRateLimit and the failed-credentials precheck of StageAuthenticate; the gateway
// calls Allow with KeyForConnection (rl.typing) and DeviceOrTokenKey (rl.gateway_connect).
//
// Example:
//
//	l, err := ratelimit.New(cfg, clock.System(), nil) // nil: the global meter provider
//	if err != nil {
//		return err // a class without limits, a value below 1
//	}
//	go l.Run(ctx) // sweeper; ends with ctx
//	ratelimit.Install(chain, l)
//	d, err := l.Allow("rl.typing", ratelimit.KeyForConnection(connID))
package ratelimit
