// SPDX-License-Identifier: Apache-2.0

// Package fcm is the FCM adapter of notify: it implements the outgoing port app.PushProvider
// (M3-19) for push registrations with provider = fcm (docs/spec/13-push-notifications.md
// 5.2, 6.2, 7.2; brief M3-21).
//
// Endpoints: one request POST https://fcm.googleapis.com/v1/projects/{push.fcm.project_id}/messages:send
// per hint, and the OAuth token endpoint https://oauth2.googleapis.com/token. Both go only
// through the egress client of platform/egress over HTTP/1.1 with keep-alive, with exactly these
// two hosts as the adapter's targets (19 SEC-NET-1, SEC-NET-2); no redirect is followed and TLS
// is always verified. The token endpoint is fixed: a service account whose token_uri names
// another URL is refused.
//
// OAuth: the service account that the secret provider returns for push.fcm.service_account_ref
// signs an RS256 assertion (header alg, typ JWT and kid; claims iss, scope, aud, iat and exp one
// hour later), which the JWT bearer grant of RFC 7523 exchanges for an access token. The token
// is reused for half its lifetime, with at most one token request in flight; each token request
// reads the service account again, so a rotated key is used without a restart. A 401 renews the
// token once; a credential error pauses the adapter for push.retry_cap, after which one send
// probes again.
//
// Payload: a data message only, without a notification block and without collapse_key
// (R-PN-08), whose data holds only the keys of 13 6.2 (IDs, numbers and fixed keys, never a name
// or a text), built from structs and checked against the FCM data limit before any request. The
// badge sync of 13 8 goes as k = sync with priority normal. Device tokens, the assertion, the
// access token and the key never reach a log record, a span or an error text (13 7.3,
// SEC-LOG-4).
//
// internal/wire builds the adapter only when FCM is configured (Config.Enabled) and only in the
// roles that run notify.push_send (worker and all):
//
//	a, err := fcm.New(ctx, cfg, fcm.Deps{
//		Egress: egressCfg, EgressDeps: egressDeps, Secrets: provider, Clock: clock.System(), Logger: log,
//	})
//	// a joins the provider set of the push path under a.Name() == "fcm".
package fcm
