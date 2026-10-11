// SPDX-License-Identifier: Apache-2.0

// Package actor describes the principal of a request (ID, kind, device, session, step-up time,
// scopes) and declares the Authenticator with Revalidate, which checks an authenticated principal
// again without a new proof. The identity module implements it; platform/httpserver and gateway
// use it.
package actor
