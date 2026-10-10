# 05 Aktionskatalog der Autorisierung

Status: Entwurf M0. Normativ. Diese Liste ist die vollständige Menge der Aktionen, die
`authorize(principal, action, resource) -> Decision` im Modul `authz` kennt (B7).
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Sync und Ereignisse: [04-sync-protokoll.md](04-sync-protokoll.md).

## 1. Verbindliche Regeln

1. **Eine Prüfstelle.** Jede Route, jede WebSocket-Op, jede Ereignis-Zustellung, jeder Push,
   jede Suche, jeder Datei-Abruf und jeder Agenten-Aufruf ruft `authorize` mit einer Aktion aus
   diesem Katalog auf. Es gibt keine Rechte-Logik außerhalb von `authz`.
2. **Kein Sichtrecht ergibt 404.** Darf ein Prinzipal eine Ressource nicht lesen, ist die
   Antwort `404 not_found`, gleich ob es sie gibt oder nicht. `403 forbidden` gibt es nur, wenn
   der Prinzipal die Ressource lesen darf, aber die nötige höhere Stufe oder ein Zusatzrecht
   fehlt. Listen, Zähler, Suche und Vorschläge filtern in der Abfrage (P2) und verraten nichts.
3. **REST.** Jede Operation im OpenAPI-Dokument trägt `x-authz-action` mit genau einer ID aus
   diesem Katalog. Das API-Team setzt das mit einer Lint-Regel durch (`authz-action-required`:
   Feld fehlt; `authz-action-known`: ID nicht im Katalog). Zusätzlich prüft der Server beim
   Start, dass jede registrierte Route eine bekannte Aktion hat, und ein Test zählt alle Routen.
4. **WebSocket und Ereignisse.** Jede Op im AsyncAPI-Dokument trägt `x-authz-action`; jeder
   dauerhafte und kurzlebige Ereignistyp trägt `x-receive-action`. Abschnitt 6 und 7 sind die
   normativen Tabellen. Ein Test schlägt fehl, wenn ein Typ oder eine Op ohne Eintrag
   registriert ist.
5. **Katalog ist geschlossen.** Eine unbekannte Aktion ist ein Programmfehler: `authorize`
   lehnt ab, die Anfrage endet mit `500`, ein Alarm wird ausgelöst.
6. **Agenten-Aufruf.** Jede Auslösung eines Agenten (Nachricht, Nachtrag, Befehl,
   Interaktion, Sitzungswechsel) prüft zusätzlich `agent.invoke` (Abschnitt 5). Die Invarianten
   I1 bis I3 der Baseline hängen an diesen Aktionen:
   - I1 wird bei `grant.create`, `grant.revoke`, `group.member.add`, `group.member.remove`,
     `group.delete`, `owner.add`, `owner.remove`, `space.operator.set`, `channel.operator.set`,
     `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`,
     `user.disable` und `user.enable` transaktional geprüft.
   - I2 ist Teil von `agent.invoke` und wird bei jeder Zustellung eines Aufrufs erneut geprüft.
   - I3 prüft der Adapter; der Server prüft beim Lauf-Start (`run.start`) die Marke erneut.
7. **Rechte-Version.** Aktionen mit Spalte "Rechte" = `ja` erhöhen `permission_version` und
   schreiben `access.changed` für alle Betroffenen ([04-sync-protokoll.md](04-sync-protokoll.md),
   Abschnitt 8).

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
| `owner` | Prinzipal ist aktiver Owner |
| `agent-self` | Prinzipal ist der Agent, dem die Ressource gehört (eigener Lauf, eigene Sitzung) |
| `participant` | Mensch ist Teilnehmer des DM-Kanals |

Mehrere Werte mit "oder" sind Alternativen. "und" heißt beides.

### 2.2 Ziel einer Ressource

Das Ziel bestimmt, auf welchem Pfad Freigaben gelten:

| Ressource | Pfad |
|---|---|
| `message`, `reaction`, `pin`, `run`, `run_step`, `component`, `interaction`, `agent_session` | Thread -> Kanal -> Bereich -> Instanz |
| `thread` | Kanal -> Bereich -> Instanz |
| `file` | Kanal der Nachricht, an der sie hängt; vor dem Anhängen `scope_channel_id` |
| `channel` | Kanal -> Bereich -> Instanz |
| `space` | Bereich -> Instanz |
| `instance` | Instanz |
| DM-Kanal und alles darin | nur `dm_participants`; Freigaben gelten nicht |

### 2.3 Wirksame Stufe

1. Owner: Stufe `trigger` und alle Zusatzrechte auf allen Zielen außer DM-Kanälen.
2. Sonst: höchste Stufe aller gültigen Freigaben (nicht widerrufen, nicht abgelaufen) des
   Prinzipals und seiner gültigen Gruppen auf dem Pfad. Zusatzrechte: Vereinigung der
   Zusatzrechte derselben Freigaben.
3. DM-Teilnehmer: Stufe `write` und Zusatzrecht `upload` im DM-Kanal.
4. Agenten und Apps: höchstens `write` (R-DM-08). Apps brauchen zusätzlich den Scope aus 2.6.

### 2.4 Spalten "Owner", "Step-up", "Audit", "Rechte"

- **Owner** `ja`: nur Owner, unabhängig von Freigaben.
- **Step-up** `ja`: Die Sitzung braucht eine frische Passkey-Bestätigung (`auth_sessions.
  step_up_at` jünger als `auth.step_up_max_age`). Sonst `401 step_up_required`. Step-up-Aktionen
  können nur Menschen ausführen; Agenten- und App-Tokens werden abgelehnt (R-AZ-03).
- **Audit** `ja`: Erlaubnis und Ablehnung kommen ins Audit-Log. `abl`: nur Ablehnungen. Jede
  Ablehnung jeder Aktion wird geprüft protokolliert; gleiche Ablehnungen desselben Prinzipals
  für dieselbe Aktion werden je `audit.deny_aggregate_window` zu einem Eintrag mit Zähler
  zusammengefasst (R-AZ-04).
- **Rechte** `ja`: siehe Regel 1.7.

### 2.5 Entscheidung

`authorize` liefert einen dieser Werte:

| Entscheidung | HTTP | Bedeutung |
|---|---|---|
| `allow` | | erlaubt |
| `not_found` | `404` | kein Leserecht auf die Ressource oder Ressource fehlt |
| `forbidden` | `403` | Leserecht vorhanden, Stufe oder Zusatzrecht fehlt |
| `step_up_required` | `401` | frische Passkey-Bestätigung fehlt |
| `invariant_violation` | `409` | Änderung würde I1 oder eine andere Invariante verletzen |
| `unauthenticated` | `401` | kein gültiger Prinzipal |

Rate-Limits prüft eine eigene Schicht vor `authorize` mit der Klasse aus dem Katalog (`429`).

### 2.6 Scopes für Apps

Eine App braucht neben Freigaben einen bewilligten Scope (`app_installations.granted_scopes`):

| Aktionen | Scope |
|---|---|
| `message.read`, `thread.read`, `channel.read`, `space.read`, `run.read` | `messages:read` |
| `message.create`, `message.update`, `message.delete`, `reaction.*`, `thread.create` | `messages:write` |
| `file.read` | `files:read` |
| `file.upload` | `files:write` |
| `command.register`, `command.remove` | `commands:write` |
| `component.update`, `interaction.respond` | `components:write` |
| `user.read` | `users:read` |
| Ereignis-Abos und `gateway.connect` | `events:subscribe` |
| `webhook_in.post` | ohne Scope; Token des Eingangs-Webhooks |

Agenten haben alle Scopes außer `users:read` implizit; ihre Grenze sind die Freigaben.

## 3. Rate-Limit-Klassen

Jede Klasse hat einen Schlüssel (wonach gezählt wird) und zwei Werte
`ratelimit.<klasse>.per_minute` und `ratelimit.<klasse>.burst` (Token-Bucket). Standardwerte in
Abschnitt 10.

| Klasse | Schlüssel | Zweck |
|---|---|---|
| `rl.public_auth` | Client-IP | Anmeldung, Refresh, öffentliche Endpunkte |
| `rl.gateway_connect` | Gerät bzw. Token | WebSocket-Verbindungsaufbau |
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

## 4. Aktionen

### 4.1 Öffentlich und Anmeldung

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `instance.discovery.read` | JWKS, Protokollversionen, minimales Branding | instance | public | nein | nein | nein | nein | `rl.public_auth` |
| `auth.login.begin` | Anmeldung starten (OIDC, Passkey-Challenge, Header-Login) | instance | public | nein | nein | abl | nein | `rl.public_auth` |
| `auth.login.complete` | Anmeldung abschließen, Gerät und Geräteschlüssel registrieren, Sitzung anlegen | instance | public | nein | nein | ja | nein | `rl.public_auth` |
| `auth.token.refresh` | Refresh-Token rotieren; Wiederverwendung sperrt die Familie | auth_session | public (gültiges Refresh-Token) | nein | nein | abl | nein | `rl.public_auth` |
| `auth.logout` | eigene Sitzung beenden | auth_session | self | nein | nein | ja | nein | `rl.write` |
| `auth.step_up` | Passkey-Bestätigung für Step-up | auth_session | self | nein | nein | ja | nein | `rl.public_auth` |
| `status.public.read` | Statusseite ohne Anmeldung, nur wenn `status.public_enabled` | instance | public | nein | nein | nein | nein | `rl.public_auth` |

### 4.2 Instanz und Verwaltung

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `instance.read` | Instanz-Einstellungen für Clients lesen (Branding, Sprache, Schalter) | instance | auth | nein | nein | nein | nein | `rl.read` |
| `instance.settings.update` | Instanz-Einstellung ändern | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `instance.branding.update` | Branding ändern (Name, Logo, Farben) | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `feature_flag.read` | alle Schalter mit Regeln lesen | instance | owner | ja | nein | nein | nein | `rl.read` |
| `feature_flag.update` | Schalter ändern | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `signing_key.rotate` | Server-Schlüssel wechseln | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `identity_provider.read` | Anmelde-Treiber lesen | instance | owner | ja | nein | nein | nein | `rl.read` |
| `identity_provider.manage` | Anmelde-Treiber anlegen, ändern, abschalten | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `owner.add` | Menschen zum Owner machen | user | owner | ja | ja | ja | ja | `rl.admin` |
| `owner.remove` | Owner-Rolle entziehen; mindestens ein Owner bleibt | user | owner | ja | ja | ja | ja | `rl.admin` |
| `audit.read` | Audit-Log lesen und filtern | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `audit.verify` | Hash-Kette prüfen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `stats.read` | Statistik lesen; Instanz nur Owner, Bereich mit `manage` | instance, space | owner oder manage | nein | nein | nein | nein | `rl.read` |
| `access.explain` | "Wer hat Zugriff?" mit Herkunft (direkt, Gruppe, geerbt) | space, channel | owner oder manage | nein | nein | nein | nein | `rl.read` |
| `access.view_as` | "Als Person ansehen", nur lesend | user | owner | ja | ja | ja | nein | `rl.admin` |

### 4.3 Menschen, Identität, Geräte, Tokens

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `user.read` | Profil und Verzeichnis von Menschen lesen | user | human; Apps mit Scope | nein | nein | nein | nein | `rl.read` |
| `user.create` | Menschen anlegen (kein Selbst-Registrieren) | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `user.update` | Profil eines anderen Menschen ändern | user | owner | ja | ja | ja | nein | `rl.admin` |
| `user.disable` | Menschen sperren: Sitzungen und Geräte widerrufen, Läufe abbrechen | user | owner | ja | ja | ja | ja | `rl.admin` |
| `user.enable` | Sperre aufheben | user | owner | ja | ja | ja | ja | `rl.admin` |
| `self.profile.update` | eigenes Profil ändern (Anzeigename, Avatar, Sprache, Zeitzone) | user | self | nein | nein | nein | nein | `rl.write` |
| `identity.link` | weitere Identität verknüpfen | user | self | nein | ja | ja | nein | `rl.admin` |
| `identity.unlink` | Identität lösen | user | self oder owner | nein | ja | ja | nein | `rl.admin` |
| `passkey.register` | Passkey hinzufügen; Step-up, wenn schon einer existiert | user | self | nein | ja | ja | nein | `rl.admin` |
| `passkey.revoke` | Passkey widerrufen | passkey | self oder owner | nein | ja | ja | nein | `rl.admin` |
| `device.list` | Geräte lesen | user | self oder owner | nein | nein | nein | nein | `rl.read` |
| `device.approve` | wartendes Gerät freigeben; eigenes Gerät von einem aktiven eigenen Gerät aus | device | owner oder self | nein | ja | ja | ja | `rl.admin` |
| `device.revoke` | Gerät sperren; Verbindungen schließen mit `4012` | device | self oder owner | nein | ja | ja | ja | `rl.admin` |
| `session.list` | Anmeldesitzungen lesen | user | self oder owner | nein | nein | nein | nein | `rl.read` |
| `session.revoke` | Sitzung beenden | auth_session | self oder owner | nein | nein | ja | ja | `rl.admin` |
| `token.list` | API-Tokens lesen (nur Präfix) | principal | owner; self bei eigenen | nein | nein | nein | nein | `rl.read` |
| `token.create` | API-Token für Agent oder App erzeugen; persönliches Token nur mit Schalter `tokens.personal_enabled` | principal | owner; self bei eigenem | nein | ja | ja | nein | `rl.admin` |
| `token.revoke` | API-Token widerrufen | api_token | owner; self bei eigenem | nein | nein | ja | ja | `rl.admin` |

### 4.4 Gruppen und Freigaben

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `group.read` | Gruppen und Mitglieder lesen; eigene Gruppen auch ohne Owner | group | owner oder self | nein | nein | nein | nein | `rl.read` |
| `group.create` | Gruppe anlegen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `group.update` | Gruppe umbenennen | group | owner | ja | ja | ja | nein | `rl.admin` |
| `group.delete` | Gruppe löschen (weich); ihre Freigaben werden widerrufen | group | owner | ja | ja | ja | ja | `rl.admin` |
| `group.member.add` | Mitglied hinzufügen, optional mit Ablauf | group | owner | ja | ja | ja | ja | `rl.admin` |
| `group.member.remove` | Mitglied entfernen | group | owner | ja | ja | ja | ja | `rl.admin` |
| `grant.read` | Freigaben eines Ziels oder Subjekts lesen | space, channel, principal | owner oder manage | nein | nein | nein | nein | `rl.read` |
| `grant.create` | Freigabe anlegen. Mit `manage` nur bis zur eigenen Stufe und nur eigene Zusatzrechte (R-AZ-05). Agenten und Apps nie `trigger`. | instance, space, channel | owner oder manage | nein | ja | ja | ja | `rl.admin` |
| `grant.revoke` | Freigabe widerrufen; mit `manage` nur auf Zielen unterhalb der eigenen `manage`-Freigabe | grant | owner oder manage | nein | ja | ja | ja | `rl.admin` |

### 4.5 Bereiche, Kanäle, Seitenleiste

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `space.read` | Bereich sehen; gilt bei `read` auf dem Bereich oder auf mindestens einem seiner Kanäle | space | read | nein | nein | nein | nein | `rl.read` |
| `space.create` | Bereich anlegen | instance | owner | ja | nein | ja | nein | `rl.admin` |
| `space.update` | Name, Icon, Farbe ändern | space | manage | nein | nein | ja | nein | `rl.admin` |
| `space.reorder` | globale Reihenfolge der Bereiche | instance | owner | ja | nein | ja | nein | `rl.admin` |
| `space.archive` | Bereich archivieren | space | owner | ja | ja | ja | ja | `rl.admin` |
| `space.operator.set` | Standard-Bediener setzen (I1) | space | owner | ja | ja | ja | ja | `rl.admin` |
| `sidebar.update` | eigenen Seitenleisten-Zustand ändern (einklappen, eigene Reihenfolge) | space | self und read | nein | nein | nein | nein | `rl.write` |
| `channel.read` | Kanal sehen, Kopfdaten lesen | channel | read | nein | nein | nein | nein | `rl.read` |
| `channel.create` | Kanal anlegen | space | manage | nein | nein | ja | nein | `rl.admin` |
| `channel.update` | Name, Thema ändern | channel | manage | nein | nein | ja | nein | `rl.admin` |
| `channel.reorder` | Reihenfolge der Kanäle im Bereich | space | manage | nein | nein | ja | nein | `rl.admin` |
| `channel.behavior.read` | Prompt, Skills, Erwähnungsregeln, Backfill lesen | channel | manage | nein | nein | nein | nein | `rl.read` |
| `channel.behavior.update` | Prompt, Skills, Erwähnungsregeln, `auto_thread`, Backfill ändern | channel | manage | nein | ja | ja | nein | `rl.admin` |
| `channel.operator.set` | Bediener setzen oder Erbe einschalten (I1) | channel | owner | ja | ja | ja | ja | `rl.admin` |
| `channel.project.bind` | Projektbindung eines Forums setzen oder ändern | channel | owner | ja | ja | ja | nein | `rl.admin` |
| `channel.archive` | Kanal archivieren | channel | manage | nein | ja | ja | ja | `rl.admin` |
| `forum_tag.manage` | Tags eines Forums anlegen, ändern, löschen | channel | manage | nein | nein | ja | nein | `rl.admin` |
| `dm.create` | Direktnachricht mit anderen Menschen anlegen; nur mit Schalter `dm.enabled` | instance | human | nein | nein | nein | nein | `rl.write` |
| `dm.leave` | DM verlassen | channel | participant | nein | nein | nein | nein | `rl.write` |

### 4.6 Threads und Agenten-Sitzungen

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `thread.read` | Thread und Verlauf lesen | thread | read | nein | nein | nein | nein | `rl.read` |
| `thread.create` | Thread aus Nachricht oder Forum-Beitrag anlegen | channel | write | nein | nein | nein | nein | `rl.message` |
| `thread.title.update` | Titel ändern; setzt `title_source = manual`. Bediener-Agent darf nur bei `auto` setzen. | thread | write und (Ersteller oder manage oder Bediener-Agent) | nein | nein | nein | nein | `rl.write` |
| `thread.close` | Thread "Erledigt" setzen | thread | write | nein | nein | nein | nein | `rl.write` |
| `thread.reopen` | Thread wieder öffnen (auch implizit durch neue Nachricht) | thread | write | nein | nein | nein | nein | `rl.write` |
| `thread.move` | in anderen Kanal verschieben; Ziel mit gleichem Bediener | thread | manage auf Quelle und Ziel | nein | nein | ja | nein | `rl.admin` |
| `thread.tags.set` | Tags setzen; Tags mit `restricted` brauchen `manage` | thread | write | nein | nein | nein | nein | `rl.write` |
| `agent_session.read` | aktuelle und frühere Sitzungs-Referenzen, Modell, Kosten | thread | read | nein | nein | nein | nein | `rl.read` |
| `agent_session.reset` | neue Sitzung im Thread beginnen (z. B. `/new`) | thread | trigger und `agent.invoke` | nein | nein | nein | nein | `rl.trigger` |
| `agent_session.bind` | vorhandene externe Sitzung an den Thread binden (Fortsetzen) | thread | trigger und `agent.invoke` | nein | nein | ja | nein | `rl.trigger` |
| `agent_session.report` | Adapter meldet externe Sitzungs-Referenz und Modell | agent_session | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |

### 4.7 Nachrichten, Reaktionen, Anheften, Vorlagen

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `message.read` | Nachricht lesen | message | read | nein | nein | nein | nein | `rl.read` |
| `message.create` | Nachricht senden; mit Anhängen zusätzlich `+upload`. Löst sie einen Agenten aus, gilt zusätzlich `agent.invoke`. | thread | write | nein | nein | nein | nein | `rl.message` |
| `message.update` | eigene Nachricht bearbeiten, mit `expected_version` | message | write und Autor | nein | nein | nein | nein | `rl.message` |
| `message.delete` | eigene Nachricht löschen; fremde nur mit `manage` (Moderation) | message | (write und Autor) oder manage | nein | nein | ja bei fremder | nein | `rl.message` |
| `message.revisions.read` | frühere Fassungen lesen | message | read | nein | nein | nein | nein | `rl.read` |
| `reaction.add` | Reaktion setzen | message | write | nein | nein | nein | nein | `rl.write` |
| `reaction.remove` | eigene Reaktion entfernen | reaction | write und self | nein | nein | nein | nein | `rl.write` |
| `pin.create` | Nachricht anheften | message | manage | nein | nein | nein | nein | `rl.write` |
| `pin.delete` | Anheften lösen | message | manage | nein | nein | nein | nein | `rl.write` |
| `mention.list` | eigene Erwähnungen lesen (gefiltert durch `message.read`) | user | self | nein | nein | nein | nein | `rl.read` |
| `template.read` | Vorlagen lesen (eigene und des Kanals) | channel | read | nein | nein | nein | nein | `rl.read` |
| `template.manage` | eigene Vorlagen pflegen; Kanal-Vorlagen mit `manage` | channel | self oder manage | nein | nein | nein | nein | `rl.write` |

### 4.8 Dateien und Suche

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `file.upload` | Datei für einen Kanal hochladen | channel | write und +upload | nein | nein | nein | nein | `rl.upload` |
| `file.read` | Datei, Vorschaubild oder Transkript abrufen; erst nach Scan | file | read | nein | nein | nein | nein | `rl.read` |
| `file.delete` | eigene, nicht angehängte Datei löschen; angehängte über `message.delete` | file | self oder manage | nein | nein | nein | nein | `rl.write` |
| `file.transcribe` | Transkription einer Sprachnachricht anstoßen (Schalter `voice.enabled`) | file | write | nein | nein | nein | nein | `rl.upload` |
| `search.query` | Volltextsuche; Treffer gefiltert durch `message.read`, `thread.read`, `file.read` | instance | auth | nein | nein | nein | nein | `rl.search` |

### 4.9 Persönliche Daten, Benachrichtigung, Sync

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `read_state.update` | Gelesen-Stand vorrücken | thread | self und read | nein | nein | nein | nein | `rl.write` |
| `read_state.mark_unread` | Gelesen-Stand zurücksetzen | thread | self und read | nein | nein | nein | nein | `rl.write` |
| `draft.read` | eigene Entwürfe lesen | user | self | nein | nein | nein | nein | `rl.read` |
| `draft.write` | Entwurf speichern oder löschen | thread, channel | self und write | nein | nein | nein | nein | `rl.write` |
| `settings.read` | eigene Einstellungen lesen | user | self | nein | nein | nein | nein | `rl.read` |
| `settings.update` | eigene Einstellung ändern | user | self | nein | nein | nein | nein | `rl.write` |
| `notification_pref.update` | Benachrichtigungsstufe für Bereich, Kanal oder Thread | space, channel, thread | self und read | nein | nein | nein | nein | `rl.write` |
| `quiet_hours.update` | Ruhezeiten pflegen | user | self | nein | nein | nein | nein | `rl.write` |
| `push.register` | Push-Ziel des eigenen aktiven Geräts registrieren | device | self | nein | nein | nein | nein | `rl.write` |
| `push.unregister` | Push-Ziel entfernen | push_registration | self | nein | nein | nein | nein | `rl.write` |
| `push.deliver` | interne Prüfung vor jedem Push (Zustellung, P3) | thread | read | nein | nein | nein | nein | keine |
| `sync.snapshot.read` | Momentaufnahme für Neuladen | instance | auth | nein | nein | nein | nein | `rl.read` |
| `events.catch_up` | `GET /v1/events`; jedes Ereignis zusätzlich mit seiner Empfangs-Aktion | instance | auth | nein | nein | nein | nein | `rl.read` |
| `gateway.connect` | WebSocket `identify` oder `resume` | instance | auth (Mensch mit aktivem Gerät, Agent, App mit `events:subscribe`) | nein | nein | abl | nein | `rl.gateway_connect` |
| `gateway.focus` | Threads fokussieren; nicht lesbare werden still ignoriert | thread | read | nein | nein | nein | nein | `rl.read` |
| `typing.start` | Tippen anzeigen | thread | write | nein | nein | nein | nein | `rl.typing` |

### 4.10 Agenten, Befehle, Läufe, Interaktionen

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `agent.read` | Agenten-Karte lesen (Name, Modus-Abzeichen, Präsenz) | agent | owner oder read auf einem Kanal, den der Agent bedient | nein | nein | nein | nein | `rl.read` |
| `agent.create` | Agent anlegen | instance | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.update` | Name, Beschreibung, `operator_config` ändern | agent | owner | ja | ja | ja | nein | `rl.admin` |
| `agent.disable` | Agent sperren; Tokens und Verbindungen enden, Läufe brechen ab | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.billing_mode.set` | Backend-Richtlinie eines Agenten umschalten, mit Grund (I4, I1) | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.billing_mode.set_all` | Backend-Richtlinie aller Agenten umschalten, mit Grund (I4, I1) | instance | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.credential_holder.set` | `credential_holder` setzen (I1) | agent | owner | ja | ja | ja | ja | `rl.admin` |
| `agent.invoke` | Agenten auslösen; Prüfung in Abschnitt 5 | thread (mit Agent) | trigger und Regeln aus 5 | nein | nein | abl | nein | `rl.trigger` |
| `agent.heartbeat` | Zustand und Ist-Modus melden | agent | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `agent.events.consume` | Ereignisse als Verbraucher empfangen und bestätigen (`ack`) | agent | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `agent.config.read` | Prompt, Skills, Projektbindung der Kanäle lesen, die der Agent bedient | channel | agent-self und wirksamer Bediener | nein | nein | nein | nein | `rl.read` |
| `command.read` | Befehlsliste und Optionen sehen (Befehle sichtbarer Agenten) | channel | read | nein | nein | nein | nein | `rl.read` |
| `command.register` | Befehl registrieren oder ändern | command | agent-self bzw. App mit Scope; Geltungsbereich muss im eigenen Bereich liegen | nein | nein | ja | nein | `rl.admin` |
| `command.remove` | Befehl entfernen | command | agent-self bzw. App mit Scope | nein | nein | ja | nein | `rl.admin` |
| `command.autocomplete` | Vorschläge für eine Option vom Agenten holen | channel | trigger und `agent.invoke` ohne Lauf | nein | nein | nein | nein | `rl.trigger` |
| `command.invoke` | Befehl ausführen; Steuerbefehl `stop` prüft zusätzlich `run.cancel` | channel, thread | `required_level` des Befehls und `agent.invoke` | nein | nein | abl | nein | `rl.trigger` |
| `run.read` | Lauf, Schritte, Kennzahlen lesen | run | read | nein | nein | nein | nein | `rl.read` |
| `run.overview.read` | "Läuft gerade" lesen, gefiltert durch `run.read` | instance | auth | nein | nein | nein | nein | `rl.read` |
| `run.start` | Lauf starten mit gültiger Aufruf-Marke (CAS `queued -> running`) | run | agent-self und Marke gültig | nein | nein | abl | nein | `rl.agent_runtime` |
| `run.update` | Live-Box-Patch des eigenen Laufs | run | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `run.finish` | Lauf beenden: `completed`, `failed`, `cancelled`, `waiting_input` | run | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |
| `run.cancel` | Lauf abbrechen | run | trigger und (Auslöser des Laufs oder manage) | nein | nein | ja | nein | `rl.trigger` |
| `run.followup.create` | Nachtrag zu einem aktiven Lauf (über `message.create`) | thread | write und `agent.invoke` | nein | nein | abl | nein | `rl.message` |
| `component.update` | eigene Komponenten deaktivieren oder ändern | component | Autor der Nachricht (agent-self bzw. App) | nein | nein | nein | nein | `rl.write` |
| `interaction.create` | Komponente bedienen (einmalig); `required_capability` und `restricted_to` beachten | component | trigger und `agent.invoke` | nein | nein | ja bei `approve_actions`, sonst abl | nein | `rl.interaction` |
| `interaction.respond` | Agent bestätigt oder beantwortet eine Interaktion | interaction | agent-self | nein | nein | nein | nein | `rl.agent_runtime` |

### 4.11 Apps und Webhooks

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `app.read` | Apps, Manifeste, Installationen lesen | app | owner | ja | nein | nein | nein | `rl.read` |
| `app.install` | App installieren, Scopes bewilligen | instance | owner | ja | ja | ja | ja | `rl.admin` |
| `app.scopes.update` | bewilligte Scopes ändern | app | owner | ja | ja | ja | ja | `rl.admin` |
| `app.uninstall` | App deinstallieren; Tokens und Abos enden | app | owner | ja | ja | ja | ja | `rl.admin` |
| `webhook_out.read` | ausgehende Abos und Zustellungen lesen | app | owner | ja | nein | nein | nein | `rl.read` |
| `webhook_out.manage` | ausgehendes Abo anlegen, ändern, löschen | app | owner | ja | ja | ja | nein | `rl.admin` |
| `webhook_out.redeliver` | Zustellung wiederholen | webhook_delivery | owner | ja | nein | ja | nein | `rl.admin` |
| `webhook_out.deliver` | interne Prüfung vor jeder Zustellung: Empfangs-Aktion des Ereignisses für die App | app | Freigaben und Scope der App | nein | nein | nein | nein | keine |
| `webhook_in.create` | Eingangs-Webhook für einen Kanal anlegen | channel | manage | nein | ja | ja | nein | `rl.admin` |
| `webhook_in.revoke` | Eingangs-Webhook widerrufen | incoming_webhook | manage | nein | ja | ja | nein | `rl.admin` |
| `webhook_in.post` | Nachricht über Eingangs-Webhook; schreibt als App mit `origin = app` | channel | public mit Token; App braucht write | nein | nein | abl | nein | `rl.webhook_in` |

### 4.12 Import, Kosten, Datenschutz, Status

| ID | Beschreibung | Ressource | Recht | Owner | Step-up | Audit | Rechte | RL-Klasse |
|---|---|---|---|---|---|---|---|---|
| `import.read` | Importquellen, Fortschritt, Fehler lesen | import_source | owner | ja | nein | nein | nein | `rl.read` |
| `import.manage` | Quelle anlegen, Trockenlauf, Start, Pause, Abbruch | import_source | owner | ja | ja | ja | nein | `rl.admin` |
| `cost.read.thread` | Kosten eines Laufs und Threads (Kopfzeile, Live-Box) | thread | read | nein | nein | nein | nein | `rl.read` |
| `cost.read.aggregate` | Kosten je Bereich, Agent, Instanz | instance, space, agent | owner; Bereich mit manage | nein | nein | nein | nein | `rl.read` |
| `price.read` | Preistabelle lesen | instance | owner | ja | nein | nein | nein | `rl.read` |
| `price.manage` | Preise mit Quelle und Abrufzeit pflegen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `cost.reconcile` | Schätzungen mit Abrechnung abgleichen | instance | owner | ja | ja | ja | nein | `rl.admin` |
| `export.channel` | Kanal-Verlauf exportieren | channel | read und +export | nein | ja | ja | nein | `rl.export` |
| `gdpr.request.create` | Export oder Löschung für sich selbst; für andere nur Owner | user | self oder owner | nein | ja | ja | nein | `rl.export` |
| `gdpr.request.read` | Auftrag und Stand lesen | gdpr_request | self oder owner | nein | nein | nein | nein | `rl.read` |
| `gdpr.request.decide` | Auftrag freigeben oder ablehnen | gdpr_request | owner | ja | ja | ja | nein | `rl.admin` |
| `gdpr.export.download` | Exportpaket herunterladen | gdpr_request | self (Betroffene Person) oder owner | nein | ja | ja | nein | `rl.export` |
| `status.read` | Statusseite lesen | instance | auth | nein | nein | nein | nein | `rl.read` |
| `status.manage` | Komponenten und Störungen pflegen | instance | owner | ja | nein | ja | nein | `rl.admin` |

## 5. Agenten-Aufruf `agent.invoke`

`authorize(P, agent.invoke, {thread T, agent A, trigger_message M | interaction I | command C})`
ist `allow` genau dann, wenn alle Punkte gelten. Er läuft beim Annehmen der Anfrage und noch
einmal unmittelbar vor jeder Zustellung an den Adapter (I2).

1. P ist ein aktiver Mensch. Agenten, Apps, Importe und Systemnachrichten lösen nie aus
   (`messages.origin = 'human'`).
2. P hat wirksam `trigger` auf dem Kanal von T. Bei DM-Kanälen gibt es keine Agenten.
3. A ist aktiv und eines von beiden:
   - A ist `threads.operator_agent_id` von T (bei neuen Threads: wirksamer Bediener des
     Kanals), und die Erwähnungsregel des Kanals ist erfüllt
     ([03-datenmodell.md](03-datenmodell.md), `channels`); oder
   - A ist in M erwähnt, `channels.allow_mentioned_agents` ist `true`, und A hat selbst
     mindestens `write` auf dem Kanal.
4. Backend-Richtlinie: Bei `agents.billing_mode = 'personal'` gilt `P = credential_holder_id`
   (I2). Sonst Ablehnung mit Grund `billing_mode_personal_not_holder`, fail closed, Audit.
5. Bei einer Interaktion: Komponente aktiv und nicht abgelaufen; P hat `required_capability`
   (z. B. `approve_actions`); bei `restricted_to = run_starter` ist P der Auslöser des Laufs oder
   Owner. Die Einmal-Regel setzt die Komponente in derselben Transaktion auf `used`.
6. Bei Forum-Kanälen mit Projektbindung: `project_bindings.ready = true`. Sonst Ablehnung mit
   Grund `project_not_ready`.
7. Der Thread hat höchstens einen aktiven Lauf. Gibt es einen, wird die Auslösung ein Nachtrag
   (`run.followup.create`) bzw. bei Steuerbefehlen ein `command.invoked`.

Folgen bei `allow`: Lauf `queued`, `trigger_state`, Ereignis an den Agenten, Aufruf-Marke bei
Zustellung ([04-sync-protokoll.md](04-sync-protokoll.md), Abschnitt 15). Die Marke (JWS, EdDSA)
enthält: `iss` (Instanz), `aud` (Agent), `sub` (P), `jti`, `iat`, `exp`, `run_id`,
`message_id` bzw. `interaction_id`, `thread_id`, `channel_id`, `level`, `billing_mode`,
`permission_version` (I3).

Folgen bei Ablehnung: Die Nachricht wird gespeichert, wenn `message.create` erlaubt war, aber
ohne Lauf; `trigger_state = failed` mit Grund, damit der Mensch es sieht. Bei Ablehnung erst
vor der Zustellung: Lauf `failed` mit Grund.

## 6. Dauerhafte Ereignistypen und Empfangs-Aktion

Ein Empfänger bekommt ein Ereignis nur, wenn `authorize(empfänger, receive_action, resource)`
erlaubt. Persönliche Ereignisse (`audience = self`) gehen nur an den genannten Prinzipal.

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
| `thread.updated` | thread | `thread.read` | Titel, Tags, Bediener |
| `thread.closed` | thread | `thread.read` | |
| `thread.reopened` | thread | `thread.read` | |
| `thread.moved` | thread | `thread.read` | Geltung: Zielkanal |
| `thread.moved_out` | channel | `channel.read` | Geltung: Quellkanal; Empfänger ohne Zugriff aufs Ziel entfernen den Thread |
| `agent_session.changed` | thread | `thread.read` | neue Sitzung, Umbinden |
| `run.state_changed` | run | `run.read` | mit Zusammenfassung |
| `space.created` | space | `space.read` | |
| `space.updated` | space | `space.read` | Name, Icon, Reihenfolge, Bediener |
| `space.archived` | space | `space.read` | |
| `channel.created` | channel | `channel.read` | |
| `channel.updated` | channel | `channel.read` | Name, Thema, Reihenfolge, Bediener |
| `channel.settings_changed` | channel | `channel.read` | nur `settings_version`; Inhalte per `channel.behavior.read` bzw. `agent.config.read` |
| `channel.archived` | channel | `channel.read` | |
| `command.changed` | command | `command.read` | registriert, geändert, entfernt |
| `file.updated` | file | `file.read` | Scan-Ergebnis, Vorschaubild, Transkript |
| `user.updated` | user | `user.read` | |
| `agent.updated` | agent | `agent.read` | inklusive Wechsel der Backend-Richtlinie (Abzeichen) |
| `instance.updated` | instance | `instance.read` | Branding, Schalter (nur Schlüssel, Werte per REST) |
| `status.updated` | instance | `status.read` | |
| `grant.changed` | grant | `grant.read` | für Verwaltende; das Subjekt bekommt `access.changed` |
| `group.changed` | group | `group.read` | |
| `access.changed` | principal | self | Abschnitt 8 in 04 |
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
| `interaction.created` | interaction | self (Agent) und `agent.events.consume` | vor Zustellung `agent.invoke` |
| `command.invoked` | command | self (Agent) und `agent.events.consume` | vor Zustellung `agent.invoke` |
| `agent.config_changed` | channel | self (Agent) und `agent.config.read` | Prompt, Skills, Projektbindung geändert |

Kurzlebige Ereignisse:

| Ereignistyp | Ressource | Empfangs-Aktion | Zusatzbedingung |
|---|---|---|---|
| `typing.started` | thread | `thread.read` | Thread im Fokus |
| `run.updated` | run | `run.read` | Thread im Fokus |
| `agent.presence_changed` | agent | `agent.read` | |

Ausgehende Webhooks und Push nutzen dieselbe Empfangs-Aktion (`webhook_out.deliver`,
`push.deliver`).

## 7. WebSocket-Ops und Aktion

| Op | Richtung | Aktion |
|---|---|---|
| `identify` | Client an Server | `gateway.connect` |
| `resume` | Client an Server | `gateway.connect` |
| `heartbeat` | Client an Server | `gateway.connect` (Sitzung muss gültig sein); bei Agenten zusätzlich `agent.heartbeat` |
| `reauth` | Client an Server | `gateway.connect` |
| `focus` | Client an Server | `gateway.focus` je Thread |
| `typing.start` | Client an Server | `typing.start` |
| `typing.stop` | Client an Server | `typing.start` |
| `ack` | Agent an Server | `agent.events.consume` |
| `run.update` | Agent an Server | `run.update` |
| `hello`, `ready`, `resumed`, `cursor`, `catchup.complete`, `resync_required`, `heartbeat_ack`, `reauth_required`, `error` | Server an Client | keine eigene; Teil der Sitzung aus `gateway.connect` |
| `dispatch` | Server an Client | Empfangs-Aktion des Ereignistyps (Abschnitt 6) |

## 8. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-AZ-01 | `403` nur bei vorhandenem Leserecht, sonst `404`. | Baseline B7; verrät keine Existenz und bleibt für Berechtigte verständlich. |
| R-AZ-02 | DM-Kanäle sind nur für Teilnehmer lesbar, auch nicht für Owner. | Vertraulichkeit zwischen Menschen; Owner-Zugriff nur über DSGVO- oder Rechtsweg. Offene Frage an den Owner. |
| R-AZ-03 | Step-up-Aktionen nur durch Menschen. | Ein Token kann keine Passkey-Bestätigung erbringen; Verwaltung per Token wäre eine Umgehung. |
| R-AZ-04 | Alle Ablehnungen ins Audit, gleiche Ablehnungen je Fenster zusammengefasst. | Studie C verlangt Protokoll jeder Ablehnung; ohne Zusammenfassung füllt Abtasten das Log. |
| R-AZ-05 | `manage` darf Freigaben nur bis zur eigenen Stufe und mit eigenen Zusatzrechten vergeben, `manage` eingeschlossen. | "delegierbar" laut Baseline, ohne Rechte-Ausweitung über die eigene Freigabe hinaus. |
| R-AZ-06 | Layout-Aktionen (Bereich anlegen, sortieren, umbenennen) ohne Step-up; Rechte, Identität, Agenten, Kosten, Export, Löschung mit Step-up. | Step-up dort, wo Schaden entsteht; sonst wird er zur Gewohnheit weggeklickt. |
| R-AZ-07 | `run.cancel` nur durch Auslöser, `manage` oder Owner. | Bei Organisations-Agenten sollen sich Menschen nicht gegenseitig Läufe abbrechen. |
| R-AZ-08 | `pin.create` braucht `manage`. | Angeheftetes sieht jeder; das ist Moderation. |
| R-AZ-09 | Kanal-Verhalten (Prompt, Skills) ist nur mit `manage` lesbar und nie in Ereignissen. | Prompts können interne Hinweise enthalten; Leserecht auf den Kanal reicht nicht. |
| R-AZ-10 | Bediener und Projektbindung setzen nur Owner. | Beides wirkt auf I1 und auf die Werkzeugmacht des Agenten. |
| R-AZ-11 | Interne Zustellprüfungen (`push.deliver`, `webhook_out.deliver`) sind eigene Aktionen. | Regel 1.1: auch Zustellungen laufen durch den Katalog und sind testbar. |

## 9. Tests, die diesen Katalog halten

1. Routen-Test: jede registrierte Route hat eine bekannte `x-authz-action`.
2. Ereignis-Test: jeder registrierte Ereignistyp und jede Op hat einen Eintrag in 6 bzw. 7.
3. Fall-Tabelle: Prinzipal x Ziel x Aktion x erwartete Entscheidung, inklusive abgelaufener,
   widerrufener und überlappender Freigaben, Gruppen mit Ablauf, DM, Owner, Agent, App.
4. 404-Test: für jede lesende Route mit fremder ID ist die Antwort gleich der für eine nicht
   existierende ID (Status, Körper, Zeitverhalten im Rahmen).
5. I1-Test: jede Aktion aus Regel 1.6 wird mit einer Änderung geprüft, die I1 verletzen würde.
6. I2-Test: Rechte-Entzug zwischen Annahme und Zustellung führt zu Lauf `failed`, kein Aufruf.

## 10. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `audit.deny_aggregate_window` | `60s` | Zusammenfassungsfenster für gleiche Ablehnungen |
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
