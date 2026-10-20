// SPDX-License-Identifier: Apache-2.0

// Package session is the session manager of the host service (docs/spec/24-terminal.md 5 and 6,
// brief M2-45): for every session.open of the server it opens one data connection, answers Noise
// message 1 as responder, checks the terminal session token in the fixed order of 24 5.3 (with the
// two steps of D-M2-45 for att and wdi), sends message 2, waits for the frame start, runs the gate
// (HD6, HD10, HD11) and the local confirmation, and only then has the PTY helper of the session
// user named by osu start the PTY. It relays the frames between the data connection and the helper,
// keeps the scrollback in memory, measures idle and detach, and reports session.started and
// session.ended over the control connection.
//
// It runs in the service user. It never sees terminal content in clear outside the memory of one
// session, and its logs, reports and errors carry IDs, names of session users, profile names and
// codes only (24 10.1).
package session
