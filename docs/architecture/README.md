# Architecture diagrams

All diagrams are Mermaid sources (`.mmd`) with two rendered SVGs next to them, `<name>.svg` for
the light and `<name>.dark.svg` for the dark color scheme. The source is authoritative; the SVGs
are regenerated on every change. Text and rules for them are in
[../spec/02-architecture.md](../spec/02-architecture.md). Names of tables, events and actions
come from the core contract ([00](../spec/00-glossary.md), [03](../spec/03-data-model.md),
[04](../spec/04-sync-protocol.md), [05](../spec/05-actions.md)).

## Index

### C4

| Diagram | Content | Source | Image |
|---|---|---|---|
| Context | An instance with people and external systems, the protocol for each relationship. | [c4-1-context.mmd](c4-1-context.mmd) | [light](c4-1-context.svg), [dark](c4-1-context.dark.svg) |
| Container | Roles `api` and `worker` of the binary, PostgreSQL, object storage, clamd, Collector, apps, adapters, host service. | [c4-2-container.mmd](c4-2-container.mmd) | [light](c4-2-container.svg), [dark](c4-2-container.dark.svg) |
| Server components | Modules of the monolith with all allowed dependencies according to 02-architecture.md, 4.4. | [c4-3-server-components.mmd](c4-3-server-components.mmd) | [light](c4-3-server-components.svg), [dark](c4-3-server-components.dark.svg) |

### Flows in the server

| Diagram | Content | Source | Image |
|---|---|---|---|
| Request path | Middleware chain of a REST request up to the proof check of `authorize`. | [flow-request-path.mmd](flow-request-path.mmd) | [light](flow-request-path.svg), [dark](flow-request-path.dark.svg) |
| Event path | Unit of work with phases, `event_log`, log tail, hub, agent and worker consumers. | [flow-event-path.mmd](flow-event-path.mmd) | [light](flow-event-path.svg), [dark](flow-event-path.dark.svg) |

### Sequences

| Diagram | Content | Source | Image |
|---|---|---|---|
| Send a message | Sending with `client_msg_id`, delivery to connected clients, push to a device without a connection. | [seq-1-send-message.mmd](seq-1-send-message.mmd) | [light](seq-1-send-message.svg), [dark](seq-1-send-message.dark.svg) |
| Agent run | Triggering, I2 and invocation token on delivery, run start via compare-and-swap, live box patches, follow-up, completion. | [seq-2-agent-run.mmd](seq-2-agent-run.mmd) | [light](seq-2-agent-run.svg), [dark](seq-2-agent-run.dark.svg) |
| Reconnect | `resume` or `identify` on another node, catch-up with a buffer, `resync_required`. | [seq-3-reconnect.mmd](seq-3-reconnect.mmd) | [light](seq-3-reconnect.svg), [dark](seq-3-reconnect.dark.svg) |
| Revoke a grant | Revocation with an open connection across two nodes, cancellation of the active run. | [seq-4-revoke-grant.mmd](seq-4-revoke-grant.mmd) | [light](seq-4-revoke-grant.svg), [dark](seq-4-revoke-grant.dark.svg) |
| Switch the backend mode | Switching to `personal` with step-up, I1 check, configured and reported mode at the adapter. | [seq-5-backend-mode.mmd](seq-5-backend-mode.mmd) | [light](seq-5-backend-mode.svg), [dark](seq-5-backend-mode.dark.svg) |
| Push with extension | Push without content, the Notification Service Extension fetches the message through the API. | [seq-6-push-nse.mmd](seq-6-push-nse.mmd) | [light](seq-6-push-nse.svg), [dark](seq-6-push-nse.dark.svg) |
| File upload | Upload session, upload directly or through a presigned URL, scan with clamd, thumbnail. | [seq-7-file-upload.mmd](seq-7-file-upload.mmd) | [light](seq-7-file-upload.svg), [dark](seq-7-file-upload.dark.svg) |
| Open a terminal | Step-up, `terminal.open` with SR10, I6 and SR12, terminal session token, Noise `NK` through the relay, PTY only after `start`. | [seq-8-open-terminal.mmd](seq-8-open-terminal.mmd) | [light](seq-8-open-terminal.svg), [dark](seq-8-open-terminal.dark.svg) |

### Data model (ERD, from 03-data-model.md, split by module)

| Diagram | Content | Source | Image |
|---|---|---|---|
| Instance and identity | Modules `instance` and `identity`. | [erd-1-instance-identity.mmd](erd-1-instance-identity.mmd) | [light](erd-1-instance-identity.svg), [dark](erd-1-instance-identity.dark.svg) |
| Permissions and structure | Modules `authz` and `chat`. | [erd-2-permissions-structure.mmd](erd-2-permissions-structure.mmd) | [light](erd-2-permissions-structure.svg), [dark](erd-2-permissions-structure.dark.svg) |
| Agents and apps | Modules `agents` and `apps`. | [erd-3-agents-apps.mmd](erd-3-agents-apps.mmd) | [light](erd-3-agents-apps.svg), [dark](erd-3-agents-apps.dark.svg) |
| Messages and files | Modules `messaging` and `files`. | [erd-4-messages-files.mmd](erd-4-messages-files.mmd) | [light](erd-4-messages-files.svg), [dark](erd-4-messages-files.dark.svg) |
| Runs and costs | Modules `runs` and `billing`. | [erd-5-runs-costs.mmd](erd-5-runs-costs.mmd) | [light](erd-5-runs-costs.svg), [dark](erd-5-runs-costs.dark.svg) |
| Sync | Module `sync`: event log, idempotency, read state, drafts, settings. | [erd-6-sync.mmd](erd-6-sync.mmd) | [light](erd-6-sync.svg), [dark](erd-6-sync.dark.svg) |
| Terminal | Module `terminal`: hosts and terminal sessions. | [erd-7-terminal.mmd](erd-7-terminal.mmd) | [light](erd-7-terminal.svg), [dark](erd-7-terminal.dark.svg) |
| Audit and privacy | Modules `audit` and `privacy`. | [erd-8-audit-privacy.mmd](erd-8-audit-privacy.mmd) | [light](erd-8-audit-privacy.svg), [dark](erd-8-audit-privacy.dark.svg) |
| Import, status, statistics | Modules `importer`, `status` and `stats`. | [erd-9-import-status-statistics.mmd](erd-9-import-status-statistics.mmd) | [light](erd-9-import-status-statistics.svg), [dark](erd-9-import-status-statistics.dark.svg) |
| Notification | Module `notify`. | [erd-10-notification.mmd](erd-10-notification.mmd) | [light](erd-10-notification.svg), [dark](erd-10-notification.dark.svg) |

## Gallery

Each graphic in the color scheme of the page: the light rendering (`<name>.svg`) in the light
scheme, the dark rendering (`<name>.dark.svg`) in the dark scheme.

### Context

![Context](c4-1-context.svg#only-light)
![Context](c4-1-context.dark.svg#only-dark)

### Container

![Container](c4-2-container.svg#only-light)
![Container](c4-2-container.dark.svg#only-dark)

### Server components

![Server components](c4-3-server-components.svg#only-light)
![Server components](c4-3-server-components.dark.svg#only-dark)

### Request path

![Request path](flow-request-path.svg#only-light)
![Request path](flow-request-path.dark.svg#only-dark)

### Event path

![Event path](flow-event-path.svg#only-light)
![Event path](flow-event-path.dark.svg#only-dark)

### Send a message

![Send a message](seq-1-send-message.svg#only-light)
![Send a message](seq-1-send-message.dark.svg#only-dark)

### Agent run

![Agent run](seq-2-agent-run.svg#only-light)
![Agent run](seq-2-agent-run.dark.svg#only-dark)

### Reconnect

![Reconnect](seq-3-reconnect.svg#only-light)
![Reconnect](seq-3-reconnect.dark.svg#only-dark)

### Revoke a grant

![Revoke a grant](seq-4-revoke-grant.svg#only-light)
![Revoke a grant](seq-4-revoke-grant.dark.svg#only-dark)

### Switch the backend mode

![Switch the backend mode](seq-5-backend-mode.svg#only-light)
![Switch the backend mode](seq-5-backend-mode.dark.svg#only-dark)

### Push with extension

![Push with extension](seq-6-push-nse.svg#only-light)
![Push with extension](seq-6-push-nse.dark.svg#only-dark)

### File upload

![File upload](seq-7-file-upload.svg#only-light)
![File upload](seq-7-file-upload.dark.svg#only-dark)

### Open a terminal

![Open a terminal](seq-8-open-terminal.svg#only-light)
![Open a terminal](seq-8-open-terminal.dark.svg#only-dark)

### Instance and identity

![Instance and identity](erd-1-instance-identity.svg#only-light)
![Instance and identity](erd-1-instance-identity.dark.svg#only-dark)

### Permissions and structure

![Permissions and structure](erd-2-permissions-structure.svg#only-light)
![Permissions and structure](erd-2-permissions-structure.dark.svg#only-dark)

### Agents and apps

![Agents and apps](erd-3-agents-apps.svg#only-light)
![Agents and apps](erd-3-agents-apps.dark.svg#only-dark)

### Messages and files

![Messages and files](erd-4-messages-files.svg#only-light)
![Messages and files](erd-4-messages-files.dark.svg#only-dark)

### Runs and costs

![Runs and costs](erd-5-runs-costs.svg#only-light)
![Runs and costs](erd-5-runs-costs.dark.svg#only-dark)

### Sync

![Sync](erd-6-sync.svg#only-light)
![Sync](erd-6-sync.dark.svg#only-dark)

### Terminal

![Terminal](erd-7-terminal.svg#only-light)
![Terminal](erd-7-terminal.dark.svg#only-dark)

### Audit and privacy

![Audit and privacy](erd-8-audit-privacy.svg#only-light)
![Audit and privacy](erd-8-audit-privacy.dark.svg#only-dark)

### Import, status, statistics

![Import, status, statistics](erd-9-import-status-statistics.svg#only-light)
![Import, status, statistics](erd-9-import-status-statistics.dark.svg#only-dark)

### Notification

![Notification](erd-10-notification.svg#only-light)
![Notification](erd-10-notification.dark.svg#only-dark)

## Conventions

- **C4 as a flowchart.** Context, container and component diagrams use the C4 notation (person,
  system, container, external) as a Mermaid flowchart. Reason: the C4 mode of Mermaid arranges the
  boxes in a fixed grid, and with more than a handful of relationships the labels overlap.
  The container and component diagrams use the ELK layout (`config.layout: elk`).
- **Paths without prefix.** REST paths are written without the version prefix; the prefix is set by
  [../spec/06-api.md](../spec/06-api.md). Where it is clearer, the action is shown instead of the path
  (for example `run.finish`).
- **ERDs.** The files `erd-*.mmd` are the only ERDs of the project; 03 only refers to them.
  Every table from 03, section 4 (except the technical tables from 4.13) appears exactly once with
  columns (keys, foreign keys, state columns), in the diagram of its module. Tables of
  other modules appear there only with their key. Every foreign key relationship from 03 is
  drawn, as often as there are foreign keys between the two tables; excepted are pure
  log columns (name ends in `_by`, target `principals(id)`). Complete columns, types and
  indexes are only in 03. ERDs with many child tables use `direction LR` so that they do not grow
  too wide.
- **Readability.** No diagram is wider or taller than the aspect ratio `MAX_ASPECT` in
  `tools/docs/render_diagrams.py`. If one becomes too wide, it is split by module or rotated.
- **Colors and font from the design system.** Every diagram renders twice, with
  `design/build/mermaid.config.json` to `<name>.svg` and with `design/build/mermaid.dark.config.json`
  to `<name>.dark.svg` (Mermaid theme `base`), which `tools/design-tokens/tokens.py` generates from
  the tokens of the light and of the dark theme with one variable map. The gallery shows each
  rendering only in its scheme (`#only-light`, `#only-dark`). Texts and lines meet the contrast
  gate of the design tokens in both renderings (see "Render and check"). Node classes (`classDef`)
  use only token colors and the same colors in both renderings:
  accent (spruce) for own containers and steps, loch for data stores, moss for edge modules,
  gorse for checks, granite for external systems. The docs site colors its embedded
  Mermaid blocks through `docs/design/site-theme.css` (also generated) to match the color scheme.
- **Module edges.** An arrow in the component diagram means "may call the port". Edges to
  `internal/kernel` are not drawn because every module has them.

## Render and check

Re-render (the `.svg` and `.dark.svg` of all diagrams or only of the named ones); afterwards all
checks run:

```sh
uv run --project tools --locked --quiet python tools/docs/render_diagrams.py --write [docs/architecture/<name>.mmd ...]
```

Check: `make diagrams`. The gate checks that every diagram renders, that every `.mmd` has an `.svg`
and a `.dark.svg`, each rendered with the design theme of its scheme and stamped with the digest of
the source and that theme, that no diagram exceeds the aspect ratio `MAX_ASPECT`, that `.mmd` files
name only token colors, that texts and lines are readable in both renderings
(`tools/docs/diagram_contrast.py`, with `color.contrast` and the thresholds `rules.THRESHOLDS` of
`tools/design-tokens`: every text color of the Mermaid theme against the backgrounds it is drawn on
and every `classDef` or `style` color against its fill at the `text` ratio, every line and border
color against the canvas at the `ui` ratio, and each SVG carries the background and text color of
its theme) and that the ERDs match 03 (`tools/docs/erd_check.py`: every table exactly
once in full, every foreign key relationship drawn). For Chromium without a sandbox,
mermaid-cli needs a Puppeteer configuration outside the repo (`MMDC_PUPPETEER_CONFIG`). The
comparison of the module edges is described in [../spec/02-architecture.md](../spec/02-architecture.md),
section 7.3.
