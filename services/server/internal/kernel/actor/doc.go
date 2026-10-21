// SPDX-License-Identifier: Apache-2.0

// Package actor describes the principal of a request (ID, kind, device, session, step-up time,
// scopes) and declares the Authenticator with Revalidate, which checks an authenticated principal
// again without a new proof. The identity module implements it; platform/httpserver and gateway
// use it.
//
// ActingFor is the acting-for binding of guard rule SR9 (07 section "SR9"): the result of the check
// of the request header Invocation-Token, which an agent sends on every request inside a run
// (Credentials.InvocationTokens). Its status is bound, run_inactive or invalid, and its zero value
// means no header (brief M2-14 design note 2); authorize intersects a bound agent's rights with the
// triggering person's current rights and denies an invalid header or an ended run.
package actor
