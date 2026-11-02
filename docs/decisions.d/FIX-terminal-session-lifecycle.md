# FIX-terminal-session-lifecycle Detach, reattach and close in the owner account mode (2026-11-02)

Owner report of 2026-11-02 (desktop app on macOS, terminal to his own server host in the owner
account mode of [ADR 0073](../adr/0073-terminal-in-the-owners-own-account.md)): he pressed
"Detach" on purpose, left, and expected to pick the same shell up later from any of his devices, the way `tmux` or `screen` work. Instead the tab said "The terminal session ended: it
was detached for too long", offered no way back, and "End" did nothing, so he had to open a second
session.

## What the live system showed

Read-only evidence from the owner's instance (the API log, the rows of `terminal_sessions`, the
audit and the host service process), no change and no terminal session opened:

- The session was opened after a step-up with the device key (`terminal.open` allowed); about ten
  minutes later the stream of the device and the data connection of the host closed with `1000`
  (the explicit "Detach"), and the session was closed with `detach_expired` exactly
  `terminal.detach_grace` (15 minutes) after that. The audit entry `terminal.terminate` names
  `detach_expired`.
- The host service process ran without a restart from before the opening until after the end, so
  no `host_restarted`; no other error was logged. The second session about an hour later is the
  one the owner opened instead.
- Cause: the windows of ADR 0068 (15 minutes detach grace, 30 minutes idle counted across a detach,
  12 hours maximum) were made for separate session users on shared hosts and also applied to the
  owner's own administrator shell. The client then left the ended tab with two disabled buttons.

## Rulings

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-TSL-1 | 2026-11-02 | A session in the owner account mode (its assignment is the owner account, ADR 0073) stays attachable for `terminal.owner_account_detach_grace` (default `24h`) after a detach and lives at most `terminal.owner_account_max_session_duration` (default `168h`). The server sends the detach value in the `hello` of the session's data connection (it knows the session from the path), so the host service uses it without a new field; relay and `terminal.sweep` use the same windows. Sessions of separate session users keep `terminal.detach_grace` and `terminal.max_session_duration`. | The owner wants to detach in the evening and pick the shell up the next day from another device. A 12 hour maximum would end that shell before the detach lifetime. Security reasoning: the shell is the owner's own account on his own host, reachable only by an active owner (SR12, the `own` check of the host); every reattach after `terminal.quick_reattach_window` needs a fresh step-up with the second factor of a device of the owner; the Noise stream, the audit of every opening and attach and the revocation rules are unchanged; a detached PTY reads no input, and its scrollback stays only in the memory of the host service. A bound remains: 24 hours detached and 7 days in total, both configurable. |
| D-TSL-2 | 2026-11-02 | In the owner account mode idle time pauses while the session is detached and starts again with the reattach: the host service sets its idle reference to the attach, and the server counts the last grant as activity for such a session and skips the idle limit while it is detached. The idle timeout of an attached session (`terminal.idle_timeout`) is unchanged. | Idle counted across a detach ended a deliberately detached shell after 30 minutes, whatever the detach grace. While attached, an unattended open shell still ends. |
| D-TSL-3 | 2026-11-02 | Step-up on every reattach stays as in 24 6.2: from another device always, from the same device only within `terminal.quick_reattach_window`. | Brief: the step-up on every reattach stays; the long detach lifetime must not make a reattach cheaper. |
| D-TSL-4 | 2026-11-02 | A tab whose session ended or was blocked shows one button "Close" (`terminalCloseTab`) instead of the disabled "Detach" and "End"; "Close" removes the tab. "End" of a live or detached session ends it on the host and closes its tab. A refresh of the session list ends the tab of a session the server closed meanwhile (the app slept and missed the event), and a closed session never comes back as a tab. The same on the desktop (macOS, Windows) and on the phone screen M8. | The owner could not get rid of the dead tab and saw no effect of "End". |
| D-TSL-5 | 2026-11-02 | A tab whose stream dropped while it was attached (detach causes connection lost, server restart, slow consumer, host unavailable) attaches again by itself once per change: when the connection of the engine becomes live again (sleep, network change) or the app comes back to the front. A detach by the person, another device (`4103`), a stale token (`4101`) or a refused login stay detached until the person reattaches. A reattach after the quick window asks for the step-up as usual. | Brief: reconnect after sleep or a network change if the session was attached when the connection dropped. One attempt per change, no retry loop (15 23.2 item 8). |
| D-TSL-6 | 2026-11-02 | "Sessions" next to the tab actions lists the person's sessions on the host that are not closed: name (profile and number), start time, "connected" or "detached since", and "Reattach" (German "Wieder verbinden") for a detached one; the same list on every device. The German fixed term for "reattach" becomes "Wieder verbinden" (term table of 01 section 9). | Brief: every detached session is listed with "Wieder verbinden" on every device; the owner's own wording. |

## Not changed

- The separate-user setup of ADR 0068 keeps all its windows; nothing about other people's hosts
  changes.
- The scrollback stays in the memory of the host service (`host_service.scrollback_bytes`); a
  restart of the host service still ends every session (`host_restarted`, 24 6.2), as `tmux` would
  lose its sessions on a reboot.
- The web console keeps its own view and its passkey per attach (ADR 0070); it shows the new
  buttons only where it passes the new callbacks, which it does not in this change.
