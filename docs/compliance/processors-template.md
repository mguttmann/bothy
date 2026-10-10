# Vorlage: Auftragsverarbeiter und Empfänger

Status: Vorlage M0. Für Betreiber einer Instanz. Keine Rechtsberatung.

Zweck: Liste aller Dienste, an die eine Instanz personenbezogene Daten gibt, mit dem, was das Produkt
dorthin sendet. Grundlage für Verträge nach Art. 28 DSGVO, für das Verzeichnis
([ropa-template.md](ropa-template.md), Feld "Empfänger") und für Übermittlungen in Drittländer.

So benutzt du die Vorlage:

1. Datei in die eigene Dokumentation kopieren.
2. Je tatsächlich genutztem Dienst eine Zeile in Abschnitt 2 ausfüllen; nicht genutzte Zeilen
   löschen. Die Spalte "Was das Produkt sendet" beschreibt das Produkt und gilt für jede Instanz.
3. Rolle (Auftragsverarbeiter, eigener Verantwortlicher, kein Empfänger) bewertet der Betreiber.
4. Bei jedem neuen Agenten, jeder neuen App und jedem neuen Anbieter nachziehen.

## 1. Was das Produkt an Dienste gibt

| Dienstart | Was das Produkt sendet | Was es nie sendet | Quelle |
|---|---|---|---|
| Push (APNs, FCM, WNS) | Push-Token, App-Kennung, IDs von Thread und Nachricht, neutraler Platzhaltertext, Zeitpunkt | Nachrichteninhalte, Namen, Dateien | ADR 0031, [../spec/13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md) |
| Identitätsanbieter (OIDC) | Anmeldeanfrage mit `state`, `nonce`, PKCE; der Anbieter liefert Subjekt, Name, optional E-Mail und Gruppen | Inhalte | [../spec/08-identitaet.md](../spec/08-identitaet.md) |
| LLM-Anbieter (über die Agenten-Laufzeit, nicht über den Server) | Thread-Inhalte, frühere Nachrichten bis `history_backfill_limit`, Dateien, Autoren als Handle, Kanal-Prompt | Zugangsdaten anderer Personen; E-Mail-Adressen | [../spec/20-datenschutz.md](../spec/20-datenschutz.md), Abschnitt 6 |
| Telemetrie-Backend | Kennzahlen, Traces, Logs ohne Inhalte, pseudonyme IDs | Inhalte, IP-Adressen, Namen | [../spec/20-datenschutz.md](../spec/20-datenschutz.md), Abschnitt 7 |
| Absturzberichte der Apps | bereinigte Stacktraces, App-Version, Plattform | Inhalte, Tokens, Namen | [../spec/19-sicherheit.md](../spec/19-sicherheit.md), SEC-LOG-7 |
| Objektspeicher in der Cloud | Dateien, Vorschaubilder, Exportpakete, Lösch-Journal | | [../spec/12-dateien.md](../spec/12-dateien.md) |
| Hosting (Cloud-VM, verwaltete Datenbank) | alles, was die Instanz speichert | | [../spec/16-betrieb.md](../spec/16-betrieb.md) |
| Transkriptions-Dienst | Audio von Sprachnachrichten, Sprache | Namen, andere Inhalte | [../spec/12-dateien.md](../spec/12-dateien.md) |
| Apps und Integrationen | Ereignisse ihrer Abos im Rahmen von Scopes und Freigaben | was ihre Freigaben nicht erlauben | [../spec/07-rechte.md](../spec/07-rechte.md), Abschnitt 2.10 |
| Sicherungsziel | verschlüsselte Abbilder | Schlüssel der Sicherung | [../spec/19-sicherheit.md](../spec/19-sicherheit.md), SEC-BAK-* |

Kein Empfänger im Sinne dieser Liste: der Virenscanner, solange er lokal läuft (er lädt nur
Signaturen herunter); der Hersteller des Produkts.

## 2. Liste

| Dienst | Anbieter | Rolle | Zweck | Daten | Betroffene | Ort der Verarbeitung | Drittland-Garantie | Vertrag (AVV) | Unterauftragsverarbeiter | Löschfrist beim Anbieter | Geprüft am |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Push iOS, macOS | `<Apple>` | `<...>` | Benachrichtigung | siehe 1 | Mitglieder mit Apple-Geräten | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Push Android | `<Google Firebase>` | `<...>` | Benachrichtigung | siehe 1 | Mitglieder mit Android-Geräten | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Push Windows (falls genutzt) | `<Microsoft>` | `<...>` | Benachrichtigung | siehe 1 | Mitglieder mit Windows | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Identitätsanbieter | `<z. B. Microsoft Entra ID>` | `<...>` | Anmeldung | siehe 1 | alle Mitglieder | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| LLM-Anbieter je Agent | `<z. B. Anthropic>` | `<...>` | Agenten-Verarbeitung | siehe 1 | siehe Verzeichnis V2 | `<Region>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Agenten-Laufzeit, falls nicht selbst betrieben | `<...>` | `<...>` | Ausführung der Agenten | siehe 1 | siehe V2 | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Telemetrie | `<z. B. Grafana-Stack selbst betrieben, New Relic, Datadog, Azure Monitor>` | `<...>` | Betrieb | siehe 1 | Mitglieder (pseudonym) | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Absturzberichte | `<z. B. Sentry oder "aus">` | `<...>` | Fehlersuche Apps | siehe 1 | Mitglieder | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Hosting | `<Anbieter oder eigener Betrieb>` | `<...>` | Betrieb der Instanz | alle | alle | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Objektspeicher | `<Anbieter oder lokal>` | `<...>` | Dateien | Inhalte | alle | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Transkription | `<Anbieter oder lokal>` | `<...>` | Sprachnachrichten | Audio | Sprechende | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| Sicherungsziel | `<Anbieter oder Ort>` | `<...>` | Sicherung | verschlüsselt alle | alle | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |
| App `<Name>` | `<Anbieter der App>` | `<...>` | `<...>` | Abos: `<Ereignistypen>`, Scopes: `<...>` | `<...>` | `<...>` | `<...>` | `<Link>` | `<Link>` | `<...>` | `<Datum>` |

## 3. Beispiel mit öffentlichen Angaben: Anthropic API

Nur zur Orientierung; vor Verwendung erneut prüfen. Alle Angaben abgerufen 2026-10-10.

| Feld | Angabe | Quelle |
|---|---|---|
| Vertrag | Das Data Processing Addendum ist per Verweis Teil der Commercial Terms; Kunde ist Verantwortlicher, Anthropic Auftragsverarbeiter | https://www.anthropic.com/legal/data-processing-addendum [belegt] |
| Unterauftragsverarbeiter | Liste mit Widerspruchsfrist laut DPA | https://www.anthropic.com/subprocessors [belegt] |
| Training | "Anthropic may not train models on Customer Content from Services." | https://www.anthropic.com/legal/commercial-terms [belegt] |
| Löschfrist | API-Ein- und Ausgaben "within 30 days"; bei Verstoß gegen Richtlinien bis zu 2 Jahre | https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data [belegt] |
| Region | `inference_geo` mit Werten `global` und `us`; keine EU-Option genannt | https://platform.claude.com/docs/en/manage-claude/usage-cost-api [belegt] |
| Hinweis an Nutzer | Kunde weist Nutzer darauf hin, dass Ausgaben ohne Prüfung nicht verlässlich sind | Commercial Terms, D.3 [belegt] |
| Anmeldung | Persönliche Abos (OAuth) nur für die eigene, gewöhnliche Nutzung; Produkte Dritter dürfen keine Claude.ai-Zugangsdaten sammeln, speichern oder vermitteln | https://code.claude.com/docs/en/legal-and-compliance [belegt] |
