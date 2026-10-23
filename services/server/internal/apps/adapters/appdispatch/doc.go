// SPDX-License-Identifier: Apache-2.0

// Package appdispatch implements the source port runs.AppDispatcher (M2-25 design note 9;
// docs/spec/11-extensions.md section 9): it tells runs how an app is reached (Way) and sends a
// command, an interaction (Post) or an autocomplete request (Ask) to the interaction address of
// the app's manifest, delivery.interactions_url.
//
// A request is signed like an outgoing webhook of M2-26: the body is the envelope of the event
// with sync_epoch, the headers are webhook-id, webhook-timestamp and webhook-signature with v1a of
// every key of purpose webhook_out (no v1: HMAC belongs to a subscription). It goes out through
// one egress client of purpose app_interaction with the host policy of webhook.allowed_hosts, no
// redirects, the time limit app.interaction_timeout and the answer bound http.max_json_body_bytes.
// The address is checked against webhook.allowed_hosts before the egress client is called,
// because a matching egress.allowed_hosts entry would admit a host whatever the policy says
// (SEC-NET-2). A 200 to Post is written as a message of the app through messaging; a 200 to Ask
// gives the suggestions; a 202 defers; every other answer is a reason of runs.AppReasons.
//
// Each request writes one log line with the allowed attributes of
// docs/observability/signals.md 5.3 and server.address (the host); never the path of the
// address, a header value, a body, an option or form value, a reply text or a signature
// (SEC-LOG-2, SEC-LOG-4).
//
// Example:
//
//	d, err := appdispatch.New(cfg, egressCfg, appdispatch.Deps{
//		Installations: appdispatch.NewInstallations(appsRepo), Log: syncRepo, Keys: keys, State: state,
//		Messages: messagingService, Logger: log,
//	}, egress.Deps{Logger: log, Metrics: metrics})
//	out, err := d.Post(ctx, runs.AppPost{AppID: app, EventID: ev, ReplyThreadID: thread})
package appdispatch
