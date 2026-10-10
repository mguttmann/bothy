# Modul `chat`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Bereiche, Seitenleisten-Zustand, Kanäle mit Verhalten, Skills, Projektbindung, DM-Teilnehmer, Forum-Tags, Threads mit Thread-Sequenz.

## Port (Wurzelpaket `internal/chat`)

- Eingehend: `Structure` (Bereiche, Kanäle), `Threads` (Anlegen, `AppendToThread`, Schließen, Öffnen, Verschieben, Info), `AgentConfig`.
- Gibt In-Tx-Ereignisse aus: `ThreadMoved`, `ChannelArchived`, `SpaceArchived`.
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

`space.read`, `space.create`, `space.update`, `space.reorder`, `space.archive`, `space.operator.set`, `sidebar.update`, `channel.read`, `channel.create`, `channel.update`, `channel.reorder`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`, `channel.project.bind`, `channel.archive`, `forum_tag.manage`, `dm.create`, `dm.leave`, `thread.read`, `thread.create`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.move`, `thread.tags.set`, `agent.config.read`.

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
