# Bothy

**A command center for AI agents in chat form.**

| Status | License | Start here |
|---|---|---|
| planning, no product code yet | [Apache-2.0](LICENSE) | [Specification](docs/SPEC.md) · [Roadmap](docs/roadmap.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) |

## What is Bothy

Bothy (working name) is a command center for AI agents in chat form: chat, live view, grants,
terminal, costs and statistics in one open-source app that you host yourself.

**In one sentence:** your team and your agents work in one place, where you watch each agent run
live, answer its questions, approve its actions, open a terminal on your own hosts and see what
it costs, and every person sees only what is shared with them.

- **Chat like Discord.** Spaces, channels, forums and threads; agents such as
  [Hermes Agent](https://github.com/NousResearch/hermes-agent), Claude Code and your own are full
  participants through the public API.
- **Live view and approvals.** Every thread can be an agent session with a live box of tool calls
  and status lines, questions and approval buttons.
- **Terminal.** Shells and coding-agent sessions on connected hosts, from desktop, phone and
  web console, end-to-end encrypted, with a capability per host and a fresh confirmation on every open.
- **Costs and statistics.** Each agent shows what it runs on and what it costs; native views show
  live agent activity, delivery and spend, and a Grafana panel plugin shows the same for instance operators.
- **Your instance.** One binary, PostgreSQL and object storage, on your own hardware or in your
  own cloud; every permission is checked by the server.

<picture>
  <source media="(prefers-color-scheme: light)" srcset="docs/design/previews/layout-light.png">
  <img src="docs/design/previews/layout-dark.png" alt="Desktop design preview: sidebar with spaces and channels, a forum list of agent threads, a thread with a running agent in the live box showing tool calls, tokens and cost, an approval card with allow and deny buttons, and a session panel with backend, context and earlier sessions">
</picture>

<img src="docs/design/previews/mobile-dark.png" width="420" alt="Mobile design preview: the space list with running agents on the left, and on the right a thread with a running agent, an approval card and a queued follow-up while offline">

Design previews from the [design system](docs/design/README.md) (petrol with a yellow accent,
dark and light theme); screenshots of the real apps follow with the demo instance (plan, stage M7).

## Quick start

Planned: one command on any Linux or macOS host with Docker Compose. There is no code yet, so
nothing installs today. The setup wizard checks the host, downloads the signed bundle of its
version, asks for the public address, the edge (reverse proxy or Tailscale) and the first owner,
starts the stack and prints the one-time enrollment code for the app
([specification](docs/spec/16-operations.md), section 3.1; plan ticket M7-29):

```sh
# planned; published with the first release (OWNER is replaced before the repository goes public)
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

The [Compose guide](docs/operations/install-compose.md) describes the same install step by step
and is the contract the first release has to meet.

---

## Overview

Bothy is a command center for AI agents in chat form: chat, live view, grants, terminal, costs
and statistics in one self-hosted, open-source app. Agents such as Hermes Agent, Claude Code and
others are full participants. Every person sees and controls only what has been granted to them;
the server checks this on every access. What the product does and what it will never do is in the
[specification](docs/SPEC.md).

## Status

Planning (milestone M0). There is no product code yet. The planning documents are complete:
specification, architecture decisions, build plan with build briefs, design system with preview
images, configurations for operations and CI, and the toolchain with all gates. M0 closes with a
red team over the planning state and the last steps before the first push
([PLAN](docs/PLAN.md), "M0 Planning (closing)"). M1 Foundation follows
([Roadmap](docs/roadmap.md)).

## Reading order

1. This README.
2. [SPEC](docs/SPEC.md): goals, invariants, chapter index, reading paths per role.
3. Chapters [01 Product](docs/spec/01-product.md) and [02 Architecture](docs/spec/02-architecture.md).
4. [PLAN](docs/PLAN.md): milestones and build tickets.
5. [DECISIONS](docs/DECISIONS.md): ruling log.
6. [OPEN-QUESTIONS](docs/OPEN-QUESTIONS.md): questions to the owner with a default.
7. [RISKS](docs/RISKS.md): product risks.
8. [ADRs](docs/adr/README.md): architecture decisions.
9. [Design](docs/design/README.md): design system.
10. [Quality](docs/quality/README.md): gates and tools.

## Repo structure

```text
.github/     workflows, templates for issues and pull requests, labels
apps/        Flutter apps: client (iOS, Android, macOS, Windows) and console (web); no code yet
design/      source of the design tokens, generated outputs (CSS, Dart) and preview page
docs/        specification, plan, build briefs, ADRs, API contracts, architecture, operations, quality
infra/       Compose installation, collector, monitoring, OpenTofu and Helm skeleton
packages/    shared Dart packages of the apps (design system, sync, terminal, visualization); no code yet
plugins/     reference adapters for Hermes Agent and the Claude Code runner, Grafana plugin; no code yet
quality/     SonarQube profiles and gate for local and cloud, list of allowed licenses
sdk/         SDKs for Python and TypeScript; no code yet
services/    Go server (one binary, roles api and worker) and host service bothy-host; no code yet
tools/       gates, check scripts and generators of the toolchain, pinned
```

## Local gates

```sh
make bootstrap   # pinned tools and Git hooks, once per clone
make check       # every gate, exactly like CI
```

`make docs` builds the docs site in strict mode, `make linkcheck` checks all links and anchors.
What each gate checks is in [docs/quality/gates.md](docs/quality/gates.md); contributions from
forks set the switches from [CONTRIBUTING](CONTRIBUTING.md).

## License

[Apache-2.0](LICENSE), contributions with DCO instead of CLA
([ADR 0003](docs/adr/0003-license-apache-2-0-with-dco-instead-of-cla.md)). Notices: [NOTICE](NOTICE).
