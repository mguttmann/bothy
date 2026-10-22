// SPDX-License-Identifier: Apache-2.0

// Package profiles builds the starts of the terminal profiles shell, claude and command of the
// host service (docs/spec/24-terminal.md 11.1 and 7.4): which profiles the host offers, and for
// each start the program, argv[0], the arguments, the working directory and an environment that
// is built from scratch from an allow list plus TERM, never by removing from an inherited one.
//
// The PTY helper of a session user calls it as that session user. It never resolves
// host_service.claude_bin, the search list or the working directories again: those come from the
// result of doctor (HD6, HD7, HD10), and the paths of the session user are resolved only with the
// hostos.Resolver of the helper. A refused start is a *StartError whose Detail is a close_detail
// of 24 10.3.
package profiles
