---
status: accepted
date: 2026-11-01
decision-makers: Owner, planning lead
consulted: Builder check (fresh reviewer), red team terminal
informed: all teams
---

# Terminal in the owner's own account (owner account mode)

Partly supersedes [ADR 0068](0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md):
item 1 ("runs as a user without administrator rights") and item 7 with ruling T12 ("every person
has their own session user") no longer hold without exception. On a host in the owner account
mode the host service and the one session user are the existing account of the instance owner,
with administrator rights. Everything else of ADR 0068 stays, in particular the server as the
single decision point, the Noise `NK` stream, the session token, step-up on every opening, I6,
SR10, audit and the idle timeout. Normative: [24-terminal.md](../spec/24-terminal.md) section 2.4;
design note: [terminal-owner-account.md](../design/terminal-owner-account.md); rulings:
[decisions.d/FEAT-terminal-admin-user.md](../decisions.d/FEAT-terminal-admin-user.md).

## Context and Problem Statement

ADR 0068 followed the owner requirement of 2026-10-10: terminal sessions run as a separate session
user without administrator rights. On 2026-11-01 the owner changed this requirement for his own
machine (translated): "but I want to run there as admin, I want to get things done" and "from the
app I want to work in the terminal on the server, that was the requirement, that was the plan". He
wants a terminal from the apps on his own server machine, as his normal administrator account.

The owner has no physical access to that machine. A setup that creates new local users, needs root
for a system service, shows a dialog or needs a person at the keyboard cannot be completed. The
separate-user setup of ADR 0068 needs all of that (system services, new hidden users, a local
confirmation), and its self-checks refuse an administrator account (HD1, HD2, HD3), a session user
that equals the service user (HD1), the owner's own subscription login (HD6) and programs in a
folder of the account (HD10, HD11).

How can the owner get a shell in his own administrator account without weakening what protects
everybody else?

## Decision Drivers

* Owner decision of 2026-11-01: a real administrator shell on his own host, from the apps.
* Completable remotely: no new local users, no root, no dialog, nobody at the machine.
* Nobody but an owner can ever reach that shell, not even with a wrong assignment in the server.
* The rest of the terminal security model stays as it is: server authorization, per-host
  permission, step-up on every opening, Noise end-to-end, audit, idle timeout.
* Defaults stay: a host installed as before keeps separate users without administrator rights.

## Considered Options

* Owner account mode: an explicit, per-host mode in which the host service and the one session user
  are the owner's existing account, installed without root, opted into in the server per assignment
* A session user with administrator rights next to the separate service user
* SSH from the apps to the owner's account
* Keep ADR 0068 and tell the owner to use `sudo` in a session user

## Decision Outcome

Chosen option: "Owner account mode", because it is the only option that needs no root and no new
user on the host, and it keeps the server as the decision point.

1. **Host.** `host_service.owner_account` names the account. The host service runs as this account
   (macOS: a LaunchAgent in the account's GUI domain; Linux: a systemd user unit), and
   `host_service.session_users` is exactly this one name. `bothy-host install --owner-account`
   sets it up as that account, without administrator rights. The PTY helper is a child process of
   the host service on a socket in the state directory; the peer check compares the uid of the
   account and the PID of the child. Windows is not supported in this version: a per-user
   background service there needs either a stored password or an elevation.
2. **Host token check.** The host service accepts a session token for the owner account only with
   `own = true` and checks this before it binds the session user to a person; otherwise
   `session_user_mismatch` (`4104`). Even a wrong assignment in the server cannot give a
   non-owner this shell.
3. **Explicit opt-in in the server.** The host reports the owner account with
   `owner_account = true`. An assignment of that name needs `owner_account = true` in the request,
   and only an active owner can make it, only for himself (`403 forbidden`,
   `reason: owner_account_owner_only`). A name reported with a different flag than stored no longer
   counts for SR12.
4. **On every opening.** SR12 additionally requires an active owner for an owner account
   assignment (`403 terminal_denied`, `reason: owner_account_owner_only`); losing the owner role
   ends the running sessions. Every opening and attach in this mode is audited with
   `owner_account = true`.
5. **Self-checks.** In this mode the checks that would refuse the mode by design pass with
   warnings: HD1 (session user equals the service user, warning `owner_account_mode`), HD2, HD3,
   HD9, the helper part of HD8 and the HD10 codes `peer_check_unavailable`, `ptrace_scope_low` and
   `helper_unprotected`; HD6 counts a subscription login in the owner account as his own (warning).
   The rules for other people stay: HD4 and the service side of HD8 keep the state directory away
   from every other person account, HD10 and HD11 accept the owner account as an owner of programs
   but still refuse a program or folder that any other person account or everyone can write.
6. **No local confirmation.** The mode refuses `host_service.require_local_confirm = true`: nobody
   sits at the machine to answer it. Step-up with the second factor on every opening replaces it.
7. **Clients.** The host settings show the owner account with a warning and an explicit checkbox
   for the assignment; the open dialog warns before the step-up that the session runs with
   administrator rights.

### Consequences

* Good, because the owner can open a real administrator shell on his own machine from every app,
  and the steering session can set it up remotely without root.
* Good, because a non-owner cannot reach the shell: the server refuses the assignment and the
  opening, and the host refuses a token without `own = true` before it binds the account.
* Bad, because a session in this mode can read the keys of the host and replace the host service.
  Whoever gets the owner's second factor and password gets the machine. That is the owner's
  explicit choice and the same as an SSH login; threat model T-TM-16 records it.
* Bad, because the separation of people (T12, T13) does not exist on such a host: it is a host for
  exactly one owner. Other people get no terminal there.
* Bad, because a subscription login in the owner's account is not checked against the declaration;
  the Anthropic terms allow use by its holder, and only the owner reaches this account.
* Bad, because Windows hosts cannot use the mode yet.

### Confirmation

* Server: `TestOwnerAccountAssignment` and `TestTerminalGuard_OwnerAccount` (owner may bind
  himself, non-owner and another person 403, a flag that differs from the report 400, opening by a
  demoted owner 403), `TestOpenAuditsOwnerAccount` (the flag in the audit entry).
* Host: `TestVerifyOwnerAccountNeedsOwn` (a token without `own` binds nothing), the doctor tests
  `TestOwnerAccountMode*` (HD checks pass, HD6 passes for the owner account and still fails for a
  separate session user), `TestInstallOwnerAccount*` (plans without root, without `dscl`,
  `useradd`, `/Library/LaunchDaemons` or `/etc/systemd/system`) and the child helper test
  `TestChildHelpers`.
* Red team over the whole change: nobody but the owner reaches the shell, nothing else weaker.

## Pros and Cons of the Options

### Owner account mode

* Good, because it needs neither root nor a new user, so it is remote-completable.
* Good, because the server stays the decision point and the host enforces `own` on its own.
* Bad, because the separation of the host service from the session (keys, helper) is given up for
  this host.

### Session user with administrator rights next to a separate service user

* Good, because the keys would stay with the service user.
* Bad, because a session with administrator rights reads the keys of the service user anyway
  (`sudo`), so the separation would only be apparent [derived].
* Bad, because it still needs root to create the service user and the system services.

### SSH from the apps

* Bad, because the host opens a port and the product requires a private network (requirement 21),
  and the server is no longer the decision point (ADR 0068).

### `sudo` in a session user

* Bad, because it needs the setup with new users and root that cannot be done remotely, and HD3
  refuses a passwordless `sudo`.

## More Information

* Owner decisions of 2026-11-01 (quoted above), changing the requirement of 2026-10-10.
* Related: ADR 0068, ADR 0070, threat model T-TM-16.
