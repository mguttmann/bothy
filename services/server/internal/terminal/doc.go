// SPDX-License-Identifier: Apache-2.0

// Package terminal is the port of the terminal module: hosts and their pairing, the declaration of
// the LLM login (I6), the session users per person (ruling T12), terminal sessions, session
// tokens, the control connections of the host services and the relay of the streams. Other
// modules import only this package (docs/spec/02-architecture.md, "Layers within a module").
//
// The port Hosts holds the operations of the API group Hosts and the registration of
// host.updated (hosts.go), the port Sessions the terminal sessions (sessions.go); the
// implementation of authz.HostSource lives in app.
package terminal
