# Webhooks

Status: Entwurf M0. Normativ sind der Abschnitt `webhooks:` in [openapi.yaml](openapi.yaml) (ein
Eintrag je Ereignistyp mit Schema) und dieses Dokument (Zustellregeln). Entscheidung: ADR 0016.
Konventionen der API: [docs/spec/06-api.md](../spec/06-api.md). Ereignisse und Empfangs-Aktionen:
[docs/spec/05-aktionen.md](../spec/05-aktionen.md), Abschnitt 6.

Es gibt zwei Richtungen:

| Richtung | Wer | Zweck |
|---|---|---|
| ausgehend | Server an eine installierte App | Ereignisse, die die App abonniert hat, signiert an eine HTTPS-Adresse |
| eingehend | externes System an den Server | Nachricht als App in einen Kanal schreiben, über eine geheime URL |

Plugins laufen nie im Server-Prozess (B6). Webhooks und das Gateway sind ihre einzigen Wege, von
Ereignissen zu erfahren.

## 1. Abonnieren

1. Die App ist installiert, hat den Scope `events:subscribe` und die gewünschten Typen in
   `event_subscriptions` (`installApp`, `setAppScopes`).
2. Ein Owner legt das Abo an (`createWebhookSubscription`, Aktion `webhook_out.manage`, Step-up).
   Die Adresse muss `https` nutzen und ihr Host in `webhook.allowed_hosts` stehen (R-DM-17);
   sonst `400 webhook_host_not_allowed`.
3. Je Abo gilt eine Teilmenge der abonnierten Typen (`event_types`). Ein Abo kann pausiert
   (`state = paused`) und wieder aktiviert werden.

## 2. Zustellung

Jede Zustellung ist ein `POST` mit genau einem Ereignis:

```http
POST /chat-events HTTP/1.1
Host: hooks.example.com
Content-Type: application/json
User-Agent: chat-server-webhooks/<server-version>
webhook-id: 0192f4a0-7b1c-7a3e-8c1d-5f0e9a2b4c6d
webhook-timestamp: 1791624600
webhook-signature: v1a,<Ed25519-Signatur in Base64> v1,<HMAC-SHA256 in Base64>

{"id":"0192f4a0-7b1c-7a3e-8c1d-5f0e9a2b4c6d","seq":5301,"type":"message.created", ...}
```

| Teil | Regel |
|---|---|
| Körper | Der Ereignis-Umschlag aus openapi.yaml (`MessageCreatedEvent` usw.), dieselbe Form wie im Gateway und in `GET /events`. Standard Webhooks empfiehlt `type`, `timestamp`, `data`; unser Umschlag hat `type` oben und `occurred_at` statt `timestamp`, weil eine Form für alle Kanäle wichtiger ist als die Empfehlung [belegt, W1] ([ADR 0052](../adr/0052-ein-ereignis-umschlag-fuer-gateway-rest-und-webhooks.md)). |
| `webhook-id` | Ereignis-ID (`id` im Umschlag). Gleich bei jeder Wiederholung und bei einer vom Owner angestoßenen Neuzustellung. |
| `webhook-timestamp` | Unix-Sekunden dieses Versuchs. |
| `webhook-signature` | Leerzeichen-getrennte Liste: immer `v1a,<base64>` (Ed25519), zusätzlich `v1,<base64>` (HMAC-SHA256), wenn das Abo HMAC nutzt. Während eines Schlüsselwechsels stehen beide Schlüssel in der Liste. |
| Signierter Inhalt | `{webhook-id}.{webhook-timestamp}.{Körper}`, der Körper Byte für Byte wie gesendet [belegt, W1]. |
| Epoche | Jede Zustellung trägt das Zustellfeld `sync_epoch` (04, 2.2 und 7.5; [ADR 0059](../adr/0059-sync-epoche-nach-wiederherstellung.md)). |
| Inhalt | Nur Felder, die die App mit der Empfangs-Aktion sehen darf (04, 2.2). Kanal-Prompts, Skills und Geheimnisse nie (R-AZ-09). Große Nutzlasten werden gekürzt und tragen `truncated: true`; die App lädt per REST nach. |

### 2.1 Signaturschlüssel

1. `v1a` nutzt den Server-Schlüssel mit Zweck `webhook_out` (03, `server_signing_keys`). Die
   öffentlichen Schlüssel stehen unter `GET /api/v1/instance/webhook-keys` im Format `whpk_<base64>`
   [belegt, W1]. Das Abo nennt seinen `signing_key_kid`.
2. Schlüsselwechsel (`rotateSigningKey`, Zweck `webhook_out`): Der neue Schlüssel erscheint mit
   `state = next` in der Liste, bevor er signiert. Danach signiert der Server mit beiden, bis der
   alte `not_after` erreicht.
3. `v1` (HMAC) ist je Abo zuschaltbar (`enable_hmac`). Das Geheimnis wird nicht gespeichert,
   sondern abgeleitet: `HMAC-SHA256(Hauptschlüssel, "webhook-v1:" || subscription_id || ":" ||
   Generation)`, Hauptschlüssel aus dem Secret-Provider (B12). Der Server zeigt es einmal als
   `whsec_<base64>` an. `rotate_hmac_secret` erhöht die Generation; für
   `webhook.secret_rotation_overlap` signiert der Server mit alter und neuer Generation
   (03 `webhook_subscriptions.hmac_generation` und `hmac_rotated_at`).
4. Grund für zwei Verfahren: Standard Webhooks empfiehlt asymmetrische Signaturen [belegt, W1];
   die Referenzbibliotheken nennen in ihren Beschreibungen aber HMAC, `v1a` ist dort nicht belegt
   [vermutet, W2]. HMAC hält die Hürde für Empfänger niedrig.

## 3. Prüfung beim Empfänger

1. Rohen Körper lesen, nicht erst als JSON parsen und neu serialisieren [belegt, W1].
2. `webhook-timestamp` prüfen: Abweichung von der eigenen Uhr höchstens `webhook.receiver_tolerance`.
3. Signaturen prüfen: jede Signatur der Liste gegen die vertrauten Schlüssel versuchen; eine
   gültige genügt. `v1a`-Schlüssel einmal abrufen und als Vertrauensliste halten, nicht aus der
   Anfrage übernehmen [belegt, W1]. HMAC zeitkonstant vergleichen.
4. Doppel erkennen: `webhook-id` merken, mindestens für die Dauer der Wiederholungen
   (`webhook.retry_max_age`).
5. Schnell mit `2xx` antworten, innerhalb `webhook.delivery_timeout` ([11-erweiterungen.md](../spec/11-erweiterungen.md)); die Arbeit danach erledigen.
6. Reihenfolge über `seq` herstellen, Objektstand über `resource.version` (04, 9). Ändert sich
   `sync_epoch`, wurde die Instanz wiederhergestellt: den gemerkten Stand nach `seq` verwerfen und
   mit der neuen Zeitlinie weitermachen, sonst gälten ihre kleineren Nummern als alt.

## 4. Wiederholungen und Abschalten

| Antwort | Wirkung |
|---|---|
| `2xx` | Erfolg |
| `410` | Abo wird abgeschaltet (`state = disabled`) [belegt, W1] |
| `429`, `502`, `503`, `504` | Drosseln; `Retry-After` wird beachtet [belegt, W1] |
| `3xx` | Fehler; Umleitungen werden nicht verfolgt [belegt, W1] |
| andere Status, Zeitüberschreitung, Verbindungsabbruch | Fehler |

1. Wiederholung mit exponentiellem Backoff und vollem Jitter zwischen
   `webhook.retry_backoff_base` und `webhook.retry_backoff_cap` (03), bis
   `webhook.retry_max_age` erreicht ist. Dann wird die Zustellung `dead`.
2. Nach `webhook.max_consecutive_failures` (03) Fehlschlägen in Folge wird das Abo `disabled`.
   Das steht im Audit-Log und in der Admin-Konsole. Das Produkt verschickt keine E-Mails
   (Baseline Abschnitt 4).
3. Jeder Versuch prüft vorher `webhook_out.deliver` und die Empfangs-Aktion des Ereignisses für
   die App (05, R-AZ-11). Hat die App den Zugriff inzwischen verloren, wird die Zustellung `dead`
   mit `last_error = not_authorized`, ohne Anfrage.
4. Ein Owner kann jede Zustellung erneut anstoßen (`redeliverWebhook`); sie trägt dieselbe
   `webhook-id`.
5. Antwortkörper werden nicht gespeichert (03, `webhook_deliveries.last_error`).

## 5. Reihenfolge und Doppel

1. Zustellung mindestens einmal (at-least-once).
2. Je Abo streng in `seq`-Reihenfolge, wie auf dem Gateway: Das nächste Ereignis geht erst
   hinaus, wenn das vorige mit `2xx` angenommen ist ([11-erweiterungen.md](../spec/11-erweiterungen.md),
   Abschnitt 7). Grund: Empfänger können Ereignisse wie im Gateway der Reihe nach anwenden, und
   bei Abos von Agenten gilt `2xx` als Bestätigung bis zu dieser `seq`
   ([04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitt 15.6).
3. `webhook_subscriptions.last_delivered_seq` ist die höchste mit `2xx` angenommene `seq`.
   Scheitert ein Ereignis bis `webhook.retry_max_age`, wird die Zustellung `dead` und das Abo
   `disabled`; nach dem Wiedereinschalten durch den Owner geht es mit dem Folgeereignis weiter.

## 6. Sicherheit der Zustellung

1. Nur `https`, nur Hosts aus `webhook.allowed_hosts`, keine Umleitungen.
2. Zustellungen laufen über den Egress-Client des Servers ([19-sicherheit.md](../spec/19-sicherheit.md),
   SEC-NET-1 bis SEC-NET-5): Er prüft die Adresse nach der DNS-Auflösung beim Verbindungsaufbau
   und lehnt Loopback-, Link-Local- und private Adressen ab, außer für Einträge in
   `egress.allowed_hosts` mit `allow_private`. Grund: Schutz vor DNS-Rebinding und SSRF
   (Baseline Abschnitt 4).
3. Zeitlimit je Versuch `webhook.delivery_timeout`; Standard Webhooks empfiehlt 15 bis 30 Sekunden
   [belegt, W1].

## 7. Ereignistypen für Apps und Agenten

Erzeugt aus openapi.yaml. Ereignisse mit Zielgruppe `self` bekommt nur die App oder der Agent, die
selbst Empfänger sind (eigene Befehle, eigene Komponenten, eigene Rechte). Ereignisse mit
"nur Agenten-Abo" gehen nur an Abos eines Agenten (Webhook als Transport,
[04-sync-protokoll.md](../spec/04-sync-protokoll.md) Abschnitt 15.6).

<!-- BEGIN GENERATED: webhook_events -->
| Ereignistyp | Empfangs-Aktion | Empfänger und Scope zusätzlich zu `events:subscribe` | Schema |
|---|---|---|---|
| `message.created` | `message.read` | `messages:read` | `MessageCreatedEvent` |
| `message.updated` | `message.read` | `messages:read` | `MessageUpdatedEvent` |
| `message.deleted` | `message.read` | `messages:read` | `MessageDeletedEvent` |
| `reaction.added` | `message.read` | `messages:read` | `ReactionAddedEvent` |
| `reaction.removed` | `message.read` | `messages:read` | `ReactionRemovedEvent` |
| `pin.added` | `thread.read` | `messages:read` | `PinAddedEvent` |
| `pin.removed` | `thread.read` | `messages:read` | `PinRemovedEvent` |
| `thread.created` | `thread.read` | `messages:read` | `ThreadCreatedEvent` |
| `thread.updated` | `thread.read` | `messages:read` | `ThreadUpdatedEvent` |
| `thread.closed` | `thread.read` | `messages:read` | `ThreadClosedEvent` |
| `thread.reopened` | `thread.read` | `messages:read` | `ThreadReopenedEvent` |
| `thread.moved` | `thread.read` | `messages:read` | `ThreadMovedEvent` |
| `thread.moved_out` | `channel.read` | `messages:read` | `ThreadMovedOutEvent` |
| `agent_session.changed` | `thread.read` | `messages:read` | `AgentSessionChangedEvent` |
| `run.state_changed` | `run.read` | `messages:read` | `RunStateChangedEvent` |
| `space.created` | `space.read` | `messages:read` | `SpaceCreatedEvent` |
| `space.updated` | `space.read` | `messages:read` | `SpaceUpdatedEvent` |
| `space.archived` | `space.read` | `messages:read` | `SpaceArchivedEvent` |
| `channel.created` | `channel.read` | `messages:read` | `ChannelCreatedEvent` |
| `channel.updated` | `channel.read` | `messages:read` | `ChannelUpdatedEvent` |
| `channel.settings_changed` | `channel.read` | `messages:read` | `ChannelSettingsChangedEvent` |
| `channel.archived` | `channel.read` | `messages:read` | `ChannelArchivedEvent` |
| `command.changed` | `command.read` | keiner (Metadaten, 05 Abschnitt 2.6) | `CommandChangedEvent` |
| `file.updated` | `file.read` | `files:read` | `FileUpdatedEvent` |
| `user.updated` | `user.read` | `users:read` | `UserUpdatedEvent` |
| `agent.updated` | `agent.read` | keiner (Metadaten, 05 Abschnitt 2.6) | `AgentUpdatedEvent` |
| `instance.updated` | `instance.read` | keiner (Metadaten, 05 Abschnitt 2.6) | `InstanceUpdatedEvent` |
| `status.updated` | `status.read` | keiner (Metadaten, 05 Abschnitt 2.6) | `StatusUpdatedEvent` |
| `access.changed` | `self` | nur eigene (App oder Agent ist Empfänger) | `AccessChangedEvent` |
| `agent.invocation` | `agent.events.consume` | nur Agenten-Abo (Agent ist Empfänger) | `AgentInvocationEvent` |
| `agent.followup` | `agent.events.consume` | nur Agenten-Abo (Agent ist Empfänger) | `AgentFollowupEvent` |
| `run.cancel_requested` | `agent.events.consume` | nur Agenten-Abo (Agent ist Empfänger) | `RunCancelRequestedEvent` |
| `agent.config_changed` | `agent.config.read` | nur Agenten-Abo (Agent ist Empfänger) | `AgentConfigChangedEvent` |
| `interaction.created` | `agent.events.consume` | nur eigene (App oder Agent ist Empfänger) | `InteractionCreatedEvent` |
| `command.invoked` | `agent.events.consume` | nur eigene (App oder Agent ist Empfänger) | `CommandInvokedEvent` |
<!-- END GENERATED: webhook_events -->

Persönliche Ereignisse von Menschen (Gelesen-Stand, Entwürfe, Einstellungen), Verwaltungsereignisse
(Freigaben, Gruppen, Import, DSGVO) und Agenten-Ereignisse (Aufrufe, Nachträge) gibt es nicht als
Webhook.

## 8. Eingangs-Webhooks

1. Ein Mensch mit `manage` auf dem Kanal legt den Eingangs-Webhook an
   (`createIncomingWebhook`, Step-up). Er wählt eine installierte App als schreibenden Prinzipal
   und optional einen festen Thread. Die Antwort enthält die URL mit Token genau einmal.
2. Externe Systeme senden `POST /api/v1/hooks/{webhook_id}/{token}` mit `content_md`, optional
   `title` (neuer Thread, wenn kein fester Thread) und `client_msg_id` (Idempotenz für
   Wiederholungen des Absenders).
3. Die Nachricht hat `origin = app`, Autor ist die App. Sie löst nie Agenten aus (03,
   `incoming_webhooks`). Die App braucht `write` auf dem Kanal.
4. Unbekannte oder widerrufene Webhooks und falsche Tokens ergeben `404`. Rate-Limit-Klasse
   `rl.webhook_in` je Webhook.
5. Der Token steht in der URL, weil viele Absender nur eine URL konfigurieren können. Server und
   Proxys schwärzen den Pfad nach `/hooks/{webhook_id}/` in Logs und Traces. Wechsel: neuen
   Webhook anlegen, alten widerrufen.

## 9. Konfigurationswerte dieses Dokuments

| Name | Standard | Bedeutung |
|---|---|---|
| `webhook.retry_max_age` | `72h` | danach wird eine Zustellung `dead` |
| `webhook.secret_rotation_overlap` | `24h` | so lange gelten alte und neue HMAC-Generation |
| `webhook.receiver_tolerance` | `5m` | empfohlene Toleranz für `webhook-timestamp` beim Empfänger |

`webhook.allowed_hosts`, `webhook.max_consecutive_failures`, `webhook.retry_backoff_base` und
`webhook.retry_backoff_cap` stehen in 03, Abschnitt 9; `webhook.delivery_timeout` in
[11-erweiterungen.md](../spec/11-erweiterungen.md); `egress.allowed_hosts` in
[19-sicherheit.md](../spec/19-sicherheit.md). Der Standard für `webhook.retry_max_age`
liegt nahe dem Beispielplan von Standard Webhooks (rund 75 Stunden) [abgeleitet, W1].

## Quellen

Abgerufen am 2026-10-10.

| Nr | Quelle | Inhalt | Marker |
|---|---|---|---|
| W1 | https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md | Kopfzeilen, signierter Inhalt, `v1`/`v1a`, Präfixe `whsec_`/`whpk_`, mehrere Signaturen, Statusregeln, Backoff-Beispiel, Zeitlimit 15 bis 30 s, empfohlene Körperform | [belegt] |
| W2 | https://github.com/standard-webhooks/standard-webhooks | Referenzbibliotheken je Sprache; Unterstützung von `v1a` dort nicht beschrieben | [vermutet] |
