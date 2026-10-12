// SPDX-License-Identifier: Apache-2.0

// Package gateway is the port of the gateway module: the WebSocket endpoint, gateway sessions, the
// hub, focus, typing, relaying ephemeral events and agent connections; it owns no tables. Other
// modules import only this package (docs/spec/02-architecture.md, "Layers within a module").
package gateway
