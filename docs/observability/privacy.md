# Datenschutz der Telemetrie

Status: Entwurf M0. Regeln zu [17-observability.md](../spec/17-observability.md) 4. Gilt für
Server, `bothy-backupd`, Adapter-SDKs und Collector. Datenschutz allgemein: Kapitel 20.

## 1. Was nie in Telemetrie steht

| Klasse | Beispiele |
|---|---|
| Inhalte | Nachrichtentext, Entwürfe, Kanal-Prompts, Skill-Namen, Dateinamen, Datei-Inhalte, Transkripte, Suchbegriffe, Formularwerte von Komponenten |
| KI-Inhalte | Prompts, Antworten, Systemanweisungen, Werkzeug-Definitionen, Argumente und Ergebnisse von Werkzeugen |
| Geheimnisse | Tokens, Passwörter, Schlüssel, Cookies, Kopfzeilen `Authorization` und `DPoP` |
| Direkte Personendaten | E-Mail-Adressen, Anzeigenamen, Handles von Menschen, IP-Adressen, Geräte-Namen |
| Freie Texte | Fehlermeldungen fremder Bibliotheken, Query-Strings, rohe Pfade |

## 2. Zwei Linien

| Linie | Wo | Wie |
|---|---|---|
| 1 | an der Quelle (Server, SDK) | Instrumentierung setzt nur Attribute aus dem Katalog ([signals.md](signals.md) 5.3); HTTP-Instrumentierung ohne `url.query`; Datenbank-Spans ohne Parameter; Log-Nachrichten sind Konstanten |
| 2 | im Collector | `transform/scrub` löscht bekannte Inhaltsfelder; `redaction/allowlist` entfernt in Spans und Logs alle Schlüssel außerhalb der Allowlist (auch Resource-Attribute) und ersetzt Werte, die wie Token, Bearer-Kopfzeile, Anthropic-Schlüssel oder E-Mail-Adresse aussehen, durch HMAC-SHA-256; `transform/scrub_metrics` löscht Personen-Attribute an Metriken fremder Quellen |

Die zweite Linie fängt Fehler der ersten. Lokal geprüft am 2026-10-10 mit otelcol-contrib 0.162.0
und der echten Datei `infra/otel/collector.yaml`: Ein Span mit Inhalt in `url.query`,
`message.content`, `gen_ai.input.messages` und `exception.message` und ein Log mit `prompt` kamen
ohne diese Werte an; eine E-Mail-Adresse und eine Bearer-Kopfzeile in erlaubten Attributen kamen
als HMAC an [belegt]. Dabei zeigte sich: Der Redaction-Prozessor filtert auch Resource-Attribute;
`service.name` und `deployment.environment.name` stehen darum ausdrücklich in der Allowlist.

Belege für den Prozessor (abgerufen 2026-10-10):
https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/redactionprocessor/README.md
(Allowlist schließt fehlerfrei, `hmac-sha256` verlangt einen Schlüssel von mindestens 32 Byte;
die Mindestlänge meldete `otelcol-contrib validate`) [belegt].

## 3. Pseudonyme

1. Menschen erscheinen in Spans und Logs nur als `bothy.principal.pseudo_id`. Ableitung: Zeitraum
   `p` = ganze Zahl aus Unix-Sekunden geteilt durch `privacy.telemetry_pseudonym_rotation` (UTC);
   Zeitraum-Schlüssel `K_p` = HMAC-SHA-256(Geheimnis aus `telemetry.pseudonym_key_ref`,
   `"bothy/telemetry-pseudonym/v1/"` + `p` dezimal); Pseudonym = HMAC-SHA-256(`K_p`, Prinzipal-ID
   als UUID in Kleinbuchstaben), hex, gekürzt auf 32 Zeichen.
2. Wer die Datenbank und das Geheimnis hat, kann ein Pseudonym einem Menschen zuordnen
   (`bothy telemetry pseudonym <principal_id> [--at <Zeitpunkt>]` für die Fehlersuche des Owners).
   Ein Telemetrie-Anbieter allein kann es nicht.
3. Pseudonyme wechseln je Zeitraum und sind über Zeiträume ohne das Geheimnis nicht verknüpfbar;
   das begrenzt Profile über lange Zeit. Es gibt nur ein Geheimnis zu hinterlegen
   ([16-betrieb.md](../spec/16-betrieb.md) 5.4). Ein Wechsel des Geheimnisses selbst trennt
   zusätzlich alle alten Pseudonyme.
4. Agenten und Apps sind keine Menschen; ihre Handles stehen im Klartext.
5. Geräte-IDs stehen nie in Spans, Logs oder Metriken, auch nicht als Pseudonym
   ([19-sicherheit.md](../spec/19-sicherheit.md) SEC-LOG-4).
6. Objekt-IDs (Thread, Kanal, Nachricht, Lauf, Datei) stehen im Klartext in Spans und Logs, nie
   an Metriken. Sie sind zufällige UUIDs ohne Inhalt; nur mit Datenbankzugang lassen sie sich
   auflösen, und dann gelten die Rechte aus `authorize`.

## 4. Keine Überwachung einzelner Menschen

1. Metriken tragen nie Attribute, die einen Menschen bezeichnen.
2. Dashboards und Alarme zeigen keine Werte je Person.
3. Die In-App-Statistik zeigt Zähler zu Menschen nur ab `privacy.stats_min_group_size`
   ([admin-statistics.md](admin-statistics.md)).
4. Grund: Telemetrie dient dem Betrieb. Eine Auswertung von Verhalten oder Leistung einzelner
   Beschäftigter kann in Deutschland mitbestimmungspflichtig sein (§ 87 Abs. 1 Nr. 6
   Betriebsverfassungsgesetz) [abgeleitet]. Ob ein Betrieb Telemetrie einführen darf, klärt der
   Betreiber; das Produkt macht die zweckfremde Auswertung schwer.

## 5. Aufbewahrung

| Ort | Frist | Schlüssel |
|---|---|---|
| Prometheus (lokal) | `BOTHY_PROMETHEUS_RETENTION` | Flag `--storage.tsdb.retention.time` |
| Loki (lokal) | `BOTHY_LOKI_RETENTION` | `limits_config.retention_period`, Compactor |
| Tempo (lokal) | `BOTHY_TEMPO_RETENTION` | `block_retention` |
| stdout-Logs (Docker) | Rotation nach `BOTHY_LOG_MAX_SIZE` und `BOTHY_LOG_MAX_FILE` | Log-Treiber |
| externe Backends | laut Vertrag des Anbieters | Einstellung beim Anbieter |

Die Fristen der Telemetrie sind kürzer als die des Audit-Logs und unabhängig davon
([audit-vs-telemetry.md](audit-vs-telemetry.md)). Ein Löschauftrag nach DSGVO (Kapitel 20)
erreicht Telemetrie nicht gezielt; darum enthält sie keine direkten Personendaten, und die Fristen
sind kurz.

## 6. Kanarien-Test (Gate)

1. Testdaten enthalten in jedem Inhaltsfeld ein zufälliges Geheimwort je Lauf (Nachricht,
   Dateiname, Suchbegriff, Prompt, Formularwert, Werkzeug-Argument eines Test-Adapters,
   Anzeigename, E-Mail).
2. Ein Szenario läuft durch API, Gateway, Push (Testanbieter), Adapter und Collector.
3. Der Collector exportiert mit dem Overlay `local` und zusätzlich in eine Datei (Test-Overlay
   `infra/compose/test/otel/file.yaml` mit `file`-Exporter, nur für Tests, nicht im
   Release-Bündel; geprüft mit `otelcol-contrib validate` in `tools/opscheck`); stdout der
   Container wird mitgeschnitten.
4. Das Geheimwort darf in keiner Ausgabe stehen. Ein Treffer bricht das Gate und nennt die Stelle.
5. Zusätzlich: Jede Prinzipal-ID der Testdaten darf im Klartext nicht vorkommen.

## 7. Apps

Apps senden keine Telemetrie an den Collector. Absturzberichte gehen nur an ein Ziel, das der
Herausgeber in `distribution.yaml` setzt ([18-release.md](../spec/18-release.md) 8). Vor dem
Versand entfernt die App Nachrichtentexte, Eingaben, Dateinamen und Query-Strings (Kapitel 15,
Team E). Ohne Ziel sendet die App nichts.
