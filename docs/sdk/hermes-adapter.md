# Hermes adapter

Reference of the Hermes adapter `bothy_hermes_adapter` (docs/spec/09-agents.md section 17).
`make docs` renders it from the google docstrings of the source with mkdocstrings. Installation,
configuration, the billing-mode switch and how to run the tests are in
[plugins/hermes-adapter/README.md](../../plugins/hermes-adapter/README.md); the rulings are
`D-M2-33-*` in the decision log.

## Layers

Only `platform` (the Hermes platform plugin) and `hermes_config` (Hermes' own configuration
reader and writer, the credential resolution and the enabled toolsets) import Hermes. Every
other module depends on the protocols of `runtime` and runs without Hermes; the unit tests fake
the runtime, and the Fake-Hermes tests run the pinned Hermes in a subprocess.

| Module | Content |
|---|---|
| `runtime` | The seam between the logic and Hermes: `Runtime` and `RuntimeEvents`. |
| `config` | The configuration from `PlatformConfig.extra` and the token file. |
| `actual_mode` | The actual billing mode from the kind of credential Hermes resolves. |
| `tool_class` | The actual tool class from the enabled toolsets. |
| `context_format` | The run context as the fixed system part with data blocks. |
| `live_box` | Steps, status line and draft of a running turn. |
| `cards` | Cards for clarify, approval and slash confirmations. |
| `store` | Resolved callbacks and open cards. |
| `heartbeat` | The heartbeat with the Hermes report. |
| `handler` | The runtime side of the SDK loop. |
| `installer` | Install into a profile and switch the billing mode. |
| `entry` | The plugin's `register(ctx)`; imports neither Hermes nor the SDK when Hermes loads it. |
| `defaults` | Defaults of the configuration keys. |

::: bothy_hermes_adapter.runtime

::: bothy_hermes_adapter.config

::: bothy_hermes_adapter.actual_mode

::: bothy_hermes_adapter.tool_class

::: bothy_hermes_adapter.context_format

::: bothy_hermes_adapter.live_box

::: bothy_hermes_adapter.cards

::: bothy_hermes_adapter.store

::: bothy_hermes_adapter.heartbeat

::: bothy_hermes_adapter.handler

::: bothy_hermes_adapter.installer

::: bothy_hermes_adapter.entry

::: bothy_hermes_adapter.defaults
