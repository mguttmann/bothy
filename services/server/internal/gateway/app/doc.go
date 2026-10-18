// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the WebSocket gateway (docs/spec/04-sync-protocol.md
// section 6, brief M1-64): hello, identify, resume, heartbeat and reauth, the op registry with the
// proof of the authorization check, the deadlines of a connection and the in-memory store of the
// gateway sessions with its sweeper. It reaches identity, authz, the rate limiter, sync and the
// delivery only through the ports of ports.go, delivery.go and the kernel; adapters/ws carries
// the frames.
package app
