// SPDX-License-Identifier: Apache-2.0

// Package computer is the session kind computer of the host service (docs/spec/27-computer-use.md
// sections 4 to 9, building block C3): the enablement mirror of the helper, the controller lock
// (one live computer session per host), the checks of message 1 (5.3), the state with the pause
// while the screen is locked (8.2), the coordinate mapping with scale IDs (6.5), image chunking at
// the Noise limit (6.3), the ends of 8.3 with their reports, and the local channel to the computer
// helper with the peer rule of 9.4.
//
// The package keeps images only in memory and never writes one to disk; it logs codes, IDs and
// action kinds, never coordinates, text, key names, app names or image bytes (13.1).
//
// Dependencies: internal/frame (the computer frames), internal/noise (the handshake with the
// prologue bothy-computer/1), internal/session (the data connection and the JWS profile shared
// with terminal sessions), internal/control (the host control ops) and internal/confirm (the
// console user rule).
package computer
