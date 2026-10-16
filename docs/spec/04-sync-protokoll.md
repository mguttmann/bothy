# 04 Ereignis- und Sync-Protokoll

Status: Entwurf M0, nach den Rückmeldungen aller Kapitel integriert. Normativ für das Gateway
(AsyncAPI 3.0), die REST-Nachholschnittstelle (OpenAPI 3.1), die Clients und die Agenten-Adapter.
Alle Pfade stehen unter dem Präfix `/api/v1` ([06-api.md](06-api.md), R-API-01).
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Aktionen und Ereignistypen: [05-aktionen.md](05-aktionen.md).

Alle Grenzen, Fenster und Intervalle sind benannte Konfigurationswerte. Ihre Standardwerte
stehen nur in Abschnitt 18.

## 1. Ziele

1. Kein Ereignis geht verloren: Ein Client, der wiederkommt, bekommt alles, was er sehen darf,
   oder die klare Anweisung, neu zu laden.
2. Nichts wird doppelt angelegt: Jede schreibende Anfrage ist wiederholbar.
3. Die Reihenfolge in einem Thread ist für alle gleich.
4. Rechte gelten bei jeder Zustellung, nicht nur bei der Anfrage. Ein Entzug wirkt sofort.
5. Ein Agenten-Adapter ist ein normaler Client mit dauerhaftem Cursor. Er verpasst nichts und
   startet nichts doppelt.

## 2. Ereignisse

### 2.1 Zwei Arten

| Art | Gespeichert | Sequenz | Nachholbar | Beispiele |
|---|---|---|---|---|
| dauerhaft (durable) | ja, in `event_log` | globale `seq` | ja, bis `sync.event_retention` | `message.created`, `thread.closed`, `run.state_changed`, `access.changed` |
| kurzlebig (ephemeral) | nein | keine | nein | `typing.started`, `run.updated`, `agent.presence_changed`, `notification.created`, `command.autocomplete_requested` |

Regel: Alles, was ein Client nach einer Pause kennen muss, ist dauerhaft. Kurzlebige Ereignisse
sind reine Beschleunigung; ihr Verlust wird durch den nächsten dauerhaften Zustand oder einen
Abruf geheilt. Die vollständige Liste der dauerhaften Typen mit der Empfangs-Aktion steht in
[05-aktionen.md](05-aktionen.md), Abschnitt 6.

Terminal ([24-terminal.md](24-terminal.md)): Zustände von Rechnern (`host.updated`) und eigenen
Terminal-Sitzungen (`terminal_session.changed`, persönlich) sind dauerhafte Ereignisse, die
Verbindung eines Rechners (`host.presence_changed`) ist kurzlebig. Der Terminal-Strom selbst ist
kein Ereignis: Er läuft Ende-zu-Ende verschlüsselt über eigene WebSocket-Verbindungen außerhalb des
Gateways und steht nie im Ereignis-Log, in Momentaufnahmen oder im Nachholen. Grund: Das Log ist für
Zustände, nicht für Byte-Ströme, und alles darin lesen Server und Sicherungen im Klartext.

Live-Statistik ([25-visualisierung.md](25-visualisierung.md)): Der Live-Rahmen `stats.live` ist
ein kurzlebiges Ereignis, das nur Verbindungen mit einem Abo (`stats.subscribe`) bekommen
(Abschnitt 12, Punkt 4). Er steht nie im Ereignis-Log; der nächste Rahmen ersetzt einen
verlorenen.

### 2.2 Umschlag (Envelope)

Jedes Ereignis, dauerhaft oder kurzlebig, hat denselben Umschlag, auf dem Gateway, im
REST-Nachholen und in Webhooks ([ADR 0052](../adr/0052-ein-ereignis-umschlag-fuer-gateway-rest-und-webhooks.md)). Auf der Leitung ist es JSON. Es gibt kein JSON-`null`:
Ein fehlendes Feld heißt "kein Wert" ([ADR 0053](../adr/0053-kein-json-null-in-api-und-ereignissen.md)).

| Feld | Typ | Pflicht | Bedeutung |
|---|---|---|---|
| `id` | UUIDv7 | ja | eindeutige Ereignis-ID, Dedupe-Schlüssel |
| `seq` | int64 | bei dauerhaften | globale Sequenz; fehlt bei kurzlebigen Ereignissen |
| `type` | string | ja | Ereignistyp, z. B. `message.created` |
| `schema_version` | int | ja | Version der Nutzlast dieses Typs; Änderungen nur additiv |
| `occurred_at` | RFC 3339 | ja | Zeit der Transaktion (nur Anzeige, nie Reihenfolge) |
| `actor` | `{id, kind}` | nein | wer es ausgelöst hat; `kind` ist `user`, `agent`, `app`, `system`; `id` fehlt bei `system` (Systemaktion des Servers, 05 Regel 10); `actor` fehlt bei Ereignissen von Jobs ohne Prinzipal und ohne Systemaktion (03, `event_log.actor_kind`) |
| `scope` | `{space_id, channel_id, thread_id}` | ja | Ort; nicht zutreffende Felder fehlen |
| `resource` | `{kind, id, version}` | ja | betroffenes Objekt und seine Version nach der Änderung; `version` fehlt, wo das Ereignis keine neue Version genau dieses Objekts meldet (Abschnitt 9, Punkt 3) |
| `audience` | `"self"` | nein | gesetzt bei persönlichen Ereignissen |
| `payload` | object | ja | Nutzlast nach Schema `<type>.v<schema_version>` |
| `redacted` | bool | nein | `true`: Nutzlast wurde nach Löschung geschwärzt |
| `truncated` | bool | nein | `true`: Nutzlast wurde auf `sync.event_payload_max_bytes` gekürzt; der Client lädt per REST nach |
| `correlation_id` | UUID | nein | Trace-ID der auslösenden Anfrage (03, `event_log.correlation_id`) |
| `trace_context` | string | nein | W3C `traceparent` der auslösenden Anfrage; nur in Zustellungen an Agenten-Adapter und nur, wenn die Anfrage aufgezeichnet wurde |
| `client_msg_id` | UUIDv7 | nein | Echo der Client-ID, damit der Absender sein Ausgangsfach abgleicht |
| `permission_version` | int64 | bei dauerhaften | Rechte-Version zur Zeit des Commits |
| `invocation_token` | string | nein | Zustellfeld: frisch ausgestellte Aufruf-Marke (JWS, I3) bei Ereignissen an Agenten, die einen Aufruf tragen (Abschnitt 15.3) |
| `redelivered` | bool | nein | Zustellfeld: erneute Zustellung an einen Agenten-Verbraucher nach `agent.ack_timeout` |
| `sync_epoch` | UUIDv7 | nein | Zustellfeld: Epoche der Zeitlinie (7.5), nur in Webhook-Zustellungen; ein Empfänger, der eine neue Epoche sieht, setzt seinen Stand nach `seq` zurück ([ADR 0059](../adr/0059-sync-epoche-nach-wiederherstellung.md)) |

Zustellfelder entstehen je Zustellung und stehen nie im Ereignis-Log.

Regeln für die Nutzlast:

1. Sie enthält nur Felder, die ein Empfänger mit der Empfangs-Aktion des Typs sehen darf.
   Daten, die mehr Rechte brauchen (z. B. Kanal-Prompt, Skills, Projektbindung), stehen nie in
   Ereignissen. Das Ereignis nennt dann nur die neue Version (`channels.settings_version`);
   Berechtigte laden per REST nach.
2. Sie ist kleiner als `sync.event_payload_max_bytes`. Für größere Objekte (lange Nachricht)
   baut der Erzeuger eine verkleinerte Nutzlast mit Verweis und setzt `truncated: true`
   (03, `event_log.truncated`); der Anhänger lehnt eine zu große Nutzlast ab, statt sie zu
   kürzen. Der Client lädt per REST nach.
3. Sie enthält keine Geheimnisse und keine Tokens.
4. Nachrichteninhalte stehen nur in Ereignissen mit Ressource `message` (`message.*`; `reaction.*`
   tragen das Emoji). Alle anderen Ereignisse nennen Nachrichten nur per ID, auch `agent.invocation`
   und `agent.followup`; den Inhalt liest der Empfänger per REST mit einer Prüfung durch
   `authorize`. Grund: Das Schwärzen bei Löschung (03, Abschnitt 6.5) erreicht so jeden Inhalt.
5. Eine geschwärzte Nutzlast bleibt gültig nach ihrem Schema: `message.created` und
   `message.updated` tragen die Löschmarke der Nachricht (`content_md` leer, ohne Anhänge,
   Erwähnungen, Komponenten und Reaktionen, mit `deleted_at`); `reaction.added` und
   `reaction.removed` tragen `emoji: ""` und `count: 0`; `pin.added` und `pin.removed` bleiben
   unverändert, weil sie keinen Inhalt tragen. Ein neuer Ereignistyp mit Ressource `message` legt
   seine Schwärzung mit seinem Schema fest; ohne sie scheitert das Löschen (Test).

### 2.3 Rahmen auf dem WebSocket

Jeder Rahmen ist ein JSON-Textrahmen der Form `{"op": "<name>", "d": {...}}`. Ein Rahmen ist
höchstens `sync.ws_max_frame_bytes` groß. Ereignisse kommen als `{"op": "dispatch", "d":
<Umschlag>}`; Aufruf-Marke und `redelivered` stehen als Zustellfelder im Umschlag.

## 3. Globale, commit-geordnete Sequenz

### 3.1 Warum `bigserial` nicht reicht

Eine PostgreSQL-Sequenz vergibt Nummern beim Aufruf, nicht beim Commit, und sie ist nicht
transaktional. Beispiel: Transaktion A holt `seq = 10`, Transaktion B holt `seq = 11`. B
committet zuerst. Ein Leser sieht 11 und setzt seinen Cursor auf 11. Danach committet A mit 10.
Der Leser fragt "alles nach 11" und verpasst 10 für immer. Außerdem hinterlässt jede
zurückgerollte Transaktion eine Lücke; ein Leser kann eine Lücke nicht von einer noch offenen
Transaktion unterscheiden.

### 3.2 Muster mit Zähler-Zeile

Die Zeile `sequence_counters('event_seq')` wird in derselben Transaktion gesperrt, die die
Domänenänderung schreibt, und zwar als letzter Schritt vor dem Commit:

```sql
BEGIN;  -- READ COMMITTED
  -- 1. Domänenänderung (Beispiel: neue Nachricht)
  UPDATE threads
     SET last_message_seq = last_message_seq + 1,
         last_activity_at = now(), message_count = message_count + 1,
         version = version + 1
   WHERE id = $thread_id
  RETURNING last_message_seq;                          -- -> $thread_seq

  INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id,
                        client_msg_id, client_payload_sha256, origin, kind, content_md, ...)
  VALUES ($id, $thread_id, $channel_id, $thread_seq, $author, $client_msg_id, $hash,
          'human', 'text', $content, ...);

  -- 2. Als letzter Schritt: Zähler sperren und n Nummern holen
  UPDATE sequence_counters
     SET value = value + $n            -- n = Anzahl Ereignisse dieser Transaktion
   WHERE name = 'event_seq'
  RETURNING value;                     -- -> $last; Nummern sind $last - $n + 1 .. $last

  INSERT INTO event_log (seq, id, type, ..., permission_version)
  VALUES ($last - $n + 1, ...), ..., ($last, ...);

  SELECT pg_notify('event_log', $last::text);
COMMIT;
```

Warum das stimmt:

1. Die Zeilensperre aus dem `UPDATE` hält bis zum Commit. Eine zweite Transaktion wartet an
   derselben Zeile.
2. In `READ COMMITTED` liest die wartende Transaktion nach dem Commit der ersten den neuen
   Zählerstand neu. Sie bekommt also die nächsten Nummern.
3. PostgreSQL macht eine Transaktion sichtbar, bevor es ihre Sperren freigibt. Wenn B die
   Sperre bekommt, ist A schon sichtbar. Darum gilt: Sieht ein Leser `seq = k`, sieht er auch
   jede kleinere Nummer.
4. Rollt eine Transaktion zurück, rollt der Zähler mit. Es gibt keine Lücken. Ein Leser darf
   darum einfach `WHERE seq > $cursor ORDER BY seq` lesen.

Kosten: Alle Schreibtransaktionen mit Ereignissen sind am Ende serialisiert. Die Sperre hält
nur vom letzten Schritt bis zum Commit (inklusive WAL-Flush). Darum steht der Zähler-Schritt
immer zuletzt. Schreibende Transaktionen mit Ereignissen dürfen nicht in `REPEATABLE READ` oder
`SERIALIZABLE` laufen; dort bricht das Muster mit Serialisierungsfehlern. Die Sperrreihenfolge
steht in [03-datenmodell.md](03-datenmodell.md), Abschnitt 6.3. Ein Lasttest hält den Durchsatz
als Qualitäts-Gate fest: mindestens `perf.event_commit_min_per_s` im Referenzprofil
([02-architektur.md](02-architektur.md), Abschnitt 17; [ADR 0063](../adr/0063-leistungs-gate-fuer-das-ereignis-log.md)).

### 3.3 Weitere Zähler

Dasselbe Muster gilt für `audit_seq` (Hash-Kette im Audit-Log) und für `permission_version`
(Rechte-Version). Die Rechte-Version wird bei Rechte-Änderungen als erstes gesperrt, damit alle
Rechte-Änderungen serialisiert sind.

## 4. Thread-Sequenz

Jede Nachricht hat `thread_seq`, lückenlos je Thread, vergeben über die Zeilensperre auf
`threads` (Muster oben, Schritt 1). Sie bestimmt die Anzeige-Reihenfolge. Die globale `seq`
bestimmt die Zustell-Reihenfolge. Beide sind in einem Thread gleich geordnet, weil beide aus
derselben Transaktion kommen und die Thread-Sperre vor der Zähler-Sperre genommen wird.

Gelöschte Nachrichten behalten ihre `thread_seq` als Tombstone. Ein Client erkennt eine echte
Lücke in `thread_seq` als fehlende Daten und lädt den Bereich per REST nach.

## 5. Idempotente Schreibvorgänge

### 5.1 Nachrichten: `client_msg_id`

1. Der Client erzeugt beim Verfassen eine UUIDv7 als `client_msg_id`. Er speichert sie mit dem
   Eintrag im Ausgangsfach, bevor er sendet.
2. Eindeutig ist das Paar `(author_principal_id, client_msg_id)`, dauerhaft (kein Ablauf).
3. Der Server rechnet `client_payload_sha256` über die kanonische Anfrage (Thread, Inhalt,
   Antwort-Bezug, Anhänge).
4. Ablauf im Server: zuerst `SELECT` nach dem Paar. Gibt es die Nachricht, antwortet er mit dem
   Original (`200`, Kopfzeile `Idempotent-Replay: true`), ohne neue Ereignisse. Ist der Hash
   anders, antwortet er `422 idempotency_key_reuse`. Gibt es sie nicht, folgt das Muster aus
   Abschnitt 3.2. Schlagen zwei gleiche Anfragen gleichzeitig an, verletzt die zweite den
   Unique-Index; ihre Transaktion rollt samt Thread-Nummer zurück, und der Server antwortet
   wie bei einer Wiederholung.
5. Der Server prüft das UUID-Format (Version 7), nicht die Uhrzeit darin. Ein Ausgangsfach
   darf also tagealte Einträge senden.

Interaktionen nutzen dasselbe Verfahren mit `client_interaction_id`.

### 5.2 Alle anderen Schreibvorgänge: `Idempotency-Key`

Jede andere schreibende REST-Operation akzeptiert die Kopfzeile `Idempotency-Key` (UUIDv7).
Clients setzen sie bei jeder Operation aus dem Ausgangsfach. Der Server speichert Anfrage-Hash
und Antwort in `idempotency_keys` für `sync.idempotency_key_ttl`. Wiederholung mit gleichem
Hash liefert die gespeicherte Antwort; anderer Hash: `422 idempotency_key_reuse`; Erstanfrage
läuft noch: `409 request_in_progress`. Operationen mit Versionsprüfung (`If-Match`) sind
zusätzlich durch die Version geschützt.

## 6. WebSocket-Gateway

### 6.1 Endpunkt und Version

Ein Endpunkt `wss://<host>/api/v1/gateway`. Die Protokollversion steht in `identify` und `hello`.
Die unterstützten Versionen stehen an einer Stelle: `info.x-protocol-versions` in
[asyncapi.yaml](../api/asyncapi.yaml); `hello` und die Discovery nennen dieselben.
Änderungen innerhalb einer Hauptversion sind nur additiv: neue Ops, neue Felder, neue
Ereignistypen. Clients ignorieren Unbekanntes. Ein Gerät hält genau eine Verbindung: Ein neues
`identify` oder `resume` desselben Geräts schließt eine ältere offene Verbindung mit `4017`
(Abschnitt 6.4). Grund: Fokus, Tippen und "Person schaut zu" gelten je Gerät, und ein fehlerhafter
Client soll nicht beliebig viele Verbindungen halten. Mit `cluster.mode = multi` gilt das über alle
api-Knoten (Geräte-ID über `NOTIFY ephemeral`, Abschnitt 12 Punkt 3;
[02-architektur.md](02-architektur.md), Abschnitt 11.6).

### 6.2 Ops

Client an Server:

| Op | Zweck |
|---|---|
| `identify` | Anmeldung einer neuen Gateway-Sitzung |
| `resume` | Fortsetzen einer Gateway-Sitzung nach kurzer Trennung |
| `heartbeat` | Lebenszeichen mit letzter verarbeiteter `seq` |
| `reauth` | neuer Zugriffstoken während der Verbindung |
| `focus` | Liste der Threads, die gerade sichtbar sind (für Live-Box-Patches und Tippen), mit `idle` (bool): `true` heißt "offen, aber unbenutzt"; die Verbindung bekommt weiter Patches, zählt aber nicht als "Person schaut zu" ([13-push-benachrichtigungen.md](13-push-benachrichtigungen.md), Abschnitt 4.4) |
| `typing.start` / `typing.stop` | Tippen-Anzeige; nur Menschen (Abschnitt 12) |
| `ack` | nur Agenten: Bestätigung bis `seq` |
| `run.update` | nur Agenten: Patch an einem eigenen Lauf (gleichwertig zu REST) |
| `command.autocomplete_result` | nur Agenten und Apps: Vorschläge zu einer Anfrage (gleichwertig zu REST, Abschnitt 15.7) |
| `stats.subscribe` / `stats.unsubscribe` | Live-Statistik abonnieren (`space_id` optional, sonst Instanz) bzw. beenden; höchstens ein Abo je Verbindung (Abschnitt 12, Punkt 4) |

Server an Client:

| Op | Zweck |
|---|---|
| `hello` | erste Nachricht nach dem Upgrade |
| `ready` | Sitzung bereit, mit Momentaufnahme |
| `resumed` | Fortsetzung bestätigt |
| `dispatch` | ein Ereignis |
| `cursor` | Cursor vorrücken ohne Ereignis (übersprungene, nicht sichtbare Ereignisse) |
| `catchup.complete` | Nachholen beendet, ab jetzt live |
| `resync_required` | Nachholen nicht möglich, neu laden |
| `heartbeat_ack` | Antwort auf `heartbeat` |
| `reauth_required` | Zugriffstoken läuft bald ab |
| `error` | Fehler zu einer Op, Verbindung bleibt |

Jede Op ist in [05-aktionen.md](05-aktionen.md) einer Aktion zugeordnet. Scheitert eine Op an
ihrer Rate-Limit-Klasse, an `authorize` oder an ihren Regeln, antwortet der Server mit `error` und
dem Problem-Code aus [06-api.md](06-api.md) (z. B. `rate_limited`, `forbidden`,
`validation_failed`); die Verbindung bleibt. Geschlossen wird nur mit den Codes aus 6.4.

### 6.3 Lebenszyklus

1. **Upgrade.** Der Client öffnet die Verbindung. Der Server prüft beim Upgrade nichts außer
   der Herkunft. Die Rate-Limit-Klasse `rl.gateway_connect` zählt bei `identify` und `resume`,
   sobald Gerät bzw. Token geprüft sind (05, Abschnitt 3); erschöpft: Schließen mit `4008`.
2. **`hello`.** Der Server sendet sofort `{heartbeat_interval_ms, resume_window_ms,
   protocol_versions, dpop_nonce, server_time, sync_epoch}` (Schema `GatewayHello`; die Werte kommen
   aus `sync.heartbeat_interval` und `sync.resume_window`). `dpop_nonce` ist frisch und gilt `auth.dpop_nonce_ttl`
   ([08-identitaet.md](08-identitaet.md)).
3. **`identify`.** Innerhalb von `sync.identify_timeout` sendet der Client:
   - Menschen: `access_token`, `dpop` (DPoP-Beweis nach RFC 9449 mit `htm = "GET"`, `htu` =
     HTTPS-Form der Gateway-URL, `nonce` = `dpop_nonce` aus `hello`, `ath` = Hash des Tokens),
     `device_id`, `protocol_version`, `cursor` und `sync_epoch` (beide fehlen beim ersten
     Verbinden), `client` (Plattform und Version).
   - Agenten und Apps: `api_token`, bei `dpop_jkt` am Token zusätzlich `dpop`, `consumer`
     (Name des Verbrauchers, Standard `default`), `protocol_version`.
   Der Server prüft Token, Signatur, Ablauf, `cnf.jkt` gegen den aktiven DPoP-Schlüssel des
   Geräts, Gerät `active`, Sitzung nicht widerrufen, Prinzipal `active`. Ein Gerät im Zustand
   `pending` wird mit `4016` abgewiesen. Dann ruft er `authorize(principal, gateway.connect,
   instance)` auf.
   Der DPoP-Beweis steckt in `identify`, nicht in Kopfzeilen des Upgrades. Grund: nicht jeder
   Client kann Kopfzeilen beim WebSocket-Upgrade setzen; der Server-Nonce aus `hello` verhindert
   Wiedereinspielen.
4. **`ready`.** Der Server antwortet mit `{gateway_session_id, principal, device_id,
   permission_version, head_seq, sync_epoch, snapshot}`. `snapshot` enthält: sichtbare Bereiche und Kanäle
   mit Versionen, Seitenleisten-Zustand, Ungelesen- und Erwähnungszähler, laufende Läufe
   (Zusammenfassung), Präsenz der sichtbaren Agenten, Versionen von Einstellungen und Entwürfen,
   die für den Prinzipal ausgewerteten Feature-Flags.
   Inhalte von Threads sind nicht enthalten; der Client lädt sie per REST.
   Der Server meldet die Verbindung bei der Zustellung an, bevor er `head_seq` liest; jedes
   spätere Ereignis wird gepuffert oder gesendet, jedes frühere steckt in Momentaufnahme oder
   Nachholen. So geht zwischen `ready` und Live-Betrieb nichts verloren.
   Hat der Client einen `cursor` gesendet, beginnt danach das Nachholen (Abschnitt 7).
5. **Heartbeat.** Der Client sendet alle `heartbeat_interval_ms` ein `heartbeat {last_seq}`. Den
   ersten Heartbeat verzögert er um einen Zufallsanteil von `heartbeat_interval_ms`, damit nach
   einem Server-Neustart nicht alle gleichzeitig senden. Der Server antwortet `heartbeat_ack`.
   Fehlen `sync.heartbeat_miss_limit` Heartbeats in Folge, schließt der Server mit `4009`.
   Fehlt dem Client ein `heartbeat_ack` innerhalb von `heartbeat_interval_ms`, schließt er selbst
   und verbindet neu.
6. **Token-Erneuerung.** Vor Ablauf des Zugriffstokens sendet der Server `reauth_required`
   (Vorlauf `sync.reauth_lead_time`). Der Client holt per Refresh einen neuen Token und sendet
   `reauth {access_token, dpop}` mit dem aktuellen Nonce. Ohne gültiges `reauth` bis zum Ablauf
   schließt der Server mit `4011`. Agenten und Apps bekommen kein `reauth_required`; ihr API-Token
   hat ein festes `expires_at`, bei dem der Server mit `4011` schließt. Weiter geht es nur mit
   einem neuen Token, das der Owner ausstellt.
7. **`resume`.** Nach einer Trennung innerhalb von `resume_window_ms` sendet der Client statt
   `identify` ein `resume {gateway_session_id, access_token, dpop, cursor, sync_epoch}`. Der Server
   prüft Token und Sitzung wie bei `identify`, stellt Fokus und Abos wieder her und antwortet
   `resumed`, dann folgt das Nachholen ab `cursor`. Kurzlebige Ereignisse der Pause sind
   verloren. Ist die Gateway-Sitzung unbekannt oder abgelaufen, schließt der Server mit `4006`;
   der Client macht dann `identify`.
8. **Ende.** Der Client schließt mit `1000`. Der Server schließt mit einem Code aus 6.4.

### 6.4 Schließ-Codes

| Code | Name | Client-Reaktion |
|---|---|---|
| `1000` | normal | nicht neu verbinden, außer die App ist im Vordergrund |
| `1001` | going_away | neu verbinden mit Backoff, `resume` versuchen |
| `4000` | unknown_error | neu verbinden mit Backoff |
| `4001` | unknown_op | Fehler melden, neu verbinden |
| `4002` | decode_error | Fehler melden, neu verbinden |
| `4003` | not_identified | Op vor `identify` gesendet; neu verbinden |
| `4004` | authentication_failed | Token erneuern; scheitert das, abmelden |
| `4005` | already_identified | Fehler melden, neu verbinden |
| `4006` | session_invalid | `identify` statt `resume` |
| `4007` | protocol_version_unsupported | App-Update anzeigen, nicht neu verbinden |
| `4008` | rate_limited | nur `rl.gateway_connect` bei `identify` und `resume`: Wartezeit aus dem Schließgrund abwarten |
| `4009` | heartbeat_timeout | neu verbinden mit Backoff, `resume` versuchen |
| `4011` | token_expired | Token erneuern, dann `resume` |
| `4012` | revoked | Sitzung, Gerät oder Prinzipal widerrufen: lokale Daten löschen, abmelden, nicht neu verbinden |
| `4013` | server_restart | sofort mit Zufallsverzögerung neu verbinden, `resume` versuchen |
| `4014` | slow_consumer | Ausgangswarteschlange voll oder Schreibfrist `gateway.write_timeout` verpasst: neu verbinden, Nachholen ab Cursor |
| `4015` | consumer_replaced | nur Agenten: ein anderer Prozess hat denselben Verbraucher übernommen; nicht neu verbinden |
| `4016` | device_pending | Gerät wartet auf Freigabe: nicht neu verbinden, bis `GET /api/v1/devices/{id}` den Zustand `active` meldet |
| `4017` | device_replaced | eine neuere Verbindung desselben Geräts ist aktiv: nicht von selbst neu verbinden, erst bei Rückkehr in den Vordergrund oder auf Bedienung |
| `4018` | forbidden | Anmeldung gültig, aber `gateway.connect` verweigert (z. B. `scope_missing` einer App): Fehler anzeigen, Token nicht erneuern, nicht von selbst neu verbinden |

Die Terminal-Verbindungen (Strom des Geräts, Steuer- und Datenverbindung des Host-Dienstes) sind
eigene WebSocket-Kanäle mit eigenen Schließ-Codes `4100` bis `4114`
([24-terminal.md](24-terminal.md), [asyncapi.yaml](../api/asyncapi.yaml)); die Codes dieser Tabelle
gelten dort nur, wo der Kanal sie nennt.

Zuordnung beim Verbindungsaufbau (Grund: ein Code je Ursache, damit der Client richtig reagiert):

| Ursache | Code |
|---|---|
| kein `identify` und kein `resume` innerhalb von `sync.identify_timeout` | `4003` |
| Rahmen über `sync.ws_max_frame_bytes`, Binärrahmen, ungültiges JSON oder UTF-8, doppelter Schlüssel, unbekanntes Feld | `4002` |
| Token oder DPoP-Beweis ungültig, `device_id` passt nicht | `4004` |
| Anmeldung gültig, aber sonstige Ablehnung `forbidden` (z. B. `scope_missing`); ein neues Token hilft nicht | `4018` |
| Zugriffstoken abgelaufen | `4011` |
| Sitzung, Gerät, Token, Prinzipal oder App-Installation widerrufen bzw. nicht aktiv | `4012` |
| Gerät oder bereitgestellter Mensch wartet auf Freigabe | `4016` |
| Datenbank beim Aufbau von `ready` nicht erreichbar | `4000` |

### 6.5 Neu verbinden

Der Client wartet vor dem Versuch `n` (ab 0) eine Zeit gleichverteilt zwischen 0 und
`min(sync.reconnect_backoff_cap, sync.reconnect_backoff_base * 2^n)` (Full Jitter). Nach einer
erfolgreichen `ready`- oder `resumed`-Antwort beginnt `n` wieder bei 0. Netzwechsel und
Rückkehr in den Vordergrund lösen einen sofortigen Versuch aus, der den Zähler nicht erhöht.

## 7. Nachholen (Catch-up)

### 7.1 Cursor

Der Cursor eines Clients ist die höchste globale `seq`, die der Server für diesen Prinzipal
**geprüft** hat, nicht nur die höchste zugestellte. Darum sendet der Server `cursor {seq}`, wenn
er nicht sichtbare Ereignisse übersprungen hat, spätestens nach `sync.cursor_advance_every`
übersprungenen Ereignissen und am Ende jeder Seite. Der Client speichert den Cursor nach jeder
verarbeiteten Seite dauerhaft.

### 7.2 Ablauf

1. Der Server liest Seiten zu `sync.catchup_page_size`:
   `SELECT ... FROM event_log WHERE seq > $cursor AND seq <= $head AND (channel_id =
   ANY($visible_channels) OR space_id = ANY($fully_visible_spaces) OR (channel_id IS NULL AND
   space_id = ANY($visible_spaces)) OR ($instance_wide AND space_id IS NOT NULL) OR
   audience_principal_id = $principal OR (space_id IS NULL AND channel_id IS NULL AND
   audience_principal_id IS NULL)) ORDER BY seq LIMIT $page`.
   `$visible_channels`, `$visible_spaces` (Bereiche mit mindestens einem sichtbaren Kanal),
   `$fully_visible_spaces` (Bereiche mit Freigabe auf dem Bereich) und `$instance_wide` (Owner
   oder Freigabe auf der Instanz) liefert `authz` aus dem Rechte-Cache.
   Grund für die Bereiche: Ein Kanal, der nach dem Aufbau des Caches in einem sichtbaren Bereich
   entsteht, steht noch nicht in `$visible_channels`; seine Ereignisse dürfen trotzdem nicht
   fehlen.
2. Jedes Ereignis läuft zusätzlich durch `authorize(principal, <receive_action>, resource)`.
   Erst dann wird es als `dispatch` gesendet. Der SQL-Filter ist die Leistung, `authorize` die
   Wahrheit.
3. Neue Live-Ereignisse während des Nachholens werden je Verbindung gepuffert und erst nach dem
   Nachholen in `seq`-Reihenfolge gesendet. Es gibt keine Vermischung.
4. Am Ende sendet der Server `catchup.complete {head_seq}`. Danach ist die Verbindung live.
5. Übersteigt der Puffer `sync.outbound_queue_max`, schließt der Server mit `4014`. Der Client
   setzt ab seinem Cursor neu auf.
6. Kurzlebige Ereignisse stellt der Server vor `catchup.complete` nicht zu, und er verwirft sie,
   wenn die Ausgangswarteschlange voll ist, statt die Verbindung zu schließen (nie `4014`);
   `run.updated` holt der Zustand nach 11.3 Punkt 3 nach. Grund: Sie sind reine Beschleunigung
   (2.1) und dürfen keine Verbindung kosten.

### 7.3 `resync_required`

Der Server sendet `resync_required {reason, head_seq, sync_epoch}` statt Ereignissen, wenn:

- die gesendete `sync_epoch` nicht der aktuellen entspricht (Wiederherstellung aus der
  Sicherung, Abschnitt 7.5), Grund `epoch_changed`;
- `cursor < min_retained_seq - 1` (Ereignisse wurden nach `sync.event_retention` entfernt),
  Grund `retention`;
- `head_seq - cursor > sync.catchup_max_events`, Grund `too_far_behind`;
- `cursor > head_seq`, Grund `cursor_ahead` (zweites Netz, falls ein Client ohne Epoche fragt).

Der Client verwirft dann seinen Server-Zustand im Cache (das Ausgangsfach bleibt), lädt
`GET /api/v1/sync/snapshot`, setzt Cursor und Epoche auf die gelieferten Werte und lädt Threads
bei Bedarf per REST. Auf dem Gateway bleibt die Verbindung offen und ist ab `head_seq` live;
Ereignisse bis zum `head_seq` der neuen Momentaufnahme verwirft der Client nach Abschnitt 9,
Punkt 2. Neu verbinden muss er nicht. Grund: Jeder Schließ-Code würde eine falsche Reaktion
auslösen, und die Live-Zustellung ab `head_seq` verliert nichts.

### 7.4 REST-Variante

`GET /api/v1/events?after=<seq>&epoch=<sync_epoch>&limit=<n>` liefert dieselben Ereignisse mit
denselben Regeln: `{events, next_cursor, head_seq, sync_epoch}` oder `410 resync_required` mit
Grund. Sie dient SDKs, Tests und Adaptern ohne WebSocket. `limit` ist höchstens
`sync.catchup_page_size`.

### 7.5 Sync-Epoche

Die globale Sequenz gilt innerhalb einer Zeitlinie, benannt durch `instance.sync_epoch`
([03-datenmodell.md](03-datenmodell.md), Abschnitt 6.7). Nach einer Wiederherstellung setzt
`bothy restore finalize` eine neue Epoche.

1. `hello`, `ready`, `resumed`, die Momentaufnahme und `GET /api/v1/events` nennen die Epoche.
2. Der Client speichert sie zusammen mit dem Cursor und sendet beide bei `identify`, `resume`
   und beim REST-Nachholen.
3. Passt die gesendete Epoche nicht, antwortet der Server `resync_required` mit Grund
   `epoch_changed`, auch wenn `cursor <= head_seq` ist. Grund: Nach den ersten Schreibvorgängen
   der neuen Zeitlinie überholt sie alte Cursor; ohne Epoche würde ein Client fremde Ereignisse
   mit gleicher Nummer still überspringen.
4. Agenten-Verbraucher, Webhook-Abos und Worker-Verbraucher setzt derselbe Befehl auf den neuen
   `head_seq`; sie holen nichts aus der alten Zeitlinie nach.

### 7.6 Verbraucher im Worker (Push, Webhooks)

Push und ausgehende Webhooks entstehen im `worker` aus dem Log, nicht im Schreibpfad
([02-architektur.md](02-architektur.md), Abschnitt 11.5, [ADR 0051](../adr/0051-worker-verbraucher-des-ereignis-logs-mit-cursor.md)). Je Verbraucher gibt es eine
Zeile in `event_consumer_cursors`. Eine Runde liest ab `last_seq` in `seq`-Reihenfolge, legt je
Ereignis und Empfänger einen Job an und rückt den Cursor in derselben Transaktion vor. Vor dem
Versand prüft jeder Job erneut `push.deliver` bzw. `webhook_out.deliver` mit den aktuellen
Rechten.

## 8. Sichtbarkeit bei Rechte-Änderungen

### 8.1 Grundregel

Es gelten die Rechte zum Zeitpunkt der Zustellung, nicht die zum Zeitpunkt des Ereignisses.
Ein Gateway-Prozess aktualisiert den Rechte-Cache eines Prinzipals, wenn er im Log an dessen
Ereignis `access.changed` ankommt. Alle Ereignisse danach prüft er mit dem neuen Stand.
Strukturänderungen (`channel.created`, `channel.updated` mit `read_only`, `channel.archived`,
`space.archived`, `thread.moved`) wendet er ebenfalls in Log-Reihenfolge auf den Cache an, weil sie
den Pfad oder den Schreibschutz einer Ressource ändern. `channel.archived` und `space.archived` sind
zusätzlich Rechte-Änderungen mit eigenem `access.changed` (05, Spalte "Rechte";
[07-rechte.md](07-rechte.md), Abschnitt 4.5); die übrigen kommen ohne neue Rechte-Version.

### 8.2 Was beim Ändern passiert

Jede Transaktion, die Rechte ändert (Freigabe, Gruppe, Owner, Bediener, Backend-Richtlinie,
Werkzeugklasse, Status eines Prinzipals, Gerät oder Sitzung), erhöht die Rechte-Version und
schreibt für jeden betroffenen Prinzipal ein persönliches Ereignis `access.changed`. Das gilt
auch für den Ablauf von Freigaben und Mitgliedschaften: Er wirkt bei jedem `authorize` sofort,
und der Job `authz.expiry_sweep` schreibt alle `authz.expiry_sweep_interval` die Rechte-Version,
`access.changed` und `run.cancel_requested` für die Abgelaufenen (Systemaktionen
`grant.expire`, `group.member.expire`). Vor einem Ablauf bekommt das Subjekt einmal
`access.expiring` ([07-rechte.md](07-rechte.md), Abschnitt 9.4).

```json
{
  "permission_version": 418,
  "added":   [{"kind": "channel", "id": "...", "level": "write"}],
  "removed": [{"kind": "space", "id": "..."}],
  "changed": [{"kind": "channel", "id": "...", "level": "read", "previous_level": "trigger"}]
}
```

Ein Eintrag kann `kind: "host"` haben: Er meldet das Zusatzrecht `terminal` auf einem Rechner,
ohne `level`, mit `capabilities: ["terminal"]` ([24-terminal.md](24-terminal.md), Abschnitt 8). Wer
es verliert, dessen Terminal-Sitzungen auf dem Rechner enden in derselben Transaktion
(`access_revoked`).

Betroffen sind alle Prinzipale, deren sichtbare Ziele oder Stufen sich ändern können: bei Freigaben
das Subjekt (bei Gruppen alle aktuellen Mitglieder), bei Owner-Änderung und Sperre die Person, beim
Bediener-Wechsel der alte und der neue Bediener-Agent, beim Archivieren eines Bereichs oder Kanals
jeder mit einer Stufe über `read` auf dem Ziel (Freigaben auf Ziel und Vorfahren, Gruppenmitglieder,
Owner, Bediener). Ändert eine Aktion ein Ziel statt eines Subjekts, bestimmt `authz` die Betroffenen
selbst aus dem Ziel ([02-architektur.md](02-architektur.md), Abschnitt 8.1). Den Inhalt berechnet
`authz` als Unterschied der sichtbaren Ziele vor und nach der Änderung; wer keinen Unterschied hat,
bekommt kein `access.changed`.

### 8.3 Was der Client bei Entzug bekommt und tut

1. Der Server sendet `access.changed` sofort, ohne auf andere Ereignisse zu warten.
2. Er stellt danach kein Ereignis mehr aus den entfernten Zielen zu, beendet deren Fokus und
   verwirft gepufferte Ereignisse dieser Ziele.
3. Der Client löscht für entfernte Ziele alle Inhalte aus dem lokalen Cache: Threads,
   Nachrichten, Dateien, Vorschaubilder, Suchindex, Entwürfe. Er schließt offene Ansichten mit
   dem Hinweis "Kein Zugriff mehr".
4. Einträge im Ausgangsfach für entfernte Ziele scheitern beim Senden mit `404`. Der Client
   markiert sie als fehlgeschlagen und behält den Text lokal zum Kopieren.
5. Hat der Prinzipal die Stufe `trigger` auf einem Thread verloren, in dem ein von ihm
   ausgelöster Lauf aktiv ist, schreibt der Server `run.cancel_requested` mit Grund
   `access_revoked` an den Agenten. Der Agent bricht ab und meldet `cancelled` (fail closed,
   Ruling L23). Dasselbe gilt mit Grund `channel_read_only`, wenn ein Kanal schreibgeschützt
   wird.
6. Push: Jeder Push läuft vor dem Versand durch `authorize`. Es gibt nichts zu löschen.

### 8.4 Bei Erweiterung

Der Client bekommt `access.changed` mit `added`. Ältere Ereignisse der neuen Ziele vor seinem
Cursor holt er nicht nach; er lädt die Struktur der neuen Ziele per REST und Inhalte bei Bedarf.

### 8.5 Widerruf von Sitzung, Gerät oder Prinzipal

Der Server schließt alle betroffenen Verbindungen sofort mit `4012`. Der Client löscht alle
lokalen Daten dieses Kontos. Zugriffstokens tragen die Sitzungs-ID `sid`; `authorize` prüft
den Widerrufsstand aus einem Cache, der an der Rechte-Version hängt. Ein widerrufener Token ist
damit vor seinem Ablauf wirkungslos.

## 9. Zustellgarantie

1. Dauerhafte Ereignisse werden mindestens einmal zugestellt (at-least-once). Doppel entstehen
   bei Wiederverbinden und Wiederholungen.
2. Der Client verwirft jedes Ereignis mit `seq <= ` seinem gespeicherten Cursor. Weil eine
   Verbindung streng in `seq`-Reihenfolge liefert, reicht dieser Vergleich.
3. Zusätzlich wendet der Client eine Änderung an einem Objekt nur an, wenn
   `resource.version` größer ist als die lokale Version. Damit sind auch Daten aus REST und aus
   Ereignissen ohne Konflikt mischbar. `resource.version` steht genau dann, wenn die Tabelle des
   Objekts eine Spalte `version` hat (03) und das Ereignis deren neuen Wert meldet. Es fehlt
   darum auch bei diesen Typen, deren Objekt eine Version hat, die das Ereignis nicht ändert:
   - `reaction.*` und `pin.*`: Sie erhöhen `messages.version` nicht (sonst bekäme der Autor nach
     jeder Reaktion `412` beim Bearbeiten); ihre Nutzlast trägt absolute Werte (Anzahl je Emoji,
     angeheftet ja oder nein).
   - `read_state.updated` (Ressource `thread`) und `sidebar.updated` (Ressource `space`): Ihre
     Nutzlast trägt die Version des persönlichen Teilobjekts; der Client vergleicht diese.
   - `channel.settings_changed` und `agent.config_changed` (Ressource `channel`, Nutzlast mit
     `settings_version`), `thread.moved_out` (der Quellkanal ändert sich nicht),
     `agent_session.changed` (Ressource `thread`), `agent.invocation`, `agent.followup` und
     `run.cancel_requested` (Ressource `run`), `command.invoked` (Ressource `command`).
   Ohne `resource.version` wendet der Client das Ereignis in `seq`-Reihenfolge an. Grund: Eine
   fremde Version im Feld des Objekts würde spätere echte Änderungen verwerfen lassen. Der Katalog
   ([ADR 0049](../adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle.md)) trägt je Ereignistyp, ob er `resource.version` hat; die Rahmen-Konformität (G9,
   [06-api.md](06-api.md) Abschnitt 15) prüft jede Zustellung dagegen.
4. Kurzlebige Ereignisse werden höchstens einmal zugestellt (at-most-once).

## 10. Reihenfolge

| Bereich | Garantie |
|---|---|
| global, dauerhafte Ereignisse | Zustellung in `seq`-Reihenfolge = Commit-Reihenfolge |
| je Thread | streng nach `thread_seq`; gleich für alle Empfänger |
| je Objekt | streng nach `resource.version` |
| kurzlebig | keine Garantie untereinander; `run.updated` ist über `version` geordnet |
| zwischen Prozessen | jeder Gateway-Prozess liest dasselbe Log in derselben Reihenfolge |

Zeitstempel ordnen nie etwas.

## 11. Live-Box-Streaming

### 11.1 Lauf-Dokument

Ein Lauf hat ein Dokument (Schema `run_document.v1`): `state`, `version`, `status_line`,
`last_text_excerpt`, `model`, `metrics` (inklusive `last_api_retry`), `cost`,
`billing_mode_used`, `plan` (`items` mit `id`, `text`, `status`), `steps` (Objekt, Schlüssel ist
die Schritt-ID, jeder Schritt mit `ordinal`, `parent_step_id`, `kind`, `name`, `title`,
`status`, Zeiten), `followups` (Objekt, Schlüssel ist die Nachrichten-ID), Zeiten. Welche Pfade
der Adapter ändern darf, legt [09-agenten.md](09-agenten.md), Abschnitt 5.3 fest. Schritte sind als
Objekt gespeichert, nicht als Liste, damit Patches über stabile Pfade laufen; die Reihenfolge
kommt aus `ordinal`.

### 11.2 Vom Adapter zum Server

Der Adapter sendet `run.update {run_id, base_version, ops}` (WebSocket) oder `PATCH
/api/v1/runs/{id}` (REST). `ops` ist eine Liste von Operationen nach RFC 6902, beschränkt auf
`add`, `replace` und `remove`, mit JSON-Pointer-Pfaden ins Lauf-Dokument. Der Server prüft:
Prinzipal ist der Agent des Laufs, Lauf ist aktiv, `base_version` passt, Pfade sind erlaubt
(z. B. nicht `state`; Zustände wechseln nur über eigene Endpunkte). Er speichert die Änderung,
erhöht `runs.version` und antwortet mit der neuen Version. Bei falscher `base_version`
antwortet er `409 base_version_conflict` mit dem aktuellen Dokument. Der Adapter drosselt selbst auf höchstens eine
Änderung je `agent.run_update_min_interval`.

### 11.3 Vom Server zu den Clients

1. Zustandswechsel (`queued`, `running`, `waiting_input`, Ende) sind dauerhafte Ereignisse
   `run.state_changed` mit Zusammenfassung (Zustand, Dauer, Kosten, Zahl der Schritte). Daraus
   speisen sich Seitenleiste und "Läuft gerade".
2. Inhaltliche Änderungen sind das kurzlebige Ereignis `run.updated {run_id, base_version,
   version, ops}`. Es geht nur an Verbindungen, die den Thread im `focus` haben und
   `run.read` dürfen. Ein Client fokussiert höchstens `sync.max_focused_threads` Threads; ein
   `focus` mit mehr IDs ergibt `error` mit `validation_failed`, und der bisherige Fokus bleibt.
3. Zusammenfassen: Der Server sammelt je Lauf und Verbindung Änderungen für
   `sync.run_patch_coalesce_interval`. Dann sendet er einen Rahmen von der ältesten
   `base_version` bis zur neuesten `version`. Spätere `replace` auf denselben Pfad ersetzen
   frühere. Ist die Ausgangswarteschlange der Verbindung voll, verwirft der Server die
   gesammelten Patches und sendet stattdessen das ganze Dokument als `run.updated` mit
   `snapshot`.
4. Client-Regel: Ist `base_version` gleich der lokalen Version, wendet er die Ops an. Ist
   `version` kleiner oder gleich der lokalen, ignoriert er den Rahmen. Sonst (Lücke) holt er
   `GET /api/v1/runs/{id}` und ersetzt sein Dokument. Er zeichnet immer nur den neuesten Stand.
5. Beim Fokussieren eines Threads mit aktivem Lauf sendet der Server einmal das ganze Dokument.
6. Nach dem Ende klappt die Live-Box auf eine Zeile zusammen. Die Zusammenfassung kommt aus
   `run.state_changed`; Details lädt der Client beim Aufklappen per REST.

## 12. Tippen und Agenten-Präsenz

1. **Tippen.** Der Client sendet `typing.start {thread_id}` höchstens einmal je
   `sync.typing_min_interval`, solange getippt wird, und `typing.stop` beim Leeren oder
   Senden. Der Server prüft `typing.start` (Aktion in [05-aktionen.md](05-aktionen.md); nur
   Menschen) und sendet `typing.started {thread_id, principal_id, expires_at}` an Verbindungen mit
   Fokus auf dem Thread und Leserecht, nicht an Verbindungen des Tippenden selbst. Ohne Erneuerung
   endet die Anzeige nach `sync.typing_ttl`; Clients beenden sie auch bei der nächsten Nachricht
   dieses Prinzipals im Thread. `typing.stop` erzeugt kein Ereignis; es setzt nur die Drossel der
   Verbindung zurück, damit ein neues `typing.start` sofort gilt. Ein laufender Lauf zeigt das
   Tippen des Agenten; Agenten senden kein eigenes Tippen (Ruling L9).
2. **Präsenz von Agenten.** Adapter senden `POST /api/v1/agents/me/heartbeat` (oder implizit
   über das Gateway-`heartbeat`) mit Zustand, Laufzeit-Version, `reported_billing_mode`,
   `billing_modes_supported`, `billing_evidence`, Werkzeugklasse und aktiven Läufen (Schema
   `agent_heartbeat.v1`, [09-agenten.md](09-agenten.md), Abschnitt 4.8). Adapter, die je Lauf
   dem Soll folgen, lassen `reported_billing_mode` weg und nennen beide Modi in
   `billing_modes_supported`; bindend ist dann die Prüfung je Lauf (Ruling L8). Ein Wechsel des
   Ist-Modus oder der Werkzeugklasse wird sofort gespeichert und auditiert. Der Server leitet
   `online`, `degraded` oder `offline` ab; `offline` gilt nach `agent.presence_timeout` ohne
   Heartbeat. Bei einer Änderung sendet er das kurzlebige `agent.presence_changed {agent_id,
   state, reported_billing_mode}` an alle mit `agent.read`. `ready` enthält den aktuellen Stand.
   Menschen haben keine Präsenz-Anzeige.
3. **Mehrere Knoten.** Mit `cluster.mode = multi` verteilt ein Knoten Tippen, Präsenz,
   Hinweise und Live-Box-Wechsel zusätzlich über `pg_notify('ephemeral', ...)`, nur mit IDs und
   Versionen; der empfangende Knoten liest Inhalte aus der Datenbank
   ([03-datenmodell.md](03-datenmodell.md), Abschnitt 6.6).
4. **Live-Statistik.** Ein Mensch mit `stats.read` sendet `stats.subscribe {space_id?}`. Der
   Server prüft `stats.read` auf Instanz bzw. Bereich und sendet danach alle `stats.live_interval`
   das kurzlebige Ereignis `stats.live` mit einem Live-Rahmen (Schema `StatsLiveFrame`,
   [25-visualisierung.md](25-visualisierung.md), Abschnitt 5) an diese Verbindung. Ein neues
   `stats.subscribe` ersetzt das Abo, `stats.unsubscribe` beendet es, `resume` stellt es wieder
   her. Jeder Rahmen läuft durch den Zustellmodus mit der Empfangs-Aktion `stats.read` und ist auf
   die sichtbaren Kanäle des Empfängers gefiltert. Verliert der Empfänger das Recht, endet das Abo
   mit einem `error` (`forbidden`). Agenten und Apps können nicht abonnieren (`stats.read` verlangt
   Owner oder `manage`). Mit `cluster.mode = multi` baut jeder Knoten seine Rahmen selbst; Raten
   kommen aus `stats_minutes` und sind darum auf allen Knoten gleich.

## 13. Offline-Verhalten des Clients

### 13.1 Lokaler Cache

Der Client hält Struktur, zuletzt geöffnete Threads, Nachrichten, Vorschaubilder, eigene
Einstellungen und Entwürfe in einer lokalen, verschlüsselten Datenbank (Schutzklasse je
Plattform siehe Sicherheitskapitel). Der Cache ist nur eine Kopie; die Wahrheit liegt im
Server. Einträge älter als `client.cache_max_age` ohne Zugriff werden entfernt.

### 13.2 Ausgangsfach (Outbox)

1. Jede schreibende Bedienung landet zuerst im Ausgangsfach, mit eigener ID
   (`client_msg_id` für Nachrichten, `client_interaction_id` für Interaktionen,
   `Idempotency-Key` für alles andere), Zeitpunkt und erwarteter Version, wo nötig.
2. Das Ausgangsfach ist je Thread streng FIFO. Der Client sendet den nächsten Eintrag eines
   Threads erst nach der Antwort auf den vorherigen. Verschiedene Threads laufen parallel.
3. Die Oberfläche zeigt wartende Nachrichten sofort mit Zustand "wird gesendet" an der Stelle
   nach der letzten bestätigten `thread_seq`.
4. Wiederholung bei Netzfehler, Zeitüberschreitung, `5xx` und `429`: Backoff mit Full Jitter
   zwischen `client.outbox_retry_base` und `client.outbox_retry_cap`. Dieselbe ID bei jeder
   Wiederholung.
5. Dauerhafte Fehler (`400`, `404`, `410`, `413`, `422`) beenden die Wiederholung. Der Eintrag
   bleibt als "fehlgeschlagen" mit Grund sichtbar; der Mensch kann bearbeiten, erneut senden
   (neue ID) oder verwerfen. Nichts wird still verworfen.
6. Kommt das Echo-Ereignis `message.created` mit passender `client_msg_id`, ersetzt der Client
   den wartenden Eintrag durch die Server-Nachricht.

### 13.3 Bearbeiten und Löschen

| Fall | Regel |
|---|---|
| Bearbeiten einer noch nicht gesendeten Nachricht | ändert den Eintrag im Ausgangsfach; keine Server-Anfrage |
| Löschen einer noch nicht gesendeten Nachricht | entfernt den Eintrag; wurde er schon gesendet, aber nicht bestätigt, folgt ein Lösch-Eintrag |
| Bearbeiten einer gesendeten Nachricht | `PATCH` mit `If-Match` (Version). Passt die Version nicht: `412 version_conflict` mit aktueller Fassung (RFC 9110). Der Client zeigt beide Fassungen; der Mensch entscheidet. Keine automatische Zusammenführung. |
| Bearbeiten einer inzwischen gelöschten Nachricht | `410 message_deleted`; der Eintrag wird verworfen, der Text bleibt zum Kopieren |
| Löschen einer gesendeten Nachricht | `DELETE` ist idempotent; schon gelöscht ergibt `204` |
| Thread inzwischen erledigt | Senden öffnet ihn wieder (Baseline); kein Konflikt |
| Thread verschoben | ID bleibt gleich; Senden klappt ohne Änderung |

### 13.4 Abgleich nach dem Wiederverbinden

1. Verbinden mit `resume` oder `identify` und gespeichertem Cursor.
2. Nachholen bis `catchup.complete`. Echos eigener Nachrichten bestätigen Einträge im
   Ausgangsfach.
3. Danach sendet der Client das Ausgangsfach in Reihenfolge.
4. Bei `resync_required` bleibt das Ausgangsfach unberührt und wird nach dem Neuladen
   gesendet.

## 14. Geräteübergreifende Synchronisation

Persönliche Daten ändern sich über REST. Jede Änderung schreibt ein persönliches dauerhaftes
Ereignis (`audience = self`); alle Geräte des Menschen bekommen es über das Log. Die
Gewinnerregel ist die Commit-Reihenfolge im Server, ausgedrückt durch die Version.

| Daten | Ereignis | Regel |
|---|---|---|
| Gelesen-Stand | `read_state.updated` | monoton: `last_read_seq = GREATEST(alt, neu)`. Zurücksetzen ("als ungelesen markieren") ist eine eigene Operation mit `If-Match`. Importiertes zählt über `threads.unread_floor_seq` nie als ungelesen (Abschnitt 15.8). Ungelesen- und Erwähnungszähler speichert der Server nicht (03 `read_states`): Momentaufnahme, `listMyReadStates` und `read_state.updated` nennen den berechneten Wert; dazwischen zählt der Client selbst aus `message.created`, `message.updated` und `message.deleted` (`mention_ids`). Eine neue Erwähnung schreibt kein persönliches Ereignis. Der Server rückt den Gelesen-Stand des Autors beim Senden nicht vor; das tut dessen Client per REST wie jede andere Änderung persönlicher Daten. |
| Entwürfe | `draft.updated`, `draft.deleted` | letzte Schreibung gewinnt in Server-Reihenfolge; jede Schreibung erhöht `version`. Der Client sendet `base_version`; ist sie älter als die aktuelle, antwortet der Server mit `overwrote_newer: true`, und der Client zeigt einen Hinweis. Eingehende Änderungen eines anderen Geräts übernimmt ein Gerät nur, wenn der Mensch dort gerade nicht in diesem Entwurf tippt; sonst gewinnt die nächste lokale Schreibung. Erfolgreiches Senden mit `clear_draft: true` löscht den Entwurf in derselben Transaktion. Ein leerer Entwurf (kein Text außer Leerraum, keine Dateien) wird gelöscht; ein `PUT` mit leerem Entwurf antwortet `200` mit dem leeren Endzustand. `file_ids` prüft der Server beim Speichern nur in Form und Anzahl (`message.max_attachments`); Eigentum und Zustand prüft das Senden ([12-dateien.md](12-dateien.md), Abschnitt 10). |
| Einstellungen | `settings.updated` | je Schlüssel letzte Schreibung gewinnt; Änderungen an verschiedenen Schlüsseln stören sich nicht. Standardwerte speichert der Server nicht, weil manche vom Gerät oder der Instanz abhängen; der Client wendet die Standards aus 01 und 13 an. Löschen eines Schlüssels schreibt `settings.updated` mit `deleted: true`. |
| Seitenleiste | `sidebar.updated` | je Bereich (`user_space_states`) letzte Schreibung gewinnt |
| Benachrichtigung, Ruhezeiten | `notification_pref.updated`, `quiet_hours.updated` | je Zeile letzte Schreibung gewinnt |

Ein Gerät erkennt sein eigenes Echo an `updated_by_device_id` und ignoriert es.

Listen persönlicher Daten (`listMyReadStates`, `listMyDrafts`, Momentaufnahme) nennen nur Threads
und Kanäle, die der Mensch noch lesen darf ([07-rechte.md](07-rechte.md), Abschnitt 7.4, Regel 1).
Die Zeilen bleiben gespeichert, bis ihre Aufbewahrung endet; bekommt der Mensch das Leserecht
zurück, erscheinen sie wieder.

## 15. Agenten-Adapter

### 15.1 Verbindung und Cursor

1. Ein Adapter verbindet sich wie ein Client, mit `api_token` und `consumer`.
2. Je `(agent, consumer)` gibt es einen dauerhaften Cursor in `agent_consumer_cursors`. Beim
   ersten Verbinden setzt der Server ihn auf den aktuellen `head_seq`. Ein Adapter holt also nie
   Ereignisse aus der Zeit vor seiner Aktivierung nach.
3. Je Verbraucher gibt es genau eine Verbindung. Ein neues `identify` mit demselben Verbraucher
   schreibt seine Gateway-Sitzung in `agent_consumer_cursors.holder_session_id` und schließt die
   ältere Verbindung mit `4015`. Grund: zwei Prozesse am selben Strom würden doppelt arbeiten.
   Über mehrere api-Knoten hinweg hält das Fencing: Ein Knoten speichert `acked_seq` nur mit
   `WHERE holder_session_id = <eigene Sitzung>`; bekommt er keine Zeile zurück, schließt er
   seine Verbindung mit `4015`. Bis dahin verhindern Ereignis-ID und Lauf-Start per
   Compare-and-swap doppelte Arbeit.
4. Der Server liefert ab `acked_seq + 1` in `seq`-Reihenfolge, gefiltert wie bei Menschen (der
   Agent sieht, was seine Freigaben und die Bediener-Regel erlauben) plus persönliche Ereignisse
   an ihn (`agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`,
   `command.invoked`, `agent.config_changed`, `access.changed`) und `agent.updated` zu sich
   selbst (Soll-Änderung der Backend-Richtlinie oder der Werkzeugklasse).
5. Kurzlebig, nur an eine aktive Verbindung des Agenten: `command.autocomplete_requested`
   (Abschnitt 15.7).

### 15.2 Bestätigung und erneute Zustellung

1. Der Adapter sendet `ack {seq}` nach dauerhafter Verarbeitung. Die Bestätigung gilt für alle
   Nummern bis `seq`.
2. Höchstens `agent.max_inflight_events` unbestätigte Ereignisse sind unterwegs. Danach wartet
   der Server.
3. Ohne Bestätigung innerhalb von `agent.ack_timeout` stellt der Server ab `acked_seq + 1` erneut
   zu, mit dem Zustellfeld `redelivered: true` im Umschlag.
4. Der Server speichert `acked_seq` spätestens nach `agent.ack_persist_interval`. Nach einem
   Server-Neustart kann es darum Doppel geben; der Adapter dedupliziert über die Ereignis-ID und
   über Lauf-IDs.

### 15.3 Aufruf und Lauf-Start

1. **Annahme.** Eine auslösende Nachricht (Entscheidung in [09-agenten.md](09-agenten.md),
   Abschnitt 3.4) schreibt in ihrer Transaktion: Nachricht mit `trigger_state = received`, einen
   Lauf `queued` mit `trigger_kind`, den Live-Box-Anker und das persönliche Ereignis
   `agent.invocation` an den Agenten (ohne Marke). Ausnahme: Hat die Nachricht Anhänge, die noch
   nicht endgültig sind (Scan läuft), oder ist sie eine Sprachnachricht, deren Transkript der
   Server erstellt, schreibt erst der Job, der den letzten Anhang endgültig macht, das Ereignis
   (`ReleaseInvocation` im Modul `runs`), spätestens nach `file.invocation_hold_max` bzw.
   `voice.transcript_hold_max` ([12-dateien.md](12-dateien.md), Abschnitt 10). Grund: Ein Agent
   bekommt nie ungeprüfte Bytes, und ein wartender Aufruf blockiert nicht die anderen Ereignisse
   des Verbrauchers.
2. **Zustellung.** Bei jeder Zustellung von `agent.invocation` prüft der Server unmittelbar
   davor `authorize(trigger_principal, agent.invoke, thread)` mit den aktuellen Rechten, die
   Backend-Richtlinie (I2) und die Schutzregeln SR2 und SR3. Scheitert die Prüfung, stellt er
   nicht zu, setzt den Lauf auf `failed` mit Grund, setzt `trigger_state = failed` mit
   `trigger_state_reason` und schreibt einen Audit-Eintrag (fail closed). Besteht sie, erzeugt
   er eine frische Aufruf-Marke (JWS, EdDSA, Claims in [05-aktionen.md](05-aktionen.md),
   Abschnitt 5) mit Gültigkeit `agent.invocation_token_ttl`, speichert ihre `jti` in
   `runs.invocation_jti`, setzt `runs.billing_mode_expected` auf das aktuelle Soll und legt die
   Marke als Zustellfeld `invocation_token` in den Umschlag. Mit der ersten erfolgreichen
   Zustellung wechselt `trigger_state` von `received` auf `queued`.
3. **Prüfung im Adapter.** Der Adapter prüft Kopf (`typ = agent-invocation+jwt`, `alg =
   EdDSA`), Signatur über die JWKS des Servers, Ablauf, Zielgruppe (sich selbst), dass sein
   Ist-Modus für genau diesen Lauf gleich `billing_mode` der Marke ist und dass seine
   Werkzeugklasse nicht höher als `tool_class` ist. Passt etwas nicht, meldet er
   `POST /api/v1/runs/{id}/finish` mit `state = failed` und Grund, z. B. `billing_mode_mismatch`.
4. **Start.** Sonst holt er den Kontext (`run.context.read`) und startet mit
   `POST /api/v1/runs/{id}/start {invocation_token, billing_mode_used, external_session_ref,
   model, billing_evidence}`; `external_session_ref` fehlt, wenn die Laufzeit die Referenz erst
   im Lauf erzeugt, dann legt die erste `agent_session.report` die Sitzungszeile an. Der Server
   prüft Signatur, Ablauf, `jti` gleich `runs.invocation_jti` und Ist-Modus gleich Soll und
   macht `queued -> running` als Compare-and-swap; er speichert `runs.started_jti`. Eine
   Wiederholung mit derselben Marke (Antwort verloren) liefert `200` mit demselben Ergebnis.
   Eine andere, ältere Zustellung desselben Aufrufs bekommt `409 already_started`; der Adapter
   startet dann nichts. Weicht `billing_mode_used` vom Soll ab, lehnt der Server ab und setzt
   den Lauf auf `failed` mit `billing_mode_mismatch`.
5. **Wiederaufnahme.** Wartet ein Lauf (`waiting_input`), ersetzen auch Zustellungen von
   `agent.followup` und `interaction.created` die Marke in `runs.invocation_jti`. Der Adapter
   setzt den Lauf mit dieser Marke wieder auf `running` (`POST /api/v1/runs/{id}/resume`, Aktion
   `run.start`, dieselben Prüfungen). Zustellungen während `running` ändern
   `runs.invocation_jti` nicht.
6. Mit dem Start wechselt `trigger_state` auf `working`, am Ende auf `done`, `failed` oder
   `cancelled`.

### 15.4 Nachträge

1. Nur eine Nachricht, die nach [09-agenten.md](09-agenten.md), Abschnitt 3.4 den Agenten des
   aktiven Laufs auslösen würde, wird Nachtrag: Zeile in `run_followups` (`queued`) und, wenn der
   Adapter Nachträge in den laufenden Turn geben kann (`followups = steer`), das persönliche
   Ereignis `agent.followup` an den Agenten. Andere Nachrichten bleiben normale Nachrichten und
   erreichen den Agenten nur als Kontext des nächsten Laufs. Würde die Nachricht einen anderen
   Agenten auslösen, wird sie abgelehnt (`run_active_other_agent`).
2. Die Zustellung prüft wie in 15.3 Schritt 2 (ein Nachtrag ist eine Auslösung) und hängt eine
   Marke an (`trigger_kind = followup`).
3. Der Adapter meldet `delivered` (an die Laufzeit übergeben) und `absorbed` (von der Laufzeit
   aufgenommen) als Patch auf `followups/<message_id>`. Bei `delivered` setzt der Server
   `trigger_state = followup_delivered`.
4. Endet der Lauf mit Nachträgen im Zustand `queued` oder `delivered` ohne `absorbed`, setzt
   der Server sie auf `deferred` und legt einen neuen Lauf an: die erste dieser Nachrichten ist
   die auslösende, die übrigen sind seine Nachträge. Endet er mit `cancelled` und Grund
   `user_cancelled`, werden sie `discarded` ([09-agenten.md](09-agenten.md), Abschnitt 7).
5. Ein Nachtrag an einen Lauf in `waiting_input` ist die Antwort auf die offene Rückfrage; der
   Adapter nimmt den Lauf mit dessen Marke wieder auf (15.3 Schritt 5).
6. Steuerbefehle (`commands.control = true`, z. B. Stopp) sind nie Nachträge; sie gehen als
   `command.invoked` sofort an den Agenten.

### 15.5 Abbruch

`run.cancel` (Mensch), Rechte-Entzug (8.3), Sperre des Agenten oder des Auslösers und
Schreibschutz des Kanals schreiben `run.cancel_requested` mit Grund an den Agenten. Der Agent
beendet die Laufzeit und meldet `cancelled`. Meldet er nichts innerhalb von `agent.cancel_grace`,
setzt der Server den Lauf selbst auf `cancelled` mit Grund `cancel_timeout`; spätere Patches des
Adapters zu diesem Lauf werden abgelehnt.

### 15.6 Webhook als Transport

Ein Adapter ohne laufenden Prozess (z. B. eine Funktion) kann Ereignisse per Webhook bekommen
statt über das Gateway ([09-agenten.md](09-agenten.md), Abschnitt 4.2). Das Abo ist an den
Agenten gebunden (`webhook_subscriptions.agent_id`). Es gelten dieselben Filter und Prüfungen wie
in 15.1 bis 15.3. Je Abo geht das nächste Ereignis erst nach `2xx` auf das vorige hinaus; `2xx`
gilt als Bestätigung bis zu dieser `seq`. Jede Zustellung, auch eine Wiederholung, trägt eine
frisch ausgestellte Marke. Kurzlebige Ereignisse gibt es auf diesem Weg nicht.

### 15.7 Autovervollständigung

1. Ein Client fragt Vorschläge für eine Option mit `autocomplete = true` an
   (`command.autocomplete`). Der Server prüft wie `agent.invoke` ohne Lauf (inklusive I2) bzw.
   bei App-Befehlen `required_level` und Scope.
2. Er sendet das kurzlebige Ereignis `command.autocomplete_requested {request_id, command_id,
   channel_id, thread_id, user_id, option, value, options, expires_at}` an eine aktive Gateway-Verbindung des Eigentümers, bei Agenten
   mit einer Marke (`trigger_kind = autocomplete`, ohne `run_id`). Apps ohne Gateway fragt der
   Server über ihre Interaktions-Adresse ([11-erweiterungen.md](11-erweiterungen.md),
   Abschnitt 9).
3. Der Eigentümer antwortet mit der Op `command.autocomplete_result {request_id, choices}` oder
   per REST (`command.autocomplete.respond`), höchstens `command.autocomplete_max_choices`
   Einträge.
4. Ohne Antwort in `command.autocomplete_timeout` liefert der Server eine leere Liste. Ein
   Handler ruft nie ein Modell auf.

### 15.8 Schreiben eines Imports

Ein Import-Stapel (`import.apply`, [22-import-migration.md](22-import-migration.md)) betrifft
genau einen Thread und ist eine Transaktion. Er schreibt statt `message.created` je Nachricht ein
`thread.updated` für den Thread und ein `import.progress`; Clients laden den Thread per REST,
wenn sie ihn anzeigen. Am Ende des Stapels setzt der Server `threads.unread_floor_seq` auf das
Ende des Stapels; Importiertes zählt damit für niemanden als ungelesen (03, `read_states`). Importierte Nachrichten haben `origin = import`, lösen nie Agenten aus und erzeugen
keine Hinweise.

## 16. Sequenzdiagramme

### 16.1 Nachricht senden, online

```mermaid
sequenceDiagram
    autonumber
    participant C as Client (Alex)
    participant API as API-Prozess
    participant DB as PostgreSQL
    participant GW as Gateway-Prozesse
    participant O as Andere Clients (Sam)
    C->>C: client_msg_id erzeugen, ins Ausgangsfach
    C->>API: POST /api/v1/threads/{id}/messages (client_msg_id, DPoP)
    API->>API: authorize(Alex, message.create, thread)
    API->>DB: SELECT nach (author, client_msg_id)
    DB-->>API: keine Zeile
    API->>DB: BEGIN, Thread-Zähler, INSERT message
    API->>DB: UPDATE sequence_counters event_seq, INSERT event_log, pg_notify
    API->>DB: COMMIT
    API-->>C: 201 message (thread_seq, version)
    DB-->>GW: NOTIFY event_log (seq)
    GW->>DB: SELECT event_log WHERE seq > Stand
    GW->>GW: authorize je Empfänger (message.read)
    GW-->>C: dispatch message.created (Echo, client_msg_id)
    GW-->>O: dispatch message.created
    C->>C: Eintrag im Ausgangsfach bestätigt
```

### 16.2 Senden offline, dann Wiederverbinden

```mermaid
sequenceDiagram
    autonumber
    participant C as Client (Alex)
    participant API as API-Prozess
    participant GW as Gateway
    C->>C: kein Netz: Nachricht mit client_msg_id ins Ausgangsfach
    C->>C: Anzeige "wird gesendet"
    Note over C: Netz zurück
    C->>GW: Verbinden, hello
    GW-->>C: hello (dpop_nonce)
    C->>GW: resume (cursor = 900)
    GW-->>C: resumed, dispatch 901..950, catchup.complete
    C->>API: POST message (client_msg_id)
    API--xC: Zeitüberschreitung (Antwort verloren, Nachricht aber gespeichert)
    C->>API: POST message (gleiche client_msg_id), Backoff
    API->>API: Paar existiert, Hash gleich
    API-->>C: 200 Original, Idempotent-Replay: true
    GW-->>C: dispatch message.created (seq 951, client_msg_id)
    C->>C: Eintrag bestätigt, genau eine Nachricht
```

### 16.3 Wiederverbinden mit Nachholen

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant GW as Gateway
    participant AZ as authz
    participant DB as PostgreSQL
    C->>GW: Upgrade
    GW-->>C: hello (heartbeat_interval_ms, resume_window_ms, dpop_nonce)
    C->>GW: identify (access_token, dpop, device_id, cursor = 1200)
    GW->>AZ: authorize(gateway.connect), sichtbare Kanäle
    AZ-->>GW: Rechte-Stand (permission_version)
    GW-->>C: ready (head_seq = 5300, snapshot)
    alt cursor älter als Aufbewahrung oder zu weit zurück
        GW-->>C: resync_required (reason), Verbindung bleibt live ab head_seq
        C->>C: Cache verwerfen, Ausgangsfach behalten
        C->>GW: GET /api/v1/sync/snapshot, Cursor auf dessen head_seq
    else Nachholen möglich
        loop Seiten zu sync.catchup_page_size
            GW->>DB: SELECT event_log WHERE seq > cursor AND sichtbar
            GW->>AZ: authorize je Ereignis (receive_action)
            GW-->>C: dispatch ... (nur sichtbare)
            GW-->>C: cursor (letzte geprüfte seq)
        end
        Note over GW: Live-Ereignisse in dieser Zeit gepuffert
        GW-->>C: catchup.complete (head_seq)
        GW-->>C: gepufferte Live-Ereignisse in seq-Reihenfolge
    end
    C->>GW: heartbeat (last_seq), Ausgangsfach senden
```

### 16.4 Entzug während offener Verbindung

```mermaid
sequenceDiagram
    autonumber
    participant O as Owner
    participant API as API-Prozess
    participant DB as PostgreSQL
    participant GW as Gateway
    participant S as Client (Sam)
    participant A as Agenten-Adapter
    O->>API: POST /api/v1/grants/{id}/revoke (Step-up)
    API->>DB: BEGIN, permission_version sperren und erhöhen
    API->>DB: grants.revoked_at setzen, I1 prüfen
    API->>DB: event_log access.changed (audience Sam), run.cancel_requested (audience Agent)
    API->>DB: audit_events, COMMIT
    API-->>O: 200
    DB-->>GW: NOTIFY
    GW->>GW: an access.changed: Rechte-Cache von Sam neu
    GW-->>S: dispatch access.changed (removed: Kanal Support)
    GW->>GW: Fokus beenden, Puffer für Kanal Support verwerfen
    S->>S: Cache für Kanal Support löschen, Ansicht schließen
    GW-->>A: dispatch run.cancel_requested (access_revoked)
    A->>API: POST /api/v1/runs/{id}/finish (cancelled)
    Note over GW,S: spätere Ereignisse aus Kanal Support erreichen Sam nicht mehr
```

### 16.5 Agenten-Lauf mit Nachtrag

```mermaid
sequenceDiagram
    autonumber
    participant H as Client (Alex)
    participant API as API-Prozess
    participant GW as Gateway
    participant A as Agenten-Adapter
    participant R as Agenten-Laufzeit
    H->>API: POST message (Erwähnung oder freier Kanal)
    API->>API: authorize(Alex, message.create) und Auslöse-Prüfung
    API-->>H: 201 (trigger_state received)
    Note over API: gleiche Transaktion: run queued, Live-Box-Anker, agent.invocation
    GW->>GW: vor Zustellung: authorize(Alex, agent.invoke) und I2
    GW-->>A: dispatch agent.invocation + Aufruf-Marke (JWS)
    GW-->>H: message.updated trigger_state queued
    A->>A: Signatur, Ablauf, Ist-Modus gleich Soll (I3)
    A->>API: POST /api/v1/runs/{id}/start (Marke, billing_mode_used)
    API-->>A: 200 running
    GW-->>H: dispatch run.state_changed running, trigger_state working
    A->>R: Turn starten
    loop gedrosselt
        R-->>A: Werkzeug, Status, Text
        A->>API: run.update (base_version, ops)
        API-->>GW: kurzlebig run.updated
        GW-->>H: run.updated (zusammengefasst)
    end
    H->>API: POST message (Nachtrag)
    API-->>H: 201
    GW->>GW: authorize(Alex, agent.invoke) und I2
    GW-->>A: dispatch agent.followup + Marke
    A->>R: Nachtrag einspeisen
    A->>API: Patch followups/ID delivered
    GW-->>H: message.updated trigger_state followup_delivered
    A->>API: Patch followups/ID absorbed
    R-->>A: Antwort
    A->>API: POST message (origin agent), POST /api/v1/runs/{id}/finish (completed)
    GW-->>H: message.created, run.state_changed completed, trigger_state done
    A->>GW: ack (seq)
```

## 17. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-SY-01 | DPoP-Beweis im `identify`-Rahmen mit Server-Nonce aus `hello`, nicht in Upgrade-Kopfzeilen. | Gleicher Weg für alle Clients; Nonce verhindert Wiedereinspielen. |
| R-SY-02 | Rechte gelten zur Zustellzeit; der Rechte-Cache folgt `access.changed` in Log-Reihenfolge. | Einfach, sofort wirksam, für alle Gateway-Prozesse gleich. |
| R-SY-03 | Der Cursor ist "zuletzt geprüft", nicht "zuletzt zugestellt"; der Server sendet `cursor`-Rahmen. | Wer wenig sieht, scannt nicht bei jedem Verbinden denselben fremden Verkehr erneut. |
| R-SY-04 | `resync_required` auch bei `cursor > head_seq`. | Zweites Netz neben der Sync-Epoche (R-SY-18) für Clients ohne gespeicherte Epoche. |
| R-SY-05 | Live-Box-Patches sind kurzlebig und gehen nur an fokussierte Threads; Zustände sind dauerhaft. | Hohe Patch-Rate belastet weder Log noch unbeteiligte Geräte; nach Pausen genügt der Zustand. |
| R-SY-06 | Patch-Format ist RFC 6902 mit `add`, `replace`, `remove`; Schritte als Objekt nach ID. | Standard, stabile Pfade, einfache Zusammenfassung. |
| R-SY-07 | Aufruf-Marken werden bei jeder Zustellung neu erzeugt, nicht beim Schreiben. | I2 prüft direkt vor dem Aufruf; erneute Zustellung nach Ablauf bleibt möglich. |
| R-SY-08 | Aufruf-Marken sind asymmetrisch signiert (EdDSA), Schlüssel über JWKS. | Der Adapter kann prüfen, aber keine Marke fälschen (Studie A schlug HMAC vor). |
| R-SY-09 | Ein Verbraucher je `(agent, consumer)`, der neueste gewinnt. | Ein Strom, keine doppelte Arbeit. |
| R-SY-10 | Lauf-Start ist ein Compare-and-swap mit `jti`-Prüfung. | Erneute Zustellung startet nie einen zweiten Lauf. |
| R-SY-11 | Rechte-Entzug bricht Läufe des betroffenen Auslösers ab, für alle Agenten. | Fail closed; Studie C erlaubte Fertigschreiben, das ist schwerer zu prüfen. |
| R-SY-12 | Entwürfe: letzte Schreibung gewinnt, mit Hinweis `overwrote_newer`; keine Zusammenführung. | Entwürfe sind persönlich und meist auf einem Gerät aktiv; Zusammenführen wäre teuer und fehleranfällig. |
| R-SY-13 | Gelesen-Stand ist monoton; Zurücksetzen nur mit Versionsprüfung. | Zwei Geräte können sich nicht gegenseitig auf "ungelesen" zurückwerfen. |
| R-SY-14 | Bei Geräte- oder Sitzungswiderruf löscht der Client alle lokalen Daten. | Ein verlorenes Gerät behält keine Inhalte. |
| R-SY-15 | Verteilung an Gateway-Prozesse über `LISTEN/NOTIFY` und Lesen des Logs, kein Broker. | Ein Abhängigkeitspfad weniger; das Log ist ohnehin die Quelle. |
| R-SY-16 | Kein JSON-`null` im Umschlag; fehlende Felder heißen "kein Wert" ([ADR 0053](../adr/0053-kein-json-null-in-api-und-ereignissen.md)). | Generierte Clients (dart-dio) kennen keinen Null-Typ; Go `omitempty` passt. |
| R-SY-17 | Aufruf-Marke und `redelivered` sind Zustellfelder im selben Umschlag, nie im Log ([ADR 0052](../adr/0052-ein-ereignis-umschlag-fuer-gateway-rest-und-webhooks.md)). | Ein Umschlag für Gateway, REST und Webhooks; Marken entstehen je Zustellung (R-SY-07). |
| R-SY-18 | Sync-Epoche nach jeder Wiederherstellung; Abweichung ergibt `resync_required` mit `epoch_changed` ([ADR 0059](../adr/0059-sync-epoche-nach-wiederherstellung.md)). | Erkennt eine neue Zeitlinie auch dann, wenn ihre Nummern alte Cursor schon überholt haben; R-SY-04 bleibt als zweites Netz. |
| R-SY-19 | Wiederholter `run.start` mit derselben Marke liefert das Ergebnis erneut (`runs.started_jti`). | Eine verlorene Antwort darf keinen Lauf verwaisen lassen; die Marke wechselt bei jeder Zustellung. |
| R-SY-20 | `agent.invocation` wird später geschrieben, solange Anhänge oder ein Server-Transkript nicht endgültig sind. | Agenten sehen nie ungeprüfte Bytes; ein Halten in der Zustellung würde den Strom des Verbrauchers blockieren. |
| R-SY-21 | `trigger_state` wechselt erst mit der ersten erfolgreichen Zustellung auf `queued`. | Ist der Agent offline, sieht der Mensch "Server hat es, Agent noch nicht". |
| R-SY-22 | Nur auslösende Nachrichten werden Nachträge; ein anderer Agent wird abgelehnt. | Ein Kanal mit Erwähnungspflicht schiebt Gespräche zwischen Menschen nicht in einen laufenden Turn. |
| R-SY-23 | Fencing der Agenten-Verbraucher über `holder_session_id`. | Die Regel "ein Verbraucher, eine Verbindung" gilt auch über mehrere api-Knoten. |
| R-SY-24 | Import-Stapel schreiben `thread.updated` statt `message.created` je Nachricht und setzen `threads.unread_floor_seq`. | Ein Import kann ein Vielfaches des normalen Verkehrs schreiben; Clients sollen nicht jede Nachricht einzeln empfangen. |
| R-SY-25 | Kurzlebige Ereignisse zwischen api-Knoten über `NOTIFY ephemeral` nur mit IDs, nur im Modus `multi`. | `NOTIFY` bleibt Wecksignal ohne Inhalt (R-Stack); ein Knoten braucht den Kanal nicht. |
| R-SY-26 | Live-Statistik als Abo-Op mit kurzlebigem Rahmen je Intervall auf dem Gateway, nicht als dauerhaftes Ereignis und nicht als eigener WebSocket-Kanal. | Ein Rahmen ist nach einem Intervall wertlos; das Gateway bringt Anmeldung, Rechteprüfung und Schließ-Codes schon mit ([25-visualisierung.md](25-visualisierung.md), R-VZ-05). |

## 18. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `sync.event_retention` | `30d` | so lange bleiben dauerhafte Ereignisse nachholbar |
| `sync.event_payload_max_bytes` | `65536` | größte Nutzlast eines Ereignisses |
| `sync.ws_max_frame_bytes` | `1048576` | größter WebSocket-Rahmen |
| `sync.identify_timeout` | `10s` | Zeit zwischen `hello` und `identify` |
| `sync.heartbeat_interval` | `30s` | Abstand der Heartbeats (in `hello` gemeldet) |
| `sync.heartbeat_miss_limit` | `2` | fehlende Heartbeats bis zum Schließen |
| `sync.resume_window` | `5m` | so lange ist `resume` nach Trennung möglich |
| `sync.reauth_lead_time` | `60s` | Vorlauf von `reauth_required` vor Token-Ablauf |
| `sync.reconnect_backoff_base` | `1s` | Basis des Backoffs beim Wiederverbinden |
| `sync.reconnect_backoff_cap` | `60s` | größter Abstand beim Wiederverbinden |
| `sync.catchup_page_size` | `500` | Ereignisse je Nachhol-Seite |
| `sync.catchup_max_events` | `50000` | mehr Rückstand ergibt `resync_required` |
| `sync.cursor_advance_every` | `1000` | nach so vielen übersprungenen Ereignissen ein `cursor`-Rahmen |
| `sync.outbound_queue_max` | `10000` | Rahmen im Ausgang einer Verbindung bis `4014` |
| `sync.idempotency_key_ttl` | `24h` | Aufbewahrung von `Idempotency-Key` |
| `sync.max_focused_threads` | `20` | Threads im `focus` je Verbindung |
| `sync.run_patch_coalesce_interval` | `250ms` | Sammelfenster für `run.updated` |
| `sync.typing_min_interval` | `5s` | höchstens so oft `typing.start` |
| `sync.typing_ttl` | `8s` | Tipp-Anzeige ohne Erneuerung |
| `client.cache_max_age` | `30d` | ungenutzte Cache-Einträge |
| `client.outbox_retry_base` | `2s` | Basis des Backoffs im Ausgangsfach |
| `client.outbox_retry_cap` | `5m` | größter Abstand im Ausgangsfach |
| `agent.invocation_token_ttl` | `60s` | Gültigkeit einer Aufruf-Marke |
| `agent.ack_timeout` | `30s` | ohne `ack`: erneute Zustellung |
| `agent.ack_persist_interval` | `1s` | spätestens so oft wird `acked_seq` gespeichert |
| `agent.max_inflight_events` | `100` | unbestätigte Ereignisse je Verbraucher |
| `agent.run_update_min_interval` | `500ms` | Adapter-Drossel für `run.update` |
| `agent.presence_timeout` | `90s` | ohne Heartbeat gilt der Agent als `offline` |
| `agent.cancel_grace` | `30s` | Zeit für den Adapter, einen Abbruch zu melden |

Werte anderer Kapitel, auf die dieses Kapitel verweist: `auth.dpop_nonce_ttl`
([08-identitaet.md](08-identitaet.md)), `perf.event_commit_min_per_s`
([02-architektur.md](02-architektur.md)), `file.invocation_hold_max` und
`voice.transcript_hold_max` ([12-dateien.md](12-dateien.md)), `command.autocomplete_timeout` und
`command.autocomplete_max_choices` ([09-agenten.md](09-agenten.md)),
`authz.expiry_sweep_interval` ([07-rechte.md](07-rechte.md)), `stats.live_interval`
([25-visualisierung.md](25-visualisierung.md)).
