# plugins

Status: in progress. `hermes-adapter/`, `claude-code-runner/` and `grafana-bothy-live/` hold code (see the table).

## Purpose

Reference adapters and reference apps. Every program here runs outside the server and uses only
the public API (ADR 0022). Nothing in this folder is linked into the server or loaded by
it.

## Contents

| Folder | Kind | Purpose | State |
|---|---|---|---|
| `hermes-adapter/` | agent adapter | platform plugin for the open-source framework Hermes Agent | recording present (M2-32), adapter planned (M2-33) |
| `claude-code-runner/` | agent adapter | starts Claude Code sessions per thread in dev forums | core (M2-34); follow-ups, stop and error paths M2-35, answer attachments M3-12 ([README](claude-code-runner/README.md)) |
| `grafana-bothy-live/` | Grafana panel plugin | views of the live statistics for operations from OTel metrics (ID `bothy-live-panel`, [25-visualization.md](../docs/spec/25-visualization.md), section 8) | built (M7-25), unsigned; provisioning and release ZIP M7-26 ([README](grafana-bothy-live/README.md)) |
| `github-app/` | app | reference app for code hosting from docs/spec/11-extensions.md, section 10.1 (TypeScript SDK) | planned (M2-49), the folder is created with the build ticket |
| `jira-app/` | app | reference app for a ticket system from docs/spec/11-extensions.md, section 10.2 (Python SDK) | planned (M2-50), the folder is created with the build ticket |

## Rules for all programs here

- Contract: docs/spec/09-agents.md (agents) or docs/spec/11-extensions.md (apps).
- Only through the SDK (`sdk/`) or the public API; no server-internal packages.
- No secrets in the repo, no credentials in logs; configuration through the environment or a
  file, secrets through the secret provider of the respective program.
- Own version number (SemVer) and own changelog per folder.
- Tests with the test doubles from docs/spec/09-agents.md section 16; real model calls only
  behind an explicit switch.
- White label: no company, person or host names; examples with example.com.

Exception `grafana-bothy-live/`: the panel plugin runs in Grafana, not as a client of the API. It
never talks to the server and reads only Grafana data sources; its contract is
docs/spec/25-visualization.md, section 8. The rules on secrets, version and white label
apply there as well.
