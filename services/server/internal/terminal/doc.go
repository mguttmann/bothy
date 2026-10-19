// SPDX-License-Identifier: Apache-2.0

// Package terminal is the port of the terminal module: hosts and their pairing, the declaration of
// the LLM login (I6), the session users per person (ruling T12), terminal sessions, session
// tokens, the control connections of the host services and the relay of the streams. Other
// modules import only this package (docs/spec/02-architecture.md, "Layers within a module").
//
// M2-39 builds the host records: the port Hosts with the five operations of the API group Hosts
// and the registration of host.updated; the implementation of authz.HostSource lives in app.
// Sessions and Relay come with M2-44.
package terminal
