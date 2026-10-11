# Architecture diagrams

All diagrams are Mermaid sources (`.mmd`) with the rendered SVG next to them. The source is
authoritative; the SVG is regenerated on every change. Text and rules for them are in
[../spec/02-architecture.md](../spec/02-architecture.md). Names of tables, events and actions
come from the core contract ([00](../spec/00-glossary.md), [03](../spec/03-data-model.md),
[04](../spec/04-sync-protocol.md), [05](../spec/05-actions.md)).

## Index

### C4

| Diagram | Content | Source | Image |
|---|---|---|---|
| Context | An instance with people and external systems, the protocol for each relationship. | [c4-1-context.mmd](c4-1-context.mmd) | [c4-1-context.svg](c4-1-context.svg) |
| Container | Roles `api` and `worker` of the binary, PostgreSQL, object storage, clamd, Collector, apps, adapters, host service. | [c4-2-container.mmd](c4-2-container.mmd) | [c4-2-container.svg](c4-2-container.svg) |
| Server components | Modules of the monolith with all allowed dependencies according to 02-architecture.md, 4.4. | [c4-3-server-components.mmd](c4-3-server-components.mmd) | [c4-3-server-components.svg](c4-3-server-components.svg) |

### Flows in the server

| Diagram | Content | Source | Image |
|---|---|---|---|
| Request path | Middleware chain of a REST request up to the proof check of `authorize`. | [flow-request-path.mmd](flow-request-path.mmd) | [flow-request-path.svg](flow-request-path.svg) |
| Event path | Unit of work with phases, `event_log`, log tail, hub, agent and worker consumers. | [flow-event-path.mmd](flow-event-path.mmd) | [flow-event-path.svg](flow-event-path.svg) |

### Sequences

| Diagram | Content | Source | Image |
|---|---|---|---|
| Send a message | Sending with `client_msg_id`, delivery to connected clients, push to a device without a connection. | [seq-1-send-message.mmd](seq-1-send-message.mmd) | [seq-1-send-message.svg](seq-1-send-message.svg) |
| Agent run | Triggering, I2 and invocation token on delivery, run start via compare-and-swap, live box patches, follow-up, completion. | [seq-2-agent-run.mmd](seq-2-agent-run.mmd) | [seq-2-agent-run.svg](seq-2-agent-run.svg) |
| Reconnect | `resume` or `identify` on another node, catch-up with a buffer, `resync_required`. | [seq-3-reconnect.mmd](seq-3-reconnect.mmd) | [seq-3-reconnect.svg](seq-3-reconnect.svg) |
| Revoke a grant | Revocation with an open connection across two nodes, cancellation of the active run. | [seq-4-revoke-grant.mmd](seq-4-revoke-grant.mmd) | [seq-4-revoke-grant.svg](seq-4-revoke-grant.svg) |
| Switch the backend mode | Switching to `personal` with step-up, I1 check, configured and reported mode at the adapter. | [seq-5-backend-mode.mmd](seq-5-backend-mode.mmd) | [seq-5-backend-mode.svg](seq-5-backend-mode.svg) |
| Push with extension | Push without content, the Notification Service Extension fetches the message through the API. | [seq-6-push-nse.mmd](seq-6-push-nse.mmd) | [seq-6-push-nse.svg](seq-6-push-nse.svg) |
| File upload | Upload session, upload directly or through a presigned URL, scan with clamd, thumbnail. | [seq-7-file-upload.mmd](seq-7-file-upload.mmd) | [seq-7-file-upload.svg](seq-7-file-upload.svg) |
| Open a terminal | Step-up, `terminal.open` with SR10, I6 and SR12, terminal session token, Noise `NK` through the relay, PTY only after `start`. | [seq-8-open-terminal.mmd](seq-8-open-terminal.mmd) | [seq-8-open-terminal.svg](seq-8-open-terminal.svg) |

### Data model (ERD, from 03-data-model.md, split by module)

| Diagram | Content | Source | Image |
|---|---|---|---|
| Instance and identity | Modules `instance` and `identity`. | [erd-1-instance-identity.mmd](erd-1-instance-identity.mmd) | [erd-1-instance-identity.svg](erd-1-instance-identity.svg) |
| Permissions and structure | Modules `authz` and `chat`. | [erd-2-permissions-structure.mmd](erd-2-permissions-structure.mmd) | [erd-2-permissions-structure.svg](erd-2-permissions-structure.svg) |
| Agents and apps | Modules `agents` and `apps`. | [erd-3-agents-apps.mmd](erd-3-agents-apps.mmd) | [erd-3-agents-apps.svg](erd-3-agents-apps.svg) |
| Messages and files | Modules `messaging` and `files`. | [erd-4-messages-files.mmd](erd-4-messages-files.mmd) | [erd-4-messages-files.svg](erd-4-messages-files.svg) |
| Runs and costs | Modules `runs` and `billing`. | [erd-5-runs-costs.mmd](erd-5-runs-costs.mmd) | [erd-5-runs-costs.svg](erd-5-runs-costs.svg) |
| Sync | Module `sync`: event log, idempotency, read state, drafts, settings. | [erd-6-sync.mmd](erd-6-sync.mmd) | [erd-6-sync.svg](erd-6-sync.svg) |
| Terminal | Module `terminal`: hosts and terminal sessions. | [erd-7-terminal.mmd](erd-7-terminal.mmd) | [erd-7-terminal.svg](erd-7-terminal.svg) |
| Audit and privacy | Modules `audit` and `privacy`. | [erd-8-audit-privacy.mmd](erd-8-audit-privacy.mmd) | [erd-8-audit-privacy.svg](erd-8-audit-privacy.svg) |
| Import, status, statistics | Modules `importer`, `status` and `stats`. | [erd-9-import-status-statistics.mmd](erd-9-import-status-statistics.mmd) | [erd-9-import-status-statistics.svg](erd-9-import-status-statistics.svg) |
| Notification | Module `notify`. | [erd-10-notification.mmd](erd-10-notification.mmd) | [erd-10-notification.svg](erd-10-notification.svg) |

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
- **Colors and font from the design system.** All diagrams render with
  `design/build/mermaid.config.json` (Mermaid theme `base`), which `tools/design-tokens/tokens.py`
  generates from the tokens of the light theme. Node classes (`classDef`) use only token colors:
  accent (spruce) for own containers and steps, loch for data stores, moss for edge modules,
  gorse for checks, granite for external systems. The docs site colors its embedded
  Mermaid blocks through `docs/design/site-theme.css` (also generated) to match the color scheme.
- **Module edges.** An arrow in the component diagram means "may call the port". Edges to
  `internal/kernel` are not drawn because every module has them.

## Render and check

Re-render (all `.svg` files or only the named ones); afterwards all checks run:

```sh
uv run --project tools --locked --quiet python tools/docs/render_diagrams.py --write [docs/architecture/<name>.mmd ...]
```

Check: `make diagrams`. The gate checks that every diagram renders, that every `.mmd` has an `.svg`
with the design theme, that no diagram exceeds the aspect ratio `MAX_ASPECT`, that `.mmd` files name
only token colors and that the ERDs match 03 (`tools/docs/erd_check.py`: every table exactly
once in full, every foreign key relationship drawn). For Chromium without a sandbox,
mermaid-cli needs a Puppeteer configuration outside the repo (`MMDC_PUPPETEER_CONFIG`). The
comparison of the module edges is described in [../spec/02-architecture.md](../spec/02-architecture.md),
section 7.3.
