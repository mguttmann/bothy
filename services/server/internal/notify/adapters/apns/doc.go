// SPDX-License-Identifier: Apache-2.0

// Package apns is the APNs adapter of notify: it implements the outgoing port app.PushProvider
// (M3-19) for push registrations with provider = apns (docs/spec/13-push-benachrichtigungen.md
// 5.1, 6.1, 7.1; brief M3-20).
//
// Connection: HTTP/2 over TLS to https://api.push.apple.com for environment production and
// https://api.sandbox.push.apple.com for sandbox, request POST /3/device/<token>, only through
// the egress client of platform/egress with Options.HTTP2 and exactly these two hosts as the
// adapter's targets (19 SEC-NET-1, SEC-NET-2). One client serves both environments; net/http
// keeps one multiplexed connection per host open between sends (13 12 point 5).
//
// Token method: an ES256 JWT with the header alg and kid (push.apns.key_id) and the claims iss
// (push.apns.team_id) and iat, signed with the .p8 key that the secret provider returns for
// push.apns.private_key_ref. Each environment keeps its own JWT and creates at most one per
// push.apns.jwt_refresh_interval, reading the key again at each creation; the only exception is
// the one forced replacement after 403 ExpiredProviderToken. A configuration error pauses the
// environment for one interval, after which one send probes again.
//
// Payload: only the keys of 13 6.1 (IDs, numbers and fixed keys, never a name or a text), built
// from structs, checked against Apple's size limit before any request and never truncated. The
// badge sync of 13 8 goes as {"aps":{"badge":<n>}}. Device tokens, the JWT and the key never
// reach a log record, a span or an error text (13 7.3, SEC-LOG-4).
//
// internal/wire builds the adapter only when APNs is configured (Config.Enabled) and only in
// the roles that run notify.push_send (worker and all):
//
//	a, err := apns.New(ctx, cfg, apns.Deps{
//		Egress: egressCfg, EgressDeps: egressDeps, Secrets: provider, Clock: clock.System(), Logger: log,
//	})
//	// a joins the provider set of the push path under a.Name() == "apns".
package apns
