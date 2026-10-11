# Paket `platform/egress`

Status: gebaut (M1-22). Normativ sind
[19-sicherheit.md](../../../../../docs/spec/19-sicherheit.md) Abschnitt 8 "SSRF-Verbot und
ausgehende Verbindungen" (SEC-NET-1 bis SEC-NET-5), SEC-TLS-4, R-SE-04 und R-SE-10 sowie die
[Konfigurationsreferenz](../../../../../docs/operations/configuration.md) (Abschnitt 2, Zeile
"Egress-Ziel"); bei Abweichung gilt das Kapitel.

## Zweck

Einziger Weg, auf dem der Server ausgehende HTTP-Verbindungen öffnet. Ein Ziel, das weder ein
Eintrag in `egress.allowed_hosts` noch der Verbraucher erlaubt, wird vor jeder Verbindung und
ohne DNS-Abfrage abgewiesen; die IP-Adresse wird nach der Auflösung beim Verbindungsaufbau
geprüft, und verbunden wird nur die geprüfte Adresse. Weiterleitungen, Zeit, Antwortgröße und
TLS sind begrenzt bzw. immer geprüft.

## Code

| Datei | Inhalt |
|---|---|
| `rule.go` | `Rule`, `ParseRule`: Einträge `host[:port][;allow_private]` aus `egress.allowed_hosts` |
| `addr.go` | Adressprüfung: immer gesperrte Bereiche und private Bereiche (nur mit `allow_private`) |
| `client.go` | `Config`, `Options`, `HostPolicy`, `Deps`, `New`, `Client.Do`, `Client.HTTPClient`: Prüfung vor der Verbindung, Dialer mit Adressprüfung, Weiterleitungen, Grenzen, Log und Zähler |
| `loopback.go` | `NewLoopback`: Client für `bothy healthcheck`, nur Loopback und `http` |
| `errors.go` | `ErrBlocked`, `ErrResponseTooLarge`, `BlockedError`, `Reason` |
| `doc.go` | Paketbeschreibung mit allen Regeln und einem Verbraucher-Beispiel |

## Schnittstelle

- `New(cfg, opts, deps)` baut einen Client je Verbraucher: `Config` aus `egress.*` (baut
  `internal/wire` mit `EgressConfig`), `Options` mit Zweck, Host-Policy des Verbrauchers,
  Weiterleitungsmodus und eigenen Grenzen eines Fachkapitels, `Deps` mit Resolver,
  Wurzelzertifikaten, Logger und Zähler.
- `Client.Do(req)` sendet; `Client.HTTPClient()` liefert einen `*http.Client` für Bibliotheken,
  der dieselben Prüfungen im Transport hat.
- `errors.Is(err, ErrBlocked)` erkennt jede Abweisung; `*BlockedError` nennt Grund und Host.
  Ein Körper über der Grenze liefert beim Lesen `ErrResponseTooLarge`.
- `NewLoopback(timeout)` für den Health-Check.

## Regeln

1. Erlaubt sind nur Ziele aus `egress.allowed_hosts` und aus der Host-Policy des Verbrauchers
   (`Options.Hosts`); alles andere endet mit `host_not_allowed` ohne DNS-Abfrage. Ein Eintrag
   ohne Port erlaubt nur den Standard-Port des Schemas.
2. `https` immer, `http` nur mit einem passenden Eintrag mit `allow_private`. URLs mit
   Benutzer-Teil werden abgewiesen.
3. Nach der Auflösung werden nur Adressen gewählt, die die Prüfung bestehen; gemappte
   IPv6-Adressen werden vorher entpackt. Immer gesperrt und gesperrt außer mit `allow_private`
   sind die Bereiche aus SEC-NET-3. `net.Dialer.Control` prüft die Socket-Adresse ein zweites Mal.
4. `allow_private` gibt es nur in `egress.allowed_hosts` (Betreiber); die Host-Policy eines
   Verbrauchers öffnet nie private Adressen oder `http`.
5. Weiterleitungen: `RedirectNever` gibt die 3xx-Antwort zurück, `RedirectChecked` folgt nur
   geprüften Zielen, höchstens zehn. Eine Weiterleitung mit nicht lesbarem `Location` endet in
   beiden Modi mit einem Fehler, der den Header nicht zitiert. Eine Antwort `101` (Protokollwechsel)
   ist immer ein Fehler, weil keine Frist das Lesen der gewechselten Verbindung begrenzen würde.
6. Keine Umgebungs-Proxys; TLS ab Version 1.2, Zertifikat und Hostname immer geprüft.
7. Zeitgrenze `egress.timeout` (oder Wert des Verbrauchers) für die ganze Anfrage über alle
   Weiterleitungen, auch für Auflösung und Verbindungsaufbau, die net/http vom Abbruch der Anfrage
   löst; die Adressen eines Namens teilen sich die verbleibende Zeit. Steckt eine Bibliothek den
   Transport von `HTTPClient` in einen eigenen Client, gilt die Grenze je Weiterleitungsschritt. Größengrenze
   `egress.max_response_bytes` (oder Wert des Verbrauchers) für den Körper, gezählt nach der
   Dekodierung durch den Transport; `egress.max_response_bytes` begrenzt auch die Kopfzeilen.
8. Jede Abweisung: ein Log-Eintrag auf `Warn` mit `bothy.egress.purpose`, `bothy.reason` und
   `server.address`, nie Pfad, Query, Kopfzeilen oder Inhalt; der Zähler
   `bothy.egress.blocked` über `Deps.Metrics`. Fehlertexte von `Client.Do` nennen nur Schema und
   Host.

## Abhängigkeiten

Nur Standardbibliothek (Register: `platform.egress` ohne Vendor-Gruppe). Den Zähler setzt
`platform/otel` über die Schnittstelle `Metrics` um; bis dahin zählt ein Client ohne `Metrics`
nichts. Nutzer sind Adapter (`identity/adapters/oidc` ab M1-41, Push, Webhooks,
Objektspeicher, Transkription) und `cmd/bothy` für den Health-Check.

## Lint

Die Semgrep-Regel `bothy-go-egress-only` meldet in Produktionscode außerhalb dieses Pakets
`http.DefaultClient`, `http.DefaultTransport`, `http.Get` und verwandte Funktionen (auch als
Funktionswert), eigene `http.Client`, `http.Transport`, `net.Dialer` und `tls.Dialer` (auch als
Nullwert, Strukturfeld, eingebetteter Wert, Typdefinition, Alias oder in Slices, Arrays und
Maps), Reverse-Proxys, `net.Dial`, `tls.Dial` und verwandte sowie `x.Transport = nil`; Testcode
ist ausgenommen. Was die Regel nicht sieht, nennt docs/DECISIONS.md M1-22/9. Jede nötige Konstruktion in diesem Paket trägt einen
`nosemgrep`-Kommentar mit Grund. `bothy-go-tls-verify` meldet `InsecureSkipVerify` mit jedem Wert
außer `false`, auch in Tests; dafür ersetzt `.semgrepignore` die eingebaute Ignore-Liste von
Semgrep, die Testdateien übersprang (M1-22/13).

## Kapitel

[19-sicherheit.md](../../../../../docs/spec/19-sicherheit.md),
[02-architektur.md](../../../../../docs/spec/02-architektur.md) (Code-Layout, Fristen),
[signals.md](../../../../../docs/observability/signals.md),
[configuration.md](../../../../../docs/operations/configuration.md)
