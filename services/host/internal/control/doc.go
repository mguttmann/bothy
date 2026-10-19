// SPDX-License-Identifier: Apache-2.0

// Package control is the client of the control connection of the host service (M2-43,
// docs/spec/24-terminal.md section 3.5, asyncapi channel hostControl): one outgoing WebSocket over
// TLS to wss://<server>/api/v1/hosts/connect, never a listener. It logs in with an EdDSA proof
// over the challenge of hello made with the login key of the pairing, sends report after every
// ready before it accepts session.open, keeps the connection alive with heartbeats, keeps the
// JWKS of purpose terminal for M2-45 and reconnects after a close by the policy of 24 section 3.5.
// Client implements startup.Control, so "bothy-host run" logs in, reports and closes through it,
// also on a refused start (D-W-91). The frame types are hand-written (the host module has no
// generator); the recordings in testdata/control hold them equal to the contract.
//
// No log line of this package contains a proof, the challenge, a key or report content beyond
// codes.
//
// Example:
//
//	c, err := control.New(control.Config{ServerURL: p.ServerURL, HostID: p.HostID, AuthKey: keys.AuthPrivate,
//		HostVersion: buildinfo.Version, BackoffBase: cfg.ReconnectBackoffBase, BackoffCap: cfg.ReconnectBackoffCap},
//		control.RefuseSessions{})
//	ready, err := c.Login(ctx)
package control
