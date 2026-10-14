# Modul `chat`

Status: spaces and the sidebar built (M1-52); channels (M1-53), operators (M1-54) and threads (M1-55) follow with the tickets of the etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Bereiche, Seitenleisten-Zustand, Kanäle mit Verhalten, Skills, Projektbindung, DM-Teilnehmer, Forum-Tags, Threads mit Thread-Sequenz.

## Code

- `internal/chat` (`chat.go`): the port. `Structure` with its space and sidebar part (M1-52):
  `ListSpaces`, `GetSpace`, `CreateSpace`, `UpdateSpace`, `MoveSpace`, `ArchiveSpace`,
  `PutSidebarSpace` and `ListSidebarStates` (for `getMySidebar`, M1-61); the DTOs `Space`,
  `SpaceView`, `SidebarState`, `Precondition` (the parsed If-Match) and one command per body; the
  in-transaction event `SpaceArchived{SpaceID}`; `Register` declares the durable events
  `space.created`, `space.updated`, `space.archived` and `sidebar.updated`.
- `domain/`: the space entity (every change bumps `Version` once), the checks of name
  (`SpaceName`: trimmed, not blank, at most `space.name_max_chars` Unicode characters, SEC-IN-4),
  colour and icon (keys of `kernel/designkeys` only), reason, and the sidebar key rule
  (`SidebarKey`). `domain/fracindex`: `KeyBetween(a, b)` and `Valid(key)` of the base-62
  fractional index with integer part (docs/DECISIONS.md M1-52/2).
- `app/`: `Service`, one use case per operation, and the outgoing port `Store`. `Config` binds
  `space.name_max_chars` (`wire.ChatConfig`).
- `adapters/postgres/`: `queries/spaces.sql` and `queries/sidebar.sql`, the sqlc output in
  `sqlcgen/` (entry in `services/server/sqlc.yaml`) and `Repository` (implements `app.Store`).
  Locks and writes run only inside a unit of work; a taken name is `409 conflict` on `/name`, no
  error carries SQL text, constraint names or values.
- `adapters/httpapi/`: `Spaces`, the handlers of `listSpaces`, `createSpace`, `getSpace`,
  `updateSpace`, `moveSpace`, `archiveSpace` and `putMySidebarSpace`; `ETag` from `version` on
  `getSpace` and `updateSpace`.
- `adapters/authzsource/`: `Source`, the space part of `authz.StructureSource` (`Space`,
  `SpaceArchived`, `Spaces`); `wire.NewStructureSource` joins it with the empty channel and
  thread part until M1-53 and M1-55.
- Migrations `00023_chat_spaces.sql` (`spaces`, `user_space_states` exactly as 03 section 4.5,
  sort keys `COLLATE "C"`, and `grants.target_space_id -> spaces(id)` `NOT VALID`) and
  `00024_grants_space_fk_validate.sql` (validates it). `bothy_app` may read, insert and update
  `spaces` and also delete `user_space_states`.

## The use cases (M1-52)

| Operation | Action | Rights change | Event |
|---|---|---|---|
| `listSpaces` | `space.read` (Scope, then per item in filter mode) | no | |
| `getSpace` | `space.read` | no | |
| `createSpace` | `space.create` on the instance (owner) | no | `space.created` |
| `updateSpace` | `space.update` (`manage`), If-Match | no | `space.updated` |
| `moveSpace` | `space.reorder` on the instance (owner) | no | `space.updated` |
| `archiveSpace` | `space.archive` (owner, step-up) | yes (target: the space) | `space.archived` |
| `putMySidebarSpace` | `sidebar.update` (self and read, people only) | no | `sidebar.updated` (self) |

- Visibility (07 section 4.3): `read` on the space or on one of its channels; a missing and an
  invisible space give the same `404`. `listSpaces` filters in SQL with
  `archived_at IS NULL AND ($all OR id = ANY($visible))` from `Scope` (all spaces for owners and
  instance-wide readers, built per request, so a new space needs no new rights version), then
  authorizes each row. `SpaceView` reports the effective level and capabilities from
  `authz.RightsQuery`; a space seen only through a channel reports `read` and none.
- Global order: create inserts after `after_space_id` or at the end, move after `after_id`,
  before `before_id` or at the start; both fields, the space itself and an unknown or archived
  target are `400` on the field. Create and move take the order lock (an advisory lock, then
  every space that is not archived in ascending id) and a move writes exactly its own row
  (docs/DECISIONS.md M1-52/3).
- Archive (07 section 4.5): final. A first unit authorizes and answers an archived space without
  a write (`no_change`); a second unit, a declared rights change with the space as target, sets
  `archived_at`, bumps the version, publishes `SpaceArchived` and appends `space.archived`. authz
  caps every level in the space at `read` (owners included) and writes `access.changed` with the
  space in `changed` at `read` for everyone whose level was above. `updateSpace` and `moveSpace`
  answer an archived space with `409 conflict`; `getSpace` still answers it.
- Sidebar (04 section 14): agents and apps get `403 subject_kind_not_allowed`; `collapsed` is
  required, `sort_key` must be a fractional index key, `use_global_order` removes the personal key
  and excludes `sort_key`, neither keeps the stored key. The row is upserted with version + 1 and
  the caller's device; `sidebar.updated` goes to the caller without `resource.version` (04
  section 2.2 rule 3).
- Audit: one allow entry per mutation of a space (`changed_fields`, `reason`); the sidebar is not
  audited.

Rulings: docs/DECISIONS.md M1-52.

### Tests

`domain/domain_test.go`, `domain/fracindex/fracindex_test.go` (vectors and properties),
`register_test.go`, `adapters/httpapi/*_test.go` over PostgreSQL 18 with real authz, the chat
source and the full chain with response validation against openapi.yaml (order, concurrency,
If-Match, palette keys, SEC-T07, names, visibility through a channel, visibility without a rights
version, the archive as rights change, rights per operation, events, sidebar), and
`migrations/chat_spaces_test.go`.

## Port (Wurzelpaket `internal/chat`)

- Eingehend: `Structure` (Bereiche, Kanäle, Schreibschutz), `Threads` (Anlegen für `messaging`, `AppendToThread`, Nachrichtenzahl bei Löschung, Schließen, Öffnen, Verschieben, Info), `AgentConfig`.
- Gibt In-Tx-Ereignisse aus: `ThreadMoved`, `ChannelArchived`, `ChannelReadOnlySet`, `SpaceArchived`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: `authz.StructureSource`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `authz`.

Grund: Bediener ist aktiver Agent-Prinzipal, DM-Teilnehmer sind Menschen; implementiert `StructureSource`.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`spaces`, `user_space_states`, `channels`, `channel_skill_bindings`, `project_bindings`, `dm_participants`, `forum_tags`, `threads`, `thread_tags`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`space.read`, `space.create`, `space.update`, `space.reorder`, `space.archive`, `space.operator.set`, `sidebar.update`, `channel.read`, `channel.create`, `channel.update`, `channel.reorder`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`, `channel.project.bind`, `channel.read_only.set`, `channel.archive`, `forum_tag.manage`, `dm.create`, `dm.leave`, `thread.read`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.move`, `thread.tags.set`, `agent.config.read`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `thread.created`, `thread.updated`, `thread.closed`, `thread.reopened`, `thread.moved`, `thread.moved_out`, `space.created`, `space.updated`, `space.archived`, `channel.created`, `channel.updated`, `channel.settings_changed`, `channel.archived`, `sidebar.updated`, `agent.config_changed`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hinweise für den Bau

- `Threads.AppendToThread` sperrt die Thread-Zeile und vergibt die Thread-Sequenz (03, 6.2); `messaging` ruft ihn in seiner Transaktion.
- Kanal-Prompt, Skills und Projektbindung stehen nie in Ereignissen; `channel.settings_changed` trägt nur `settings_version` (04, 2.2).
- Verschieben eines Threads nur bei gleichem wirksamen Bediener (03, `threads`).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [01-produkt.md](../../../../docs/spec/01-produkt.md), [07-rechte.md](../../../../docs/spec/07-rechte.md), [09-agenten.md](../../../../docs/spec/09-agenten.md)
