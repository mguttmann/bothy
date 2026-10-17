// SPDX-License-Identifier: Apache-2.0

// Package oidcflow holds the pure rules of the OIDC login of docs/spec/08-identity.md section 2.2
// (brief M1-41): the state machine of a login flow (started, code_issued, completed, failed), the
// app redirect URI and client_challenge rules, the PKCE helpers of both legs (S256, the
// constant-time verifier check and the server verifier derived from the flow id and the token
// pepper), the HMAC of state, nonce and login_code, the checks of an ID token over its already
// verified and parsed claims, the external identity stored in the flow (schema
// external_identity.v1) and the handle and display name of a provisioned person. Nothing here
// reads a clock, a store, the network or the configuration: times, limits, keys and random sources
// are parameters; identity/app composes the rules into the use cases.
//
// Example:
//
//	if err := oidcflow.CheckAppRedirectURI(in.AppRedirectURI, cfg.AppRedirectURIs); err != nil {
//		return err // 400 validation_failed at /app_redirect_uri
//	}
//	verifier := oidcflow.ServerVerifier(pepper, flowID)
//	challenge := oidcflow.S256(verifier) // code_challenge towards the provider
package oidcflow
