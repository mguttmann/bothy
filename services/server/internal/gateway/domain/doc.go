// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the gateway module: the close codes of the WebSocket
// gateway with their contract names and outcomes, the states of a gateway session and the
// deadlines of heartbeats and token renewal (docs/spec/04-sync-protocol.md section 6), and the
// delivery rules of M1-65: the bounded outbound queue with its purge by removed targets, the
// delivery phases and the cursor frames of a catch-up page (section 7). It imports only the
// standard library and kernel/ids.
package domain
