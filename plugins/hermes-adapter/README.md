# hermes-adapter

Status: recording present (M2-32), adapter planned (M2-33).

## Purpose

Reference adapter for Hermes Agent (https://github.com/NousResearch/hermes-agent), an open-source
agent framework. The adapter is a Hermes platform plugin (`kind: platform`). It connects
a Hermes profile as an agent principal to the server: invocations come through the gateway; answers,
live box and components go through the public API.

Hermes is one adapter among several. The server does not depend on Hermes.

## Contract

- General: docs/spec/09-agents.md sections 4 to 16.
- Hermes-specific: docs/spec/09-agents.md section 17 (mapping of the
  `BasePlatformAdapter` methods, session key through the thread ID, `busy_input_mode`,
  check of the invocation token before `handle_message`, allow-list, reported mode).
- Backend policy: docs/spec/10-backend-policy.md section 10.1.

## Fixed rules

- Only public Hermes plugin interfaces; no change to Hermes. Where there is none (login type,
  17.8), a compatibility layer encapsulates the internal function, and a contract test against
  the pinned version holds it.
- Never change the platform key (`hermes_adapter.platform_key`) after commissioning; otherwise
  every thread loses its Hermes session.
- The global `display.busy_input_mode` explicitly `steer` or `queue` (the per-platform key is not
  read); with `interrupt` the adapter reports `degraded`.
- `display.platforms.<platform_key>.tool_progress: off`, so that Hermes sends no progress bubbles
  as messages; steps come from `set_status_text`.
- No start without a valid invocation token; detect the reported mode only from the key type,
  never store, send or log the value.

## Recording (M2-32)

Before the adapter is built, M2-32 measured how the pinned Hermes version calls the surfaces of
section 17. The pinned version is in `tests/hermes_pin.toml`; tag, commit and the answers are in
docs/spec/09-agents.md 17.10. In short:

- `progress_hooks`: no call of `format_tool_event`; tool steps through `set_status_text`,
  cumulative drafts through `send_draft`, without drafts one closing `edit_message`.
- `steer`: works through the global key and through `/steer`, not through the per-platform key.
- `credential_type`: no public surface; the internal resolution is right in every profile.
- `interrupt_outcome`: `/stop` via `handle_message` returns `CANCELLED`.
- `session_switch`: `/new` with confirmation, `/resume` in the thread, foreign sessions only with
  `--all` for an admin sender; the session ID comes from the hook `pre_llm_call`.
- `own_access_policy`: `enforces_own_access_policy` with `dm_policy = "allowlist"` works.

The recording is part of the normal test run (`make test`, every `make check` that selects
`plugins`). It needs the network for the pinned downloads (Hermes commit, Python packages,
Python) and fails without a network; it never skips.

Record again (after a new pin or a change of the probe, the scenarios or the harness):

```
make -C plugins/hermes-adapter record
```

Then review the diff in `tests/fixtures/recording/` and update 09 17.10.

## Layout of the tests

| Part | Content |
|---|---|
| `tests/hermes_pin.toml` | pinned Hermes version (tag, full commit, Python line, kind of installation) |
| `tests/hermes_harness/` | harness: pin, provisioning, integrity, throwaway homes and environment, sandbox (macOS), egress recorder, fake model, control channel, gateway process group, normalization, observations, scenario run, fixtures, named time limits |
| `tests/recording/probe_plugin/` | probe platform `recording_probe`; the only code with a Hermes import |
| `tests/recording/scenarios/` | scenarios per item of 17.10 |
| `tests/recording/test_recording.py` | live recording against the pinned version |
| `tests/fixtures/recording/` | evidence: one fixture per scenario and `observations.json` |
| `tests/unit/` | tests without network and without Hermes, also for the checked-in evidence |

## What the harness guarantees and what it does not

- The checkout of the pinned commit lies read-only in a throwaway directory outside the
  repository; the installation is made from a `git archive` export into a throwaway environment.
  Checkout and installed files are unchanged after the run, otherwise it fails.
- Every Hermes process gets an environment from an allow-list, its own homes, an empty keyring,
  refusing programs for the keychain and services, and a proxy that rejects everything with `403`
  and only notes the target. Model and login are fakes.
- On macOS the Hermes process tree also runs in `sandbox-exec`: no network except the loopback
  ports of the harness's own servers, no signals and process information outside, no reading of
  file contents and no writing in the real home except in the throwaway directory.
- On Linux there is no network sandbox; the guarantee rests on no real login being reachable. A
  request that slipped through would carry only fake logins, which cost nothing at a real
  provider.
- The harness opens only ports on `127.0.0.1` with port 0 and ends only its own process group,
  never processes by name.

## What M2-33 takes over

M2-33 builds the adapter on this harness: `load_pin`, `provision` and `release`,
`ThrowawayHomes`, `build_environment`, the sandbox, `EgressRecorder`, `FakeModel` with
`FakeCredentials`, `HermesGateway`, `snapshot_checkout`, `installed_digest` and
`assert_unchanged`. The adapter then runs instead of the probe; pin, isolation and the integrity
check stay.
