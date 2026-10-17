---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Builder check (fresh reviewer)
informed: all teams
---

# Terminal through a host service with a Noise end-to-end stream and invariant I6

## Context and Problem Statement

The owner requires a terminal as a mandatory component (follow-up of 2026-10-10): shell and
sessions of a coding agent on connected hosts, in desktop, phone and web console, with
a separate permission per host, renewed passkey confirmation when opening, access only through a
private network or an end-to-end tunnel, audit, idle timeout and a separate user without
administrator rights on the host. The product must not require Tailscale (requirement 21),
the server is the authorization point (B7), agent content and credentials stay out of the
server (B10, I5). A terminal allows any use of the host's LLM login; Anthropic
allows subscription logins only for one's own use
(https://code.claude.com/docs/en/legal-and-compliance, retrieved 2026-10-10 [verified]). How does
a terminal get securely from the host to the device, and who may open it?

## Decision Drivers

* No inbound port on the host; works behind NAT, in the private network and in the cloud.
* The server decides every opening but reads no content.
* A reviewed, small standard with public test vectors instead of own cryptography.
* No back door around the permission model; agents and apps never open a terminal.
* Personal subscriptions only for their holder, also on hosts with several owners.
* An honest protection goal: what the encryption achieves and what it does not.

## Considered Options

* Host service with an outbound WebSocket connection, server as relay, Noise `NK` end-to-end
* SSH to the host over the private network (Tailscale or VPN), server only issues credentials
* Host service with a TLS stream that the server terminates and forwards (not end-to-end)
* WebRTC data channel between device and host with signaling through the server

## Decision Outcome

Chosen: "Host service with an outbound WebSocket connection, server as relay, Noise `NK`", because
only this option works without an inbound port and without requiring Tailscale, makes the server
the single decision point and still protects the content from relay, logs and database.
Normative: [24-terminal.md](../spec/24-terminal.md).

1. **Host service** `bothy-host` (separate Go binary, `services/host`) runs as a user without
   administrator rights, checks this itself (`doctor`) and talks to the server only outbound. The
   sessions run as the session user of their person (item 7), who cannot read the keys of the host
   and cannot replace its PTY helper (system service, HD10); a second live control connection
   with the same keys disables the host. With `personal` the host service does not touch any
   subscription login (D-W-17, D-W-18).
2. **Noise** `Noise_NK_25519_ChaChaPoly_SHA256` (revision 34, https://noiseprotocol.org/noise.html,
   retrieved 2026-10-10 [verified]). The device knows the static key of the host from the
   pairing. Prologue with protocol version, session ID and host ID.
3. **Terminal session token** in the first handshake packet: EdDSA JWS of the server, valid exactly
   once, bound to the ephemeral key of the device (`cek`). The PTY starts only after the first valid
   transport message. Reason: according to the specification the first message of `NK` can be
   replayed (section 7.7 [verified]).
4. **Permissions**: a separate target `host` with only the capability `terminal`, only for people
   and groups; owners implicitly. Guard rule SR10: non-owners only on their own host, the
   server host only for owners.
5. **Invariant I6**: with `llm_credentials = personal`, only the holder of the login opens a
   terminal, not even another owner; only an individual grant to that holder. Checked transactionally and
   on every opening.
6. **Step-up** fresh and consumed per opening.
7. **One session user per person** (ruling T12, see below): every person has their own
   session user on every host, never shared; without an assignment no terminal (SR12).

Distinction from [ADR 0024](0024-backend-policy-personal-organization-with-invariant.md) and
ruling L22: I1 has no owner exception because owners inherit `trigger` on every target and an agent
is reachable through many targets. A terminal is access to exactly one host, not an inherited
level. Without the owner exception, an instance with two owners could not have a personal host.
The exception restricts owners and extends nothing; L22 stays unchanged for I1.

### Addendum 2026-10-11: one session user per person and host (ruling T12)

Until this addendum, all sessions of a host ran under one shared
session user. "No subscription login of another person" then depended on HD6, a check in the
account of exactly this user. Three critique rounds found the same error class: the result could be
faked from the account (program in the `PATH`, K2-19), was switched off under macOS without
`host_service.claude_bin` (K3-01) or could be faked again through an interpreter (K3-06). A login
that a person created with `/login` was then used by every later session of other people.

Decision: the error class is fixed in one place, not per finding.

* The server holds, per host, the assignment of person to session user (`host_session_users`); the
  owner creates it at the confirmation or with `host.manage`. Without a valid assignment a
  person gets no terminal, not even an owner (SR12, `terminal_denied` with `reason: no_session_user`).
* A session user belongs to at most one person, also after the assignment ends; the name
  stays taken for the host (`409 conflict` with `reason: session_user_taken`).
* The terminal session token names the session user (`osu`); the host service starts the PTY only in
  the helper of this user and binds every session user to the first person who received it.
* HD6 remains as an additional check and notice (ruling T2 still applies: a contradiction to the
  declaration prevents the start). Its result is no longer the boundary; an empty
  `host_service.claude_bin` does not weaken enforcement.
* I6, SR10 (ruling T1) and the owner exception (ruling T2) stay unchanged and apply in addition.
* The host service reports the names of its session users (`hosts.reported_session_users`); the
  owner assigns only reported names (otherwise `400 validation_failed`), and an assignment takes
  effect only as long as the host reports its name (D-W-63). This way a typo in the assignment
  shows up there and not as a tampering warning `4104` when opening.

Rejected: treating HD6 with an empty `host_service.claude_bin` under macOS as an error (proposal
from K3-01). Reason: that closes only this one instance; every further gap in the oracle
(interpreter, modules, data of the same user ID) would remain a gap in the boundary. Separate
accounts make the rule true independently of any check. Separation by user IDs is not a
complete boundary between people (ADR 0046); that is why SR10 still applies.

### Addendum 2026-10-11: no shared locations (ruling T13)

Separate session users separate people only if no location connects their accounts again. Critique
round 4 found two such locations: working directories of the profile `claude` as one list for all
session users (project settings with hooks and `apiKeyHelper`, `CLAUDE.md` in the
folder chain then run in the account of another person) and a `CLAUDE_CONFIG_DIR` without a fixed
owner (a `/login` in it reached everyone). This is the same error class as before T12.

Decision: one rule and one check instead of a patch per location.

* On a host there is no location that one session user can write and another can read
  or execute, if it carries code, settings or logins.
* Working directories are listed per session user in `host_service.claude_workdirs` and belong to
  that user; a folder under two session users drops out of both lists (HD7); the index of the
  working directory counts only in its reported list (claim `wdi`); the `CLAUDE_CONFIG_DIR` lies, with
  `organization`, at a fixed place in the session user's home and, with `personal`, only within the
  home; programs, including their folder chain, belong to a trusted owner (`root`, under Windows
  `SYSTEM`, `Administrators` or `NT SERVICE\TrustedInstaller`); with `organization` the profile
  `claude` starts exactly `host_service.claude_bin`.
* The doctor check HD11 checks this with one rule for every configured path and the home,
  including the folder chain and the chain files in it (`CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.md`,
  `.mcp.json`, `.claude`, `.git`: what the coding agent and git load from the folders above the
  working directory), before every PTY and periodically; a violation blocks only the affected session
  user, and at a location for everyone, the whole host service. Every path is fully resolved first,
  the locations of a session user by its PTY helper; a path that cannot be resolved
  counts as not present. "Writable" includes creating entries for folders and appending for
  files. Under Windows, `bothy-host install`, after confirmation, removes the right of all
  authenticated users to create folders in the root of a drive, without changing the permissions
  of the folders below (24 section 2.2 item 6, D-W-80, D-W-83, D-W-84, D-W-86,
  D-W-87).
* Shared code is exchanged through git, never through a shared folder.

Rejected: a separate check per location (HD7 for the ownership of the working directories, HD9 for
working directories of others, as the critique proposed). Reason: every new location would again
need its own check and would again be found individually. The limit, stated honestly: HD11 does not
see folders outside the configuration, for example a shared folder in which a person starts the
coding agent themselves in their shell; per 16 section 4.4 the instance operator does not set up
such a folder. A file that a later version of the coding agent additionally loads from the folder
chain is checked by HD11 only once the list of chain files has been extended (D-W-62, D-W-68,
threat model T-TM-13).

### Consequences

* Good, because the host opens no port and the product works without a private network.
* Good, because relay, proxy, logs, database dump and an eavesdropping api process see no content.
* Good, because every opening goes through `authorize` and is recorded in the audit.
* Bad, because a malicious instance operator of the server can distribute their own key and allow
  sessions. Mitigation: key pinning per device with a hard warning, audit of every
  re-pairing, option `host_service.require_local_confirm`; listed in the threat model as a residual
  risk (ruling T3).
* Bad, because several server nodes need hash routing in the proxy so that both ends of a
  session land on the same node (ruling T6).
* Bad, because the product cannot check the content of an LLM login; I6 depends on the
  owner's declaration. The host service checks whether a subscription login is present and refuses
  the start on a contradiction; but what carries the rule is the separate session user per person
  (ruling T12).
* Bad, because the instance operator sets up an operating system user and a PTY helper per person
  and the owner must assign every person before that person gets a terminal.
* Bad, because people cannot use a shared working directory on a host; each person
  works in their own clone and exchanges changes through git (ruling T13).
* Bad, because a single person with a subscription login in their account can bring a host with
  `none` or `organization` to a standstill for everyone (ruling T2); the message names every
  affected session user for this, also on a hit at start (D-W-91), and the instance operator
  ends the standstill by removing the login
  or taking the session user out with `bothy-host session-user remove` (D-W-64,
  D-W-67, threat model T-TM-14). The same applies to a chain file of a person in a
  shared folder on the path to the locations of others; such files exist only from a time when
  this folder was writable, and the instance operator removes them (D-W-80). A home that a person
  with terminal access opens for others, by contrast, blocks only that person's own session user
  (HD11, D-W-88). If a long-running process of the person reopens the home of their former session
  user after `bothy-host session-user remove`, HD9 refuses the start; the message names this user,
  and the instance operator deletes it together with its long-running processes (D-W-109).
* Bad, because every restart of the host service ends all sessions of the host, including
  `bothy-host session-user add` and `remove`, which restart it so that the changed list of
  session users applies (D-W-90); the instance operator chooses a quiet time for this.

### Confirmation

* Conformance with public Noise test vectors (cacophony, noise-c; 24 section 5.2) in Go
  and Dart (TM-T01) and a cross test of Go against Dart (TM-T02). Since M2-38: TM-T01 is held by
  `TestVectorsTMT01` in `services/host/internal/noise` and `test/vectors_test.dart` in
  `packages/bothy_noise`; TM-T02 is held by `TestRecordingsTMT02Dart` and
  `TestRecordingsTMT02Negative` in Go and `test/recordings_test.dart` in Dart, over the
  recordings of both languages in both roles.
* Separation of users and disabling on a second control connection (TM-T22, TM-T23).
* Since M2-42 the host side of TM-T13, TM-T22, TM-T24, TM-T26, TM-T27 and the TM-T29 parts (a),
  (b), (c), (d), (e), (f), (h), (i), (j) holds the platform suite of `services/host/test/platform`
  (`TestPlatform_TMT13`, `TestPlatform_TMT22`, `TestPlatform_TMT24`, `TestPlatform_TMT26`,
  `TestPlatform_TMT27`, `TestPlatform_TMT29_Parts`, `TestPlatform_TMT21_SessionUserAddRemove`)
  on disposable GitHub-hosted runners; the unit tests `TestHD1` to `TestHD11`,
  `TestEntryScanSemantics`, `TestReportNamesRule`, the peer checks of `internal/localchan` and
  `TestHostPairAgainstServer` in `services/server/test/terminal` hold the rules on every build
  host. Until a recorded platform run proves a platform, HD10 fails there with
  `peer_check_unavailable` and the host gives no terminal (docs/DECISIONS.md D-M2-42-19).
* Replay and token tests in the host service (TM-T03 to TM-T05).
* Canary test over relay, logs, database and telemetry (TM-T06).
* Permission test matrix AZ-M54 to AZ-M61 and I6 tests (TM-T09) in `make test`.
* One session user per person: TM-T28 (without assignment, taken name, wrong `osu`, subscription in
  the account of another person, unreported name) and home folders of others (TM-T24: HD9 against
  users who are not session users; an open home of a session user blocks, per
  HD11, only that user; a long-running process reopens the home after `remove`, and HD9 names the
  former session user).
* No shared locations: TM-T29 (working directory writable by others with a hook and `apiKeyHelper`,
  folder chain, duplicate working directory, readable `CLAUDE_CONFIG_DIR`, entry of another user,
  writable program or program of another user, index of another user, chain file of another user).
* Standstill through HD6 and its end, also with a hit at start: TM-T21.
* Red team terminal at the end of M2, M4, M5, M6 (threat model, section 6.7).

## Pros and Cons of the Options

### Host service, relay, Noise `NK`

* Good, because it is outbound, without network prerequisites and end-to-end encrypted.
* Good, because `github.com/flynn/noise` (BSD-3-Clause, https://pkg.go.dev/github.com/flynn/noise,
  retrieved 2026-10-10 [verified]) brings `HandshakeNK` and `Prologue`.
* Bad, because a separate package is needed for the Dart handshake.

### SSH over the private network

* Good, because it is proven and usable with existing tools.
* Bad, because the host opens a port and the product requires a private network
  (requirement 21).
* Bad, because the server no longer decides the opening itself but only issues keys;
  revocation takes effect only when the key expires.

### TLS to the server, server forwards

* Good, because it is simple.
* Bad, because the server sees every keystroke and every output in plain text; a dump of an
  api process or a logging error exposes shell content.

### WebRTC data channel

* Good, because direct connections relieve the server.
* Bad, because STUN and TURN are needed as additional services and the relay is needed anyway.
* Bad, because the browser stack and the libraries of the platforms have a large attack
  surface.

## More Information

* Related: ADR 0020 (step-up), ADR 0024 (I1), ADR 0046 (dev isolation), ADR 0067 (JOSE profile),
  [ADR 0070](0070-terminal-in-the-web-console.md) (terminal in the web console).
* Rulings T1 to T6, T9, T10, T12 and T13 of the lead (DECISIONS, D-W-57, D-W-62).
* Review: before publication (external test P2) and when changing the Noise library.
