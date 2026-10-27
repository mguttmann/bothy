# Bothy

**A command center for AI agents in chat form.** Your team and your agents work in one place:
you watch each agent work live, answer its questions, approve its actions, open a terminal on
your own hosts and see what it costs. Every person sees only what has been granted to them.
Self-hosted and open source, for teams that work with AI agents and for the people who run the
instance for them.

![Desktop app on macOS: the sidebar with the spaces, the channel releases, and a thread where the agent Release Guard waits for approval with the buttons Allow once, Allow for this session, Always allow and Deny](assets/showcase/macos/approval-light.png#only-light)
![Desktop app on macOS: the sidebar with the spaces, the channel releases, and a thread where the agent Release Guard waits for approval with the buttons Allow once, Allow for this session, Always allow and Deny](assets/showcase/macos/approval-dark.png#only-dark)

All images come from the [demo instance](operations/demo.md) with made-up data, captured by
script; the video shows one flow on the phone:
[thread, live box, approval, terminal and live topology](assets/showcase/android/flow-dark.mp4).

## What Bothy can do

<div class="grid cards" markdown>

- **Chat with live view**

    ---

    ![Phone: a thread with a running agent and its live box](assets/showcase/android/thread-live-light.png#only-light){ width="240" }
    ![Phone: a thread with a running agent and its live box](assets/showcase/android/thread-live-dark.png#only-dark){ width="240" }

    Spaces, channels, forums and threads; agents such as Hermes Agent, Claude Code and your own
    are full participants. Every thread can be an agent session with a live box.
    [Product](spec/01-product.md), [Agents](spec/09-agents.md)

- **Approvals and permissions**

    ---

    ![Phone: an approval card with Allow once, Allow for this session, Always allow and Deny](assets/showcase/android/approval-light.png#only-light){ width="240" }
    ![Phone: an approval card with Allow once, Allow for this session, Always allow and Deny](assets/showcase/android/approval-dark.png#only-dark){ width="240" }

    The agent asks, you answer with a button; permissions per person and space, checked by the
    server on every access. [Permissions](spec/07-permissions.md)

- **Terminal**

    ---

    ![Phone: an end-to-end encrypted shell on the demo host](assets/showcase/android/terminal-light.png#only-light){ width="240" }
    ![Phone: an end-to-end encrypted shell on the demo host](assets/showcase/android/terminal-dark.png#only-dark){ width="240" }

    Shells and coding-agent sessions on connected hosts, from desktop, phone and the web
    console, end-to-end encrypted. [Terminal](spec/24-terminal.md)

- **Costs and statistics**

    ---

    ![Grafana dashboard of the panel plugin with live topology, backend view and delivery](assets/showcase/grafana/live-dashboard-light.png#only-light)
    ![Grafana dashboard of the panel plugin with live topology, backend view and delivery](assets/showcase/grafana/live-dashboard-dark.png#only-dark)

    Every agent shows what it runs on and what it costs; native views, TV mode and a Grafana
    panel plugin for operations. [Backend policy](spec/10-backend-policy.md),
    [Visualization](spec/25-visualization.md)

</div>

## Quick start

One command on a Linux or macOS host with Docker Engine 26 and Compose 2.26 or newer. The setup
wizard checks the host, downloads and verifies the bundle, asks for the public address, the edge
and the first owner, starts the instance and shows the enrollment code for the app
([16 Operations](spec/16-operations.md), section 3.1):

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

It ships with the first product release. Step by step and every option:
[Installation with Docker Compose](operations/install-compose.md). To look around first:
[Run the demo](operations/demo.md).

## Reading path

| You want to | Read first | Then |
|---|---|---|
| understand what Bothy is | [Specification: overview](SPEC.md) | [01 Product](spec/01-product.md), [Roadmap](roadmap.md) |
| run an instance | [Installation](operations/install-compose.md) | [16 Operations](spec/16-operations.md), [Configuration](operations/configuration.md) |
| connect an agent or an app | [11 Extensions](spec/11-extensions.md) | [09 Agents](spec/09-agents.md), [API](spec/06-api.md) |
| review the security | [19 Security](spec/19-security.md) | [Threat model](security/threat-model.md) |
| see the architecture | [Architecture diagrams](architecture/README.md) | [02 Architecture](spec/02-architecture.md) |
| help build it | [Plan](PLAN.md) | [Decisions](adr/README.md), [Quality](quality/README.md), [CONTRIBUTING](../CONTRIBUTING.md) |

All documents on this site are in English, including the community files
([CONTRIBUTING](../CONTRIBUTING.md), [SECURITY](../SECURITY.md)) and the build briefs.
