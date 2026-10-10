# 19 Sicherheit

Status: Entwurf M0. Normativ für Server, Clients, Admin-Konsole und Betrieb.
Die Analyse (Systemgrenzen, Assets, Angreifer, STRIDE, Bedrohungsregister mit Restrisiko) steht in
[../security/threat-model.md](../security/threat-model.md). Dieses Kapitel legt die Kontrollen fest,
auf die das Bedrohungsmodell verweist. Jede Kontrolle hat eine ID (`SEC-...`) und mindestens einen
Test (Abschnitt 14).
Rechte: [07-rechte.md](07-rechte.md). Identität und Tokens: [08-identitaet.md](08-identitaet.md).
Datenschutz: [20-datenschutz.md](20-datenschutz.md). Dev-Abschottung:
[21-dev-isolation.md](21-dev-isolation.md). Qualitäts-Gates und Lieferkette:
[23-qualitaet.md](23-qualitaet.md). Releases: [18-release.md](18-release.md).

## 1. Vertrauensannahmen

| Annahme | Folge, wenn sie bricht |
|---|---|
| Der Server-Host und sein Betriebssystem sind vertrauenswürdig. | Alles ist offen; das Produkt kann das nicht abfangen. Härtung des Hosts ist Betreiberpflicht ([16-betrieb.md](16-betrieb.md)). |
| Der Owner ist vertrauenswürdig, aber nicht allwissend. | Owner-Fehler sind möglich; darum Vorschau, Step-up, Audit, Invarianten. DMs bleiben auch vor dem Owner geschützt (07, SR6). |
| Wer schreibenden Zugriff auf die Datenbank hat, kann alles ändern. | Hash-Kette und exportierte Prüfpunkte machen Änderungen am Audit-Log erkennbar, nicht unmöglich (ADR 0021). |
| Clients, Agenten-Laufzeiten, Apps und Inhalte sind nicht vertrauenswürdig. | Der Server prüft jede Eingabe und jede Aktion selbst. |
| LLM-Ausgaben sind nicht vertrauenswürdig. | Prompt-Injection ist nicht gelöst; die Grenze sind Rechte, Werkzeugklassen und Freigabe-Karten, nicht das Modell. |

## 2. Transport

| ID | Kontrolle |
|---|---|
| SEC-TLS-1 | Klartext-HTTP nimmt der öffentliche Listener (`server.listen_addr`) nur auf Loopback oder von Gegenstellen in `server.trusted_proxies` ([16-betrieb.md](16-betrieb.md)) an (Reverse-Proxy im selben Host oder Container-Netz). Jede andere Klartext-Verbindung wird beim Verbindungsaufbau geschlossen, ohne zu lesen oder zu schreiben. Der interne Listener ist ausgenommen (R-SE-09). |
| SEC-TLS-2 | Wo der Server selbst TLS beendet (`server.tls_cert_ref` und `server.tls_key_ref`, beide gesetzt oder beide leer; mit TLS muss `server.public_url` `https` sein): mindestens TLS 1.2, TLS 1.3 bevorzugt, Cipher-Suites aus der Go-Standardbibliothek ohne eigene Liste. Grund: Eine eigene Liste veraltet; die Standardauswahl wird mit Go gepflegt [abgeleitet]. Kein Umleitungs-Listener für Klartext. |
| SEC-TLS-3 | HSTS mit `http.hsts_max_age` auf jeder Antwort, sobald `server.public_url` `https` nutzt (auch hinter einem TLS-Proxy): `Strict-Transport-Security: max-age=<Sekunden>`, ohne `includeSubDomains` (andere Subdomains gehören dem Betreiber) und ohne `preload` (nicht rücknehmbar). Mit `http`-URL (Entwicklung auf Loopback) fehlt die Kopfzeile. |
| SEC-TLS-4 | Ausgehende TLS-Verbindungen prüfen immer Zertifikat und Hostnamen. `InsecureSkipVerify` ist per Lint verboten. |
| SEC-TLS-5 | PostgreSQL über Unix-Socket, mit `sslmode=verify-full` oder im internen Compose-Netz `internal` ohne Ausgang und ohne Host-Port mit `sslmode=disable` ([16-betrieb.md](16-betrieb.md), Abschnitt 4.1, Regel 2). Virenscanner nur über Unix-Socket (clamd hat keine Anmeldung im Protokoll; ClamAV empfiehlt den lokalen Modus; https://raw.githubusercontent.com/Cisco-Talos/clamav/main/etc/clamd.conf.sample, abgerufen 2026-10-10 [belegt]). OTel Collector über Loopback oder TLS. |
| SEC-TLS-6 | Hostnamen mit öffentlichen Zertifikaten erscheinen in Certificate-Transparency-Logs (https://tailscale.com/kb/1153/enabling-https, abgerufen 2026-10-10 [belegt]). Die Betriebsdoku empfiehlt neutrale Hostnamen. |

## 3. HTTP-Sicherheitskopfzeilen

| ID | Antworten | Kopfzeilen |
|---|---|---|
| SEC-HDR-1 | alle | `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Strict-Transport-Security` (SEC-TLS-3), keine Versionsangabe in `Server` |
| SEC-HDR-2 | API (JSON) | `Cache-Control: no-store`, `Cross-Origin-Resource-Policy: same-origin`, `Content-Type: application/json` bzw. `application/problem+json` |
| SEC-HDR-3 | Konsole (HTML und Assets) | CSP nach Abschnitt 5, `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Embedder-Policy: require-corp`, `X-Frame-Options: DENY`, `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()` |
| SEC-HDR-4 | Datei-Abruf | `Content-Disposition: attachment` mit `filename*` (RFC 6266), außer für neu kodierte Rastervorschauen; `Content-Security-Policy: default-src 'none'; sandbox`; `Cache-Control: private, no-store` |

COOP und COEP machen die Konsole "cross-origin isolated". Flutter braucht das für mehrfädiges
Rendern mit WebAssembly; ohne die Kopfzeilen läuft es einfädig
(https://docs.flutter.dev/platform-integration/web/initialization, Option
`forceSingleThreadedSkwasm`, abgerufen 2026-10-10 [belegt]). Sie gehen nur, weil alle Assets vom
eigenen Origin kommen (Abschnitt 5).

## 4. CORS und WebSocket-Herkunft

| ID | Kontrolle |
|---|---|
| SEC-CORS-1 | Die API sendet keine CORS-Freigaben. Native Apps brauchen kein CORS; die Konsole ist same-origin. |
| SEC-CORS-2 | Einzige Ausnahme: öffentliche, nicht personenbezogene Antworten ohne Anmeldung (`instance.discovery.read`, JWKS) mit `Access-Control-Allow-Origin: *` und ohne Credentials. |
| SEC-CORS-3 | WebSocket-Upgrade: ist eine `Origin`-Kopfzeile vorhanden, muss sie gleich dem Origin der Instanz sein; sonst `403` vor dem Upgrade. Native Clients senden keine fremde `Origin`. Grund: Schutz der Konsolen-Sitzung (Cookie) vor Cross-Site-WebSocket-Hijacking. |
| SEC-CORS-4 | Ändernde Anfragen der Konsole tragen eine eigene Pflicht-Kopfzeile und ein passendes `Origin` (08, Abschnitt 7.5). |

## 5. Content Security Policy der Web-Konsole

Die Konsole ist eine Flutter-Web-Anwendung ([15-client-architektur.md](15-client-architektur.md),
ADR 0028). Ziel: kein Skript, kein Stil, keine Schrift und kein WebAssembly von fremden Origins.

```text
Content-Security-Policy:
  default-src 'none';
  script-src 'self' 'wasm-unsafe-eval';
  style-src 'self' 'nonce-<zufall je Antwort>';
  img-src 'self' blob: data:;
  font-src 'self';
  connect-src 'self';
  worker-src 'self' blob:;
  manifest-src 'self';
  base-uri 'none';
  form-action 'self';
  frame-ancestors 'none';
  object-src 'none';
  require-trusted-types-for 'script';
  trusted-types <Richtliniennamen des Flutter-Loaders und der Engine>;
  report-to csp
```

Regeln:

1. CanvasKit bzw. skwasm und Ersatzschriften werden mit der Konsole ausgeliefert. Der Loader bekommt
   `canvasKitBaseUrl` und `fontFallbackBaseUrl` auf den eigenen Origin; der Standard für
   Ersatzschriften wäre sonst `https://fonts.gstatic.com/s/`
   (https://docs.flutter.dev/platform-integration/web/initialization, abgerufen 2026-10-10 [belegt]).
2. Der Loader bekommt die Option `nonce`; laut Flutter-Doku gilt sie für Skripte und Stile, die die
   Engine selbst einfügt (gleiche Quelle [belegt]). Der Server erzeugt den Nonce je Antwort.
3. `'wasm-unsafe-eval'` ist nötig, weil der Renderer WebAssembly kompiliert [abgeleitet]. Kein
   `'unsafe-eval'`, kein `'unsafe-inline'` für Skripte.
4. Die Namen der Trusted-Types-Richtlinien des Flutter-Loaders und der Engine sind nicht aus
   Primärquellen belegt [vermutet: `flutter-js`, `flutter-engine`]. Der Test SEC-T05 lädt die
   Konsole in einem Headless-Browser mit erzwungener CSP und schlägt bei jeder Verletzung fehl; die
   Liste im Repo folgt diesem Test, nicht dieser Doku.
5. `connect-src 'self'` deckt in aktuellen Browsern den WebSocket zum selben Host ab [vermutet];
   SEC-T05 prüft auch das. Scheitert es, kommt `wss://<host>` hinzu.
6. Verstöße gehen an einen Endpunkt der Instanz (`report-to`), mit Rate-Limit und ohne Inhalte im
   Log.

## 6. Eingaben und Ausgaben

### 6.1 Eingabevalidierung im Server

| ID | Kontrolle |
|---|---|
| SEC-IN-1 | Jede Anfrage wird gegen das OpenAPI- bzw. AsyncAPI-Schema geprüft, bevor ein Handler läuft ([06-api.md](06-api.md), ADR 0013). Schreibende Schemas haben `additionalProperties: false`. |
| SEC-IN-2 | JSON-Dekodierung lehnt doppelte Schlüssel, ungültiges UTF-8 und zu tiefe Schachtelung ab. Grund: `client_payload_sha256` (03) und Verarbeitung dürfen keine verschiedenen Lesarten derselben Anfrage sehen. |
| SEC-IN-3 | Größengrenzen vor dem Lesen: `http.max_json_body_bytes` für JSON, `http.max_header_bytes` (beide 02), `sync.ws_max_frame_bytes` (04), `file.max_upload_bytes` (03) als Stream-Grenze. Zeitgrenzen: `http.read_header_timeout`, `http.idle_timeout` (02). |
| SEC-IN-4 | Anzeige-Texte (Anzeigenamen, Kanal- und Bereichsnamen, Titel, Dateinamen) werden auf Steuerzeichen und Bidi-Steuerzeichen geprüft und abgelehnt. Nachrichteninhalte bleiben unverändert; Clients isolieren fremde Namen beim Darstellen (Unicode-Isolation). |
| SEC-IN-5 | Datenbankzugriff nur über sqlc mit Parametern. Ein Lint verbietet zusammengesetzte SQL-Texte außerhalb des generierten Codes. |
| SEC-IN-6 | Reguläre Ausdrücke nur mit der Go-Bibliothek `regexp` (lineare Laufzeit). |
| SEC-IN-7 | Patches an Lauf-Dokumenten nur auf erlaubten JSON-Pointer-Pfaden (04, Abschnitt 11.2). |
| SEC-IN-8 | Komponenten-Definitionen (`component.v1`) und Befehls-Optionen werden mit Längen- und Mengengrenzen aus dem Schema geprüft. |

### 6.2 Darstellung in den Clients

| ID | Kontrolle |
|---|---|
| SEC-OUT-1 | Markdown ohne rohes HTML. Links nur mit `https`, `http`, `mailto` oder internen Verweisen; das Ziel wird beim Antippen bzw. Überfahren mit Domain angezeigt. |
| SEC-OUT-2 | Clients laden nie automatisch fremde Ressourcen aus Inhalten: keine Bilder über URL, keine Link-Vorschau, keine Favicons. Bilder nur aus Dateien des Servers. Grund: Ein Agent oder Angreifer könnte sonst Daten über Bild-URLs abfließen lassen und IP-Adressen der Lesenden erfahren (Bedrohung T-AI-06). |
| SEC-OUT-3 | Nachrichten von Agenten, Apps, Importen und dem System sind sichtbar unterschieden (Abzeichen nach `origin`). Systemnachrichten (`kind = 'system'`) erzeugt nur der Server. |
| SEC-OUT-4 | Freigabe-Karten zeigen einen Kopf, den der Client aus Server-Feldern baut (Agent, Lauf, nötiges Zusatzrecht), getrennt vom Text des Agenten. Ein Agent kann den Kopf nicht beeinflussen. |
| SEC-OUT-5 | Fehlerantworten nach RFC 9457 ohne Stacktrace und ohne interne Details; mit `correlation_id`. |

## 7. Upload-Härtung

| ID | Kontrolle |
|---|---|
| SEC-UP-1 | Größe vor dem Speichern begrenzt (`file.max_upload_bytes`, 03); Abbruch mit `413`, sobald die Grenze im Stream erreicht ist. |
| SEC-UP-2 | Der MIME-Typ wird vom Server erkannt, nie vom Client übernommen (03, `files.mime_type`). |
| SEC-UP-3 | Inline-Darstellung nur für Rasterbilder, die der Server neu kodiert hat (Vorschaubilder). SVG, HTML, PDF und alles andere nur als Anhang (SEC-HDR-4). |
| SEC-UP-4 | Bildgröße wird vor dem Dekodieren aus dem Dateikopf gelesen; über `file.image.max_pixels` ([12-dateien.md](12-dateien.md)) keine Vorschau. Schutz gegen Dekompressionsbomben. |
| SEC-UP-5 | Parser für nicht vertrauenswürdige Formate laufen in einem eigenen Prozess mit Speicher- und Zeitgrenze, nie im API-Prozess. M1 hat nur Bildvorschau in reinem Go (Baseline R-Objektspeicher). |
| SEC-UP-6 | Virenscan als getrennter Dienst (ADR 0033). Auslieferung erst bei `scan_status IN ('clean','skipped')` (03). Ist der Scanner nicht bereit, bleiben Dateien `pending` (fail closed). `infected`: nie auslieferbar, Hinweis an den Owner. |
| SEC-UP-7 | Archive werden im Server nie entpackt. |
| SEC-UP-8 | Objektschlüssel sind zufällig und enthalten keine Namen (03). Originalnamen nur bereinigt in `Content-Disposition`. |
| SEC-UP-9 | Eine Datei hängt nur an Nachrichten ihres Kanals `scope_channel_id` (03). Damit gelangt eine Datei nicht über Bereichsgrenzen in den Kontext eines anderen Agenten. |
| SEC-UP-10 | Ortsdaten und andere persönliche Metadaten werden aus gespeicherten Bildern immer entfernt: bei JPEG und PNG verlustfrei beim Upload (Segment- und Chunk-Filter), Vorschaubilder neu kodiert (Ruling L16, ADR 0047, [12-dateien.md](12-dateien.md) Abschnitt 7). |
| SEC-UP-11 | Clients bekommen keine dauerhaften Objektspeicher-URLs; jeder Abruf läuft über `file.read` (07, Abschnitt 8.1). |

Speicherorte, Vorschau-Varianten, Kontingente und der Scan-Dienst im Detail: [12-dateien.md](12-dateien.md).

## 8. SSRF-Verbot und ausgehende Verbindungen

Baseline Abschnitt 4: kein serverseitiges Laden fremder URLs.

| ID | Kontrolle |
|---|---|
| SEC-NET-1 | Alle ausgehenden HTTP-Verbindungen laufen über einen Egress-Client im Server (`platform/egress`). Ein Lint verbietet `http.DefaultClient`, `http.Get`, `http.Post`, eigene Transporte und Dialer außerhalb dieses Pakets. Für `bothy healthcheck` gibt es dort einen Loopback-Client (nur Loopback, nur `http`, keine Weiterleitung). Ausgenommen ist nur der OTLP-Export an den Collector: Sein Ziel kommt aus der Betreiber-Konfiguration (`OTEL_EXPORTER_OTLP_*`), nie aus Daten, und die Exporter des SDK wählen ihren Dialer selbst. |
| SEC-NET-2 | Erlaubte Ziele sind nur: konfigurierte Identitätsanbieter (Aussteller und daraus entdeckte Endpunkte desselben Anbieters), Push-Dienste, `webhook.allowed_hosts` (03), Objektspeicher, Transkriptions-Dienst und Einträge in `egress.allowed_hosts`. Alles andere: Ablehnung vor dem Verbindungsaufbau, ohne DNS-Abfrage. Umgebungs-Proxys (`HTTP_PROXY`, `HTTPS_PROXY`) gelten nicht, weil die Prüfung sonst nur den Proxy sähe. `http` nur für Einträge mit `allow_private`. |
| SEC-NET-3 | Prüfung der IP nach der DNS-Auflösung beim Verbindungsaufbau (nicht vorher), damit DNS-Rebinding nicht greift; verbunden wird die geprüfte Adresse, nie der Name erneut. Immer gesperrt, auch mit `allow_private`: unspezifizierte Adressen (`0.0.0.0/8`, `::`), Link-Local inklusive der Metadaten-Adresse der Cloud-Anbieter (`169.254.0.0/16`, `fe80::/10`; RFC 3927), Multicast (`224.0.0.0/4`, `ff00::/8`), Broadcast und reservierte Adressen (`240.0.0.0/4`), IETF-Protokollzuweisungen (`192.0.0.0/24`), Benchmarking (`198.18.0.0/15`), veraltetes Site-Local (`fec0::/10`), Discard-Only (`100::/64`), IPv4-kompatible Adressen (`::/96`), NAT64 (`64:ff9b::/96`, `64:ff9b:1::/48`), 6to4 (`2002::/16`) und Teredo (`2001::/32`); IPv4-gemappte IPv6-Adressen werden vor der Prüfung entpackt. Gesperrt außer bei einem passenden Eintrag in `egress.allowed_hosts` mit `allow_private`: Loopback, private Netze (RFC 1918), Shared Address Space (RFC 6598, auch von Overlay-Netzen benutzt), Unique Local Addresses (RFC 4193). Dokumentationsbereiche (RFC 5737, RFC 3849) sind nicht gesperrt; sie werden im Internet nicht geroutet. |
| SEC-NET-4 | Keine automatische Verfolgung von Weiterleitungen bei Webhooks; bei OIDC nur zu erlaubten Zielen. |
| SEC-NET-5 | Zeit- und Größengrenzen je Antwort (`egress.timeout`, `egress.max_response_bytes`); eine zu große Antwort ist ein Fehler, nie still gekürzt. Abgewiesene Versuche zählt `bothy.egress.blocked` ([signals.md](../observability/signals.md)) und loggt sie mit Zweck, Grund und Host, nie mit Pfad oder Inhalt. |
| SEC-NET-6 | Clients laden keine Link-Vorschauen (SEC-OUT-2). Agenten-Werkzeuge mit Netzzugriff liegen in der Laufzeit, außerhalb des Servers; ihre Grenze ist die Werkzeugklasse (07, Abschnitt 2.11). |

## 9. Logs ohne Geheimnisse und ohne Inhalte

| ID | Kontrolle |
|---|---|
| SEC-LOG-1 | Strukturierte Logs (`log/slog`). Geheimnisse haben im Code einen eigenen Typ, der beim Ausgeben immer `[redacted]` schreibt. |
| SEC-LOG-2 | Kopfzeilen werden über eine Erlaubnisliste geloggt, nicht über eine Sperrliste. `Authorization`, `DPoP`, `Cookie`, `Set-Cookie`, Aufruf-Marken, `X-Forwarded-*` und Identitäts-Kopfzeilen eines Proxys erscheinen nie. Grund: Bei einer Sperrliste rutscht jede neue Kopfzeile durch. |
| SEC-LOG-3 | Pfade werden als Routen-Muster geloggt (`/api/v1/hooks/{webhook_id}/{token}`), nie roh; Query-Strings nie. |
| SEC-LOG-4 | Nie im Log: Nachrichteninhalte, Dateinamen, Kanal-Prompts, Transkripte, Suchbegriffe, Anzeigenamen, E-Mail-Adressen, Push-Tokens. Prinzipale erscheinen nur als ID. |
| SEC-LOG-5 | IP-Adressen nur im Audit-Log und nur pseudonymisiert ([20-datenschutz.md](20-datenschutz.md), Abschnitt 4.5), nicht in allgemeinen Logs. |
| SEC-LOG-6 | Telemetrie-Attribute über eine Erlaubnisliste je Signal ([17-observability.md](17-observability.md)); keine Inhalte (Baseline B11). |
| SEC-LOG-7 | Absturzberichte von Apps werden vor dem Senden bereinigt; Inhalte, Tokens und Namen sind nicht enthalten. |

## 10. Geheimnisse

Alle Geheimnisse kommen aus der Secret-Provider-Schnittstelle (ADR 0011). In der Datenbank stehen
nur Hashes, verschlüsselte Werte mit externem Schlüssel oder Verweise (`secret_ref`, 03).

| Geheimnis | Ort | Wechsel |
|---|---|---|
| Private Signierschlüssel (Zugriffstoken, Aufruf-Marke, ausgehende Webhooks, Audit-Prüfpunkte) | vom Server erzeugt im Schlüsselspeicher `secrets.signing_key_dir` (ADR 0066): Ed25519, `kid` = Fingerabdruck nach RFC 7638, Datei `<zweck>-<kid>.pem`; öffentlicher Teil in `server_signing_keys` | Zustände `next`, `active`, `retiring`, `retired`. Planmäßig nach `signing_key.invocation_rotation_interval` (Aufruf-Marke, 09) bzw. `signing_key.access_token_rotation_interval` (Zugriffstoken), Job `instance.key_rotation` alle `signing_key.rotation_check_interval`, Systemaktion `signing_key.transition`; `webhook_out` und `audit_checkpoint` nur auf Anforderung (`signing_key.rotate` mit `mode = scheduled`). Ein Schlüssel bleibt `retiring`, bis die längste damit signierte Gültigkeit plus `auth.clock_leeway` abgelaufen ist; dann `retired`, und die private Datei wird gelöscht. Notfall: `signing_key.rotate` mit `mode = emergency` setzt alle Schlüssel des Zwecks sofort auf `retired`, weil ein Leck des Speichers auch `next` und `retiring` offenlegt. Prozesse übernehmen Änderungen spätestens nach `signing_key.cache_ttl` |
| Pepper für Token-Hashes | Secret-Provider | nur nach Kompromittierung; dann werden alle Tokens neu ausgegeben, weil Hashes unbekannter Tokens nicht umgerechnet werden können |
| Schlüssel für TOTP-Geheimnisse | Secret-Provider, `totp_key_ref` je Zeile | neuer Schlüssel für neue Einträge, alte bleiben lesbar bis zum Umschlüsseln |
| Datenbank-Zugang | Secret-Provider oder Umgebung | Betrieb |
| OIDC-Client-Zugang | `secret_ref` in `identity_providers.config` | `private_key_jwt` bevorzugt |
| Push-Zugänge (APNs-Schlüssel, FCM-Dienstkonto, WNS-Client) | `secret_ref`; Konten des Owners | nach Vorgaben der Anbieter |
| Objektspeicher-Zugang | Secret-Provider | Betrieb |
| Schlüssel für Pseudonyme im Audit-Log und in Telemetrie | Secret-Provider | nach [20-datenschutz.md](20-datenschutz.md) |
| Schlüssel der Sicherungen | außerhalb der Instanz, beim Owner | Betrieb |
| LLM-API-Schlüssel, Abo-Zugänge | nie im Server; beim Adapter bzw. in der Laufzeit der Person (Baseline I5) | Betreiber |

Regeln:

| ID | Kontrolle |
|---|---|
| SEC-SEC-1 | Der Datei-Treiber des Secret-Providers liest nur Dateien mit Modus `0400` oder `0600`, die dem Dienstbenutzer gehören; sonst Startabbruch. |
| SEC-SEC-2 | `config print` und Diagnose-Endpunkte zeigen Geheimnisse nie, auch nicht gekürzt. |
| SEC-SEC-3 | Geheimnisse stehen nie in Ereignissen, Audit-Details, Telemetrie, Fehlermeldungen oder Absturzberichten. |
| SEC-SEC-4 | Im Repository: gitleaks lokal und im Gate, eigene Regel für das Token-Präfix aus 08, Abschnitt 12 ([23-qualitaet.md](23-qualitaet.md)). |
| SEC-SEC-5 | Dateien des Secret-Providers liegen außerhalb der Sicherung der Datenbank oder nur verschlüsselt darin (Abschnitt 12). |

## 11. Kontrollen für Agenten

| ID | Kontrolle | Ort |
|---|---|---|
| SEC-AI-1 | Jeder Inhalt, den der Server einem Agenten liefert, trägt seine Herkunft: Autor, `origin` (`human`, `agent`, `app`, `import`, `system`), Art (Nachricht, Datei, Transkript, Zitat) und Kanal. Der Kanal-Prompt ist ein eigenes Feld. Adapter müssen fremde Inhalte als Daten kennzeichnen, nicht als Anweisung. | [09-agenten.md](09-agenten.md) |
| SEC-AI-2 | Werkzeugklasse mit Soll und Ist; `host` nur für Owner oder eigene Dev-Umgebung. | 07, SR2 |
| SEC-AI-3 | Auftragsbindung: Agent im Lauf hat höchstens die Rechte des Auslösers. | 07, SR9 |
| SEC-AI-4 | Komponenten mit `required_capability = 'approve_actions'` haben standardmäßig `restricted_to = 'run_starter'`. | [09-agenten.md](09-agenten.md) |
| SEC-AI-5 | Agenten lösen nie aus; Agenten-Nachrichten haben `origin = 'agent'`. | 07, SR4 |
| SEC-AI-6 | Keine automatisch geladenen fremden Ressourcen in Clients. | SEC-OUT-2 |
| SEC-AI-7 | Rate-Limits je Agent (`rl.agent_runtime`) und Auslösungen je Mensch (`rl.trigger`); Kostengrenzen je Agent. | 05; [10-backend-richtlinie.md](10-backend-richtlinie.md) |
| SEC-AI-8 | Kanal-Prompts sind kein Ort für Geheimnisse. Die Konsole sagt das beim Bearbeiten. Grund: Ein Agent kann seinen Prompt in der Antwort wiedergeben. | R-AZ-09 |

## 12. Lieferkette, Sicherungen und Audit-Integrität

Lieferkette (Details und Werkzeuge in [23-qualitaet.md](23-qualitaet.md) und
[18-release.md](18-release.md); Baseline R-Qualität):

| ID | Kontrolle |
|---|---|
| SEC-SC-1 | Abhängigkeiten mit Lockfiles und Prüfsummen (Go `go.sum`, Dart `pubspec.lock`, Python-Lock mit Hashes, npm-Lock); Updates über Renovate mit Wartefrist (ADR 0041); OSV-Scanner im Gate. |
| SEC-SC-2 | GitHub Actions per voller Commit-SHA gepinnt, `permissions: contents: read` als Standard, kein `pull_request_target`, zizmor und actionlint als Pflicht (Baseline R-Qualität). |
| SEC-SC-3 | Self-hosted Runner nie für Code aus fremden Pull Requests. GitHub: "self-hosted runners should almost never be used for public repositories" (https://docs.github.com/en/actions/reference/security/secure-use, abgerufen 2026-10-10 [belegt]). ADR 0039. |
| SEC-SC-4 | Container-Basisimages per Digest gepinnt; Releases signiert, mit SBOM und Provenance (ADR 0036). |
| SEC-SC-5 | Werkzeuge mit bekannter Kompromittierung nur in einer als sicher belegten Version (Beispiel Trivy, Baseline R-Qualität). |

Sicherungen (Anforderungen an [16-betrieb.md](16-betrieb.md)):

| ID | Kontrolle |
|---|---|
| SEC-BAK-1 | Sicherungen sind immer verschlüsselt; der Schlüssel liegt nicht auf demselben Host wie die Sicherung. |
| SEC-BAK-2 | Sicherungen enthalten keine Dateien des Secret-Providers im Klartext (SEC-SEC-5). |
| SEC-BAK-3 | Jede Wiederherstellung wendet das Lösch-Journal an ([20-datenschutz.md](20-datenschutz.md), Abschnitt 4.6) und erhöht die Rechte-Version, damit alle Caches neu aufbauen. |
| SEC-BAK-4 | Nach einer Wiederherstellung erkennen Clients den Rücksprung (`resync_required` mit `cursor_ahead`, 04, Abschnitt 7.3). |
| SEC-BAK-5 | Wiederherstellung wird regelmäßig geprobt; die Probe ist ein Test im Betriebshandbuch, nicht nur ein Text. |

Audit-Integrität (ADR 0021):

| ID | Kontrolle |
|---|---|
| SEC-AUD-1 | Die Anwendungsrolle hat auf `audit_events` nur `INSERT` und `SELECT`; `UPDATE` und `DELETE` sind gesperrt (03). |
| SEC-AUD-2 | Alle `audit.checkpoint_interval` entsteht ein signierter Prüfpunkt (Sequenz, Hash, Zeit, voriger Prüfpunkt), der an `audit.checkpoint_sink` außerhalb der Datenbank abgelegt wird. Ist kein Ziel gesetzt, zeigt die Konsole eine dauerhafte Warnung. |
| SEC-AUD-3 | `audit.verify` vergleicht die nachgerechnete Kette mit den Prüfpunkten vom Ziel, nicht aus der Datenbank. |
| SEC-AUD-4 | Kanonische Form einer Zeile (Eingang von `hash`, 03 `audit_events`): ein JSON-Objekt mit den Spaltennamen außer `hash`; `seq` als Ganzzahl; IDs und `correlation_id` als UUID in Kleinbuchstaben mit Bindestrichen; `occurred_at` als RFC 3339 in UTC mit genau drei Nachkommastellen und `Z` (der Recorder kürzt die Zeit vor dem Speichern auf Millisekunden); `prev_hash` als Hex in Kleinbuchstaben; `details` wie gespeichert; NULL-Spalten fehlen. Serialisiert nach RFC 8785 (JCS). Diese Form ist genau die API-Darstellung `AuditEvent` ohne `hash`; ein Export (`audit.read`) liefert sie unverändert, damit er ohne Server prüfbar ist. |
| SEC-AUD-5 | Schema `audit_details.v1`: ein flaches Objekt. Reservierte Felder: `ip_hmac` (64 Hex-Zeichen), `ip_key_id` (UUID aus `audit_ip_keys`), `aggregate` (`count` ab 1, `first_at`, `last_at`), `violations` (bei `i1_violation`: Liste von `{code, agent_id, credential_holder_id, principal_ids}`), `reason` (der Grund, den eine Verwaltungsaktion mit Pflichtgrund verlangt, wie ihn die Anfrage nennt; ohne Steuer- und Bidi-Zeichen nach SEC-IN-4, Länge durch das Schema der Anfrage begrenzt). Alle anderen Felder: Schlüssel `^[a-z][a-z0-9_]*$`, Werte nur UUIDs, Codes `^[a-z0-9][a-z0-9_.:-]*$`, Ganzzahlen, Wahrheitswerte, Zeiten in der Form von SEC-AUD-4 oder Listen davon. Kein Freitext, kein `null`, keine Gleitkommazahlen, nie Inhalte, Namen, E-Mail-Adressen, Tokens oder Codes von Einladungen. Ein Wert außerhalb dieser Formen ist ein Programmfehler, den der Recorder ablehnt. Braucht eine Aktion weiteren Freitext, erweitert ihr Ticket das Schema hier und im Code. |
| SEC-AUD-6 | Gleiche Ablehnungen fasst jeder Prozess je `audit.deny_aggregate_window` zusammen (05, Abschnitt 2.4); höchstens `audit.deny_aggregate_max_keys` offene Zusammenfassungen, darüber wird die älteste sofort geschrieben. Restrisiko: Ein harter Absturz verliert die offenen Zusammenfassungen eines Fensters ([threat-model.md](../security/threat-model.md)). |

## 13. Penetrationstest-Plan

### 13.1 Stufen

| Stufe | Wann | Wer | Umfang |
|---|---|---|---|
| P0 automatisch | jedes lokale Gate und CI | Werkzeuge | SAST (gosec, Semgrep für Go, Python, TypeScript; `dart analyze` und custom_lint für Dart), Abhängigkeiten (OSV-Scanner), Geheimnisse (gitleaks), Workflow-Analyse (zizmor, actionlint), schemabasiertes Fuzzing der API gegen das OpenAPI-Dokument, Go-Fuzzing der Parser (DPoP, JWS, Markdown, Patches, Uploads), Rechte-Testmatrix und Pflicht-Tests aus 07 und 08, CSP-Test der Konsole |
| P1 internes Red Team | Ende jeder Etappe, vor dem Merge nach `main` | frischer, nur lesender Prüfer plus Angriffstests auf einer Test-Instanz | Bedrohungsregister (alle Einträge mit Wahrscheinlichkeit `mittel` oder `hoch`), Diff gegen das Systemgrenzen-Diagramm, Prüfliste nach OWASP ASVS 5.0 Stufe 2 für Server und Konsole (https://owasp.org/www-project-application-security-verification-standard/, abgerufen 2026-10-10 [belegt]: 5.0.0 ist die aktuelle Fassung), OWASP Top 10 für LLM-Anwendungen 2025 für Agenten-Pfade (https://genai.owasp.org/llm-top-10/, abgerufen 2026-10-10 [belegt]) |
| P2 externer Test | vor der ersten produktiven Nutzung durch eine zweite Person und vor der Veröffentlichung | externer Dienstleister (Kosten: Entscheidung des Owners) | Rechte-Umgehung (IDOR, 404/403), Tokens und DPoP, WebSocket, Konsole (XSS, CSRF, Clickjacking), Uploads, SSRF, Aufruf-Marken, Prompt-Injection über Bereichsgrenzen, Apps und Webhooks, mobile Apps (Speicher, Schlüssel, Transport) |
| P3 Dev-Abschottung | vor der Freischaltung jeder Umgebungsart | Red Team | Ausbruchs- und Abflusstests nach [21-dev-isolation.md](21-dev-isolation.md), Abschnitt 8 |

### 13.2 Regeln

- Getestet wird nur gegen eine eigene Test-Instanz mit synthetischen Daten (Personen Alex, Sam,
  Robin; Domain `example.com`), nie gegen die produktive Instanz und nie mit echten Konten Dritter.
- Vor P2: Bedrohungsmodell aktuell, Scope und Regeln schriftlich, Kontakt für Notfälle benannt.
- Befunde werden nach Schwere eingeordnet, als Tickets geführt und nach der Behebung erneut getestet.
  Ein offener Befund mit Schwere "hoch" oder "kritisch" blockiert die Veröffentlichung.
- Jeder Befund aktualisiert das Bedrohungsregister (Pflegeregel im Bedrohungsmodell).

## 14. Tests

| ID | Test | Kontrollen |
|---|---|---|
| SEC-T01 | Klartext-Verbindung von nicht vertrauter Gegenstelle wird abgewiesen; Start mit Proxy-Header-Treiber und Nicht-Loopback-Listener scheitert | SEC-TLS-1, 08 ID-T08 |
| SEC-T02 | Lint: kein `InsecureSkipVerify`, kein `http.DefaultClient`, kein rohes SQL | SEC-TLS-4, SEC-NET-1, SEC-IN-5 |
| SEC-T03 | Kopfzeilen jeder Antwortart gegen eine Soll-Liste (API, Konsole, Datei, Fehler) | SEC-HDR-1 bis 4 |
| SEC-T04 | CORS: keine `Access-Control-Allow-*` außer an den öffentlichen Endpunkten; WebSocket mit fremdem `Origin` abgewiesen | SEC-CORS-1 bis 4 |
| SEC-T05 | Konsole im Headless-Browser mit erzwungener CSP: null Verletzungen, kein Abruf eines fremden Origins, WebSocket funktioniert | Abschnitt 5 |
| SEC-T06 | Schema-Fuzzing aller Routen: keine `5xx` außer erwarteten, keine Annahme unbekannter Felder, doppelte JSON-Schlüssel abgelehnt | SEC-IN-1, SEC-IN-2 |
| SEC-T07 | Bidi- und Steuerzeichen in Anzeigenamen abgelehnt | SEC-IN-4 |
| SEC-T08 | Uploads: falsche Typ-Angabe, SVG mit Skript, Dekompressionsbombe, Größe über Grenze, Scanner nicht bereit, infizierte Testdatei (EICAR) | SEC-UP-1 bis 7 |
| SEC-T09 | Egress: je eine Adresse aus jedem gesperrten Bereich aus SEC-NET-3 (IPv4 und IPv6, inklusive der Metadaten-Adresse), DNS-Name mit privater Antwort, Weiterleitung auf private Adresse: alle abgewiesen | SEC-NET-1 bis 5 |
| SEC-T10 | Kanarien-Test: definierte Kanarienwerte (Token, Nachrichtentext, Dateiname, Prompt, E-Mail, IP) durch alle Wege schicken; keiner erscheint in Logs, Traces, Metriken, Audit-Details, Fehlerantworten oder fremden Ereignissen | SEC-LOG-1 bis 7, SEC-SEC-3 |
| SEC-T11 | Secret-Provider-Datei mit Modus `0644` | Startabbruch, SEC-SEC-1 |
| SEC-T12 | `config print` mit gesetzten Geheimnissen | keine Geheimnisse in der Ausgabe |
| SEC-T13 | Client-Darstellung: Markdown mit HTML, `javascript:`-Link, Bild-URL, Link-Vorschau | nichts ausgeführt, nichts geladen |
| SEC-T14 | Freigabe-Karte mit Agententext, der einen Systemkopf nachahmt | Kopf aus Server-Feldern, Text klar getrennt |
| SEC-T15 | Wiederherstellung einer Sicherung mit danach gelöschter Nachricht | Nachricht bleibt gelöscht (Lösch-Journal) |
| SEC-T16 | Audit: Änderung einer Zeile über die Wartungsrolle, Neuschreiben der ganzen Kette, `UPDATE` mit der Anwendungsrolle | erkannt bzw. verweigert (SEC-AUD-1 bis 3) |
| SEC-T17 | TLS-Handshake mit einer Version unter 1.2 gegen einen Listener mit eigenem TLS | abgewiesen (SEC-TLS-2) |

## 15. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-SE-01 | Klartext nur von Loopback und vertrauten Proxys. | Compose und Reverse-Proxy funktionieren, direkter Klartext von außen nicht. |
| R-SE-02 | Konsole ohne fremde Origins, mit Nonce, Trusted Types und COOP/COEP. | Die Konsole ist das wertvollste Ziel im Browser; alles Fremde ist Angriffsfläche. |
| R-SE-03 | Erlaubnislisten für Log-Kopfzeilen und Telemetrie-Attribute statt Sperrlisten. | Neue Felder sind sonst standardmäßig sichtbar; das ist die falsche Fehlerklasse. |
| R-SE-04 | Ein Egress-Client mit IP-Prüfung beim Verbindungsaufbau. | SSRF-Schutz an einer Stelle, wirksam auch gegen DNS-Rebinding. |
| R-SE-05 | Clients laden keine fremden Ressourcen aus Inhalten. | Schließt Daten-Abfluss über Bild-URLs und Tracking der Lesenden. |
| R-SE-06 | Bild-Metadaten werden immer entfernt, ohne Schalter. | Ortsdaten in Fotos sind ein häufiges, unbemerktes Leck; ein Schalter zum Abschalten wäre eine Falle ohne Nutzen für einen Team-Chat. |
| R-SE-07 | Externer Penetrationstest vor der zweiten produktiven Person und vor der Veröffentlichung. | Studie C: Red Team vor erster echter zweiter Person; öffentliches Produkt braucht unabhängige Prüfung. |
| R-SE-08 | TLS im Server nur mit beiden Verweisen (`server.tls_cert_ref`, `server.tls_key_ref`), ohne eigene Cipher-Liste und ohne Klartext-Umleitung. | Ein Rand bleibt der Normalfall; ohne Rand soll der Server sicher allein stehen. Eine Umleitung nähme Klartext von jedem an. |
| R-SE-09 | Der interne Listener hat keinen Filter nach Gegenstelle. | Er liefert nur Health und stack-interne Webhooks, wertet keine Identitäts- oder Weiterleitungs-Kopfzeilen aus und ist nie am Host veröffentlicht. Ein Filter zwänge den Betreiber, das ganze Compose-Netz in `server.trusted_proxies` einzutragen; dann zählten Weiterleitungs-Kopfzeilen jedes Containers auch am öffentlichen Listener. |
| R-SE-10 | Strengere Sperrliste für ausgehende Verbindungen; `allow_private` öffnet nie Link-Local, Multicast oder unspezifizierte Adressen. | Bekannte SSRF-Umgehungen (gemappte IPv6, NAT64, 6to4, Teredo) und Metadaten-Dienste der Clouds; ein Operator-Eintrag soll ein internes Ziel öffnen, nicht die Metadaten. |
| R-SE-11 | Audit-Prüfung offline: kanonische Form gleich API-Darstellung, festes Schema der Details ohne Freitext. | Ein Export lässt sich ohne Server prüfen; Freitext wäre ein Ort für Inhalte und Geheimnisse. |

## 16. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `http.hsts_max_age` | `31536000s` | `max-age` für HSTS |
| `egress.allowed_hosts` | leer | zusätzliche erlaubte Ziele, Form `host[:port][;allow_private]` ([configuration.md](../operations/configuration.md), Abschnitt 2); doppelte Einträge sind ein Ladefehler |
| `egress.timeout` | `15s` | Zeitgrenze einer ausgehenden Anfrage |
| `egress.max_response_bytes` | `1048576` | größte Antwort einer ausgehenden Anfrage |
| `audit.checkpoint_interval` | `1h` | Abstand der signierten Prüfpunkte des Audit-Logs |
| `audit.checkpoint_sink` | leer | Ablageziel der Prüfpunkte außerhalb der Datenbank (z. B. Objektspeicher-Präfix mit Aufbewahrungssperre); leer erzeugt eine Warnung |
| `signing_key.cache_ttl` | `30s` | längste Zeit, die ein Prozess einen zurückgezogenen Schlüssel noch annimmt oder einen neuen nicht kennt |
| `signing_key.rotation_check_interval` | `1m` | Abstand des Jobs `instance.key_rotation` |
| `signing_key.access_token_rotation_interval` | `30d` | planmäßiger Wechsel des Schlüssels für Zugriffstokens |

Werte anderer Kapitel, auf die dieses Kapitel verweist: `server.trusted_proxies`
([16-betrieb.md](16-betrieb.md)), `http.max_json_body_bytes`, `http.max_header_bytes`,
`http.read_header_timeout`, `http.idle_timeout` ([02-architektur.md](02-architektur.md)),
`file.image.max_pixels` ([12-dateien.md](12-dateien.md)). Jeder Name steht in genau einer
Kapiteltabelle (Ruling L4).
