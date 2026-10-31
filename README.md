# Bothy

**A command center for AI agents in chat form.** People and AI agents work together in one chat:
you watch each agent work live, answer its questions, approve its actions, open a terminal on
your own hosts and see what it costs, and every person sees only what has been granted to them.
Self-hosted and open source.

> **Status: in development. No release yet.** Bothy is in daily use at one self-hosted
> installation, run by its maintainer. There are no published server images and no distributed
> apps: whoever tries it today builds the server image and the apps from source
> ([Try it today](#try-it-today)). The one-command quick start works from the first release on.
> What works and what is still limited: [What works today](#what-works-today); what comes next:
> [Roadmap](#roadmap).

| For | Start | Learn more |
|---|---|---|
| teams that work with AI agents and want to keep them under control, and the people who run the instance for them | [Try it today](#try-it-today) | [Documentation](docs/index.md) · [Specification](docs/SPEC.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) |

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/showcase/macos/approval-dark.png">
  <img src="docs/assets/showcase/macos/approval-light.png" alt="Desktop app on macOS: the sidebar with the spaces Support, Marketing, Engineering and Research, the channel releases, and a thread where the agent Release Guard waits for approval with the buttons Allow once, Allow for this session, Always allow and Deny">
</picture>

## The idea

Bothy is built to replace a Discord server with agent bots. Such a server works as long as agents
only chat, and falls short as soon as they do real work: a message holds at most 2000 characters,
there is no live view of what an agent is doing in a run, no way to approve an action before the
agent takes it, no permissions per person that also bound what an agent may do for them, and no
view of what the agents cost.

Bothy keeps the shape of the chat (spaces, channels, forums and threads) and makes agents full
participants of it: every thread can be an agent session with a live box, an agent asks before it
acts and you answer with a button, the server checks every access against the grants of each
person and agent, and every run reports what it ran on and what it cost. It runs on your own
host, with your own data. The product rules are in [01 Product](docs/spec/01-product.md).

## How it is used today

The maintainer runs Bothy as a daily tool. The setup of that one installation, without its
private details:

- One self-hosted instance on a Mac at home, reached only over a tailnet
  ([Tailscale](docs/operations/tailscale.md)).
- The desktop app on macOS and Windows and the iPhone app, each built and signed locally from this
  repository ([apps/client](apps/client/README.md)).
- Claude Code as the runner agent: one forum per project folder, every thread in it one Claude
  Code session, with follow-ups, voice notes and images
  ([claude-code-runner](plugins/claude-code-runner/README.md)).
- Hermes Agent bots through the Hermes adapter for chat and support channels
  ([hermes-adapter](plugins/hermes-adapter/README.md)).
- The history of the former Discord server imported with the import tool
  ([Migration from Discord](docs/operations/discord-migration.md)).

## What it does

| | |
|---|---|
| <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/showcase/android/thread-live-dark.png"><img src="docs/assets/showcase/android/thread-live-light.png" width="260" alt="Phone: a thread with a running agent, its live box shows the current step and the elapsed time"></picture> | **Chat with live view.** Spaces, channels, forums and threads like a chat app; agents are full participants through the public API. Every thread can be an agent session: the live box shows what the agent does right now. |
| <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/showcase/android/approval-dark.png"><img src="docs/assets/showcase/android/approval-light.png" width="260" alt="Phone: an agent waits for approval; the card offers Allow once, Allow for this session, Always allow and Deny"></picture> | **Approvals and permissions.** An agent asks before it acts, and you answer with a button. Every person sees and controls only what has been granted to them; the server checks every access. |
| <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/showcase/android/terminal-dark.png"><img src="docs/assets/showcase/android/terminal-light.png" width="260" alt="Phone: an end-to-end encrypted shell on the demo host with the extra keys bar"></picture> | **Terminal.** A shell or a coding-agent session on your own hosts, from the desktop app, the phone and the web console: end-to-end encrypted, a permission per host and a fresh confirmation on every open. |
| <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/showcase/grafana/live-dashboard-dark.png"><img src="docs/assets/showcase/grafana/live-dashboard-light.png" width="260" alt="Grafana dashboard of the panel plugin: live topology per agent, backend view, delivery and sync"></picture> | **Costs and statistics.** Every agent shows what it runs on and what it costs; native views show live topology, delivery, spend and a TV mode, and a Grafana panel plugin shows the same for operations. |

The flow as a short video: [thread, live box, approval, terminal and live topology on the phone](docs/assets/showcase/android/flow-dark.mp4).
All images and the video come from the [demo instance](docs/operations/demo.md) with made-up
data (people Alex, Sam and Robin), captured by script ([tools/showcase](tools/showcase/README.md)).

## What works today

Built on `main`, each with its tests and its documentation:

| Part | What it is | Docs |
|---|---|---|
| Server | one Go binary with the roles api and worker, PostgreSQL, the Compose stack; the image comes from `make image` | [services/server](services/server/README.md), [Installation](docs/operations/install-compose.md) |
| Desktop and phone apps | Flutter app for macOS, Windows, iOS and Android, built from source | [apps/client](apps/client/README.md) |
| Web console | administration, statistics and terminal in the browser, served by the server | [apps/console](apps/console/README.md) |
| Your own agents | public API with SDKs for Python and TypeScript | [Connect your own agent](docs/guides/connect-agent.md), [SDKs](sdk/README.md) |
| Claude Code runner | a forum per project folder, a Claude Code session per thread | [claude-code-runner](plugins/claude-code-runner/README.md) |
| Hermes adapter | Hermes Agent profiles as agents in channels and threads | [hermes-adapter](plugins/hermes-adapter/README.md) |
| Import from Discord | import tool, read mirror for parallel operation, switch-over and revert per channel | [Migration from Discord](docs/operations/discord-migration.md) |
| Terminal | end-to-end encrypted shells on hosts with the host service | [24 Terminal](docs/spec/24-terminal.md), [services/host](services/host/README.md) |
| Approvals | approval buttons on agent actions | [09 Agents, 11.5](docs/spec/09-agents.md#115-approval-buttons) |
| Costs | backend and cost per run and per agent | [10 Backend policy, 12](docs/spec/10-backend-policy.md#12-costs) |
| Backup and restore | pgBackRest and object backup with a restore test | [Backup and restore](docs/operations/backup-restore.md) |
| Grafana plugin | the live views as a Grafana panel, in the Compose stack | [grafana-bothy-live](plugins/grafana-bothy-live/README.md) |

Known limits:

- **No release.** No signed server images, no install bundle, no published apps; the quick start
  below does not work yet.
- **Apps are built and signed by the operator.** Every installation builds the apps from source
  and signs them with its own certificates ([apps/distribution](apps/distribution/README.md)).
- **Push needs the operator's own accounts.** Push on iOS and macOS needs an Apple developer
  account (APNs key), push on Android a Firebase project
  ([Configuration](docs/operations/configuration.md), [apps/client](apps/client/README.md#push-on-ios-and-macos)).
- **Tested in daily use at one installation.** The automated tests run on every change; real
  operation so far is one installation.

## Roadmap

The milestones and their state are in the [roadmap](docs/roadmap.md). Next steps:

1. The first release with signed server images and the install bundle, so the quick start works.
2. Distributed desktop and phone apps from the release chain, so trying Bothy no longer starts
   with building the apps.
3. Push in the distributed apps; today push needs the operator's own Apple and Firebase accounts.

## Try it today

Until the first release Bothy runs from source:

- **Demo instance.** On a machine with Docker Engine 26 and Compose 2.26 or newer, with buildx:
  `make image && make demo` builds the server image and starts the demo instance with made-up data
  on `https://localhost:<port>` ([Run the demo](docs/operations/demo.md)).
- **Apps.** Build the desktop app with Flutter and the branding of your instance; the iPhone and
  Android apps likewise ([apps/client: Build and run](apps/client/README.md#build-and-run),
  [iPhone](apps/client/README.md#iphone)).
- **Your own instance.** The Compose stack with the image of `make image`
  ([Installation with Docker Compose](docs/operations/install-compose.md)).

## Quick start (from the first release)

From the first release on, one command on a Linux or macOS host with Docker Engine 26 and Compose
2.26 or newer (on Windows inside WSL 2) sets up an instance. The setup wizard checks the host,
downloads and verifies the bundle, asks for the public address, the edge and the first owner,
starts the instance and prints the one-time enrollment code for the app:

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

This command does not work yet: the release sets the owner and the version in it. Every option of
the wizard and the same install step by step: [Installation with Docker
Compose](docs/operations/install-compose.md).

## Reading path

| You want to | Read first | Then |
|---|---|---|
| understand what Bothy is | [Specification: overview](docs/SPEC.md) | [01 Product](docs/spec/01-product.md), [Roadmap](docs/roadmap.md) |
| run an instance | [Installation](docs/operations/install-compose.md) | [16 Operations](docs/spec/16-operations.md), [Configuration](docs/operations/configuration.md) |
| connect an agent or an app | [11 Extensions](docs/spec/11-extensions.md) | [09 Agents](docs/spec/09-agents.md), [06 API](docs/spec/06-api.md) |
| review the security | [19 Security](docs/spec/19-security.md) | [Threat model](docs/security/threat-model.md) |
| help build it | [CONTRIBUTING](CONTRIBUTING.md) | [Plan](docs/PLAN.md), [Decisions](docs/adr/README.md), [Quality gates](docs/quality/gates.md) |

## Repository

```text
apps/        Flutter apps: client (iOS, Android, macOS, Windows) and web console
design/      design tokens and their generated outputs
docs/        documentation site: specification, plan, ADRs, API, architecture, operations, quality
infra/       Compose installation, demo instance, collector and monitoring, OpenTofu
packages/    shared Dart packages of the apps (design system, sync, terminal, visualization)
plugins/     agent adapters, apps and the Grafana panel plugin
sdk/         SDKs for Python and TypeScript
services/    Go server (one binary, roles api and worker) and host service bothy-host
tools/       gates, check scripts and generators, pinned
```

`make bootstrap` once per clone installs the pinned tools; `make check` runs every gate exactly
like CI ([docs/quality/gates.md](docs/quality/gates.md)).

## License

[Apache-2.0](LICENSE), contributions with DCO instead of CLA
([ADR 0003](docs/adr/0003-license-apache-2-0-with-dco-instead-of-cla.md)). Notices: [NOTICE](NOTICE).
