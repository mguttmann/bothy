# 05 Aktionskatalog der Autorisierung

Status: Entwurf M0, nach den Rückmeldungen aller Kapitel integriert. Normativ. Diese Liste ist
die vollständige Menge der Aktionen, die `authorize(principal, action, resource) -> Decision` im
Modul `authz` kennt (B7).
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Sync und Ereignisse: [04-sync-protokoll.md](04-sync-protokoll.md). Wie `authorize` entscheidet:
[07-rechte.md](07-rechte.md).

## 1. Verbindliche Regeln

1. **Eine Prüfstelle.** Jede Route, jede WebSocket-Op, jede Ereignis-Zustellung, jeder Push,
   jede Suche, jeder Datei-Abruf und jeder Agenten-Aufruf ruft `authorize` mit einer Aktion aus
   diesem Katalog auf. Es gibt keine Rechte-Logik außerhalb von `authz`.
2. **Kein Sichtrecht ergibt 404.** Darf ein Prinzipal eine Ressource nicht lesen, ist die
   Antwort `404 not_found`, gleich ob es sie gibt oder nicht. `403 forbidden` gibt es nur, wenn
   der Prinzipal die Ressource lesen darf, aber die nötige höhere Stufe oder ein Zusatzrecht
   fehlt. Listen, Zähler, Suche und Vorschläge filtern in der Abfrage (P2) und verraten nichts.
3. **REST.** Jede Operation im OpenAPI-Dokument trägt `x-authz-action` mit genau einer ID aus
   diesem Katalog und `x-rate-limit-class` mit der Klasse dieser Aktion. Das API-Team setzt das
   mit Lint-Regeln durch (`authz-action-required`: Feld fehlt; `authz-action-known`: ID nicht im
   Katalog). Zusätzlich prüft der Server beim Start, dass jede registrierte Route eine bekannte
   Aktion hat, und ein Test zählt alle Routen. Die einzige Quelle dieses Katalogs für Doku, Lint
   und Code ist [actions.yaml](actions.yaml) ([ADR 0049](../adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle.md)); die Tabellen der Abschnitte 3,
   4, 6 und 7 und die Listen in `docs/api/redocly.yaml` werden daraus erzeugt (`make catalog`),
   und `make catalog-check` hält sie im Gate.
4. **WebSocket und Ereignisse.** Jede Op im AsyncAPI-Dokument trägt `x-authz-action`; jeder
   dauerhafte und kurzlebige Ereignistyp trägt `x-receive-action`. Abschnitt 6 und 7 sind die
   normativen Tabellen; sie werden aus derselben Datei [actions.yaml](actions.yaml) erzeugt
   (Regel 3). Ein Test schlägt fehl, wenn ein Typ oder eine Op ohne Eintrag registriert ist.
5. **Katalog ist geschlossen.** Eine unbekannte Aktion ist ein Programmfehler: `authorize`
   lehnt ab, die Anfrage endet mit `500`, ein Alarm wird ausgelöst.
6. **Agenten-Aufruf und I1.** Jede Auslösung eines Agenten (Nachricht, Nachtrag, Befehl,
   Interaktion, Sitzungswechsel, Autovervollständigung) prüft zusätzlich `agent.invoke`
   (Abschnitt 5). Die Invarianten I1 bis I3 der Baseline hängen an diesen Aktionen:
   - I1 wird bei `grant.create`, `grant.revoke`, `grant.expire`, `group.member.add`,
     `group.member.remove`, `group.member.expire`, `group.member.sync`, `group.delete`,
     `owner.add`, `owner.remove`,
     `space.operator.set`, `channel.operator.set`, `agent.billing_mode.set`,
     `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.enable`, `user.disable`
     und `user.enable` transaktional geprüft (Prüfung in [03-datenmodell.md](03-datenmodell.md),
     Abschnitt 6.4).
   - Geführter Ablauf: `grant.create`, `group.member.add`, `owner.add`, `space.operator.set` und
     `channel.operator.set` nehmen optional `resolve_i1 = {switch_to_organization: [agent_ids],
     reason}` an. Der Server prüft dann zusätzlich `agent.billing_mode.set` je genanntem Agenten
     mit `authorize`, schaltet sie in derselben Transaktion zuerst auf `organization`, führt die
     eigentliche Änderung aus, prüft I1 und schreibt je Aktion einen Audit-Eintrag mit derselben
     `correlation_id` ([10-backend-richtlinie.md](10-backend-richtlinie.md), Abschnitt 11).
     Etappe: Das Feld kommt mit M2-21 ([PLAN.md](../PLAN.md)) in die Anfrage-Schemas. Vorher
     lehnt der Server es wie jedes unbekannte Feld mit `400 validation_failed` ab
     ([06-api.md](06-api.md), "Unbekanntes").
   - I2 ist Teil von `agent.invoke` und wird bei jeder Zustellung eines Aufrufs erneut geprüft.
   - I3 prüft der Adapter; der Server prüft beim Lauf-Start (`run.start`) die Marke erneut.
7. **Rechte-Version.** Aktionen mit Spalte "Rechte" = `ja` erhöhen `permission_version` und
   schreiben `access.changed` für alle Betroffenen ([04-sync-protokoll.md](04-sync-protokoll.md),
   Abschnitt 8).
8. **Schreibgeschützte Kanäle.** Ist `channels.read_only` gesetzt, lehnt `authorize` in diesem
   Kanal jede Aktion ab, deren Spalte "Recht" die Stufe `write` oder `trigger` verlangt
   (Nachrichten, Threads, Reaktionen, Interaktionen, Befehle, Uploads, Tippen, Agenten-Aufrufe),
   für alle Prinzipale einschließlich Owner, mit `403` und Grund `channel_read_only`. Erlaubt
   bleiben lesende und persönliche Aktionen, Aktionen mit Recht `manage` oder `owner` (z. B.
   Moderation, `channel.update`, `channel.read_only.set`) und `import.apply`. Setzen von
   `read_only` schreibt `run.cancel_requested` mit Grund `channel_read_only` für aktive Läufe im
   Kanal (R-AZ-12).
9. **Wartungsmodus.** Steht `server.maintenance_mode` auf `read_only`
   ([16-betrieb.md](16-betrieb.md)), beantwortet eine eigene Schicht vor `authorize` alle Aktionen
   mit Wirkung `write` (Abschnitt 2.7) mit `503 maintenance` und `Retry-After`, außer den
   Ausnahmen aus [ADR 0065](../adr/0065-wartungsmodus-read-only.md); lesende Aktionen und das Nachholen laufen weiter. Die Schicht ist keine
   Rechte-Logik; sie kennt nur die Wirkung aus dem Katalog.
10. **Systemaktionen.** Aktionen mit Recht `system` führt nur der Server selbst aus (Job oder
    Server-Befehl auf dem Host). Sie sind über API und Gateway nicht erreichbar. Sie stehen im
    Katalog, damit Audit-Einträge eine bekannte Aktion nennen und der Registrierungstest sie zählt.
11. **Terminal und I6.** Die Aktionen aus Abschnitt 4.14 prüfen zusätzlich die Schutzregeln SR10
    und SR11 aus [07-rechte.md](07-rechte.md). I6 wird bei `grant.create` mit Ziel `host` und bei
    `host.manage` transaktional geprüft ([03-datenmodell.md](03-datenmodell.md), Abschnitt 6.8;
    [24-terminal.md](24-terminal.md), Abschnitt 7.3) und bei jedem `terminal.open` und
    `terminal.attach` erneut. Die Terminal-Verbindungen sind eigene WebSocket-Kanäle außerhalb des
    Gateways; ihre Ops stehen in Abschnitt 7.

## 2. Begriffe der Tabellen

### 2.1 Spalte "Recht"

| Wert | Bedeutung |
|---|---|
| `public` | ohne Anmeldung; trotzdem durch `authorize` (Prinzipal `anonymous`) |
| `auth` | jeder aktive, angemeldete Prinzipal |
| `human` | jeder aktive Mensch |
| `self` | der Prinzipal ist Eigentümer der Ressource (eigenes Gerät, eigene Einstellung) |
| `read`, `write`, `trigger` | wirksame Mindeststufe auf dem Ziel der Ressource |
| `+upload`, `+approve_actions`, `+export`, `+manage` | zusätzlich nötiges Zusatzrecht auf dem Ziel |
| `manage` | Zusatzrecht `manage` auf dem Ziel (Stufe beliebig) |
| `owner` | Prinzipal ist aktiver Owner und handelt in einer Sitzung (Regel 2.3 Punkt 1) |
| `agent-self` | Prinzipal ist der Agent, dem die Ressource gehört (eigener Lauf, eigene Sitzung) |
| `app-self` | Prinzipal ist die App, der die Ressource gehört (eigener Befehl, eigene Komponente) |
| `participant` | Mensch ist Teilnehmer des DM-Kanals |
| `+terminal` | Zusatzrecht `terminal` auf dem Rechner (implizit für Owner, außer I6 sperrt; [07-rechte.md](07-rechte.md), Abschnitt 2.12) |
| `assignee` | Mensch ist Rechner-Inhaber (`hosts.assigned_user_id`) |
| `host-self` | Prinzipal ist der Rechner selbst, angemeldet mit seinem Anmeldeschlüssel ([24-terminal.md](24-terminal.md), Abschnitt 4.1) |
| `system` | nur der Server selbst (Regel 1.10) |

Mehrere Werte mit "oder" sind Alternativen. "und" heißt beides.

### 2.2 Ziel einer Ressource

Das Ziel bestimmt, auf welchem Pfad Freigaben gelten:

| Ressource | Pfad |
|---|---|
| `message`, `reaction`, `pin`, `run`, `run_step`, `component`, `interaction`, `agent_session` | Thread -> Kanal -> Bereich -> Instanz |
| `thread` | Kanal -> Bereich -> Instanz |
| `file` mit `purpose = 'attachment'` | Kanal der Nachricht, an der sie hängt; vor dem Anhängen `scope_channel_id`, und dann liest nur der Hochladende |
| `file` mit `purpose = 'avatar'` | jeder aktive, angemeldete Prinzipal (`auth`) |
| `file` mit `purpose = 'branding'` | öffentlich wie `instance.discovery.read` (Logo der Anmeldeseite) |
| `file` mit `purpose = 'gdpr_export'` | nur über `gdpr.export.download` |
| `channel` | Kanal -> Bereich -> Instanz |
| `space` | Bereich -> Instanz |
| `instance` | Instanz |
| DM-Kanal und alles darin | nur `dm_participants`; Freigaben gelten nicht |
| `host`, `terminal_session` | Rechner (bei der Sitzung: ihr Rechner); eigenes Ziel außerhalb von Instanz, Bereich und Kanal, nur Freigaben auf diesen Rechner und die Owner-Regel ([07-rechte.md](07-rechte.md), Abschnitt 2.12) |

Eine angehängte Datei folgt immer ihrer Nachricht, auch nach dem Verschieben des Threads;
`files.scope_channel_id` gilt nur vor dem Anhängen.

### 2.3 Wirksame Stufe

1. Owner: Stufe `trigger` und alle Zusatzrechte auf allen Zielen außer DM-Kanälen, und nur in
   einer Sitzung eines Menschen. Ein persönliches API-Token eines Owners handelt mit den Freigaben
   dieses Menschen, ohne Owner-Rechte (SR5 in [07-rechte.md](07-rechte.md)). Exception:
   the check without a request (section 5) takes the owner status from the current owner list
   without a session (R-AZ-29).
2. Sonst: höchste Stufe aller gültigen Freigaben (nicht widerrufen, nicht abgelaufen) des
   Prinzipals und seiner gültigen Gruppen auf dem Pfad. Zusatzrechte: Vereinigung der
   Zusatzrechte derselben Freigaben.
3. DM-Teilnehmer: Stufe `write` und Zusatzrecht `upload` im DM-Kanal.
4. Bediener-Regel: Ein Agent hat mindestens `write` auf jedem Kanal, dessen wirksamer Bediener er
   ist, und auf jedem Thread mit `threads.operator_agent_id` gleich ihm, ohne Freigabe-Zeile.
   Die Regel folgt dem Bediener; beim Wechsel bleibt kein Rest (R-AZ-13).
5. Agenten und Apps: höchstens `write` und als Zusatzrecht höchstens `upload` (R-DM-08, R-DM-20).
   Apps brauchen zusätzlich den Scope aus 2.6.
6. Handelt ein Agent im Rahmen eines Laufs, gilt zusätzlich die Auftragsbindung: höchstens die
   Rechte des auslösenden Menschen (SR9 in [07-rechte.md](07-rechte.md)).

### 2.4 Spalten "Owner", "Step-up", "Audit", "Rechte"

- **Owner** `ja`: nur Owner, unabhängig von Freigaben.
- **Step-up** `ja`: Die Sitzung braucht eine frische Bestätigung durch den Menschen
  (`auth_sessions.step_up_at` jünger als `auth.step_up_max_age`): Passkey, Step-up-Geräteschlüssel
  mit Biometrie oder Geräte-PIN, Passwort mit TOTP oder erneute OIDC-Anmeldung
  ([08-identitaet.md](08-identitaet.md), Abschnitt 6). Sonst `401 step_up_required`.
  Step-up-Aktionen können nur Menschen ausführen; Agenten- und App-Tokens und persönliche Tokens
  werden abgelehnt (R-AZ-03).
- **Step-up** `bedingt`: wie `ja`, sobald die Person schon Anmeldedaten der Art hat, die die Aktion
  einrichtet (die Beschreibung nennt die Bedingung); die erste Einrichtung braucht nur die Sitzung
  eines Menschen. Grund: Die erste Einrichtung muss ohne vorhandene Anmeldedaten gehen, und eine
  gestohlene Sitzung gewinnt damit nur einen Anmeldeweg, dessen neue Geräte `pending` bleiben
  ([08-identitaet.md](08-identitaet.md), Abschnitt 5).
- **Step-up** `frisch`: wie `ja`, aber die Bestätigung ist jünger als `terminal.step_up_max_age`,
  stammt aus einem Passkey oder einem Step-up-Geräteschlüssel (in der Web-Konsole nur Passkey mit
  `userVerification = required`) und ist noch nicht verbraucht; die Aktion verbraucht sie
  (`auth_sessions.step_up_consumed_at`, [08-identitaet.md](08-identitaet.md), Abschnitt 6.3).
  Sonst `401 step_up_required`.
- **Audit** `ja`: Erlaubnis und Ablehnung kommen ins Audit-Log. `abl`: nur Ablehnungen. Jede
  Ablehnung jeder Aktion in einer Anfrage oder Op wird protokolliert; Filter (Empfangsprüfung bei
  Zustellung und Nachholen, Prüfung je Element einer Liste) schreiben keine Ablehnung
  ([07-rechte.md](07-rechte.md), Abschnitt 4.2). Gleiche Ablehnungen (gleicher Prinzipal bzw.
  ohne Anmeldung gleiches IP-Pseudonym, gleiche Aktion, gleicher Grund) werden je
  `audit.deny_aggregate_window` zu einem Eintrag mit Zähler zusammengefasst (R-AZ-04). Weil die
  Kette nur anhängt, sammelt jeder Prozess die Ablehnungen im Speicher und schreibt je Schlüssel
  eine Zeile am Ende des Fensters, beim Herunterfahren oder sobald `audit.deny_aggregate_max_keys`
  Schlüssel offen sind (dann die älteste). Ein harter Absturz verliert höchstens die offenen
  Zusammenfassungen eines Fensters ([19-sicherheit.md](19-sicherheit.md), Abschnitt 12).
- **Rechte** `ja`: siehe Regel 1.7.

### 2.5 Entscheidung

`authorize` liefert einen dieser Werte. Den Grund-Code je Ablehnung nennt
[07-rechte.md](07-rechte.md), Abschnitt 4.1; er steht im Audit-Log.

| Entscheidung | HTTP | Bedeutung |
|---|---|---|
| `allow` | | erlaubt |
| `not_found` | `404` | kein Leserecht auf die Ressource oder Ressource fehlt |
| `forbidden` | `403` | Leserecht vorhanden, Stufe, Zusatzrecht oder Scope fehlt, oder eine Schutzregel sperrt (z. B. `channel_read_only`, `device_pending`) |
| `step_up_required` | `401` | frische Bestätigung fehlt |
| `invariant_violation` | `409` | Änderung würde I1 oder eine andere Invariante verletzen |
| `unauthenticated` | `401` | kein gültiger Prinzipal |

Ein Gerät im Zustand `pending` und ein bereitgestellter Mensch im Zustand `pending`
([08-identitaet.md](08-identitaet.md), Abschnitt 3.1) haben Tokens, aber jede Aktion außer
`self.read`, `device.list` für das eigene Gerät, `auth.token.refresh` und `auth.logout` endet mit
`403 device_pending` ([08-identitaet.md](08-identitaet.md), Abschnitt 5.2). Das Gateway lehnt
`identify` eines wartenden Geräts mit `4016` ab. Grund für den Menschen: Mit `401` müsste die App
ihre Daten löschen und könnte nicht nachfragen, ob der Owner sie freigegeben hat.

Rate-Limits prüft eine eigene Schicht vor `authorize` mit der Klasse aus dem Katalog (`429`);
den Wartungsmodus eine weitere (Regel 1.9).

### 2.6 Scopes für Apps

Eine App braucht neben Freigaben einen bewilligten Scope (`app_installations.granted_scopes`):

| Aktionen | Scope |
|---|---|
| `message.read`, `thread.read`, `channel.read`, `space.read`, `run.read`, `search.query` | `messages:read` |
| `message.create`, `message.update`, `message.delete`, `reaction.*`, `thread.create` | `messages:write` |
| `file.read` | `files:read` |
| `file.upload` | `files:write` |
| `command.register`, `command.remove`, `command.autocomplete.respond` | `commands:write` |
| `component.update`, `interaction.respond` | `components:write` |
| `user.read` | `users:read` |
| Ereignis-Abos, `gateway.connect`, `events.catch_up` | `events:subscribe` |
| `cost.reconcile.submit` | `costs:reconcile` |
| `self.read`, `instance.read`, `status.read`, `command.read`, `agent.read` | ohne Scope; nur Metadaten, die jeder angemeldete Prinzipal mit passenden Freigaben sieht |
| `webhook_in.post` | ohne Scope; Token des Eingangs-Webhooks |

Agenten haben alle Scopes außer `users:read` und `costs:reconcile` implizit; ihre Grenze sind die
Freigaben und die Bediener-Regel.

### 2.7 Wirkung (`effect`)

Jede Aktion hat eine Wirkung. `write` heißt: Die Aktion kann fachlichen Zustand in der Datenbank
ändern oder schreibt bei Erlaubnis einen Audit-Eintrag (Spalte "Audit" `ja`). `read` heißt: Sie tut
beides nicht. Der Wartungsmodus nutzt nur dieses Merkmal (Regel 1.9, [ADR 0065](../adr/0065-wartungsmodus-read-only.md)); der
maschinenlesbare Katalog trägt es je Aktion ([ADR 0049](../adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle.md)). `read` haben genau:

- die Aktionen, deren ID auf `.read` oder `.list` endet, dazu `cost.read.thread` und
  `cost.read.aggregate`, außer `audit.read` (Audit `ja`);
- `access.explain`, `search.query`, `events.catch_up`, `gateway.connect`, `gateway.focus`,
  `typing.start`, `command.autocomplete`, `command.autocomplete.respond`, `client_metrics.submit`, `console.csp_report`,
  `push.deliver`, `webhook_out.deliver` und `terminal.read_own`.

Alle anderen haben `write`. Grund für die geschlossene Liste: Eine neue Aktion ohne Eintrag ist
`write` und wird im Wartungsmodus gesperrt (fail closed); die Spalte "Recht" taugt nicht dafür, weil
etwa `settings.read` und `settings.update` beide `self` verlangen.

## 3. Rate-Limit-Klassen

Jede Klasse hat einen Schlüssel (wonach gezählt wird) und zwei Werte
`ratelimit.<klasse>.per_minute` und `ratelimit.<klasse>.burst` (Token-Bucket). Standardwerte in
Abschnitt 10.

<!-- BEGIN GENERATED: rate_limit_classes -->
| Klasse | Schlüssel | Zweck |
|---|---|---|
| `rl.public_auth` | Client-IP (IPv6: das /64-Präfix) | Anmeldung, Refresh, öffentliche Endpunkte; zählt zusätzlich jede gescheiterte Prüfung vorgelegter Anmeldedaten ([02-architektur.md](02-architektur.md), Abschnitt 9.1) |
| `rl.gateway_connect` | Gerät bzw. Token, geprüft bei `identify` und `resume`, sobald Anmeldedaten geprüft sind | WebSocket-Sitzung aufbauen |
| `rl.read` | Prinzipal | lesende Abfragen |
| `rl.write` | Prinzipal | allgemeine Schreibvorgänge |
| `rl.message` | Prinzipal | Nachrichten senden und bearbeiten |
| `rl.trigger` | Prinzipal | Agenten auslösen, Befehle, Vorschläge |
| `rl.interaction` | Prinzipal | Komponenten bedienen |
| `rl.upload` | Prinzipal | Dateien hochladen |
| `rl.search` | Prinzipal | Suche |
| `rl.typing` | Verbindung | Tippen |
| `rl.admin` | Prinzipal | Verwaltung |
| `rl.export` | Prinzipal | Export, DSGVO-Pakete |
| `rl.agent_runtime` | Agent | Heartbeat, Lauf-Patches, Bestätigungen |
| `rl.webhook_in` | Eingangs-Webhook | eingehende Webhooks |
| `rl.import` | Prinzipal | Stapel eines laufenden Imports (`import.apply`) |
| `rl.terminal` | Prinzipal | Terminal öffnen, anhängen, Strom verbinden |
| `rl.host` | Rechner (nach der Prüfung des Anmeldeschlüssels) | Steuer- und Datenverbindungen des Host-Dienstes |
<!-- END GENERATED: rate_limit_classes -->

Systemaktionen und die internen Zustellprüfungen haben keine Klasse (`keine`).

Gründe der Schlüssel: IPv6-Clients bekommen meist ein ganzes /64-Präfix; je Adresse gezählt, könnte
ein Client durch Adresswechsel beliebig viele Eimer füllen. `rl.gateway_connect` zählt nach der
Prüfung der Anmeldedaten, damit niemand den Eimer eines fremden Geräts leeren kann; vor `identify`
begrenzt `sync.identify_timeout` eine offene Verbindung ([04-sync-protokoll.md](04-sync-protokoll.md),
Abschnitt 6.3).

## 4. Aktionen

### 4.1 Öffentlich und Anmeldung

<!-- BEGIN GENERATED: actions.public -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `instance.discovery.read` | JWKS, Protokollversionen, minimales Branding, Branding-Logo | instance | public | nein | nein | nein | nein | `rl.public_auth` |
| `console.csp_report` | accept a Content Security Policy violation report of the web console, without content in the log ([19-sicherheit.md](19-sicherheit.md), section 5, rule 6; R-AZ-33) | instance | public | nein | nein | nein | nein | `rl.public_auth` |
| `auth.login.begin` | Anmeldung starten (OIDC, Passkey-Challenge, Header-Login, Geräte-Kopplung) | instance | public | nein | nein | abl | nein | `rl.public_auth` |
| `auth.login.complete` | Anmeldung abschließen (OIDC-Server-Code, Passkey, Einladungs-Code, Passwort mit TOTP, Kopplung), Gerät und Geräteschlüssel registrieren, Sitzung anlegen | instance | public | nein | nein | ja | nein | `rl.public_auth` |
| `auth.token.refresh` | Refresh-Token rotieren (mit Nachsicht `auth.refresh_reuse_grace`; im Wartungsmodus ohne Rotation, ADR 0065); Wiederverwendung sperrt die Familie | auth_session | public (gültiges Refresh-Token und DPoP-Beweis der Familie) | nein | nein | abl | nein | `rl.public_auth` |
| `auth.logout` | eigene Sitzung beenden; Push-Registrierungen dieser Sitzung werden deaktiviert | auth_session | self | nein | nein | ja | nein | `rl.write` |
| `auth.step_up` | frische Bestätigung erbringen (Passkey, Step-up-Geräteschlüssel, Passwort mit TOTP, erneute OIDC-Anmeldung) | auth_session | self | nein | nein | ja | nein | `rl.public_auth` |
| `status.public.read` | Statusseite ohne Anmeldung, nur wenn `status.public_enabled` | instance | public | nein | nein | nein | nein | `rl.public_auth` |
<!-- END GENERATED: actions.public -->

### 4.2 Instanz und Verwaltung

<!-- BEGIN GENERATED: actions.instance -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `instance.read` | Instanz-Daten für Clients: Branding, Sprache, Grenzen, für den anfragenden Prinzipal ausgewertete Feature-Flags (inklusive Rollout je Person und Gruppe) | instance | auth | nein | nein | nein | nein | `rl.read` |
| `instance.settings.read` | alle Instanz-Einstellungen lesen (Konsole) | instance | owner | ja | nein | nein | nein | `rl.read` |
| `instance.settings.update` | Instanz-Einstellung ändern | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `instance.branding.update` | Branding ändern (Name, Logo, Farbe aus den Kategorie-Farben) | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `instance.bootstrap` | erster Owner per Server-Befehl auf dem Host ([08-identitaet.md](08-identitaet.md), Abschnitt 4) | instance | system | nein | nein | ja | ja | keine |
| `feature_flag.read` | alle Schalter mit Regeln lesen | instance | owner | ja | nein | nein | nein | `rl.read` |
| `feature_flag.update` | Schalter ändern | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `signing_key.rotate` | Server-Schlüssel wechseln: `mode = scheduled` (nächsten Schlüssel ankündigen) oder `mode = emergency` (alle Schlüssel des Zwecks sofort `retired`) | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `signing_key.transition` | planmäßiger Zustandswechsel eines Signierschlüssels durch den Job `instance.key_rotation` (`next` anlegen, `active` machen, `retired` setzen) | instance | system | nein | nein | ja | nein | keine |
| `identity_provider.read` | Anmelde-Treiber lesen | instance | owner | ja | nein | nein | nein | `rl.read` |
| `identity_provider.manage` | Anmelde-Treiber anlegen, ändern, abschalten | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `owner.add` | Menschen zum Owner machen; optional mit `resolve_i1` (Regel 1.6) | user | owner | ja | ja | ja | ja | `rl.admin` |
| `owner.remove` | Owner-Rolle entziehen; mindestens ein aktiver Owner bleibt (SR8) | user | owner | ja | ja | ja | ja | `rl.admin` |
| `owner.recover` | Einladungs-Code zur Wiederherstellung für einen aktiven Owner per Server-Befehl auf dem Host (`bothy setup issue-enrollment-code`, [08-identitaet.md](08-identitaet.md), Abschnitt 4) | user | system | nein | nein | ja | nein | keine |
| `audit.read` | Audit-Log lesen und filtern | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `audit.verify` | Hash-Kette und Prüfpunkte prüfen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `stats.read` | Statistik lesen, auch Live-Rahmen, Timeline, SLO-Ampel und Backend-Ansicht und das Abo `stats.subscribe` ([25-visualisierung.md](25-visualisierung.md)); Instanz nur Owner, Bereich mit `manage`; immer auf die sichtbaren Kanäle gefiltert, Kostenfelder nur mit `cost.read.aggregate` | instance, space | owner oder manage | nein | nein | nein | nein | `rl.read` |
| `access.explain` | "Wer hat Zugriff?" mit Herkunft (direkt, Gruppe, geerbt, Bediener, indirekt über Agent) | space, channel | owner oder manage | nein | nein | nein | nein | `rl.read` |
| `access.preview` | Trockenlauf einer Aktion mit Spalte "Rechte" = `ja`: Unterschiede je Prinzipal, I1, SR8, Hinweise in Klartext; die Transaktion wird zurückgerollt; Grenzen der Delegation ergeben `allowed = false` statt einer Ablehnung ([07-rechte.md](07-rechte.md), Abschnitt 9.3) | wie die Zielaktion | Recht der Zielaktion | wie Zielaktion | nein | nein | nein | `rl.admin` |
| `access.view_as` | "Als Person ansehen", nur lesend | user | owner | ja | ja | ja | nein | `rl.admin` |
| `access.view_as.end` | end "View as person" stored on one's own console session before its end time ([07-rechte.md](07-rechte.md), section 9.2; R-AZ-32) | auth_session | self | nein | nein | ja | nein | `rl.admin` |
<!-- END GENERATED: actions.instance -->

### 4.3 Menschen, Identität, Geräte, Tokens

<!-- BEGIN GENERATED: actions.identity -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `self.read` | eigenen Prinzipal lesen (`GET /me`), für Menschen, Agenten und Apps; auch für ein wartendes Gerät | principal | auth und self | nein | nein | nein | nein | `rl.read` |
| `user.read` | Profil und Verzeichnis von Menschen lesen | user | human; Apps mit Scope | nein | nein | nein | nein | `rl.read` |
| `user.create` | Menschen anlegen (kein Selbst-Registrieren), bei lokalem Konto mit Einladungs-Code | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `user.update` | Profil eines anderen Menschen ändern | user | owner | ja | ja | ja | nein | `rl.admin` |
| `user.disable` | Menschen sperren: Sitzungen und Geräte widerrufen, Läufe abbrechen; nie den letzten aktiven Owner (SR8) | user | owner | ja | ja | ja | ja | `rl.admin` |
| `user.enable` | Sperre aufheben oder bereitgestelltes Konto aktivieren (`pending -> active`) | user | owner | ja | ja | ja | ja | `rl.admin` |
| `self.profile.update` | eigenes Profil ändern (Anzeigename, Avatar, Sprache, Zeitzone) | user | self | nein | nein | nein | nein | `rl.write` |
| `identity.read` | Identitäten, Passkeys und lokale Anmeldedaten eines Menschen lesen, ohne Geheimnisse | user | self oder owner | nein | nein | nein | nein | `rl.read` |
| `identity.link` | weitere Identität verknüpfen | user | self | nein | ja | ja | nein | `rl.admin` |
| `identity.unlink` | Identität lösen; abgelehnt, wenn kein Anmeldeweg bleibt | user | self oder owner | nein | ja | ja | nein | `rl.admin` |
| `local_credential.set` | lokales Passwort setzen und TOTP einrichten; Step-up, sobald ein Passwortweg mit TOTP eingerichtet ist | user | self | nein | bedingt | ja | nein | `rl.admin` |
| `passkey.register` | Passkey hinzufügen; Step-up, sobald ein aktiver Passkey existiert | user | self | nein | bedingt | ja | nein | `rl.admin` |
| `passkey.revoke` | Passkey widerrufen | passkey | self oder owner | nein | ja | ja | nein | `rl.admin` |
| `device.list` | Geräte lesen; ein wartendes Gerät liest sich selbst | user, device | self oder owner | nein | nein | nein | nein | `rl.read` |
| `device.approve` | wartendes Gerät freigeben; eigenes Gerät von einem aktiven eigenen Gerät aus, mit Bestätigungs-Code oder Kopplungs-Code ([08-identitaet.md](08-identitaet.md), Abschnitt 5.3) | device | owner oder self | nein | ja | ja | ja | `rl.admin` |
| `device.revoke` | Gerät sperren; Verbindungen schließen mit `4012` | device | self oder owner | nein | ja | ja | ja | `rl.admin` |
| `device.expire` | Gerät ohne Bestätigung nach `auth.device_pending_ttl`, abgelaufene bestätigte Kopplung oder Gerät ohne Kontakt nach `auth.device_inactive_revoke_after` widerrufen (Job `identity.auth_cleanup`) | device | system | nein | nein | ja | ja | keine |
| `session.list` | Anmeldesitzungen lesen | user | self oder owner | nein | nein | nein | nein | `rl.read` |
| `session.revoke` | Sitzung beenden; ihre Kind-Sitzungen enden ebenfalls | auth_session | self oder owner | nein | nein | ja | ja | `rl.admin` |
| `session.extension.create` | Kind-Sitzung für die Share-Extension der eigenen App mit eigener Refresh-Familie anlegen (die Notification Service Extension nutzt das Vorschau-Token, keine Sitzung); ersetzt eine aktive Kind-Sitzung derselben Komponente | auth_session | self | nein | nein | ja | nein | `rl.write` |
| `token.list` | API-Tokens lesen (nur Präfix) | principal | owner; self bei eigenen | nein | nein | nein | nein | `rl.read` |
| `token.create` | API-Token für Agent oder App erzeugen; persönliches Token nur für sich selbst und nur mit Schalter `tokens.personal_enabled` | principal | owner; self bei eigenem | nein | ja | ja | nein | `rl.admin` |
| `token.revoke` | API-Token widerrufen | api_token | owner; self bei eigenem | nein | nein | ja | ja | `rl.admin` |
<!-- END GENERATED: actions.identity -->

### 4.4 Gruppen und Freigaben

<!-- BEGIN GENERATED: actions.grants -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `group.read` | Gruppen und Mitglieder lesen; eigene Gruppen auch ohne Owner | group | owner oder self | nein | nein | nein | nein | `rl.read` |
| `group.create` | Gruppe anlegen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `group.update` | Gruppe umbenennen | group | owner | ja | ja | ja | nein | `rl.admin` |
| `group.delete` | Gruppe löschen (weich); ihre Freigaben werden widerrufen | group | owner | ja | ja | ja | ja | `rl.admin` |
| `group.member.add` | Mitglied hinzufügen, optional mit Ablauf und `resolve_i1`; nicht in Gruppen mit `source = oidc_claim` | group | owner | ja | ja | ja | ja | `rl.admin` |
| `group.member.remove` | Mitglied entfernen; nicht in Gruppen mit `source = oidc_claim` | group | owner | ja | ja | ja | ja | `rl.admin` |
| `group.member.expire` | abgelaufene Mitgliedschaft melden (Job `authz.expiry_sweep`): Rechte-Version, `access.changed`, Abbruch betroffener Läufe | group | system | nein | nein | ja | ja | keine |
| `group.member.sync` | Mitgliedschaften einer Person in Gruppen mit `source = oidc_claim` ihres Anbieters beim Abschluss einer OIDC-Anmeldung dem Gruppen-Claim angleichen; ein Audit-Eintrag je Anmeldung mit den Gruppen ([08-identitaet.md](08-identitaet.md), Abschnitt 3.4) | user | system | nein | nein | ja | ja | keine |
| `grant.read` | Freigaben eines Ziels oder Subjekts lesen; mit `manage` nur Freigaben auf Zielen im eigenen `manage`-Bereich ([07-rechte.md](07-rechte.md), Abschnitt 2.6); Freigaben auf Rechner nur Owner | space, channel, host, principal | owner oder manage | nein | nein | nein | nein | `rl.read` |
| `grant.create` | Freigabe anlegen. Mit `manage` nur bis zur eigenen Stufe, nur eigene Zusatzrechte, nur an Menschen und Gruppen, nie auf der Instanz (R-AZ-05). Freigaben an Agenten und Apps nur Owner; Agenten und Apps nie `trigger`, als Zusatzrecht nur `upload`. Auf einem Rechner (`host`) nur Owner, nur an Menschen und Gruppen, ohne Stufe, genau `terminal`; I6 (Regel 1.11). Optional `resolve_i1` | instance, space, channel, host | owner oder manage (auf `host` nur owner) | nein | ja | ja | ja | `rl.admin` |
| `grant.revoke` | Freigabe widerrufen; mit `manage` nur Freigaben, die man nach R-AZ-05 selbst hätte anlegen dürfen | grant | owner oder manage | nein | ja | ja | ja | `rl.admin` |
| `grant.expire` | abgelaufene Freigabe melden (Job `authz.expiry_sweep`): Rechte-Version, `access.changed`, Abbruch betroffener Läufe | grant | system | nein | nein | ja | ja | keine |
<!-- END GENERATED: actions.grants -->

Ablauf wirkt bei jedem `authorize` ab der Sekunde des Ablaufs; die beiden Systemaktionen sorgen
nur dafür, dass Clients ihren Cache räumen und Läufe enden (R-AZ-15).

### 4.5 Bereiche, Kanäle, Seitenleiste

<!-- BEGIN GENERATED: actions.structure -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `space.read` | Bereich sehen; gilt bei `read` auf dem Bereich oder auf mindestens einem seiner Kanäle | space | read | nein | nein | nein | nein | `rl.read` |
| `space.create` | Bereich anlegen | instance | owner | ja | nein | ja | nein | `rl.admin` |
| `space.update` | Name, Icon, Farbe ändern | space | manage | nein | nein | ja | nein | `rl.admin` |
| `space.reorder` | globale Reihenfolge der Bereiche | instance | owner | ja | nein | ja | nein | `rl.admin` |
| `space.archive` | Bereich archivieren | space | owner | ja | ja | ja | ja | `rl.admin` |
| `space.operator.set` | Standard-Bediener setzen (I1); optional `resolve_i1` | space | owner | ja | ja | ja | ja | `rl.admin` |
| `sidebar.update` | eigenen Seitenleisten-Zustand ändern (einklappen, eigene Reihenfolge) | space | self und read | nein | nein | nein | nein | `rl.write` |
| `channel.read` | Kanal sehen, Kopfdaten lesen | channel | read | nein | nein | nein | nein | `rl.read` |
| `channel.create` | Kanal anlegen | space | manage | nein | nein | ja | nein | `rl.admin` |
| `channel.update` | Name, Thema, Kanal-Voreinstellung der Benachrichtigung ändern | channel | manage | nein | nein | ja | nein | `rl.admin` |
| `channel.reorder` | Reihenfolge der Kanäle im Bereich | space | manage | nein | nein | ja | nein | `rl.admin` |
| `channel.behavior.read` | Prompt, Skills, Erwähnungsregeln, Backfill lesen | channel | manage | nein | nein | nein | nein | `rl.read` |
| `channel.behavior.update` | Prompt, Skills, Erwähnungsregeln, `auto_thread`, Backfill ändern | channel | manage | nein | ja | ja | nein | `rl.admin` |
| `channel.operator.set` | Bediener setzen oder Erbe einschalten (I1); optional `resolve_i1` | channel | owner | ja | ja | ja | ja | `rl.admin` |
| `channel.project.bind` | Projektbindung eines Forums setzen oder ändern | channel | owner | ja | ja | ja | nein | `rl.admin` |
| `channel.read_only.set` | Schreibschutz setzen oder aufheben (Regel 1.8), mit Grund; beim Setzen werden aktive Läufe im Kanal abgebrochen | channel | manage | nein | ja | ja | nein | `rl.admin` |
| `channel.archive` | Kanal archivieren | channel | manage | nein | ja | ja | ja | `rl.admin` |
| `forum_tag.manage` | Tags eines Forums anlegen, ändern, löschen | channel | manage | nein | nein | ja | nein | `rl.admin` |
| `dm.create` | Direktnachricht mit anderen Menschen anlegen; nur mit Schalter `dm.enabled` | instance | human | nein | nein | nein | nein | `rl.write` |
| `dm.leave` | DM verlassen | channel | participant | nein | nein | nein | nein | `rl.write` |
<!-- END GENERATED: actions.structure -->

### 4.6 Threads und Agenten-Sitzungen

<!-- BEGIN GENERATED: actions.threads -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `thread.read` | Thread und Verlauf lesen | thread | read | nein | nein | nein | nein | `rl.read` |
| `thread.create` | Thread aus Nachricht oder Forum-Beitrag anlegen; Tags beim Anlegen gelten wie bei `thread.tags.set` (Tags mit `restricted` brauchen `manage`) | channel | write | nein | nein | nein | nein | `rl.message` |
| `thread.title.update` | Titel ändern; setzt `title_source = manual`. Bediener-Agent darf nur bei `auto` setzen, die Quelle bleibt dann `auto` | thread | write und (Ersteller oder manage oder Bediener-Agent) | nein | nein | nein | nein | `rl.write` |
| `thread.close` | Thread "Erledigt" setzen | thread | write | nein | nein | nein | nein | `rl.write` |
| `thread.reopen` | Thread wieder öffnen (auch implizit durch neue Nachricht) | thread | write | nein | nein | nein | nein | `rl.write` |
| `thread.move` | in anderen Kanal verschieben; Ziel mit gleichem Bediener | thread | manage auf Quelle und Ziel | nein | nein | ja | nein | `rl.admin` |
| `thread.tags.set` | Tags setzen; Tags mit `restricted` brauchen `manage` | thread | write | nein | nein | nein | nein | `rl.write` |
| `agent_session.read` | aktuelle und frühere Sitzungs-Referenzen, Modell, Kosten | thread | read | nein | nein | nein | nein | `rl.read` |
| `agent_session.reset` | neue Sitzung im Thread beginnen (z. B. `/new`) | thread | trigger und `agent.invoke` | nein | nein | nein | nein | `rl.trigger` |
| `agent_session.bind` | vorhandene externe Sitzung an den Thread binden (Fortsetzen) | thread | trigger und `agent.invoke` | nein | nein | ja | nein | `rl.trigger` |
| `agent_session.report` | Adapter meldet externe Sitzungs-Referenz, Modell, `started` oder `bind_failed`; die erste Meldung legt die Zeile an, wenn `run.start` keine Referenz trug (Compare-and-swap) | agent_session, run | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
<!-- END GENERATED: actions.threads -->

### 4.7 Nachrichten, Reaktionen, Anheften, Vorlagen

<!-- BEGIN GENERATED: actions.messages -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `message.read` | Nachricht lesen | message | read | nein | nein | nein | nein | `rl.read` |
| `message.create` | Nachricht senden; mit Anhängen zusätzlich `+upload`. Löst sie einen Agenten aus, gilt zusätzlich `agent.invoke`. | thread | write | nein | nein | nein | nein | `rl.message` |
| `message.update` | eigene Nachricht bearbeiten, mit `If-Match` | message | write und Autor | nein | nein | nein | nein | `rl.message` |
| `message.delete` | eigene Nachricht löschen; fremde nur mit `manage` (Moderation) | message | (write und Autor) oder manage | nein | nein | ja bei fremder | nein | `rl.message` |
| `message.revisions.read` | frühere Fassungen lesen | message | read | nein | nein | nein | nein | `rl.read` |
| `reaction.add` | Reaktion setzen | message | write | nein | nein | nein | nein | `rl.write` |
| `reaction.remove` | eigene Reaktion entfernen | reaction | write und self | nein | nein | nein | nein | `rl.write` |
| `pin.create` | Nachricht anheften | message | manage | nein | nein | nein | nein | `rl.write` |
| `pin.delete` | Anheften lösen | message | manage | nein | nein | nein | nein | `rl.write` |
| `mention.list` | eigene Erwähnungen lesen (gefiltert durch `message.read`) | user | self | nein | nein | nein | nein | `rl.read` |
| `template.read` | Vorlagen lesen (eigene und des Kanals) | channel, user | read bzw. self | nein | nein | nein | nein | `rl.read` |
| `template.manage` | eigene Vorlagen pflegen (Ressource `user`); Kanal-Vorlagen mit `manage` (Ressource `channel`) | user, channel | self oder manage | nein | nein | nein | nein | `rl.write` |
<!-- END GENERATED: actions.messages -->

### 4.8 Dateien und Suche

<!-- BEGIN GENERATED: actions.files -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `file.upload` | Datei für einen Kanal hochladen (Upload-Sitzung, Teile, Abschluss); Avatar und Logo laufen über `self.profile.update`, `agent.update` bzw. `instance.branding.update` | channel | write und +upload | nein | nein | ja bei Malware-Fund | nein | `rl.upload` |
| `file.read` | Datei, Vorschaubild, Transkript oder Abruf-Ticket; erst nach Scan; Pfad je `purpose` nach 2.2 | file | read | nein | nein | nein | nein | `rl.read` |
| `file.delete` | eigene, nicht angehängte Datei löschen; angehängte über `message.delete` | file | self oder manage | nein | nein | nein | nein | `rl.write` |
| `file.transcribe` | Transkription einer Sprachnachricht durch den Server anstoßen (Schalter `voice.enabled`) | file | write | nein | nein | nein | nein | `rl.upload` |
| `file.transcript.submit` | Transkript einer Sprachnachricht speichern (`transcript_source = 'agent'`), mit `If-Match` | file | agent-self: Bediener des Threads, an dessen Nachricht die Datei hängt, und read | nein | nein | nein | nein | `rl.agent_runtime` |
| `search.query` | Volltextsuche; Treffer gefiltert durch `message.read`, `thread.read`, `file.read` | instance | auth | nein | nein | nein | nein | `rl.search` |
<!-- END GENERATED: actions.files -->

### 4.9 Persönliche Daten, Benachrichtigung, Sync

<!-- BEGIN GENERATED: actions.personal -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `read_state.update` | Gelesen-Stand vorrücken | thread | self und read | nein | nein | nein | nein | `rl.write` |
| `read_state.mark_unread` | Gelesen-Stand zurücksetzen, mit `If-Match` | thread | self und read | nein | nein | nein | nein | `rl.write` |
| `draft.read` | eigene Entwürfe lesen | user | self | nein | nein | nein | nein | `rl.read` |
| `draft.write` | Entwurf speichern oder löschen, auch in schreibgeschützten und archivierten Kanälen (Senden prüft `message.create`) | thread, channel | self und read | nein | nein | nein | nein | `rl.write` |
| `settings.read` | eigene persönliche Daten lesen: Einstellungen, Gelesen-Stände, Seitenleiste, Benachrichtigungsstufen, Ruhezeiten | user | self | nein | nein | nein | nein | `rl.read` |
| `settings.update` | eigene Einstellung ändern | user | self | nein | nein | nein | nein | `rl.write` |
| `notification_pref.update` | Benachrichtigungsstufe für Bereich, Kanal oder Thread | space, channel, thread | self und read | nein | nein | nein | nein | `rl.write` |
| `quiet_hours.update` | Ruhezeiten pflegen | user | self | nein | nein | nein | nein | `rl.write` |
| `push.register` | Push-Ziel des eigenen aktiven Geräts registrieren; liefert einmal das Vorschau-Token | device | self | nein | nein | nein | nein | `rl.write` |
| `push.unregister` | Push-Ziel entfernen | push_registration | self | nein | nein | nein | nein | `rl.write` |
| `push.deliver` | interne Prüfung vor jedem Push, jedem Hinweis über das Gateway und jedem Abruf über den Vorschau-Endpunkt (P3); hints without a thread (`security`, `admin`) use the recipient rule of R-AZ-30 instead | thread | read | nein | nein | nein | nein | keine |
| `client_metrics.submit` | Leistungsdaten der App ohne Inhalte und ohne IDs senden, nur bei `privacy.client_metrics` ([15-client-architektur.md](15-client-architektur.md)) | instance | human | nein | nein | nein | nein | `rl.write` |
| `sync.snapshot.read` | Momentaufnahme für Neuladen | instance | auth | nein | nein | nein | nein | `rl.read` |
| `events.catch_up` | `GET /api/v1/events`; jedes Ereignis zusätzlich mit seiner Empfangs-Aktion | instance | auth | nein | nein | nein | nein | `rl.read` |
| `gateway.connect` | WebSocket `identify` oder `resume` | instance | auth (Mensch mit aktivem Gerät, Agent, App mit `events:subscribe`) | nein | nein | abl | nein | `rl.gateway_connect` |
| `gateway.focus` | Threads fokussieren; nicht lesbare werden still ignoriert | thread | read | nein | nein | nein | nein | `rl.read` |
| `typing.start` | Tippen anzeigen; nur Menschen, Agenten zeigen Tippen über ihren Lauf (04, Abschnitt 12) | thread | human und write | nein | nein | nein | nein | `rl.typing` |
<!-- END GENERATED: actions.personal -->

### 4.10 Agenten, Befehle, Läufe, Interaktionen

<!-- BEGIN GENERATED: actions.agents -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `agent.read` | Agenten-Karte lesen (Name, Modus-Abzeichen, Werkzeugklasse, Präsenz); der Agent liest sich selbst, auch das Soll seiner Backend-Richtlinie | agent | owner oder agent-self oder read auf einem Kanal, den der Agent bedient | nein | nein | nein | nein | `rl.read` |
| `agent.create` | Agent anlegen | instance | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.update` | Name, Beschreibung, Avatar, `operator_config` ändern | agent | owner | ja | ja | ja | nein | `rl.admin` |
| `agent.disable` | Agent sperren; Tokens und Verbindungen enden, Läufe brechen ab | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.enable` | Sperre eines Agenten aufheben (I1) | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.billing_mode.set` | Backend-Richtlinie eines Agenten umschalten, mit Grund (I4, I1) | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.billing_mode.set_all` | Backend-Richtlinie mehrerer Agenten in einer Transaktion umschalten, mit Grund (I4, I1); auf `personal` nur mit ausdrücklicher Liste | instance | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.credential_holder.set` | `credential_holder` setzen (I1) | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.tool_class.set` | Werkzeugklasse (`chat`, `scoped`, `host`) setzen, mit Grund ([07-rechte.md](07-rechte.md), Abschnitt 2.11) | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.invoke` | Agenten auslösen; Prüfung in Abschnitt 5 | thread (mit Agent) | trigger und Regeln aus 5 | nein | nein | abl | nein | `rl.trigger` |
| `agent.heartbeat` | Zustand, Ist-Modus, belegbare Modi, Werkzeugklasse und aktive Läufe melden | agent | agent-self | nein | nein | ja bei Wechsel des Ist-Modus oder der Werkzeugklasse | nein | `rl.agent_runtime` |
| `agent.events.consume` | Ereignisse als Verbraucher empfangen und bestätigen (`ack`) | agent | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `agent.config.read` | Prompt, Skills, Projektbindung der Kanäle lesen, die der Agent bedient | channel | agent-self und wirksamer Bediener | nein | nein | nein | nein | `rl.read` |
| `agent.triggerers.read` | Menge der Menschen, die diesen Agenten auslösen dürfen, mit `permission_version` (Erlaubnisliste im Adapter) | agent | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `command.read` | Befehlsliste und Optionen sehen (eingebaute, des Bedieners, erlaubter Gäste und Apps mit `write`) | channel | read | nein | nein | nein | nein | `rl.read` |
| `command.register` | Befehl registrieren oder ändern; Geltungsbereich nur auf Zielen, die der Eigentümer bedient oder auf denen er `write` hat | command | agent-self bzw. app-self mit Scope | nein | nein | ja | nein | `rl.admin` |
| `command.remove` | Befehl entfernen | command | agent-self bzw. app-self mit Scope | nein | nein | ja | nein | `rl.admin` |
| `command.autocomplete` | Vorschläge für eine Option vom Eigentümer des Befehls holen | channel | trigger und `agent.invoke` ohne Lauf (bei App-Befehlen `required_level`) | nein | nein | nein | nein | `rl.trigger` |
| `command.autocomplete.respond` | Vorschläge zu einer Anfrage liefern (Gateway-Op oder REST) | command | agent-self bzw. app-self, Eigentümer des Befehls | nein | nein | nein | nein | `rl.agent_runtime` |
| `command.invoke` | Befehl ausführen. Agenten-Befehle prüfen `required_level` und `agent.invoke`; Modell-Befehle bei aktivem Lauf: Ablehnung `run_active`. App-Befehle prüfen `required_level` und Scope, ohne `agent.invoke` und I2. Steuerbefehl `stop` prüft zusätzlich `run.cancel` | channel, thread | `required_level` des Befehls und Regeln wie beschrieben | nein | nein | abl | nein | `rl.trigger` |
| `run.read` | Lauf, Schritte, Kennzahlen lesen | run | read | nein | nein | nein | nein | `rl.read` |
| `run.overview.read` | "Läuft gerade" lesen, gefiltert durch `run.read` | instance | auth | nein | nein | nein | nein | `rl.read` |
| `run.context.read` | Kontext eines eigenen aktiven Laufs lesen (Schema `run_context.v1` mit Vertrauensmarke je Nachricht, [09-agenten.md](09-agenten.md), Abschnitt 4.6) | run | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `run.start` | Lauf starten mit gültiger Marke: `queued -> running` und `waiting_input -> running` (CAS); dieselbe Marke wiederholt liefert das Ergebnis erneut | run | agent-self und Marke gültig | nein | nein | abl | nein | `rl.agent_runtime` |
| `run.token.renew` | fresh invocation token for one's own run in `queued` or `waiting_input` with the current unconsumed token, even an expired one (`runs.started_jti` NULL or not equal to `jti`, in SQL `IS DISTINCT FROM`), or the token before the last renewal (`runs.renewed_from_jti`, lost response); repeats the checks before a delivery (agent.invoke, I2, SR2, SR3) with current permissions ([09-agenten.md](09-agenten.md), section 4.5.1) | run | agent-self und Marke aktuell | nein | nein | abl | nein | `rl.agent_runtime` |
| `run.update` | Live-Box-Patch des eigenen Laufs | run | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `run.finish` | Lauf beenden oder warten lassen: `completed`, `failed`, `cancelled`, `waiting_input` | run | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `run.cancel` | Lauf abbrechen | run | trigger und (Auslöser des Laufs oder manage) | nein | nein | ja | nein | `rl.trigger` |
| `run.followup.create` | Nachtrag zu einem aktiven Lauf desselben Agenten (über `message.create`) | thread | write und `agent.invoke` | nein | nein | abl | nein | `rl.message` |
| `component.update` | eigene Komponenten deaktivieren oder ändern | component | Autor der Nachricht (agent-self bzw. app-self) | nein | nein | nein | nein | `rl.write` |
| `interaction.create` | Komponente bedienen (Einmal-Regel je Nachricht); `required_capability` und `restricted_to` (`any`, `run_starter`, `owner`) beachten; bei Komponenten eines Agenten zusätzlich `agent.invoke` | component | trigger | nein | nein | ja bei `approve_actions`, sonst abl | nein | `rl.interaction` |
| `interaction.respond` | Eigentümer bestätigt oder beantwortet eine Interaktion | interaction | agent-self bzw. app-self | nein | nein | nein | nein | `rl.agent_runtime` |
<!-- END GENERATED: actions.agents -->

### 4.11 Apps und Webhooks

<!-- BEGIN GENERATED: actions.apps -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `app.read` | Apps, Manifeste, Installationen lesen | app | owner | ja | nein | nein | nein | `rl.read` |
| `app.install` | App installieren, Scopes bewilligen | instance | owner | ja | ja | ja | ja | `rl.admin` |
| `app.scopes.update` | bewilligte Scopes ändern | app | owner | ja | ja | ja | ja | `rl.admin` |
| `app.suspend` | App abschalten ohne Verlust der Bewilligungen: Tokens gesperrt, Zustellungen pausiert, Befehle ausgeblendet, Komponenten `disabled` | app | owner | ja | ja | ja | ja | `rl.admin` |
| `app.resume` | abgeschaltete App wieder einschalten | app | owner | ja | ja | ja | ja | `rl.admin` |
| `app.uninstall` | App deinstallieren; Tokens und Abos enden | app | owner | ja | ja | ja | ja | `rl.admin` |
| `webhook_out.read` | ausgehende Abos und Zustellungen lesen | app, agent | owner | ja | nein | nein | nein | `rl.read` |
| `webhook_out.manage` | ausgehendes Abo einer App oder eines Agenten anlegen, ändern, löschen, HMAC-Geheimnis rotieren | app, agent | owner | ja | ja | ja | nein | `rl.admin` |
| `webhook_out.redeliver` | Zustellung wiederholen | webhook_delivery | owner | ja | nein | ja | nein | `rl.admin` |
| `webhook_out.deliver` | interne Prüfung vor jeder Zustellung: Empfangs-Aktion des Ereignisses für die App bzw. den Agenten | app, agent | Freigaben und Scope des Empfängers | nein | nein | nein | nein | keine |
| `webhook_out.transition` | automatic state change of an outgoing subscription by the consumer `webhooks` or the job `apps.webhook_deliver`: `disabled` after `410`, after `webhook.max_consecutive_failures` failures in a row, after a delivery that fails until `webhook.retry_max_age`, or on a gap in the event log; `paused` by the delivery quota `webhook.max_pending_per_subscription` ([11-erweiterungen.md](11-erweiterungen.md), sections 6 and 7; [webhooks.md](../api/webhooks.md), sections 4 and 5) | app, agent | system | nein | nein | ja | nein | keine |
| `webhook_in.read` | Eingangs-Webhooks eines Kanals lesen (ohne Token) | channel | manage | nein | nein | nein | nein | `rl.read` |
| `webhook_in.create` | Eingangs-Webhook für einen Kanal anlegen | channel | manage | nein | ja | ja | nein | `rl.admin` |
| `webhook_in.revoke` | Eingangs-Webhook widerrufen | incoming_webhook | manage | nein | ja | ja | nein | `rl.admin` |
| `webhook_in.post` | Nachricht über Eingangs-Webhook; schreibt als App mit `origin = app` | channel | public mit Token; App braucht write | nein | nein | abl | nein | `rl.webhook_in` |
<!-- END GENERATED: actions.apps -->

### 4.12 Import, Kosten, Datenschutz, Status

<!-- BEGIN GENERATED: actions.operations -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `import.read` | Importquellen, Fortschritt, Fehler lesen | import_source | owner | ja | nein | nein | nein | `rl.read` |
| `import.manage` | Quelle anlegen, Start, Pause, Abbruch, Abschluss, Eigentumsliste je Kanal (der Trockenlauf liest nur und braucht `import.read` und Leserechte) | import_source | owner | ja | ja | ja | nein | `rl.admin` |
| `import.apply` | Stapel mit Herkunftsschlüsseln einer laufenden Quelle schreiben (Objekte eines Threads, Dateien, Cursor); nur in Kanälen dieser Quelle, auch schreibgeschützten ([22-import-migration.md](22-import-migration.md), Abschnitt 8) | import_source | owner | ja | nein | abl | nein | `rl.import` |
| `cost.read.thread` | Kosten eines Laufs und Threads (Kopfzeile, Live-Box) | thread | read | nein | nein | nein | nein | `rl.read` |
| `cost.read.aggregate` | Kosten je Bereich, Agent, Instanz | instance, space, agent | owner; Bereich mit manage | nein | nein | nein | nein | `rl.read` |
| `price.read` | Preistabelle lesen | instance | owner | ja | nein | nein | nein | `rl.read` |
| `price.manage` | Preise mit Quelle und Abrufzeit pflegen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `cost.reconcile` | Schätzungen von Hand mit der Abrechnung abgleichen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `cost.reconcile.submit` | Abgleich-App übermittelt `reconciled`-Einträge zu vorhandenen Läufen; löscht nichts ([10-backend-richtlinie.md](10-backend-richtlinie.md), Abschnitt 12.4; [ADR 0055](../adr/0055-kostenabgleich-als-eigene-app-admin-key-nie-im-server.md)) | instance | App mit Scope `costs:reconcile` | nein | nein | ja | nein | `rl.admin` |
| `export.channel` | Kanal-Verlauf exportieren | channel | read und +export | nein | ja | ja | nein | `rl.export` |
| `gdpr.request.create` | Export oder Löschung für sich selbst; für andere nur Owner | user | self oder owner | nein | ja | ja | nein | `rl.export` |
| `gdpr.request.read` | Auftrag und Stand lesen | gdpr_request | self oder owner | nein | nein | nein | nein | `rl.read` |
| `gdpr.request.decide` | Auftrag freigeben oder ablehnen; der Export der eigenen Daten wird bei `privacy.export_requires_approval = false` vom System freigegeben | gdpr_request | owner | ja | ja | ja | nein | `rl.admin` |
| `gdpr.export.download` | Exportpaket herunterladen | gdpr_request | self (Betroffene Person) oder owner | nein | ja | ja | nein | `rl.export` |
| `status.read` | Statusseite lesen | instance | auth | nein | nein | nein | nein | `rl.read` |
| `status.manage` | Komponenten und Störungen pflegen | instance | owner | ja | nein | ja | nein | `rl.admin` |
<!-- END GENERATED: actions.operations -->

### 4.13 Dev-Umgebungen (Etappe)

Gehören zur Etappe "Mehrbenutzer-Dev-Umgebung"; Routen entstehen mit ihr
([21-dev-isolation.md](21-dev-isolation.md)).

<!-- BEGIN GENERATED: actions.dev -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `dev_environment.read` | Umgebung, Zustand und Prüfergebnisse lesen | dev_environment | owner oder self | nein | nein | nein | nein | `rl.read` |
| `dev_environment.manage` | Umgebung anlegen, freigeben, sperren, zurückziehen | dev_environment | owner | ja | ja | ja | ja | `rl.admin` |
| `dev_environment.report` | Runner meldet Prüfergebnisse und Bereitschaft | dev_environment | agent-self (Runner der Umgebung) | nein | nein | ja | nein | `rl.agent_runtime` |
<!-- END GENERATED: actions.dev -->

### 4.14 Rechner und Terminal

Normativ zusammen mit [24-terminal.md](24-terminal.md). Etappe M2.

<!-- BEGIN GENERATED: actions.terminal -->
| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `host.read` | Rechner lesen: Name, Zustand, Plattform, Profile, Fingerabdruck, LLM-Erklärung, Prüfergebnisse; Owner alle, sonst Rechner, deren Inhaber man ist oder auf denen man eine Freigabe `terminal` hat | host | owner oder assignee oder +terminal | nein | nein | nein | nein | `rl.read` |
| `host.manage` | Rechner anlegen, ändern (Name, Inhaber), Kopplungs-Code ausstellen (auch Neukopplung), Fingerabdruck bestätigen, LLM-Anmeldung erklären, sperren, entsperren, zurückziehen; I6 (Regel 1.11) | instance, host | owner | ja | ja | ja | ja | `rl.admin` |
| `host.pair` | Host-Dienst löst einen Kopplungs-Code ein und meldet seine öffentlichen Schlüssel | host | public (gültiger Kopplungs-Code) | nein | nein | ja | nein | `rl.public_auth` |
| `host.connect` | Steuerverbindung und Datenverbindungen des Host-Dienstes; Meldungen (Profile, Prüfergebnisse, Start und Ende von Sitzungen) | host | host-self | nein | nein | abl; ja bei Wechsel von Server-Host oder Prüfergebnis | nein | `rl.host` |
| `terminal.open` | Terminal-Sitzung öffnen; SR10, SR11 (I6), Grenzen; gibt die Sitzungs-Marke aus | host | human und +terminal | nein | frisch | ja | nein | `rl.terminal` |
| `terminal.attach` | an eine eigene, nicht geschlossene Sitzung anhängen, auch von einem anderen Gerät; neue Marke. Step-up frisch, außer vom selben Gerät in derselben Anmeldesitzung innerhalb von `terminal.quick_reattach_window` nach der Trennung; diese Ausnahme gilt nie in der Web-Konsole | terminal_session | self und +terminal | nein | frisch | ja | nein | `rl.terminal` |
| `terminal.stream` | Strom-Verbindung des Geräts zu einer eigenen Sitzung öffnen; nur das Gerät der aktuellen Marke | terminal_session | self | nein | nein | abl | nein | `rl.terminal` |
| `terminal.read_own` | eigene Terminal-Sitzungen lesen; Owner lesen alle | terminal_session, host | self oder owner | nein | nein | nein | nein | `rl.read` |
| `terminal.close` | Sitzung beenden; eigene, Owner jede | terminal_session | self oder owner | nein | nein | ja | nein | `rl.write` |
| `terminal.terminate` | Sitzung durch den Server beenden: Entzug, Leerlauf, Höchstdauer, Trennung über `terminal.detach_grace`, Sperre, Abmeldung, Zustandswechsel des Rechners, Ende des Prozesses | terminal_session | system | nein | nein | ja | nein | keine |
<!-- END GENERATED: actions.terminal -->

Agenten, Apps und persönliche Tokens erreichen keine dieser Aktionen außer `host.pair` und
`host.connect`, die keinen Prinzipal aus `principals` haben (`subject_kind_not_allowed`, SR5).
Einen Scope für Apps gibt es nicht.

## 5. Agenten-Aufruf `agent.invoke`

`authorize(P, agent.invoke, {thread T, agent A, trigger_message M | interaction I | command C})`
ist `allow` genau dann, wenn alle Punkte gelten. Er läuft beim Annehmen der Anfrage und noch
einmal unmittelbar vor jeder Zustellung an den Adapter (I2). Welche Nachricht überhaupt einen
Agenten auslöst, entscheidet [09-agenten.md](09-agenten.md), Abschnitt 3.4.

1. P ist ein aktiver Mensch. Agenten, Apps, Importe und Systemnachrichten lösen nie aus
   (`messages.origin = 'human'`).
2. P hat wirksam `trigger` auf dem Kanal von T. Bei DM-Kanälen gibt es keine Agenten. Der Kanal
   ist nicht schreibgeschützt (Regel 1.8).
3. A ist aktiv und eines von beiden:
   - A ist der Bediener von T: `threads.operator_agent_id` bei `kind = 'thread'`, der wirksame
     Bediener des Kanals im Hauptverlauf und bei neuen Threads; die Erwähnungsregel des Kanals
     ist erfüllt ([03-datenmodell.md](03-datenmodell.md), `channels`); oder
   - A ist in M erwähnt, `channels.allow_mentioned_agents` ist `true`, A hat selbst mindestens
     `write` auf dem Kanal, und A hat `billing_mode = 'organization'` (ein Agent mit `personal`
     ist nie Gast-Agent, weil I1 Gast-Kanäle nicht zu seinen Zielen zählt).
4. Backend-Richtlinie: Bei `agents.billing_mode = 'personal'` gilt `P = credential_holder_id`
   (I2). Sonst Ablehnung mit Grund `billing_mode_personal_not_holder`, fail closed, Audit.
5. Werkzeugklasse (SR2): Ist die höhere von `agents.tool_class` und `agents.reported_tool_class`
   gleich `host`, ist P ein aktiver Owner, oder SR3 weist eine bereite eigene Dev-Umgebung von P
   nach. Sonst `tool_class_owner_only`.
6. Bei einer Interaktion: Komponente aktiv und nicht abgelaufen; P hat `required_capability`
   (z. B. `approve_actions`); `restricted_to = 'run_starter'`: P ist Auslöser des Laufs oder
   Owner; `restricted_to = 'owner'`: P ist Owner. Die Einmal-Regel setzt die Komponente in
   derselben Transaktion auf `used` und die übrigen `single_use`-Komponenten der Nachricht auf
   `disabled`.
7. Bei Forum-Kanälen mit Projektbindung: `project_bindings.ready = true` (sonst
   `project_not_ready`) und SR3 aus [07-rechte.md](07-rechte.md) (Dev-Umgebung, Etappe).
8. Der Thread hat höchstens einen aktiven Lauf. Gibt es einen von A, wird eine Auslösung durch
   eine Nachricht ein Nachtrag (`run.followup.create`); gehört er einem anderen Agenten,
   Ablehnung `run_active_other_agent`. Steuerbefehle gehen als `command.invoked` sofort an den
   Agenten; Modell-Befehle werden bei aktivem Lauf abgelehnt (`run_active`).

App-Befehle und Komponenten von Apps prüfen `agent.invoke` und I2 nicht, weil keine
Agenten-Laufzeit beteiligt ist; geprüft werden `required_level`, `required_capability`,
`restricted_to`, Freigaben und Scope der App ([11-erweiterungen.md](11-erweiterungen.md),
Abschnitt 9).

**Check without a request.** In these cases the server evaluates the permissions of a person
without a request of this person (R-AZ-29):

- `agent.invoke` immediately before every delivery of `agent.invocation`, `agent.followup`,
  `interaction.created` and `command.invoked` to an agent
  ([04-sync-protokoll.md](04-sync-protokoll.md), section 15.3 step 2, section 15.4 item 2 and
  section 15.6), at `run.token.renew` (04 section 15.3 step 3) and for the new run from deferred
  follow-ups (04 section 15.4 item 4);
- `command.invoke` and `interaction.create`, when the server checks an app command or the use of
  a component of an app again with the permissions of that moment before handing it to the app
  ([11-erweiterungen.md](11-erweiterungen.md), section 9; R-SY-02);
- the permissions of the triggering person in the acting-for binding (rule 2.3 item 6, SR9 in
  [07-rechte.md](07-rechte.md)).

The following applies:

- The person is a principal of kind `user` without session, device, token, `view_as` and
  `acting_for`. Only the server itself builds this principal, never the authentication of a
  request.
- Blocks come only from the person's status (SR7): `disabled` or `deleted` gives
  `principal_inactive`, `pending` gives `principal_pending`; in the acting-for binding such a
  person has no permissions. Sessions, devices and tokens of the person do not count.
- The owner status comes from the current owner list (`owners` with `revoked_at IS NULL`),
  without a session (exception to rule 2.3 item 1 and R-AZ-14); everything else from rule 2.3
  applies unchanged. A permission state computed this way is never the own permission state of
  the principal of a request, not even of a personal API token of the same person; in the
  acting-for binding it only narrows.
- `authorize` with this principal and an action other than `agent.invoke`, `command.invoke` or
  `interaction.create` is a programming error and ends like an unknown action (rule 1.5).
- If such a check writes an audit entry, this person is the actor, without device and session.

Folgen bei `allow`: Lauf `queued`, Live-Box-Anker, `trigger_state`, Ereignis an den Agenten,
Aufruf-Marke bei Zustellung ([04-sync-protokoll.md](04-sync-protokoll.md), Abschnitt 15). Die
Marke ist eine JWS mit Kopf `alg = EdDSA`, `kid` und `typ = agent-invocation+jwt` (RFC 8725,
Abschnitt 3.11). Claims:

| Claim | Inhalt |
|---|---|
| `iss` | `instance.instance_id` |
| `aud` | Prinzipal-ID des Agenten |
| `sub` | Prinzipal-ID des auslösenden Menschen |
| `jti`, `iat`, `exp` | UUIDv7 der Marke, Ausstellung, Ablauf (`iat + agent.invocation_token_ttl`) |
| `trigger_kind` | `message`, `followup`, `interaction`, `command` oder `autocomplete` |
| `run_id` | Lauf; fehlt nur bei `trigger_kind = autocomplete` |
| `message_id` oder `interaction_id` oder `command_id` | auslösendes Objekt |
| `thread_id`, `channel_id`, `space_id` | Ort |
| `level` | wirksame Stufe von `sub` auf dem Kanal zur Zustellzeit |
| `billing_mode` | Soll aus `agents.billing_mode` zur Zustellzeit (I3) |
| `tool_class` | Soll aus `agents.tool_class` zur Zustellzeit |
| `dev_environment_id` | Dev-Umgebung des Laufs, nur in der Etappe Dev-Umgebung |
| `permission_version` | Rechte-Version zur Zustellzeit |

Der Adapter startet keinen Lauf, wenn sein Ist-Modus nicht gleich `billing_mode` ist
(`billing_mode_mismatch`) oder seine tatsächliche Werkzeugklasse höher als `tool_class` ist
(`tool_class_mismatch`).

Folgen bei Ablehnung: Die Nachricht wird gespeichert, wenn `message.create` erlaubt war, aber
ohne Lauf; `trigger_state = failed` und `trigger_state_reason` mit dem Grund, damit der Mensch es
sieht. Bei Ablehnung erst vor der Zustellung: Lauf `failed` mit Grund, `trigger_state = failed`.

## 6. Dauerhafte Ereignistypen und Empfangs-Aktion

Ein Empfänger bekommt ein Ereignis nur, wenn `authorize(empfänger, receive_action, resource)`
erlaubt. Persönliche Ereignisse (`audience = self`) gehen nur an den genannten Prinzipal.

<!-- BEGIN GENERATED: events.durable -->
| Ereignistyp | Ressource | Empfangs-Aktion | Hinweis |
|---|---|---|---|
| `message.created` | message | `message.read` | Echo mit `client_msg_id` an den Autor |
| `message.updated` | message | `message.read` | Bearbeiten, `trigger_state`, Komponenten-Zustand |
| `message.deleted` | message | `message.read` | Tombstone |
| `reaction.added` | message | `message.read` | |
| `reaction.removed` | message | `message.read` | |
| `pin.added` | message | `thread.read` | |
| `pin.removed` | message | `thread.read` | |
| `thread.created` | thread | `thread.read` | |
| `thread.updated` | thread | `thread.read` | Titel, Tags, Bediener; beim Import einmal je Stapel statt `message.created` je Nachricht |
| `thread.closed` | thread | `thread.read` | |
| `thread.reopened` | thread | `thread.read` | |
| `thread.moved` | thread | `thread.read` | Geltung: Zielkanal |
| `thread.moved_out` | channel | `channel.read` | Geltung: Quellkanal; Empfänger ohne Zugriff aufs Ziel entfernen den Thread |
| `agent_session.changed` | thread | `thread.read` | neue Sitzung, Umbinden, Bindung nach Import |
| `run.state_changed` | run | `run.read` | mit Zusammenfassung |
| `space.created` | space | `space.read` | |
| `space.updated` | space | `space.read` | Name, Icon, Reihenfolge, Bediener; ändert sich der Standard-Bediener, folgt je erbendem, nicht archiviertem Kanal ein `channel.updated` mit neuer Version, weil sich dessen wirksamer Bediener ändert |
| `space.archived` | space | `space.read` | |
| `channel.created` | channel | `channel.read` | |
| `channel.updated` | channel | `channel.read` | Name, Thema, Reihenfolge, Bediener, `read_only`, `default_notification_level`; Änderungen an Forum-Tags (nur die neue Version, Tags per REST) |
| `channel.settings_changed` | channel | `channel.read` | nur `settings_version`; Inhalte per `channel.behavior.read` bzw. `agent.config.read` |
| `channel.archived` | channel | `channel.read` | |
| `command.changed` | command | `command.read` | registriert, geändert, entfernt |
| `file.updated` | file | `file.read` | Scan-Ergebnis, Vorschaubild, Transkript |
| `user.updated` | user | `user.read` | |
| `agent.updated` | agent | `agent.read` | auch beim Anlegen (es gibt keinen eigenen Typ); inklusive Wechsel der Backend-Richtlinie und der Werkzeugklasse; erreicht auch den Agenten selbst (Soll-Änderung) |
| `instance.updated` | instance | `instance.read` | Branding, Schalter (nur Schlüssel, Werte per REST) |
| `status.updated` | instance | `status.read` | |
| `grant.changed` | grant | `grant.read` | für Verwaltende; das Subjekt bekommt `access.changed` |
| `group.changed` | group | `group.read` | |
| `access.changed` | principal | self | Abschnitt 8 in 04 |
| `access.expiring` | principal | self | Freigabe oder Mitgliedschaft läuft innerhalb von `authz.expiry_reminder_lead` ab; an das Subjekt (bei Gruppen die Person) und an den Erteilenden |
| `session.revoked` | auth_session | self | zusätzlich Schließen mit `4012` |
| `read_state.updated` | thread | self | |
| `draft.updated` | draft | self | |
| `draft.deleted` | draft | self | |
| `settings.updated` | user | self | |
| `sidebar.updated` | space | self | |
| `notification_pref.updated` | user | self | |
| `quiet_hours.updated` | user | self | |
| `gdpr.request_updated` | gdpr_request | `gdpr.request.read` | |
| `import.progress` | import_source | `import.read` | |
| `agent.invocation` | run | self (Agent) und `agent.events.consume` | vor Zustellung `agent.invoke` für den Auslöser |
| `agent.followup` | run | self (Agent) und `agent.events.consume` | vor Zustellung `agent.invoke` für den Auslöser |
| `run.cancel_requested` | run | self (Agent) und `agent.events.consume` | |
| `interaction.created` | interaction | self (Agent oder App, Eigentümer der Komponente); Agent zusätzlich `agent.events.consume`, App `events:subscribe` | vor Zustellung an Agenten `agent.invoke` |
| `command.invoked` | command | self (Agent oder App, Eigentümer des Befehls); Agent zusätzlich `agent.events.consume`, App `events:subscribe` | vor Zustellung an Agenten `agent.invoke` |
| `agent.config_changed` | channel | self (Agent) und `agent.config.read` | Prompt, Skills, Projektbindung geändert |
| `host.updated` | host | `host.read` | auch beim Anlegen; Zustand, Inhaber, LLM-Erklärung, Profile, Prüfergebnisse; `key_changed` bei Neukopplung; nie Schlüssel |
| `terminal_session.changed` | terminal_session | self | an die Person der Sitzung: Zustand, Gerät, `close_reason`; nie Inhalte |
<!-- END GENERATED: events.durable -->

Kurzlebige Ereignisse:

<!-- BEGIN GENERATED: events.ephemeral -->
| Ereignistyp | Ressource | Empfangs-Aktion | Zusatzbedingung |
|---|---|---|---|
| `typing.started` | thread | `thread.read` | Thread im Fokus |
| `run.updated` | run | `run.read` | Thread im Fokus |
| `agent.presence_changed` | agent | `agent.read` | |
| `host.presence_changed` | host | `host.read` | |
| `stats.live` | instance, space | `stats.read` | nur an Verbindungen mit Abo `stats.subscribe`; Inhalt gefiltert auf die sichtbaren Kanäle, Kosten nur mit `cost.read.aggregate` ([25-visualisierung.md](25-visualisierung.md), Abschnitt 5) |
| `notification.created` | thread | self und `push.deliver`; for a hint without a thread: self and the recipient rule of R-AZ-30 | nur an Verbindungen des Empfängers, statt eines Pushes an dieses Gerät ([13-push-benachrichtigungen.md](13-push-benachrichtigungen.md), Abschnitt 6.3) |
| `command.autocomplete_requested` | command | self (Eigentümer des Befehls); Agent zusätzlich `agent.events.consume` | trägt eine Marke mit `trigger_kind = autocomplete`; nur an eine Gateway-Verbindung des Eigentümers |
<!-- END GENERATED: events.ephemeral -->

Ausgehende Webhooks und Push nutzen dieselbe Empfangs-Aktion (`webhook_out.deliver`,
`push.deliver`).

## 7. WebSocket-Ops und Aktion

<!-- BEGIN GENERATED: ws_ops -->
| Op | Richtung | Aktion |
|---|---|---|
| `identify` | Client an Server | `gateway.connect` |
| `resume` | Client an Server | `gateway.connect` |
| `heartbeat` | Client an Server | `gateway.connect` (Sitzung muss gültig sein, verbraucht nichts aus `rl.gateway_connect`); bei Agenten zusätzlich `agent.heartbeat` |
| `reauth` | Client an Server | `gateway.connect` (verbraucht nichts aus `rl.gateway_connect`) |
| `focus` | Client an Server | `gateway.focus` je Thread |
| `typing.start` | Client an Server | `typing.start` |
| `typing.stop` | Client an Server | `typing.start` |
| `ack` | Agent an Server | `agent.events.consume` |
| `run.update` | Agent an Server | `run.update` |
| `command.autocomplete_result` | Agent oder App an Server | `command.autocomplete.respond` |
| `stats.subscribe`, `stats.unsubscribe` | Client an Server | `stats.read` auf Instanz bzw. Bereich |
| `hello`, `ready`, `resumed`, `cursor`, `catchup.complete`, `resync_required`, `heartbeat_ack`, `reauth_required`, `error` | Server an Client | keine eigene; Teil der Sitzung aus `gateway.connect` |
| `dispatch` | Server an Client | Empfangs-Aktion des Ereignistyps (Abschnitt 6) |
<!-- END GENERATED: ws_ops -->

Terminal-Kanäle ([24-terminal.md](24-terminal.md); [asyncapi.yaml](../api/asyncapi.yaml)):

<!-- BEGIN GENERATED: channel_ops -->
| Kanal | Op | Richtung | Aktion |
|---|---|---|---|
| Strom des Geräts `/terminal/sessions/{session_id}/stream` | `hello` | Server an Gerät | `terminal.stream` |
| | `identify` | Gerät an Server | `terminal.stream` |
| | Binärrahmen (Noise) | beide | `terminal.stream` (Sitzung muss gültig sein) |
| | `error` | Server an Gerät | `terminal.stream` |
| Steuerung `/hosts/connect` | `hello`, `ready`, `heartbeat_ack`, `error`, `session.open`, `session.close` | Server an Rechner | `host.connect` |
| | `identify`, `heartbeat`, `report`, `session.started`, `session.ended` | Rechner an Server | `host.connect` |
| Daten `/hosts/sessions/{session_id}/stream` | `hello`, `identify`, Binärrahmen (Noise) | beide | `host.connect` (Sitzung gehört dem Rechner) |
| | `error` | Server an Rechner | `host.connect` |
<!-- END GENERATED: channel_ops -->

## 8. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-AZ-01 | `403` nur bei vorhandenem Leserecht, sonst `404`. | Baseline B7; verrät keine Existenz und bleibt für Berechtigte verständlich. |
| R-AZ-02 | DM-Kanäle sind nur für Teilnehmer lesbar, auch nicht für Owner. Der einzige Weg im Produkt ist der DSGVO-Export einer Person (`gdpr.request.create` für andere, Owner, Step-up, Audit); er enthält nur deren eigene Nachrichten. Weitere Wege (behördliche Anordnung) liegen außerhalb des Produkts. | Vertraulichkeit zwischen Menschen (Ruling L21); Voreinstellung, die der Owner bestätigt. |
| R-AZ-03 | Step-up-Aktionen nur durch Menschen in einer Sitzung. | Ein Token kann keine Bestätigung durch den Menschen erbringen; Verwaltung per Token wäre eine Umgehung. |
| R-AZ-04 | Alle Ablehnungen von Anfragen und Ops ins Audit, gleiche Ablehnungen je Fenster zusammengefasst; Filter bei Zustellung und in Listen schreiben keine. | Studie C verlangt Protokoll jeder Ablehnung; ohne Zusammenfassung füllt Abtasten das Log; ein Filter ist keine Anfrage, sonst schriebe jedes Ereignis eine Zeile je Nicht-Empfänger. |
| R-AZ-05 | `manage` darf Freigaben nur bis zur eigenen Stufe, mit eigenen Zusatzrechten (`manage` eingeschlossen), nur an Menschen und Gruppen und nicht auf der Instanz vergeben; Widerruf nur im selben Rahmen. | "delegierbar" laut Baseline, ohne Rechte-Ausweitung über die eigene Freigabe hinaus; Freigaben an Agenten und Apps sind Datenflüsse zu Auftragsverarbeitern und bleiben beim Owner. |
| R-AZ-06 | Layout-Aktionen (Bereich anlegen, sortieren, umbenennen) ohne Step-up; Rechte, Identität, Agenten, Kosten, Export, Löschung mit Step-up. | Step-up dort, wo Schaden entsteht; sonst wird er zur Gewohnheit weggeklickt. |
| R-AZ-07 | `run.cancel` nur durch Auslöser, `manage` oder Owner. | Bei Organisations-Agenten sollen sich Menschen nicht gegenseitig Läufe abbrechen. |
| R-AZ-08 | `pin.create` braucht `manage`. | Angeheftetes sieht jeder; das ist Moderation. |
| R-AZ-09 | Kanal-Verhalten (Prompt, Skills) ist nur mit `manage` lesbar und nie in Ereignissen. | Prompts können interne Hinweise enthalten; Leserecht auf den Kanal reicht nicht. |
| R-AZ-10 | Bediener, Projektbindung und Werkzeugklasse setzen nur Owner. | Alle drei wirken auf I1 oder auf die Werkzeugmacht des Agenten. |
| R-AZ-11 | Interne Zustellprüfungen (`push.deliver`, `webhook_out.deliver`) sind eigene Aktionen. | Regel 1.1: auch Zustellungen laufen durch den Katalog und sind testbar. |
| R-AZ-12 | Schreibschutz eines Kanals sperrt alle `write`- und `trigger`-Aktionen für alle Prinzipale, auch Owner; nur `import.apply` schreibt. | Ein Lese-Spiegel hat genau einen führenden Ort; im Spiegel darf kein Agent ausgelöst werden. Freigaben können Owner nicht beschränken. |
| R-AZ-13 | Bediener-Regel: Der wirksame Bediener hat implizit `write`; `space.operator.set` und `channel.operator.set` verlangen keine eigene Freigabe des Agenten. | Folgt dem Bediener ohne zweite, leicht vergessene Freigabe; beim Wechsel bleibt kein Rest. Ersetzt den Vorschlag, beim Setzen eine Freigabe zu verlangen. |
| R-AZ-14 | Owner-Rechte nur in einer Sitzung, nie über ein persönliches API-Token; the exception is the check without a request (R-AZ-29). | Ein gestohlenes Token ohne Gerätebindung darf nie die ganze Instanz öffnen. |
| R-AZ-15 | Abläufe, die Erstinstallation, die Wiederherstellung am Host, der Abgleich von Claim-Gruppen, der automatische Widerruf von Geräten und planmäßige Schlüsselwechsel sind Systemaktionen im Katalog (`grant.expire`, `group.member.expire`, `group.member.sync`, `instance.bootstrap`, `owner.recover`, `device.expire`, `signing_key.transition`). | Jeder Audit-Eintrag nennt eine bekannte Aktion; Abläufe erreichen Clients und Läufe über `access.changed`; diese Vorgänge sind sicherheitsrelevant und gehören ins Audit, ohne eine Aktion eines Menschen vorzutäuschen. |
| R-AZ-16 | Ein wartendes Gerät bekommt Tokens, darf aber nur sich selbst lesen (`403 device_pending`). | Die App kann ohne neue Anmeldung auf die Freigabe warten; Inhalte erreicht sie erst danach. |
| R-AZ-17 | `resolve_i1` führt "erst umstellen, dann freigeben" in einer Transaktion aus, mit beiden Prüfungen. | Der geführte Ablauf der Konsole braucht kein Zwischenstadium, in dem I1 verletzt wäre. |
| R-AZ-18 | App-Befehle und App-Komponenten ohne `agent.invoke` und I2. | Keine Agenten-Laufzeit, kein Abo; Grenze sind Freigaben, Scope und `required_level`. |
| R-AZ-19 | Gast-Agenten nur mit `billing_mode = 'organization'`. | Gast-Kanäle zählen nicht zu den I1-Zielen; ein persönlicher Gast wäre sonst von Fremden auslösbar. |
| R-AZ-20 | Auslösung eines anderen Agenten bei aktivem Lauf wird abgelehnt (`run_active_other_agent`), nicht zum Nachtrag. | Ein Nachtrag gehört zum Lauf seines Agenten; ein fremder Agent bekäme sonst Text, der nicht an ihn ging. |
| R-AZ-21 | `settings.read` deckt alle eigenen persönlichen Daten ab; keine eigene Lese-Aktion je Art. | Alle sind `self`, ohne Unterschied in Rechten; weniger Aktionen, gleiche Wirkung. |
| R-AZ-22 | Health hat keine Aktion; Liveness und Readiness liegen auf dem internen Listener außerhalb der öffentlichen API (Ruling L2, [16-betrieb.md](16-betrieb.md)). | Betriebs-Vertrag, kein Produkt-API; kein Auth-Bypass im öffentlichen Listener. |
| R-AZ-23 | Terminal-Aktionen nur für Menschen in einer Sitzung, mit Step-up `frisch` je Öffnung; der Rechner handelt nur über `host.pair` und `host.connect`. | Owner verlangt Passkey beim Öffnen; ein Terminal ist Fernzugriff auf einen Rechner ([24-terminal.md](24-terminal.md), R-TM-07). |
| R-AZ-24 | Rechner sind ein eigenes Ziel außerhalb des Baums; Freigaben dort nur durch Owner. | Eine Freigabe auf Instanz oder Bereich darf nie still einen Rechner öffnen; `manage` ist auf Rechnern sinnlos. |
| R-AZ-27 | Automatic state changes of outgoing subscriptions (`disabled` after `410`, after `webhook.max_consecutive_failures`, after `webhook.retry_max_age` and on a gap; `paused` by the delivery quota) are the system action `webhook_out.transition`: per change one audit entry without an actor in the transaction of the change, with the subscription, the old and new state and the cause as a code. Changes by the owner stay `webhook_out.manage`; the disabling at uninstallation is covered by the entry of `app.uninstall`. | Rule 1.10: every audit entry names a known action, and [webhooks.md](../api/webhooks.md), section 4, requires the disabling in the audit log; as with R-AZ-15, a system action does not pretend to be an action of a person. One action for all causes, because the cause is in the details (D-W-104). |
| R-AZ-29 | Check without a request (section 5): if the server evaluates the permissions of a person without the person's request (`agent.invoke` before every delivery to an agent, at `run.token.renew` and for the run from deferred follow-ups; `command.invoke` and `interaction.create` before handing over to an app; the permissions of the triggering person in the acting-for binding), this person is a principal without session, device and token: blocks come only from the person's status, the owner status from the current owner list (exception to R-AZ-14). Only the server builds this principal, only for these cases, and the permission state computed this way is never the own permission state of the principal of a request. | None of these cases has a request, session, device or token of the person; with R-AZ-14 alone no invocation of an owner would pass the repeated check (AZ-M19, every agent with `personal` or `host`), and no run of an owner would read its own context in the acting-for binding. The acceptance has already applied R-AZ-14 to the real request; afterwards the server evaluates the same person with the permissions of the moment, and a block or the loss of the owner status takes effect at once. A request with a personal token still gets no owner permissions. Remainder: an owner who triggered through grants with a personal token is evaluated as an owner afterwards; in the acting-for binding the run then reaches as far as the permissions of the agent. A permission of another person never arises from this (D-W-104). |
| R-AZ-30 | Hints without a thread (kinds `security` and `admin`, [13-push-benachrichtigungen.md](13-push-benachrichtigungen.md), section 2) are not checked with `push.deliver` on a thread. Instead the server evaluates, at the time of each decision and each send, the recipient rule of 13 section 2 with the rights of that moment: the recipient is an active human; `security` goes only to the other active devices of the human whose new device waits for approval; `admin` goes only to the owners or administrators that [07-rechte.md](07-rechte.md) and [20-datenschutz.md](20-datenschutz.md) name for the reminder. `notification.created` for such a hint goes only to connections of that recipient (self). The payload names only the kind and the target ID, as for every hint. | `push.deliver` has the resource type `thread`, and these kinds have no thread, so without a rule no check would be defined for them and the push path could not be built (M3-17, M3-19, M3-23). The recipient rule is the same check the chapter already states, evaluated at send time like `push.deliver` (P3), so a revoked device or a lost owner status stops the hint at once; it grants no read right on any content (D-W-108). |
| R-AZ-32 | New action `access.view_as.end` on the person's own console session (`auth_session`, self, without step-up, audit always, `rl.admin`): it ends "View as person" stored on the console session (03 `auth_sessions`, R-DM-49) before `authz.view_as_max_duration`; `access.view_as` from a console session starts the stored view. While a view is stored, the actions `access.view_as.end`, `auth.logout`, `auth.step_up` and `self.read` on the person's own session are decided without `view_as` ([07-rechte.md](07-rechte.md) section 4.2 steps 4 and 7 would otherwise deny them); every other decision of the session follows 07 section 9.2, and the first request after the end time gets `step_up_required`. The module `identity` owns the action, because the state lives on `auth_sessions`. | [07-rechte.md](07-rechte.md) section 9.2 requires audit at start and end and operations to start and end the view; ending must not need a fresh step-up, which `access.view_as` requires, so that an owner can always leave the view (D-W-100 extended to M5 and M6). |
| R-AZ-33 | New public action `console.csp_report` on the instance (no login, without step-up and audit, `rl.public_auth`, owned by `instance`): the endpoint of the web console's `report-to` accepts violation reports, logs only the directive, the disposition and the blocked origin and stores nothing; its effect is `read` (05 2.7), so reports are accepted in maintenance mode as well ([19-sicherheit.md](19-sicherheit.md), section 5, rule 6). | Browsers send reports without the session cookie in many cases, so the endpoint cannot require a session; every operation needs an action from the catalogue (05 section 1), and the rate class of public endpoints bounds abuse (D-W-123). |

## 9. Tests, die diesen Katalog halten

1. Routen-Test: jede registrierte Route hat eine bekannte `x-authz-action` und deren
   Rate-Limit-Klasse.
2. Ereignis-Test: jeder registrierte Ereignistyp und jede Op hat einen Eintrag in 6 bzw. 7.
3. Fall-Tabelle: Prinzipal x Ziel x Aktion x erwartete Entscheidung, inklusive abgelaufener,
   widerrufener und überlappender Freigaben, Gruppen mit Ablauf, DM, Owner, Owner mit
   persönlichem Token, Agent mit Bediener-Regel, App, wartendes Gerät, schreibgeschützter Kanal.
4. 404-Test: für jede lesende Route mit fremder ID ist die Antwort gleich der für eine nicht
   existierende ID (Status, Körper, Zeitverhalten im Rahmen).
5. I1-Test: jede Aktion aus Regel 1.6 wird mit einer Änderung geprüft, die I1 verletzen würde,
   und mit `resolve_i1`, das sie zulässig macht.
6. I2-Test: Rechte-Entzug zwischen Annahme und Zustellung führt zu Lauf `failed`, kein Aufruf.
7. Systemaktionen: keine Route und keine Op nennt eine Aktion mit Recht `system`.
8. Terminal: jede Op der Terminal-Kanäle hat einen Eintrag in Abschnitt 7; die Fälle AZ-M54 bis
   AZ-M60 aus [07-rechte.md](07-rechte.md) und TM-T07 bis TM-T10 aus
   [24-terminal.md](24-terminal.md) laufen in der Fall-Tabelle.

## 10. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `audit.deny_aggregate_window` | `60s` | Zusammenfassungsfenster für gleiche Ablehnungen |
| `audit.deny_aggregate_max_keys` | `10000` | höchstens so viele offene Zusammenfassungen je Prozess; begrenzt den Speicher bei Fluten ohne Anmeldung |
| `status.public_enabled` | `false` | Statusseite ohne Anmeldung |
| `ratelimit.rl.public_auth.per_minute` / `.burst` | `30` / `10` | je Client-IP |
| `ratelimit.rl.gateway_connect.per_minute` / `.burst` | `12` / `4` | je Gerät oder Token |
| `ratelimit.rl.read.per_minute` / `.burst` | `600` / `100` | je Prinzipal |
| `ratelimit.rl.write.per_minute` / `.burst` | `120` / `30` | je Prinzipal |
| `ratelimit.rl.message.per_minute` / `.burst` | `60` / `20` | je Prinzipal |
| `ratelimit.rl.trigger.per_minute` / `.burst` | `30` / `10` | je Prinzipal |
| `ratelimit.rl.interaction.per_minute` / `.burst` | `60` / `20` | je Prinzipal |
| `ratelimit.rl.upload.per_minute` / `.burst` | `30` / `10` | je Prinzipal |
| `ratelimit.rl.search.per_minute` / `.burst` | `60` / `20` | je Prinzipal |
| `ratelimit.rl.typing.per_minute` / `.burst` | `30` / `5` | je Verbindung |
| `ratelimit.rl.admin.per_minute` / `.burst` | `60` / `20` | je Prinzipal |
| `ratelimit.rl.export.per_minute` / `.burst` | `2` / `2` | je Prinzipal |
| `ratelimit.rl.agent_runtime.per_minute` / `.burst` | `1200` / `200` | je Agent |
| `ratelimit.rl.webhook_in.per_minute` / `.burst` | `60` / `20` | je Eingangs-Webhook |
| `ratelimit.rl.import.per_minute` / `.burst` | `120` / `30` | je Owner für `import.apply` |
| `ratelimit.rl.terminal.per_minute` / `.burst` | `20` / `5` | je Mensch |
| `ratelimit.rl.host.per_minute` / `.burst` | `120` / `30` | je Rechner |
