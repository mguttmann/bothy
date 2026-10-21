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
connection, `ready`, `report`) in `internal/control`, M2-44 the data connections and M2-45 the
sessions with PTY, profiles and the periodic runs of HD6 and HD11.

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
with `1000` over the same connection. Until M2-45 the sessions are `RefuseSessions`: every
`session.open` is answered with `session.ended {open_failed, pty_failed}`.

Reconnect policy by close code: `1000`, `1001`, `4000`, `4002`, `4009`, `4014`, `4113` and any
unknown code reconnect with exponential backoff and full jitter from
`host_service.reconnect_backoff_base` to `host_service.reconnect_backoff_cap` (reset after a
`ready`); `4013` after a random delay below the base; `4008` after `retry_after` of the close
reason, else the cap; `4111` ends all PTYs and waits the cap before each attempt; `4110`, `4112`
and `4114` end all PTYs and stop `run` with exit code `78`, which no service definition restarts.
The frames are hand-written; `testdata/control/host_frames.json` and `server_frames.json` hold
them equal to the contract in both directions (`make recordings` rewrites the host recording).


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
`organization`, read by the sessions of M2-45). Every file is written atomically.

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
| `testdata/fakeclaude/` | stand-in for `claude auth status` ([README](testdata/fakeclaude/README.md)) |
| `test/platform/` | the platform suite (build tag `platform`) |
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
make release-build VERSION=$(cat VERSION) OUT=<dir>   # unsigned binaries for five platforms
make platform-test                                   # only on a disposable machine, see below
```

The platform suite needs administrator rights and creates and deletes users, services and ACLs.
It runs only in the workflow `host-platform.yml` on throwaway GitHub-hosted runners
(docs/DECISIONS.md D-M2-42-19), never on a workstation.
