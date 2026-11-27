// SPDX-License-Identifier: Apache-2.0

// Package computermcp is the stdio MCP server bothy-computer-mcp of docs/spec/27-computer-use.md
// (building block C1): it offers the tools of section 7 under the server name bothy_computer,
// asks the runner through the request folder of section 10.2 to list desktops and open a
// session, pins the host's Noise key per runner (10.4), attaches with the single-use token,
// runs the Noise initiator with the prologue bothy-computer/1 (5.1 steps 8 and 9) and sends each
// action as one cu_action frame (6.3).
//
// The server writes nothing to disk except the pin file and its request files, keeps images only
// in memory, and logs only action kinds, codes and durations (27 10.5 and 13.2). Rulings of this
// package: docs/decisions.d/CU-05.md.
//
// Parts: Server (JSON-RPC over stdio, both protocol eras), the tools (tools.go), the request
// client of the runner folder (runner.go), the pin store (pins.go), the attach client with the
// session state (attach.go) and the splitting of long type texts (split.go).
package computermcp
