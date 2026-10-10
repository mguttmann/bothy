# Vorlage: Verzeichnis der Verarbeitungstätigkeiten (Art. 30 DSGVO)

Status: Vorlage M0. Für Betreiber einer Instanz. Keine Rechtsberatung.

So benutzt du die Vorlage:

1. Datei in die eigene Dokumentation kopieren (nicht im Produkt-Repository pflegen).
2. Alles in `<spitzen Klammern>` ersetzen. Vorbefüllte Angaben beschreiben, was das Produkt tut;
   prüfe sie gegen deine Konfiguration (Agenten, Apps, Telemetrie, Push).
3. Bei jeder Änderung an Agenten, Apps, Anbietern oder Fristen das Verzeichnis nachziehen.

Pflicht und Ausnahme: Art. 30 Abs. 5 DSGVO befreit Organisationen mit weniger als 250 Beschäftigten.
Die Befreiung gilt nicht, wenn die Verarbeitung ein Risiko birgt, nicht nur gelegentlich erfolgt
oder besondere Kategorien bzw. Daten über Straftaten betrifft (https://gdpr-info.eu/art-30-gdpr/,
abgerufen 2026-10-10 [belegt]). Ein Team-Chat verarbeitet laufend; die Befreiung greift daher in
der Regel nicht [abgeleitet].

Grundlage der Inhalte: [../spec/20-datenschutz.md](../spec/20-datenschutz.md) (Datenkategorien je
Tabelle, Fristen), [../spec/19-sicherheit.md](../spec/19-sicherheit.md) (technische Maßnahmen),
[processors-template.md](processors-template.md) (Empfänger und Auftragsverarbeiter).

## 1. Verantwortlicher (Art. 30 Abs. 1 lit. a)

| Feld | Angabe |
|---|---|
| Name | `<Organisation>` |
| Anschrift | `<Anschrift>` |
| Kontakt | `<E-Mail oder Telefon der verantwortlichen Stelle>` |
| Vertretung in der EU (falls nötig) | `<...>` |
| Datenschutzbeauftragte Person | `<Name, Kontakt oder "nicht bestellt">` |
| Gemeinsam Verantwortliche | `<keine oder Name>` |
| Stand des Verzeichnisses | `<Datum>` |
| Produktversion | `<Version der Instanz>` |

## 2. Tätigkeiten

Jede Tätigkeit hat die Pflichtfelder aus Art. 30 Abs. 1 lit. b bis g.

### V1 Team-Chat

| Feld | Angabe |
|---|---|
| Zweck (b) | interne Kommunikation in Bereichen, Kanälen und Threads; Dateien und Sprachnachrichten teilen; Suche |
| Rechtsgrundlage | `<vom Betreiber einzutragen>` |
| Betroffene (c) | Beschäftigte und weitere Mitglieder der Instanz; Dritte, die in Inhalten vorkommen |
| Datenkategorien (c) | Stammdaten (Handle, Anzeigename, Avatar, E-Mail zur Anzeige), Inhalte (Nachrichten, Dateien, Transkripte, Reaktionen, Erwähnungen), Kommunikationsmetadaten (Zeitpunkte, Thread, Kanal, DM-Teilnehmer), persönliche Einstellungen (Gelesen-Stand, Entwürfe, Ruhezeiten). Inhalte können besondere Kategorien nach Art. 9 enthalten: `<Einschätzung des Betreibers>` |
| Empfänger (d) | Mitglieder mit Freigabe auf den jeweiligen Kanal; Owner (außer Direktnachrichten); Hosting `<Anbieter oder eigener Betrieb>`; Objektspeicher `<...>` |
| Drittland (e) | `<keine oder Land und Garantie>` |
| Löschfristen (f) | Inhalte `retention.content` = `<Wert>`; Löschen durch Personen jederzeit; Konten nach DSGVO-Auftrag pseudonymisiert (`privacy.account_deletion_content` = `<Wert>`); Ereignis-Log `sync.event_retention` = `<Wert>`; Sicherungen `<Sicherungsfrist>` |
| Technische und organisatorische Maßnahmen (g) | siehe Abschnitt 3, Bezug M1 bis M8 |

### V2 Agenten-Verarbeitung

| Feld | Angabe |
|---|---|
| Zweck | Aufgaben durch KI-Agenten in Kanälen und Threads (Antworten, Werkzeuge, Freigabe-Karten) |
| Rechtsgrundlage | `<...>` |
| Betroffene | Menschen, die Agenten auslösen; Personen, deren Nachrichten im Kontext eines Laufs stehen; Dritte in Inhalten |
| Datenkategorien | Inhalte des Threads und bis zu `history_backfill_limit` frühere Nachrichten, Dateien des Threads, Autor als Handle und Art, Kanal-Prompt, Lauf-Kennzahlen, Kosten |
| Empfänger | je Agent eintragen: Laufzeit `<Name, Betreiber, Standort>`, LLM-Anbieter `<Name>`, Werkzeuge mit externer Wirkung `<Liste>`; Agenten mit Werkzeugklasse `host` oder `scoped` gesondert nennen |
| Drittland | `<je LLM-Anbieter; Region der Verarbeitung>` |
| Löschfristen | im Produkt wie V1; bei Laufzeit und LLM-Anbieter nach deren Fristen `<eintragen>` |
| Maßnahmen | M2, M3, M6; Agenten-Abzeichen und KI-Hinweis (20, Abschnitt 8) |

Tabelle der Agenten (je Agent eine Zeile):

| Agent | Bediente Bereiche und Kanäle | Werkzeugklasse | Backend-Richtlinie | Laufzeit | LLM-Anbieter, Region | Gedächtnis über Läufe hinweg |
|---|---|---|---|---|---|---|
| `<Name>` | `<...>` | `<chat, scoped, host>` | `<personal, organization>` | `<...>` | `<...>` | `<ja, nein, unbekannt>` |

### V3 Anmeldung, Geräte und Sicherheitsprotokoll

| Feld | Angabe |
|---|---|
| Zweck | Zugangsschutz, Gerätebindung, Nachweis von Verwaltungsaktionen und Ablehnungen |
| Rechtsgrundlage | `<...>` |
| Betroffene | Mitglieder, Owner |
| Datenkategorien | Identitäten beim Anbieter (Subjekt), Geräte (Name, Plattform, öffentliche Schlüssel), Sitzungen (Zeiten, Anmeldeweg, letzte IP), Audit-Log (Akteur-ID, Aktion, Entscheidung, IP nur als Pseudonym) |
| Empfänger | Owner; Identitätsanbieter `<Name>` |
| Drittland | `<...>` |
| Löschfristen | Sicherheitsdaten `retention.security_after_end` = `<Wert>`; Audit-Log `retention.audit` = `<Wert>`; IP-Pseudonym-Schlüssel nach `privacy.audit_ip_key_retention` = `<Wert>` vernichtet |
| Maßnahmen | M1, M4, M5 |

### V4 Push-Benachrichtigungen

| Feld | Angabe |
|---|---|
| Zweck | Hinweis auf neue Nachrichten auf Mobilgeräten und Desktop |
| Rechtsgrundlage | `<...>` |
| Betroffene | Mitglieder mit registrierten Geräten |
| Datenkategorien | Push-Token des Geräts, App-Kennung, IDs von Nachricht und Thread, neutraler Platzhaltertext, Zeitpunkt. Keine Inhalte. |
| Empfänger | `<Apple Push Notification service, Firebase Cloud Messaging, ggf. Windows Push Notification Services>` |
| Drittland | `<...>` |
| Löschfristen | Registrierung mit Gerät oder Konto; beim Anbieter nach dessen Fristen |
| Maßnahmen | M6 |

### V5 Betrieb und Telemetrie

| Feld | Angabe |
|---|---|
| Zweck | Stabilität, Fehlersuche, Kapazität, Statusseite |
| Rechtsgrundlage | `<...>` |
| Betroffene | Mitglieder (pseudonym) |
| Datenkategorien | Kennzahlen, Traces und Logs ohne Inhalte; Prinzipal-IDs nur als wechselnde Pseudonyme; keine IP-Adressen |
| Empfänger | Telemetrie-Backend `<Name oder eigener Betrieb>`; Absturzberichte der Apps `<Anbieter oder "aus">` |
| Drittland | `<...>` |
| Löschfristen | `<Frist im Backend>`; Pseudonym-Schlüssel wechseln nach `privacy.telemetry_pseudonym_rotation` = `<Wert>` |
| Maßnahmen | M7 |

### V6 Kostenerfassung für Agenten

| Feld | Angabe |
|---|---|
| Zweck | Zuordnung und Abgleich von Kosten der Agenten je Lauf, Thread, Bereich, Agent |
| Rechtsgrundlage | `<...>` |
| Betroffene | Menschen, die Läufe auslösen |
| Datenkategorien | Kostenbuchungen je Lauf, Modell, Tokenzahlen; über den Lauf einer Person zuordenbar |
| Empfänger | Owner; Bereichsverwaltung für eigene Bereiche |
| Löschfristen | `retention.cost` = `<Wert>` |
| Maßnahmen | M2; Statistik je Person standardmäßig aus (20, Abschnitt 9) |

### V7 Import aus früheren Systemen

| Feld | Angabe |
|---|---|
| Zweck | Übernahme früherer Verläufe `<z. B. aus einem Discord-Server>` |
| Rechtsgrundlage | `<...>` |
| Betroffene | Personen der Quelle, auch solche ohne Konto in der Instanz |
| Datenkategorien | importierte Nachrichten, Dateien, Namen der Quelle, Herkunftsschlüssel mit Kennungen der Quelle |
| Empfänger | wie V1 |
| Löschfristen | Import-Zustand `retention.import_after_completion` = `<Wert>`; Inhalte wie V1 |
| Maßnahmen | wie V1; Import ist nur lesend gegenüber der Quelle ([../spec/22-import-migration.md](../spec/22-import-migration.md)) |

### V8 Betroffenenanfragen

| Feld | Angabe |
|---|---|
| Zweck | Erfüllung der Rechte nach Art. 15 bis 22 DSGVO |
| Rechtsgrundlage | Art. 6 Abs. 1 lit. c DSGVO |
| Betroffene | antragstellende Personen |
| Datenkategorien | Auftrag, Stand, Exportpaket |
| Empfänger | Owner, betroffene Person |
| Löschfristen | Exportpakete `privacy.export_ttl` = `<Wert>`; Aufträge `retention.privacy_job` = `<Wert>` |
| Maßnahmen | Step-up beim Abruf, Zeitgrenze, Audit |

### V9 Sicherungen

| Feld | Angabe |
|---|---|
| Zweck | Wiederherstellung nach Ausfall |
| Rechtsgrundlage | `<...>` |
| Betroffene | alle aus V1 bis V8 |
| Datenkategorien | Abbild von Datenbank und Objektspeicher, verschlüsselt |
| Empfänger | Sicherungsziel `<Anbieter oder Ort>` |
| Löschfristen | `<Sicherungsfrist>`; gelöschte Daten kommen bei Wiederherstellung nicht zurück (Lösch-Journal, 20 Abschnitt 4.6) |
| Maßnahmen | M8 |

`<weitere Tätigkeiten, z. B. Apps und Integrationen: je installierter App eine Tätigkeit oder ein Empfänger-Eintrag>`

## 3. Technische und organisatorische Maßnahmen (Art. 30 Abs. 1 lit. g, Art. 32)

Vorbefüllt mit Produktmaßnahmen; ergänze den Betrieb (Host, Netz, Personal).

| Nr | Maßnahme | Produkt | Betrieb `<ergänzen>` |
|---|---|---|---|
| M1 | Zugangskontrolle | kein Selbst-Registrieren, Gerätebestätigung, gerätegebundene Tokens (DPoP), Step-up für Verwaltung ([../spec/08-identitaet.md](../spec/08-identitaet.md)) | `<Identitätsanbieter, MFA-Pflicht>` |
| M2 | Zugriffskontrolle | Freigaben je Bereich und Kanal, eine Prüfstelle, Schutzregeln, Direktnachrichten auch vor Owner geschützt ([../spec/07-rechte.md](../spec/07-rechte.md)) | `<Freigabe-Prozess, Überprüfung>` |
| M3 | Trennung | Werkzeugklassen, Dev-Umgebungen je Person mit VM-Grenze ([../spec/21-dev-isolation.md](../spec/21-dev-isolation.md)) | `<...>` |
| M4 | Transportverschlüsselung | TLS, HSTS, keine Klartext-Verbindungen von außen ([../spec/19-sicherheit.md](../spec/19-sicherheit.md)) | `<Zertifikate, Proxy>` |
| M5 | Protokollierung | Audit-Log hash-verkettet, IP pseudonymisiert, getrennt von Telemetrie | `<Prüfintervall der Hash-Kette>` |
| M6 | Datensparsamkeit | Push ohne Inhalt, keine Link-Vorschau, keine fremden Ressourcen in Clients, Metadaten aus Bildern entfernt | `<...>` |
| M7 | Pseudonymisierung | Telemetrie ohne Inhalte mit wechselnden Pseudonymen | `<...>` |
| M8 | Verfügbarkeit und Wiederherstellung | verschlüsselte Sicherungen, Lösch-Journal, Wiederherstellungsprobe | `<Sicherungsziel, Probe-Rhythmus>` |
| M9 | Verschlüsselung ruhender Daten | Tokens nur als Hash, Geheimnisse nur im Secret-Provider; lokale Caches verschlüsselt | `<Datenträgerverschlüsselung des Hosts>` |
| M10 | Prüfung der Wirksamkeit | Tests je Kontrolle, Red Team je Etappe, externer Test vor Veröffentlichung | `<...>` |
