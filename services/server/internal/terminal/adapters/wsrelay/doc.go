// SPDX-License-Identifier: Apache-2.0

// Package wsrelay is the WebSocket adapter of the terminal module (02 section 4.3: the port Relay
// lives only here). M2-43 builds the control connection of the host services at
// GET /api/v1/hosts/connect (asyncapi channel hostControl, docs/spec/24-terminal.md section 3.5):
// Handler upgrades and runs one connection per host service with a reader, a bounded writer and
// the deadlines of the login and the heartbeat; the codec decodes the host frames strictly into
// the generated Host* types of api/rest; Links is the per-process registry of the connections
// behind app.ControlLinks and publishes host.presence_changed. M2-44 adds the data connection and
// the device stream.
//
// Example:
//
//	links := wsrelay.NewLinks(publisher, log)
//	h, err := wsrelay.NewHandler(cfg, wsrelay.Deps{UseCases: svc, Links: links, Schemas: schemas})
//	mux.Handle("GET /api/v1/hosts/connect", h)
package wsrelay
