// SPDX-License-Identifier: Apache-2.0

// Package confirm is the local confirmation of terminal sessions (docs/spec/24-terminal.md 6.4,
// Ruling T3, ADR 0070, brief M2-45 Design note 15). With host_service.require_local_confirm the
// host service asks the person at the machine before every PTY start and before every replay of
// an attach; the answer comes from a confirmation agent (bothy-host confirm-agent) that runs in
// the desktop session of that person, because the service cannot show anything in another
// user's desktop.
//
// Endpoint: the only endpoint this package opens is a Unix socket in a folder the service manager
// or install creates (mode 0666, so every local user can connect) or a named pipe that rejects
// remote clients; never a TCP or UDP listener (brief M2-45 requirement 29). Access is decided by
// the peer check of Rule, not by the mode: the user of the connected process must be neither the
// service user nor a configured or former session user, and must own the active console session,
// so a long-running process of a session user can never approve its own PTYs.
//
// Protocol: JSON lines, each bounded by maxLine. The service sends
// {"op":"confirm","id","kind","profile","session_user","timeout_ms"} and {"op":"cancel","id"};
// the agent answers {"op":"answer","id","allow"}. Both sides decode strictly: an unknown member,
// a JSON null, a missing member, an unknown op or trailing data make a line malformed. The
// service closes an agent that sends a malformed line and ignores an answer to an id it does not
// wait for (a late answer after a timeout); the agent ignores a malformed line of the service,
// which then gets no answer and counts it as a denial. A request goes to every accepted agent;
// the first answer wins and the other agents get cancel. Everything that is not an allow within
// host_service.local_confirm_timeout is a denial.
package confirm
