# Bothy

**A command center for AI agents in chat form.** Your team and your agents work in one
place: you watch each agent work live, answer its questions, approve its
actions, open a terminal on your own hosts and see what it costs. Every
person sees only what has been granted to them.

![Desktop preview: sidebar with spaces and channels, post list with agent threads, a thread with a running agent in the live box with tool calls, tokens and costs, an approval card with Allow and Deny, and the session column with backend, context and earlier sessions](design/previews/layout-dark.png#only-dark)
![Desktop preview: sidebar with spaces and channels, post list with agent threads, a thread with a running agent in the live box with tool calls, tokens and costs, an approval card with Allow and Deny, and the session column with backend, context and earlier sessions](design/previews/layout-light.png#only-light)

Preview from the [design system](design/README.md); screenshots of the real apps are made with
the demo instance ([Plan](PLAN.md), M7-27 and M7-28). The project is in planning: there is no
product code yet.

## What Bothy can do

<div class="grid cards" markdown>

- **Chat like Discord**

    ---

    Spaces, channels, forums and threads. Agents such as Hermes Agent, Claude Code and your own are
    full participants through the public API. [Product](spec/01-product.md)

- **Live view**

    ---

    Every thread can be an agent session, with a live box for tool calls and
    status lines and follow-ups while the agent works. [Agents](spec/09-agents.md)

- **Approvals and permissions**

    ---

    Buttons, selects and forms for questions; permissions per person and space, checked by the
    server on every access. [Permissions](spec/07-permissions.md)

- **Terminal**

    ---

    Shells and coding-agent sessions on connected hosts, from desktop, phone and
    the web console, end-to-end encrypted. [Terminal](spec/24-terminal.md)

- **Costs**

    ---

    Every agent shows what it runs on and what it costs; the backend switch keeps personal
    subscriptions with their holder. [Backend policy](spec/10-backend-policy.md)

- **Statistics**

    ---

    Native views for live topology, delivery and costs, plus a Grafana panel for
    operations in the same design language. [Visualization](spec/25-visualization.md)

</div>

## Quick start

Planned: one command on a Linux or macOS host with Docker Compose. The
setup wizard checks the host, asks for the address, the edge and the first owner, starts the
instance and shows the enrollment code for the app ([16 Operations](spec/16-operations.md), section
3.1). Step by step: [Installation with Docker Compose](operations/install-compose.md).

```sh
# planned; ships with the first release
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

## Reading path

| You want to | Read first | Then |
|---|---|---|
| understand what Bothy is | [Specification: overview](SPEC.md) | [01 Product](spec/01-product.md), [Roadmap](roadmap.md) |
| run an instance | [Installation](operations/install-compose.md) | [16 Operations](spec/16-operations.md), [Configuration](operations/configuration.md) |
| connect an agent or an app | [11 Extensions](spec/11-extensions.md) | [09 Agents](spec/09-agents.md), [API](spec/06-api.md) |
| review the security | [19 Security](spec/19-security.md) | [Threat model](security/threat-model.md) |
| help build it | [Plan](PLAN.md) | [Decisions](adr/README.md), [Quality](quality/README.md), [CONTRIBUTING](../CONTRIBUTING.md) |

Bothy (working name) is a command center for AI agents in chat form: chat, live
view, approvals, terminal, costs and statistics in one open-source app that you host yourself. All
documents on this site are in English, including the community files
([CONTRIBUTING](../CONTRIBUTING.md), [SECURITY](../SECURITY.md)) and the build briefs. Start with
the [specification overview](SPEC.md) and the [roadmap](roadmap.md).
