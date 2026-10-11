# hermes-adapter

Status: planned. This folder contains no code yet.

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

- Only public Hermes plugin interfaces; no change to Hermes.
- Never change the platform key (`hermes_adapter.platform_key`) after commissioning; otherwise
  every thread loses its Hermes session.
- `busy_input_mode` explicitly `steer` or `queue`; with `interrupt` the adapter reports
  `degraded`.
- No start without a valid invocation token; detect the reported mode only from the key type,
  never read, store or send the value.

## Planned structure

| Part | Content |
|---|---|
| `plugin.yaml` | manifest of the Hermes plugin |
| Adapter module | the only module with a Hermes import; subclass of `BasePlatformAdapter` |
| Logic modules | mapping, context format, allow-list; without a Hermes import, testable on their own |
| Installer | writes the plugin configuration and the profile settings, checks them with Hermes' own YAML reader before and after |
| Tests | logic tests; fake Hermes tests in a subprocess with a pinned Hermes version, a throwaway `HERMES_HOME` and a fake model endpoint |

## To check before the build

See docs/spec/09-agents.md section 17.10.
