# services/host

The host service `bothy-host`: an own Go binary on connected hosts for the terminal.
Normative: [24 Terminal](../../docs/spec/24-terminal.md),
[ADR 0068](../../docs/adr/0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md).
Installation and operation: [16 Operations, 4.4](../../docs/spec/16-operations.md).

It runs as users without administrator rights, connects only outwards to the server, is the
Noise peer of the device and, per terminal session, starts a PTY in the helper of the person's
own session user.

## State

M2-38 built the Noise core. M2-42 adds installation, the configuration file, the state
directory, the self-checks HD1 to HD11 (`doctor`), the local channel to the PTY helpers with its
peer check, the start sequence of `run` and `pair`. M2-43 implements the `Control` port (control
connection, `ready`, `report`) in `internal/control`. M2-45 runs the terminal sessions end to
end: one data connection per `session.open`, the Noise responder and the check of the session
token, the gate of HD6, HD10 and HD11 before every PTY and every
`host_service.credential_check_interval`, the local confirmation, the PTY with its process tree
in the helper of the session user, the profiles `shell`, `claude` and `command`, the scrollback
and the idle and detach timers. Revocation, the server-side limits, audit and metrics follow with
M2-46.

## Control connection (M2-43)

`internal/control` is the client of the control connection (24 section 3.5): one outgoing
WebSocket to `wss://<server>/api/v1/hosts/connect`, only from the `https` URL of the pairing
record, with the system roots and the proxy of the environment, never a listener and never a skip
of the TLS verification. It signs the proof of `identify` with the login key of the state
directory (header `alg` EdDSA, `typ` `host-auth+jwt`, `kid` the RFC 7638 thumbprint; claims
`challenge`, `host_id`, `iat`), sends `report` as the first frame after every `ready`, sends
`heartbeat` every `heartbeat_interval_ms` and treats `heartbeat_interval_ms` times
(`heartbeat_miss_limit` + 1) of silence as a dead connection. `is_server_host` is HD5 or-ed with
whether the server of the connection is this machine. `Client` implements `startup.Control`:
`run` logs in through it (a stored report goes first after every reconnect, every later `ready`
reaches `startup.Service.OnReady`), and on a refused start it logs in with empty `live_sessions`,
retries for at most `host_service.reconnect_backoff_cap`, sends exactly one `report` and closes
with `1000` over the same connection. `run` binds the session manager of `internal/session` as
`control.Sessions`; only when it cannot be built (for example before pairing) the sessions stay
`RefuseSessions`, which answers every `session.open` with `session.ended {open_failed,
pty_failed}`.

Reconnect policy by close code: `1000`, `1001`, `4000`, `4002`, `4009`, `4014`, `4113` and any
unknown code reconnect with exponential backoff and full jitter from
`host_service.reconnect_backoff_base` to `host_service.reconnect_backoff_cap` (reset after a
`ready`); `4013` after a random delay below the base; `4008` after `retry_after` of the close
reason, else the cap; `4111` ends all PTYs and waits the cap before each attempt; `4110`, `4112`
and `4114` end all PTYs and stop `run` with exit code `78`, which no service definition restarts.
The frames are hand-written; `testdata/control/host_frames.json` and `server_frames.json` hold
them equal to the contract in both directions (`make recordings` rewrites the host recording).

## Sessions (M2-45)

Process split (docs/spec/24-terminal.md 2.2). The service user process (`bothy-host run`) holds
the keys, the control and data connections, Noise, the check of the session token, the `jti`
memory (only in memory), the bindings of session users to people (`bindings.json`), the
scrollback (a ring of `host_service.scrollback_bytes` per session, only in memory), the idle and
detach timers and the gate. Each PTY helper (`bothy-host pty-helper`, one per session user) holds
the PTYs of its person, resolves that person's places and runs that person's checks; it never
sees a key of the host or a session token. Between them runs only the local channel.

Per `session.open` the manager opens one outgoing data connection to
`/api/v1/hosts/sessions/{session_id}/stream` with the TLS settings of the control connection,
reads `hello` (its `clock_leeway_ms`, `idle_timeout_ms` and `detach_grace_ms` are the server
values of that session), sends `identify` with `session_id`, answers Noise message 1 as responder
and checks the token in the order of 24 5.3 (steps 0 to 13). A refusal closes the data connection
with `4101` or `4104` and the `close_detail` as close reason. Then it sends message 2, waits for
`start` until `HostSessionOpen.expires_at`, runs the gate and the local confirmation, starts the
PTY in the helper of `osu` and sends `session.started`. Every failure of an opening after
message 2, other than the loss of the data connection, is `session.ended {open_failed,
<detail>}`, and the server closes the data connection (the host service closes it with `1000`
only after the opening window). `session.close` and a server close with `4100` end the PTY
without `session.ended`; a `4104` refusal, a faulty first frame or a denied confirmation of an
attach end it too; every other close or loss leaves it detached.

Gate (`internal/session/gate.go`): before every PTY start one `Light` run asks only the helper of
`osu`, the periodic run asks every helper. HD10 or a failed peer check of a session channel give
`helper_untrusted`, HD6 `subscription_login_detected`, HD11 at a place for all `shared_location`
(each ends all running sessions with `host_policy_violation`); HD11 of one session user refuses
new sessions of that session user and ends its running ones, no other session user is affected;
any other start-refusing finding refuses new sessions with `pty_failed`.
Every change of the result sends `report`.

## Session ops of the local channel

Kind `0x00` carries JSON ops `{"op","id","d"}`:

| Op | From | Body |
|---|---|---|
| `pty.start` | service | `session_id`, `profile`, `kind`, `declaration`, `program` and `args` (`command`), `workdir_index` and `workdir` (`claude`), `cols`, `rows`, `api_key` (only `claude` at `organization`) |
| `pty.resize` | service | `session_id`, `cols`, `rows` |
| `pty.stop` | service | `session_id`; ends the PTY with its whole tree at once |
| `pty.started` | helper | `session_id` |
| `pty.exited` | helper | `session_id`, `exit_code` (Unix: the status, or 128 plus the signal number; Windows: the exit code) |
| `pty.failed` | helper | `session_id`, `close_detail` (`pty_failed` or `shared_location`) |

Kind `0x01` carries one frame: the 16-byte session ID followed by the bytes of one `data` frame,
in both directions; the helper reads PTY output in chunks of at most `localchan.MaxData`, so the
body stays within the codec limit. The service reads `claude_api_key` for each start and sends it
only in that `pty.start`; the helper puts it only into that child's environment. With `claude`
the helper starts only when `workdir` is one of the working directories of its last `check.run`
and its identity equals the one it reported then. When the channel breaks the helper ends all its
PTYs with their trees, and the service ends their sessions with `process_exited`.

## Profiles and their environments

`internal/profiles` builds the start specification of each profile from scratch, never by
removing from an inherited environment; `TERM` is `xterm-256color` for every profile.

- Unix allow list: `HOME`, `USER`, `LOGNAME`, `SHELL` of the session user, `PATH` cleaned from the
  helper's environment (absolute, existing folders not writable by group or others, after
  resolution, without duplicates), `LANG`, `LC_*` and `TMPDIR` when present. Windows: the
  environment block of the helper's token without inheritance.
- `shell`: the login shell (`argv[0]` `-<name>`, listed in `/etc/shells`; Windows
  `powershell.exe -NoLogo`), working directory the home.
- `command`: the resolved `program` with `args`, working directory the home.
- `claude` at `organization`: exactly the resolved `host_service.claude_bin`, `PATH` the search
  list of HD6 first, `ANTHROPIC_API_KEY` from `claude_api_key`, `CLAUDE_CONFIG_DIR`
  `<home>/.config/bothy-host/claude`, working directory the entry `wdi` names; on Windows the
  OAuth and provider variables are removed from the block.
- `claude` at `personal`: the login shell runs `claude` from its own `PATH` with the
  `CLAUDE_CONFIG_DIR` the login-shell probe reported, which must resolve inside the home
  (`shared_location` otherwise); no key. OQ-03 found both login paths documented
  (`profiles.PersonalLoginDocumented`, docs/DECISIONS.md D-M2-45-1).
- `none` or an unknown declaration: no `claude` profile. `profiles.Offered` lists `claude` only
  when at least one session user has a working directory after HD7.

## Local confirmation

With `host_service.require_local_confirm = true` the service opens the confirmation endpoint and
asks before every PTY start and every replay of an attach; no agent, no answer within
`host_service.local_confirm_timeout` or a denial give `local_confirm_denied`. The person at the
machine runs `bothy-host confirm-agent` in their desktop session, started by an autostart entry
the operator creates (LaunchAgent, XDG autostart file, `Run` key; 16 4.4). The agent shows a
dialog with "deny" as default (macOS `osascript`, Linux `zenity` or `kdialog`, Windows
`MessageBoxW`) and denies without a dialog program. The endpoint admits only a user who is not
the service user, not a current or former session user and owns the active console session; it
checks this at the connection and before every request and logs `confirm_peer_refused`.

| | Linux | macOS | Windows |
|---|---|---|---|
| confirmation endpoint | `/run/bothy-host-confirm/confirm.sock` (`RuntimeDirectory=bothy-host-confirm`) | `/Library/Application Support/bothy-host/confirm/confirm.sock` (folder by `install`) | pipe `\\.\pipe\bothy-host-confirm` (`PIPE_REJECT_REMOTE_CLIENTS`) |
| console owner | user of the active `seat0` session (`loginctl`) | owner of `/dev/console` | user of the active console session |

## Platform specifics and limits

- Linux: the helper unit has `Delegate=yes`; the helper moves into the leaf `helper` of its
  cgroup and clones each session into `s-<session id>` (`CLONE_INTO_CGROUP`); the end writes
  `cgroup.kill`. Without a delegated cgroup v2 subtree the helper serves no session.
- macOS: each session is its own POSIX session; the end kills every process of the session user
  found by session, process group or parent chain in process table snapshots. The run file
  `<home>/.config/bothy-host/run/pty-sessions.json` (mode `0600`) lets the next helper start end
  trees that a killed helper left. Limit: a process that left the session with `setsid` after its
  parent ended is not found; it runs in the person's own session user like a long-running process.
  The run file lets the next start end recorded trees also when their leader is gone.
- Windows: ConPTY; the leader starts suspended, joins a Job Object with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and without breakaway, then runs; the end terminates the
  job. Windows is tested only on the hosted runner of `host-platform.yml` (Windows Server); no
  Windows 10 or 11 desktop run is recorded.
- Every end kills the tree at once, without a grace period.

## Commands

| Command | Who runs it | Exit codes |
|---|---|---|
| `install [--session-user <name>] [--drive-root <drive>] [--drive-root-confirm yes\|no] [--config <path>]` | administrator | `0`, `1`, `2` |
| `session-user add <name>`, `session-user remove <name>` | administrator | `0`, `1` (bound name), `2` |
| `restart` | administrator | `0`, `1`, `2` |
| `pair --server <url> --code <code> [--ca-file <pem>]` | service user, or an administrator through the service | `0`, `1`, `2` |
| `doctor` | service user, or an administrator through the service | `0` passed, `1` a check failed, `2` |
| `run` | the service manager, as the service user | `0`, `1`, `78` refused start |
| `pty-helper --session-user <name>` | the service manager, as the session user | `0`, `1`, `2` |
| `confirm-agent` | the person at the machine, in their desktop session | `0`, `1` (endpoint unavailable), `2` |

A command that needs administrator rights and lacks them exits `2` with
`bothy-host <command>: administrator rights required`. A refused start exits `78`; no service
definition restarts it, only `bothy-host restart` does.

## Identities, files and paths

| | Linux | macOS | Windows |
|---|---|---|---|
| service user | `bothy-host` (system user) | hidden user `bothy-host` | virtual account `NT SERVICE\bothy-host` |
| session users | `useradd --create-home --user-group`, home `0700` | hidden users, own primary group, not in `staff`, home `0700` | local accounts, password only for the service control manager, profile ACL only for the user |
| program | `/opt/bothy-host/bin/bothy-host` | `/Library/Application Support/bothy-host/bin/bothy-host` (ad hoc signed with the hardened runtime) | `%ProgramFiles%\bothy-host\bothy-host.exe` |
| configuration | `/etc/bothy-host/config.yaml` | `/Library/Application Support/bothy-host/config.yaml` | `%ProgramFiles%\bothy-host\config.yaml` |
| state directory | `/var/lib/bothy-host` | `/var/lib/bothy-host` | `%ProgramData%\bothy-host` (protected DACL) |
| host service | `bothy-host.service` | LaunchDaemon `bothy-host` | service `bothy-host` |
| helper per session user | `bothy-host-helper-<name>.socket` and `.service` | LaunchDaemon `bothy-host.helper.<name>` with `Sockets` | service `bothy-host-helper-<name>` |
| channel endpoint | `/run/bothy-host/helper-<name>.sock` | `/Library/Application Support/bothy-host/run/helper-<name>.sock` | pipe `\\.\pipe\bothy-host-helper-<name>` |
| admin endpoint | `<state>/admin.sock` (uid 0 only) | `<state>/admin.sock` (uid 0 only) | pipe `\\.\pipe\bothy-host-admin` (elevated only) |

State directory (`internal/state`): `keys` (both private keys, mode `0600`, on Windows a DPAPI
blob), `pairing.json`, `bindings.json`, `declaration.json`, `server_ca.pem` (the CA of
`pair --ca-file`, which the control connection trusts besides the system roots, M2-43),
`claude_api_key` (only at
`organization`, read for each start of the profile `claude`). Every file is written atomically.

## Configuration file

```yaml
host_service:
  session_users: [bothy-s-alex, bothy-s-sam]
  claude_bin: /opt/claude/bin/claude
  claude_workdirs:
    bothy-s-alex:
      - {display_name: Project A, path: /home/bothy-s-alex/project-a}
  profiles: [shell, claude, {name: top, display_name: Top, kind: command, program: /usr/bin/top}]
  auth_status_timeout: 30s
```

Every key is `host_service.<name>` of [24, section 17](../../docs/spec/24-terminal.md); unknown or
duplicate keys and wrong types are errors naming the key. The configuration is read at start only.

## Doctor

`doctor` prints one line per entry (`HD<n> ok`, `HD<n> WARN <code>`, `HD<n> FAIL <code>` with
`session_user=`, `user=`, `drive=` or `item=`), the summary and the declaration in use; never a
path, a content or a value. The codes are the table "Codes of `doctor`" in 24 2.2. The report to
the server carries codes and, by the rule for names, the names of session users only.

## Exit policy and the way per platform

- systemd: `Restart=on-failure`, `RestartPreventExitStatus=78`, no `NoNewPrivileges=` (HD3 must see
  a sudo rule); the helper sockets are created by systemd (`SocketUser=root`,
  `SocketGroup=bothy-host`, `SocketMode=0660`).
- launchd: `KeepAlive` `{Crashed: true}` and `GOTRACEBACK=crash`; the helper gets its listening
  socket on standard input (`inetdCompatibility`, `Wait = true`).
- Windows: failure actions on crashes only; a refused start stops with the service-specific code
  `78`; helper pipes are created with `FILE_FLAG_FIRST_PIPE_INSTANCE` and
  `PIPE_REJECT_REMOTE_CLIENTS`; the DACL of a helper service lets its session user only query it.
- Peer check: the helper passes one end of a `socketpair` (Linux, macOS); the service compares
  uid and PID of that end with the session user and the main PID of the helper service. Until a
  recorded platform run proves a platform, HD10 fails there with `peer_check_unavailable`.

## Layout

| Path | Content |
|---|---|
| `cmd/bothy-host/` | the commands |
| `internal/noise/`, `internal/frame/`, `internal/fingerprint/` | Noise core (M2-38) |
| `internal/buildinfo/` | version and platform, the list of platforms with a proven peer check |
| `internal/clock/` (+ `clocktest`) | time source and fake clock |
| `internal/config/` | the configuration file, editing that keeps comments |
| `internal/state/` | the state directory |
| `internal/hostos/` (+ `hostostest`) | port to the operating system and its fake |
| `internal/doctor/` | HD1 to HD11, report, local output, the entry scan |
| `internal/localchan/` | local channel, peer check, admin endpoint, codec |
| `internal/helper/` | the `pty-helper` process |
| `internal/install/` | plans and executors, service definitions in `units/` |
| `internal/startup/` | `run`, the `Control` port and `NoControl` |
| `internal/control/` (+ `controltest`) | the control connection (M2-43) and its TLS fake server |
| `internal/pairing/` | `pair` |
| `internal/session/` (+ `sessiontest`) | session manager, data connections, token check, gate, scrollback, timers (M2-45); fake server and Go test device |
| `internal/pty/` | PTY and process tree per platform (cgroup, process group and run file, Job Object) |
| `internal/profiles/` | offered profiles, start specifications and allow-list environments |
| `internal/confirm/` | confirmation endpoint, peer rule and the agent with its dialogs |
| `testdata/fakeclaude/` | stand-in for `claude auth status` ([README](testdata/fakeclaude/README.md)) |
| `test/platform/` | the platform suite (build tag `platform`) and its marker guard (`guard.go`, untagged) |
| `test/noisedevice/` | the Noise sidecar of the Go test device of the terminal suite in `services/server/test/terminal` (M2-48): JSON lines on stdin and stdout, `internal/noise` and `internal/frame` unchanged |
| `.go-arch-lint.yml` | architecture rules, maintained by hand, one component per package |

What the Noise core packages guarantee: no network, no process start, no logs; error texts name
codes, lengths and type bytes, never payload, plaintext, keys or tokens. After a failed handshake
step or a failed decryption the object refuses every further call.

## Development

```sh
cd services/host
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go test -run='^$' -fuzz=FuzzDecodeMessage -fuzztime=60s ./internal/localchan
GOTOOLCHAIN=local go test -run='^$' -fuzz=FuzzDecodeServerFrame -fuzztime=60s ./internal/control
GOTOOLCHAIN=local go test -run='^$' -fuzz=FuzzVerifySessionToken -fuzztime=60s ./internal/session
make release-build VERSION=$(cat VERSION) OUT=<dir>   # unsigned binaries for five platforms
make platform-test                                   # only on a disposable machine, see below
make platform-bundle OUT=<dir> [GOOS=<os> GOARCH=<arch>]  # build a platform run, run nothing
```

The platform suite needs administrator rights and creates and deletes users, services and ACLs.
It runs only in the workflow `host-platform.yml` on throwaway GitHub-hosted runners
(docs/DECISIONS.md D-M2-42-19), never on a workstation.

Platform bundle (M2-48): `make -C services/host platform-bundle OUT=<dir> [GOOS=<os>
GOARCH=<arch>]` builds, without running anything, every file a platform run needs: `bothy-host`
with the platform-suite mark, the Fake-`claude` and its `node` build, the fakes of the suite
(`suite-claude`, `bighome`), the test binary of
`test/platform` and the test binaries of `internal/pty`, `internal/session`, `internal/profiles`
and `internal/confirm`, `run.sh` (`run.ps1` on Windows) and `bundle.json` with the commit, the Go
version and the SHA-256 of every file. Without `OUT` it is a usage error (exit `2`).
`make platform-test` builds the bundle of the build host and runs it, so both paths execute the
same files. The terminal suite runs the Linux bundle as root in a fresh host container in every
`make check` (`services/server/test/terminal/e2e/hostplatform`); the same bundle, built for the
platform, runs on disposable macOS, Windows 10 and Windows 11 test machines with `run.sh suite` or
`run.ps1 suite` as administrator and writes the result file.

Marker guard: the platform test binary refuses to run unless the marker file
`/etc/termsuite-disposable` (Windows `%ProgramData%\termsuite-disposable`) exists; without it it
prints `platform: not a disposable test machine (marker file missing)` and exits `2`
(`test/platform/guard.go`, tested in every `make check`). The host image of the terminal suite
carries the marker; on a disposable test machine the run's operator creates it before the run, on
no other machine.
