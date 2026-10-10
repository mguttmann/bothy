# 20 Datenschutz

Status: Entwurf M0. Normativ für Aufbewahrung, Löschung, Export und Telemetrie. Keine Rechtsberatung:
Rechtsgrundlagen, Fristen mit rechtlicher Wirkung und Abwägungen trifft der Betreiber; das Produkt
liefert Mechanismen, Voreinstellungen und Vorlagen.
Tabellen und Aufbewahrungsklassen: [03-datenmodell.md](03-datenmodell.md), Abschnitte 3 und 4.
Rechte: [07-rechte.md](07-rechte.md). Sicherheit: [19-sicherheit.md](19-sicherheit.md). Vorlagen:
[../compliance/ropa-template.md](../compliance/ropa-template.md) (Verzeichnis der
Verarbeitungstätigkeiten), [../compliance/processors-template.md](../compliance/processors-template.md)
(Auftragsverarbeiter).

Rechtsquelle: Verordnung (EU) 2016/679 (DSGVO), https://eur-lex.europa.eu/eli/reg/2016/679/oj;
Artikeltexte gelesen über https://gdpr-info.eu, abgerufen 2026-10-10.

## 1. Rollen

| Rolle | Wer | Folge |
|---|---|---|
| Verantwortlicher | Betreiber der Instanz (Organisation des Owners) | entscheidet über Zwecke, Rechtsgrundlagen, Fristen; führt das Verzeichnis |
| Auftragsverarbeiter | Dienstleister, die der Betreiber einbindet: Hosting, Objektspeicher in der Cloud, Push-Dienste, LLM-Anbieter, Telemetrie-Backends, Transkriptions-Dienst, Identitätsanbieter | Verträge nach Art. 28 DSGVO; Liste nach der Vorlage |
| Hersteller des Produkts | Projekt des quelloffenen Produkts | verarbeitet keine Daten der Instanz; kein Auftragsverarbeiter, solange kein Hosting angeboten wird |
| Betroffene | Menschen der Instanz; Dritte, die in Inhalten vorkommen; Personen aus importierten Verläufen | Rechte nach Art. 12 bis 22 DSGVO |

## 2. Datenkategorien je Tabelle

Legende Personenbezug: **direkt** (identifiziert eine Person), **indirekt** (über eine Prinzipal-ID
oder ein Gerät einer Person zuordenbar), **Inhalt** (Freitext oder Datei; kann beliebige
personenbezogene Daten enthalten, auch von Dritten und auch besondere Kategorien nach Art. 9),
**nein**. Klasse: Aufbewahrungsklasse aus 03. Löschweg: Abschnitt 4.

| Tabelle | Personenbezug | Datenkategorien | Klasse | Löschweg |
|---|---|---|---|---|
| `instance` | nein | Kennung der Installation | `config` | keiner |
| `instance_settings` | indirekt | Einstellungen, letzte Änderung durch | `config` | bei Änderung überschrieben |
| `server_signing_keys` | nein | öffentliche Schlüssel, Verweise | `security` | nach `retired` und `retention.security_after_end` |
| `feature_flags` | indirekt | Schalter, Listen von Personen und Gruppen | `config` | bei Änderung |
| `sequence_counters` | nein | Zähler | `config` | keiner |
| `principals` | direkt | Handle, Anzeigename, Avatar, Status, Zeiten | `account` | Pseudonymisierung (4.4) |
| `users` | direkt | E-Mail (Anzeige), Sprache, Zeitzone, letzte Aktivität | `account` | Pseudonymisierung |
| `owners` | indirekt | Rolle, Grund, Zeiten | `account` | bleibt mit pseudonymer ID |
| `identity_providers` | nein | Konfiguration, Verweise auf Geheimnisse | `config` | bei Änderung |
| `user_identities` | direkt | Subjekt beim Anbieter, E-Mail bei Verknüpfung, letzte Anmeldung | `account` | löschen bei Kontolöschung |
| `passkey_credentials` | indirekt | öffentlicher Schlüssel, Name, Nutzung | `security` | löschen bei Widerruf plus Frist, bei Kontolöschung sofort |
| `devices` | direkt | Gerätename (oft mit Personennamen), Plattform, Version, letzte Sichtung | `security` | wie oben |
| `device_keys` | indirekt | öffentliche Schlüssel, Attestierung | `security` | wie oben |
| `auth_sessions` | direkt | Anmeldeweg, Zeiten, letzte IP | `security` | nach Ende plus `retention.security_after_end` |
| `refresh_token_families`, `refresh_tokens` | indirekt | Hashes, Zeiten | `security` | wie oben |
| `api_tokens` | indirekt | Name, Präfix, Hash, Nutzung | `security` | wie oben |
| `groups` | indirekt | Gruppenname (kann Organisation abbilden) | `account` | weich gelöscht, dann wie `account` |
| `group_memberships` | indirekt | Mitgliedschaft, Zeiten | `account` | bei Kontolöschung entfernt, Zeile mit pseudonymer ID |
| `grants` | indirekt | wer worauf Zugriff hat, Grund (Freitext) | `account` | bleibt mit pseudonymer ID (Nachweis) |
| `agents` | indirekt | `credential_holder_id`, gemeldeter Zustand | `account` | bleibt |
| `agent_consumer_cursors` | nein | Lesestand | `security` | mit dem Agenten |
| `apps`, `app_installations` | indirekt | Manifest, installiert durch | `account` / `config` | bleibt |
| `commands`, `channel_skill_bindings`, `project_bindings`, `forum_tags` | nein | Konfiguration | `config` / `content` | bei Änderung |
| `spaces` | indirekt | Name, angelegt durch | `content` | Archivierung; Inhalt nach `retention.content` |
| `user_space_states` | indirekt | persönliche Seitenleiste | `personal_sync` | mit dem Konto |
| `channels` | Inhalt | Name, Thema, Kanal-Prompt | `content` | Archivierung; nach `retention.content` |
| `dm_participants` | indirekt | wer mit wem schreibt | `content` | mit dem DM-Kanal |
| `threads` | Inhalt | Titel, Zeiten, angelegt durch | `content` | `retention.content` |
| `thread_tags` | indirekt | gesetzt durch | `content` | mit dem Thread |
| `agent_sessions` | indirekt | externe Sitzungs-Referenz, Kosten | `content` | mit dem Thread |
| `messages` | Inhalt | Text, Autor, Zeiten, Auslöser-Status | `content` | Löschmarke (4.2), `retention.content` |
| `message_revisions` | Inhalt | frühere Fassungen | `content` | mit der Nachricht |
| `mentions` | indirekt | wer wen erwähnt | `content` | mit der Nachricht |
| `reactions` | indirekt | wer wie reagiert | `content` | mit der Nachricht |
| `pins` | indirekt | angeheftet durch | `content` | mit der Nachricht |
| `message_templates` | Inhalt | Vorlagentext | `content` | persönliche mit dem Konto |
| `files`, `file_thumbnails` | Inhalt | Datei, Name, Transkript, Bilder | `content` | 4.3 |
| `file_upload_sessions`, `file_upload_parts` | indirekt | angemeldete Größe und Prüfsumme, Teile | `transient` | mit Abschluss oder Ablauf der Sitzung |
| `file_text_extracts` | Inhalt | aus Dateien gewonnener Text | `content` | mit der Datei |
| `message_attachments` | nein | Zuordnung | `content` | mit der Nachricht |
| `runs` | Inhalt | Auslöser, Statuszeile, letzter Textauszug, Kennzahlen | `content` | mit dem Thread |
| `run_steps` | Inhalt | Werkzeugnamen, Details (können Pfade, Befehle, Daten enthalten) | `content` | mit dem Lauf |
| `run_followups` | indirekt | Zuordnung | `content` | mit dem Lauf |
| `message_components` | Inhalt | Definition, benutzt durch | `content` | mit der Nachricht |
| `interactions` | Inhalt | Formularwerte, Prinzipal | `content` | mit der Nachricht |
| `price_table` | nein | Preise | `cost` | `retention.cost` |
| `cost_entries` | indirekt | Kosten je Lauf (über den Lauf einer Person zuordenbar) | `cost` | `retention.cost` |
| `event_log` | Inhalt | Kopien von Änderungen in Nutzlasten | `event_log` | Fenster `sync.event_retention` (04); Schwärzen bei Löschung |
| `event_consumer_cursors` | nein | Lesestand der Worker-Verbraucher | `config` | bleibt |
| `idempotency_keys` | Inhalt | gespeicherte Antworten | `transient` | `sync.idempotency_key_ttl` (04) |
| `read_states` | indirekt | Lesestand je Thread | `personal_sync` | mit dem Konto; nie für andere sichtbar (Abschnitt 9) |
| `drafts` | Inhalt | Entwürfe | `personal_sync` | mit dem Konto |
| `user_settings`, `notification_preferences` | indirekt | Einstellungen | `personal_sync` | mit dem Konto |
| `quiet_hours` | indirekt | Ruhezeiten (lassen Arbeitszeiten erkennen) | `personal_sync` | mit dem Konto |
| `push_registrations` | direkt | Push-Token des Geräts | `security` | mit Gerät oder Konto |
| `webhook_subscriptions`, `incoming_webhooks` | indirekt | Ziel, angelegt durch | `config` | bei Widerruf |
| `webhook_deliveries` | nein | Zustellversuche | `transient` | `retention.webhook_deliveries` |
| `import_sources`, `import_items` | indirekt | Kennungen der Quelle (auch fremde Personen-IDs im Herkunftsschlüssel) | `import` | `retention.import_after_completion` |
| `audit_events` | indirekt | Akteur, Aktion, Entscheidung, Ressource, pseudonymisierte IP | `audit` | `retention.audit`, Kürzen mit Anker (03) |
| `audit_checkpoints` | nein | Sequenz, Hash, Signatur | `audit` | wie `audit_events` |
| `gdpr_requests` | direkt | betroffene Person, Art, Grund, Stand | `privacy_job` | `retention.privacy_job` |
| `status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components`, `backup_runs` | nein | Betriebsstatus, Sicherungsläufe | `ops` | `retention.ops` |
| `local_credentials` (03, 08) | indirekt | Passwort-Hash, verschlüsseltes TOTP-Geheimnis | `security` | bei Kontolöschung sofort |
| `enrollment_codes` (03, 08) | indirekt | Hash, Zeiten | `security` | `retention.security_after_end` |
| `auth_login_flows` (03, 08) | indirekt | Anmeldevorgang | `transient` | `auth.login_flow_ttl` |
| `device_pairings` (03, 08) | indirekt | Kopplungsvorgang mit öffentlichen Geräteschlüsseln | `transient` | `auth.pairing_code_ttl` |
| `dev_environments` (03, 21; Etappe) | indirekt | Umgebung einer Person, Prüfergebnisse | `security` | nach `retired` plus `retention.security_after_end` |

Daten außerhalb der Datenbank:

| Ort | Inhalt | Regel |
|---|---|---|
| Objektspeicher | Dateien, Vorschaubilder, Exportpakete, Lösch-Journal | Abschnitt 4 |
| Logs | IDs, Routen-Muster, Fehler; keine Inhalte (19, Abschnitt 9) | Aufbewahrung im Betrieb ([16-betrieb.md](16-betrieb.md)) |
| Telemetrie | Kennzahlen und Traces ohne Inhalte, pseudonyme IDs (Abschnitt 7) | Aufbewahrung im Backend des Betreibers |
| Sicherungen | Abbild von Datenbank und Objektspeicher | Abschnitt 4.6 |
| Client-Cache | Kopien von Inhalten auf Geräten | 04, Abschnitt 13.1; Löschen bei Entzug und Widerruf |
| Agenten-Laufzeiten und LLM-Anbieter | Inhalte, die ein Agent bekommt | Abschnitt 6 |

## 3. Rechtsgrundlagen

Die Rechtsgrundlage legt der Betreiber je Verarbeitungstätigkeit fest. Platzhalter für das
Verzeichnis (Vorlage, Abschnitt "Tätigkeiten"):

| Verarbeitungstätigkeit | Zweck | Rechtsgrundlage (vom Betreiber einzutragen) |
|---|---|---|
| Team-Chat | interne Kommunikation | `<z. B. Art. 6 Abs. 1 lit. b oder f DSGVO; für Beschäftigte zusätzlich nationales Beschäftigtendatenschutzrecht prüfen>` |
| Agenten-Verarbeitung | Aufgaben durch KI-Agenten | `<...>` |
| Anmeldung, Geräte, Sicherheitsprotokoll | Zugangsschutz, Nachweis | `<z. B. Art. 6 Abs. 1 lit. c oder f in Verbindung mit Art. 32>` |
| Push-Benachrichtigungen | Benachrichtigung auf Geräten | `<...>` |
| Telemetrie und Betrieb | Stabilität, Fehlersuche | `<...>` |
| Kostenerfassung | Zuordnung von Kosten der Agenten | `<...>` |
| Import aus früheren Systemen | Übernahme von Verläufen | `<...>` |
| Betroffenenanfragen | Erfüllung der Rechte nach Art. 15 bis 22 | Art. 6 Abs. 1 lit. c DSGVO |

## 4. Aufbewahrung und Löschen

### 4.1 Fristen je Aufbewahrungsklasse

| Klasse | Frist (Konfigurationswert) | Mechanismus |
|---|---|---|
| `config` | bis zur Änderung | überschreiben |
| `account` | bis zur Kontolöschung, dann Pseudonymisierung | DSGVO-Auftrag (4.4) |
| `security` | `retention.security_after_end` nach Ablauf, Widerruf oder Ende | Job `privacy.retention_sweep` |
| `content` | `retention.content` (Standard: keine automatische Löschung) | Job; Löschmarken wie 4.2 |
| `personal_sync` | folgt dem Konto | mit der Kontolöschung |
| `event_log` | `sync.event_retention` (04) | Abhängen ganzer Partitionen (R-DM-15) |
| `transient` | `sync.idempotency_key_ttl` (04), `retention.webhook_deliveries` | Job |
| `audit` | `retention.audit` | Kürzen über die Wartungsrolle mit Anker-Eintrag (03, `audit_events`) |
| `cost` | `retention.cost` | Job |
| `import` | `retention.import_after_completion` nach Abschluss des Imports | Job |
| `privacy_job` | `retention.privacy_job`; Exportpakete `privacy.export_ttl` | Job |
| `ops` | `retention.ops` | Job |

`retention.content` ist bewusst ohne Standardfrist: Wie lange ein Team-Verlauf gebraucht wird, weiß
nur der Betreiber. Die Konsole zeigt einen Hinweis, bis der Owner einen Wert gesetzt oder "keine
Frist" mit Grund bestätigt hat (Audit). Grund: Speicherbegrenzung (Art. 5 Abs. 1 lit. e DSGVO) muss
der Verantwortliche festlegen; ein stiller Standard wäre eine versteckte Entscheidung.

### 4.2 Nachricht löschen

In einer Transaktion (03, Abschnitt 6.5): `content_md` leeren, `deleted_at`, Revisionen,
Erwähnungen, Komponenten, Anhänge entfernen, Suchvektor wird durch die generierte Spalte leer,
Nutzlasten der zugehörigen Ereignisse schwärzen, Ereignis `message.deleted`. Danach:

- Clients entfernen die Nachricht aus Cache und Suchindex beim Ereignis.
- Agenten-Adapter bekommen `message.deleted` und müssen die Nachricht in ihrer Laufzeit nach
  Möglichkeit entfernen (Vertrag in [09-agenten.md](09-agenten.md)). Was ein LLM-Anbieter schon
  verarbeitet hat, löscht der Anbieter nach seinen Fristen (Abschnitt 6).
- Eintrag im Lösch-Journal (4.6).

### 4.3 Dateien

Eine Datei wird mit ihrer Nachricht bzw. per `file.delete` gelöscht: Zeile `deleted_at`, Objekt und
Vorschaubilder entfernt der Aufräumjob innerhalb von `privacy.object_delete_delay`. Keine
Deduplizierung (R-DM-13): Löschen einer Datei betrifft nie die Datei einer anderen Person.

### 4.4 Konto löschen

Ein DSGVO-Auftrag `deletion` (05, `gdpr.request.*`) wird vom Owner entschieden und dann als Job
ausgeführt, in dieser Reihenfolge:

1. Konto sperren wie `user.disable` (08, Abschnitt 11), falls noch aktiv. Letzter Owner: abgelehnt.
2. Sicherheitsdaten löschen: Identitäten, Passkeys, lokale Zugangsdaten, Geräte, Schlüssel,
   Sitzungen, Tokens, Push-Registrierungen, Einladungs-Codes.
3. Persönliche Daten löschen: Seitenleiste, Gelesen-Stand, Entwürfe, Einstellungen,
   Benachrichtigungen, Ruhezeiten, persönliche Vorlagen.
4. Mitgliedschaften entfernen, Freigaben widerrufen (Zeilen bleiben mit ID als Nachweis).
5. Inhalte nach `privacy.account_deletion_content`:
   - `keep_pseudonymized` (Standard): Nachrichten, Dateien und Reaktionen bleiben, der Autor
     erscheint als "Gelöschte Person".
   - `delete`: alle eigenen Nachrichten werden wie in 4.2 gelöscht, eigene Dateien wie in 4.3.
6. Pseudonymisieren: `display_name` wird "Gelöschte Person" mit kurzer Zufallskennung, `handle`
   wird `deleted-<zufall>`, Avatar entfernt, `users.email = NULL`, Zeitzone und Sprache auf
   Standard, `principals.status = 'deleted'`, `deleted_at`.
7. Audit-Eintrag; Eintrag im Lösch-Journal.

Wichtig für den Betreiber: Pseudonymisierte Daten bleiben personenbezogen, solange eine Zuordnung
möglich ist, etwa über Inhalte (Erwägungsgrund 26 DSGVO). Die Voreinstellung
`keep_pseudonymized` hält Gesprächsverläufe für die anderen Beteiligten lesbar; ob das im Einzelfall
trägt, entscheidet der Betreiber. Die ID bleibt, damit Sequenzen, Audit-Kette und Antworten mit
Zitat konsistent bleiben (03, Grundsatz 7).

### 4.5 Audit-Log und IP-Adressen

- Das Audit-Log enthält nur IDs, Aktionen, Entscheidungen und Codes; nie Inhalte, Namen, E-Mails
  (03, `audit_events.details`). Mit der Pseudonymisierung des Prinzipals werden die IDs im Audit-Log
  pseudonym, ohne dass eine Zeile geändert wird. Die Hash-Kette bleibt dadurch gültig (ADR 0021).
- IP-Adressen stehen im Audit-Log nur als HMAC mit einem Schlüssel je Zeitraum
  (`privacy.audit_ip_key_period`). Schlüssel werden nach `privacy.audit_ip_key_retention` vernichtet.
  Danach sind die Werte nicht mehr auf eine IP zurückzuführen, die Kette bleibt unverändert.
  Innerhalb der Frist kann der Owner für eine Untersuchung prüfen, ob eine bekannte IP passt.
- `auth_sessions.last_ip` ist Klartext für die Sicherheitsansicht und fällt mit der Sitzung nach
  `retention.security_after_end`.
- Am Ende von `retention.audit` wird gekürzt; ein Anker-Eintrag hält die Prüfbarkeit (03).

### 4.6 Sicherungen und Lösch-Journal

- Gelöschte Daten bleiben in Sicherungen, bis diese nach der Sicherungsfrist ([16-betrieb.md](16-betrieb.md))
  ablaufen. Sie werden nicht in den Betrieb zurückgeholt.
- Lösch-Journal: Jede Löschung (Nachricht, Datei, Konto) schreibt einen Eintrag `{resource_kind,
  resource_id, deleted_at, reason}` ohne Inhalte. Das Journal wird laufend und getrennt vom
  Datenbank-Speicher an das Sicherungsziel geschrieben und `privacy.deletion_journal_retention`
  lang aufbewahrt; dieser Wert muss mindestens so lang sein wie die Sicherungsfrist (Startprüfung).
- Jede Wiederherstellung auf einen früheren Stand wendet das Journal an, bevor die Instanz wieder
  Anfragen annimmt (19, SEC-BAK-3). Wird mit laufender WAL-Archivierung bis zum letzten Stand
  wiederhergestellt, ist das Journal leer wirksam.

### 4.7 Daten außerhalb des Servers

| Ort | Regel |
|---|---|
| Client-Caches | Löschen bei `message.deleted`, `access.changed` mit `removed`, Widerruf (`4012`), nach `client.cache_max_age` ungenutzt (04) |
| Agenten-Laufzeiten | erhalten Lösch-Ereignisse; Speicher in der Laufzeit verantwortet der Betreiber der Laufzeit; im Verzeichnis als eigene Empfänger führen |
| LLM-Anbieter | Fristen des Anbieters; Beispiel Anthropic API: Ein- und Ausgaben "within 30 days" gelöscht, bei Richtlinienverstoß bis zu 2 Jahre (https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data, abgerufen 2026-10-10 [belegt]) |
| Push-Dienste | keine Inhalte (ADR 0031) |
| Telemetrie-Backends | keine Inhalte; Frist im Backend |

## 5. Auskunft, Export, Berichtigung

### 5.1 Ablauf

1. Die Person stellt den Auftrag selbst (`gdpr.request.create`, Step-up) oder der Owner für sie.
2. Export der eigenen Daten wird automatisch freigegeben, außer `privacy.export_requires_approval`
   ist gesetzt. Löschung braucht immer die Entscheidung des Owners (`gdpr.request.decide`), weil
   Aufbewahrungspflichten entgegenstehen können.
3. Frist: `gdpr_requests.due_at` = Eingang plus ein Kalendermonat. Verlängerung um zwei weitere
   Monate nur mit Begründung, die der Person innerhalb des ersten Monats mitgeteilt wird
   (Art. 12 Abs. 3 DSGVO, https://gdpr-info.eu/art-12-gdpr/, abgerufen 2026-10-10 [belegt]). Die
   Frist ist kein Konfigurationswert, weil sie gesetzlich ist.
4. Die Konsole warnt vor Fristablauf (Index `(state, due_at)` aus 03).
5. Das Paket liegt `privacy.export_ttl` bereit; Abruf mit Step-up (`gdpr.export.download`), dann
   gelöscht.

### 5.2 Exportformat

Ein ZIP-Archiv mit JSON-Dateien (Art. 20 Abs. 1 DSGVO verlangt ein "structured, commonly used and
machine-readable format", https://gdpr-info.eu/art-20-gdpr/, abgerufen 2026-10-10 [belegt]):

```text
export-<user_id>-<datum>.zip
  README.txt                 Erklärung der Dateien in der Sprache der Person
  manifest.json              Schema-Version, Instanz, Erstellzeit, SHA-256 jeder Datei
  profile.json               principals, users (eigene Felder)
  identities.json            verknüpfte Identitäten (ohne Geheimnisse)
  devices.json               Geräte, Sitzungen (Zeiten, Anmeldeweg, letzte IP)
  access.json                eigene Freigaben, Gruppen, Owner-Status, mit Zeiten und Grund
  messages.jsonl             eigene Nachrichten mit Revisionen, Thread- und Kanal-ID, Zeiten
  reactions.jsonl            eigene Reaktionen
  mentions.jsonl             Erwähnungen der Person (nur Nachrichten-ID und Zeit)
  files/                     eigene Uploads mit files.json (Name, Typ, Größe, Prüfsumme)
  runs.jsonl                 von der Person ausgelöste Läufe (Zustand, Zeiten, Kosten)
  interactions.jsonl         eigene Interaktionen mit Werten
  personal.json              Gelesen-Stand, Entwürfe, Einstellungen, Benachrichtigungen, Ruhezeiten, Vorlagen
  audit.jsonl                Audit-Einträge mit der Person als Akteur oder Ressource
```

Regeln:

- Nur eigene Daten und Metadaten. Nachrichten anderer Personen sind nicht enthalten (Art. 15
  Abs. 4 DSGVO: Rechte anderer). Kanal- und Thread-Namen nur, wo die Person noch lesen darf; sonst
  nur IDs.
- Keine Geheimnisse, keine Hashes, keine Schlüssel.
- Das Format ist versioniert (`manifest.json`, `schema_version`) und in [06-api.md](06-api.md)
  als Schema beschrieben.

### 5.3 Berichtigung

Profilfelder ändert die Person selbst (`self.profile.update`), eigene Nachrichten durch Bearbeiten.
Andere Daten berichtigt der Owner; ein Auftrag `rectification` dokumentiert das.

## 6. Agenten, LLM-Anbieter und Auftragsverarbeitung

- Jede Freigabe an einen Agenten und jede Bediener-Zuordnung bestimmt, welche Inhalte an die
  Laufzeit und von dort an einen LLM-Anbieter gehen. Darum legt nur der Owner Freigaben an Agenten
  an (07, Abschnitt 2.6) und die Vorschau nennt den Datenfluss.
- Die Agenten-Karte zeigt je Agent den Anbieter-Hinweis, den der Betreiber einträgt (z. B.
  "verarbeitet bei Anbieter X, Region Y"). Datenresidenz ist eine Eigenschaft der Laufzeit, nicht
  des Servers. Beispiel: Für die Anthropic API nennt die Doku als `inference_geo` nur `global` und
  `us` (https://platform.claude.com/docs/en/manage-claude/usage-cost-api, abgerufen 2026-10-10
  [belegt]); EU-Verarbeitung braucht dann einen anderen Weg, den der Betreiber wählt.
- Datensparsamkeit beim Aufruf: Der Server liefert den Thread, `history_backfill_limit` frühere
  Nachrichten (03, `channels`) und Autoren als Handle und Art, keine E-Mail-Adressen.
- Persönlicher Modus (`billing_mode = personal`): Die Person nutzt ihr eigenes Abo für eigene
  Zwecke; das Produkt speichert keine Zugangsdaten (Baseline I5).

## 7. Telemetrie

| Regel | Umsetzung |
|---|---|
| Keine Inhalte | keine Nachrichtentexte, Dateinamen, Prompts, Transkripte, Suchbegriffe, Titel (Baseline B11; 19, SEC-LOG-6) |
| Pseudonyme IDs | Prinzipal- und Geräte-IDs nur als HMAC mit Schlüssel je Zeitraum (`privacy.telemetry_pseudonym_rotation`); über Zeiträume nicht verknüpfbar |
| Keine Netzdaten | keine IP-Adressen, keine Gerätenamen, keine Push-Tokens |
| Erlaubnisliste | nur benannte Attribute je Signal ([17-observability.md](17-observability.md)) |
| LLM-Attribute | `gen_ai.*` ohne Prompt- und Antworttexte |
| Absturzberichte der Apps | ohne Inhalte und Namen; standardmäßig aus, Owner schaltet ein |
| Prüfung | Kanarien-Test (19, SEC-T10) |

## 8. Hinweispflichten

- **Information der Betroffenen** (Art. 13 DSGVO): Der Betreiber stellt sie bereit. Die Konsole hat
  ein Feld für einen Link (`privacy.notice_url`); Apps zeigen ihn bei der ersten Anmeldung und in
  den Einstellungen.
- **KI-Hinweis:** Agenten-Nachrichten tragen immer ein sichtbares Abzeichen (19, SEC-OUT-3); es ist
  nicht abschaltbar. Beim ersten Kontakt mit einem Agenten in einem Kanal zeigt die App einmal:
  "Antworten von Agenten können falsch sein. Prüfe wichtige Angaben." (`privacy.ai_notice_enabled`,
  Text über Branding anpassbar). Grundlage: Anthropic verpflichtet Kunden, Nutzer darauf hinzuweisen,
  dass Ausgaben ohne Prüfung nicht verlässlich sind (Commercial Terms, Abschnitt D.3,
  https://www.anthropic.com/legal/commercial-terms, abgerufen 2026-10-10 [belegt]).
- **Transparenz nach KI-Verordnung:** Art. 50 der Verordnung (EU) 2024/1689 verlangt, dass Menschen
  erkennen, dass sie mit einem KI-System interagieren, sofern es nicht offensichtlich ist; laut
  einer Kanzlei-Auswertung gilt das seit dem 2. August 2026
  (https://www.goodwinlaw.com/en/insights/publications/2026/08/alerts-technology-dpc-eu-ai-act-transparency-obligations-now-in-force,
  abgerufen 2026-10-10 [belegt in der Sekundärquelle]; Verordnung:
  https://eur-lex.europa.eu/eli/reg/2024/1689/oj). Das Abzeichen erfüllt den technischen Teil; ob
  Betreiber oder Hersteller Pflichten treffen, prüft der Betreiber.

## 9. Mitbestimmung (Hinweis für Betreiber)

Ein Team-Chat mit Audit-Log, Kosten je Lauf und Statistik kann eine technische Einrichtung sein,
die Verhalten oder Leistung von Beschäftigten überwachen kann. In Deutschland hat der Betriebsrat
dann nach § 87 Abs. 1 Nr. 6 BetrVG mitzubestimmen
(https://www.gesetze-im-internet.de/betrvg/__87.html, abgerufen 2026-10-10 [belegt]); andere Länder
haben eigene Regeln zur Arbeitnehmervertretung. Das Produkt unterstützt eine Vereinbarung mit diesen
Voreinstellungen:

| Funktion | Voreinstellung |
|---|---|
| Gelesen-Stand | nur für die Person selbst; keine Lesebestätigungen für andere, auch nicht für den Owner |
| Präsenz | nur Agenten haben Präsenz; Menschen nicht (04, Abschnitt 12) |
| Letzte Aktivität | nur für die Person und in der Sicherheitsansicht des Owners |
| Statistik je Person | aus (`privacy.per_person_stats_enabled`); Auswertungen nach Personengruppen erst ab `privacy.stats_min_group_size` Personen |
| Kosten je Person | nur Owner; in Threads sieht man Kosten je Lauf |
| Audit-Log | nur Owner mit Step-up; jeder Lesezugriff steht selbst im Audit-Log (05, `audit.read`) |
| "Als Person ansehen" | Step-up, Zeitgrenze, Audit, DMs ausgeschlossen (07, 9.2) |

Jede Änderung dieser Schalter steht im Audit-Log. Eine Datenschutz-Folgenabschätzung (Art. 35
DSGVO) prüft der Betreiber; Anlass können Agenten mit Zugriff auf Beschäftigtendaten sein.

## 10. Tests

| ID | Test | Erwartet |
|---|---|---|
| DP-T01 | Nachricht löschen | Inhalt, Revisionen, Anhänge, Suchtreffer, Ereignis-Nutzlasten leer; Journal-Eintrag |
| DP-T02 | Konto löschen mit `keep_pseudonymized` und mit `delete` | Schritte aus 4.4 nachweisbar; keine Klardaten in `principals`, `users`; Audit-Kette prüft weiter (`audit.verify`) |
| DP-T03 | Export | Paket entspricht dem Schema; keine fremden Nachrichten, keine Geheimnisse, keine Hashes; Prüfsummen stimmen |
| DP-T04 | Export-Ablauf | Paket nach `privacy.export_ttl` gelöscht |
| DP-T05 | Aufbewahrungsjob je Klasse mit gesteuerter Uhr | nur abgelaufene Daten entfernt |
| DP-T06 | IP-Pseudonym nach Schlüsselvernichtung | keine Zuordnung mehr möglich, Kette gültig |
| DP-T07 | Wiederherstellung mit Lösch-Journal | gelöschte Daten bleiben gelöscht (19, SEC-T15) |
| DP-T08 | `privacy.deletion_journal_retention` kürzer als Sicherungsfrist | Startabbruch |
| DP-T09 | Telemetrie-Kanarien (19, SEC-T10) | keine Inhalte, keine Klar-IDs |
| DP-T10 | Gelesen-Stand eines anderen Menschen über API oder Ereignisse abrufen | nicht möglich (`not_found`) |
| DP-T11 | Statistik je Person bei ausgeschaltetem Schalter oder zu kleiner Gruppe | keine Ausgabe |

## 11. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-DP-01 | Keine Standardfrist für Inhalte, aber Pflichtbestätigung in der Konsole. | Fristen setzt der Verantwortliche; ein stiller Standard verdeckt die Entscheidung. |
| R-DP-02 | Kontolöschung pseudonymisiert und behält standardmäßig Inhalte. | Verläufe anderer bleiben lesbar; IDs halten Sequenz und Audit konsistent. Umschaltbar. |
| R-DP-03 | IP-Adressen im Audit nur als HMAC mit vernichtbaren Schlüsseln. | Untersuchbarkeit für eine Frist, danach Unverknüpfbarkeit ohne Bruch der Hash-Kette. |
| R-DP-04 | Lösch-Journal außerhalb der Datenbank. | Wiederherstellung darf Gelöschtes nicht zurückbringen. |
| R-DP-05 | Gesetzliche Fristen (Art. 12) sind keine Konfigurationswerte. | Eine konfigurierbare gesetzliche Frist lädt zum Fehler ein. |
| R-DP-06 | Mitbestimmungsfreundliche Voreinstellungen. | Einführung im Unternehmen ohne Umbau; Überwachungsfunktionen sind bewusste Schalter. |

## 12. Abgleich mit dem Kern-Vertrag

Die beim Entwurf beantragten Ergänzungen sind entschieden (Integration vom 2026-10-10):

| Nr | Ergänzung | Entscheidung, Ort |
|---|---|---|
| K-DP-1 | Automatische Freigabe des Exports der eigenen Daten | übernommen, 03 `gdpr_requests.approved_by` (NULL = System), 05 `gdpr.request.decide` |
| K-DP-2 | IP im Audit nur als `ip_hmac` mit `ip_key_id` | übernommen, 03 `audit_events.details` |

## 13. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `retention.content` | `none` | Frist für Inhalte; `none` heißt keine automatische Löschung (Konsole verlangt Bestätigung) |
| `retention.security_after_end` | `90d` | Sicherheitsdaten nach Ende, Ablauf oder Widerruf |
| `retention.audit` | `365d` | Audit-Log (Studie C schlug zwölf Monate vor) |
| `retention.cost` | `395d` | Kosten und Preise |
| `retention.import_after_completion` | `90d` | Import-Zustand nach Abschluss |
| `retention.privacy_job` | `1095d` | DSGVO-Aufträge (ohne Exportpakete) |
| `retention.ops` | `395d` | Statusseite |
| `retention.webhook_deliveries` | `30d` | Zustellversuche ausgehender Webhooks |
| `privacy.export_ttl` | `7d` | Bereitstellung eines Exportpakets |
| `privacy.export_requires_approval` | `false` | Export der eigenen Daten braucht Freigabe des Owners |
| `privacy.account_deletion_content` | `keep_pseudonymized` | Inhalte gelöschter Konten: `keep_pseudonymized` oder `delete` |
| `privacy.object_delete_delay` | `24h` | höchste Zeit bis zum Entfernen gelöschter Objekte |
| `privacy.deletion_journal_retention` | `400d` | Aufbewahrung des Lösch-Journals; mindestens Sicherungsfrist |
| `privacy.audit_ip_key_period` | `30d` | Zeitraum je Schlüssel für IP-Pseudonyme |
| `privacy.audit_ip_key_retention` | `90d` | danach werden Schlüssel vernichtet |
| `privacy.telemetry_pseudonym_rotation` | `30d` | Wechsel des Schlüssels für pseudonyme IDs in Telemetrie |
| `privacy.per_person_stats_enabled` | `false` | Statistik je Person |
| `privacy.stats_min_group_size` | `5` | kleinste Gruppe für Auswertungen nach Personen |
| `privacy.ai_notice_enabled` | `true` | einmaliger KI-Hinweis je Kanal mit Agent |
| `privacy.notice_url` | leer | Link zur Datenschutzinformation des Betreibers |
