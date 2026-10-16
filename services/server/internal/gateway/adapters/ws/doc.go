// SPDX-License-Identifier: Apache-2.0

// Package ws is the WebSocket adapter of the gateway (brief M1-64): the HTTP handler of GET
// /api/v1/gateway over github.com/coder/websocket, the reader, writer and deadline goroutines of a
// connection, the frame codec on the generated Gateway* types of api/rest and the span
// gateway.connect. It is the only package of the server that imports github.com/coder/websocket
// besides the test client wstest.
package ws
