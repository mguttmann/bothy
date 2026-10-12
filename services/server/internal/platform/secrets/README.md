# Paket `platform/secrets`

Status: Provider gebaut (M1-09). Normativ sind
[16-betrieb.md](../../../../../docs/spec/16-betrieb.md) "Secret-Provider (ADR 0011)",
[19-sicherheit.md](../../../../../docs/spec/19-sicherheit.md) "Geheimnisse" (SEC-SEC-1 bis
SEC-SEC-3) und "Logs ohne Geheimnisse und ohne Inhalte" (SEC-LOG-1) sowie
[02-architektur.md](../../../../../docs/spec/02-architektur.md) "Ausfall einer Abhängigkeit"; bei
Abweichung gilt das Kapitel. Die Vorbereitung des Compose-Stacks (`bothy secrets init|export`)
liegt im Unterpaket [`stack`](stack/).

## Zweck

Einziger Weg, auf dem der Server ein Geheimnis liest: aus einem Verweis `<schema>:<pfad>` über
einen Treiber, gehalten höchstens `secrets.cache_ttl` lang, in einem Typ, der nie ausgegeben wird.
Eine Rotation wirkt so ohne Neustart. Module lesen nie selbst Dateien oder Umgebung; sie bekommen
Werte über ihre `Config` oder eine schmale Schnittstelle (02 "Konfigurationsladen" Punkt 5).

## Code

| Datei | Inhalt |
|---|---|
| `ref.go` | `Ref`, `ParseRef`, Grammatik der Verweise, Platzhalter `<secrets.dir>` |
| `errors.go` | Sentinel-Fehler, `RefError`, `ErrorCode` |
| `value.go` | `Value` (immer `[redacted]`), `Version` |
| `driver.go` | `Driver`, `Writer`, gemeinsame Prüfung am Anfang von `Get` |
| `file.go`, `file_unix.go`, `file_other.go` | Datei-Treiber, Besitzerprüfung je Plattform |
| `env.go` | Umgebungs-Treiber über eine Momentaufnahme |
| `provider.go` | `Config`, `Options`, `Provider` mit Cache, Rotation und Metrik |
| `doc.go` | Paketbeschreibung |

## Schnittstelle

- `ParseRef(raw, secretsDir)` liefert `Ref{Scheme, Path}`; `Ref.String()` gibt `<schema>:<pfad>`
  zurück.
- `NewProvider(Options{Dir, CacheTTL, Clock, Logger, Drivers, Env})` baut den einen Provider des
  Prozesses. Ohne `Drivers` gelten Datei-Treiber und Umgebungs-Treiber über `Env`, ohne `Clock`
  die Systemuhr, ohne `Logger` `slog.Default()`. Fehler: `Dir` nicht absolut, nicht sauber oder
  `/`, negative `CacheTTL`, ein Treiber ohne Schema, `nil` oder ein Schema doppelt.
- `(*Provider).Get(ctx, ref)` und `(*Provider).Resolve(ctx, raw)` (Parsen plus `Get`) liefern
  `(Value, Version, error)`.
- `Driver{Scheme, Get}` und `Writer{Driver, Put}`; `NewFileDriver()`, `NewEnvDriver(environ)`.
- `Value`: `NewValue` kopiert, `Bytes` gibt eine Kopie, `Len`, `Equal` in konstanter Zeit
  (`crypto/subtle`). `String`, `GoString`, `Format` (jedes Verb, Breite und Flags werden
  übergangen), `LogValue`, `MarshalJSON` und `MarshalText` ergeben genau `[redacted]`. Die Bytes
  liegen hinter einem Zeiger: Gibt `fmt` einen `Value` über Reflexion aus (unexportiertes
  Strukturfeld, Verb `%p`), erscheint nur eine Adresse. `==` vergleicht deshalb die Identität;
  für den Inhalt gilt `Equal`.

## Verweis-Grammatik

| Form | Regel |
|---|---|
| `file:<pfad>` | absolut, sauber (`path.Clean` lässt ihn gleich: kein `.` oder `..`, kein doppelter oder abschließender Schrägstrich), ohne NUL |
| `env:<NAME>` | `NAME` passt auf `^[A-Z_][A-Z0-9_]*$` |
| `azkv:...`, `awssm:...` | `unsupported_scheme`: Treiber einer späteren Etappe |
| alles andere, ohne Doppelpunkt, leeres Schema | `invalid_ref` |

`ParseRef` ersetzt vor dem Parsen jedes `<secrets.dir>` durch den wirksamen Wert von
`secrets.dir`; enthält der Verweis den Platzhalter, muss dieser Wert selbst absolut und sauber
sein. Ein ungültiger Verweis steht in keiner Meldung, auch nicht teilweise: Ein Geheimnis kann im
falschen Schlüssel stehen. Die Meldung nennt nur ein bekanntes Schema und die Regel. Gültige
`file:`- und `env:`-Verweise werden voll genannt, mit Steuerzeichen in Anführungszeichen. Einen
im Code gebauten `Ref` eines anderen Schemas (`azkv`, `awssm`, eigener Treiber) nennen die
Meldungen von `Provider.Get` und der Treiber nicht: Dessen Grammatik prüft dieses Paket nicht.

## Datei-Regeln (`file:`)

1. `os.Stat` folgt Symlinks, so funktionieren als Links eingehängte Geheimnisse (Kubernetes). Das
   Ziel muss eine reguläre Datei sein; ein Verzeichnis, Gerät oder FIFO ist `insecure_file`, bevor
   etwas geöffnet wird (ein FIFO würde das Öffnen blockieren).
2. Modus genau `0400` oder `0600`, geprüft über die Rechte samt setuid, setgid und sticky. Sonst
   `insecure_file` mit dem beobachteten Modus, etwa `mode 0644, want 0400 or 0600`.
3. Besitzer ist die effektive uid des Prozesses (SEC-SEC-1), sonst `insecure_file` mit
   `owner uid <ist>, want uid <soll>`. Ohne Unix-Besitz (andere Plattformen) liest der Treiber
   nichts.
4. Nach dem Öffnen wird die geöffnete Datei erneut geprüft und muss dieselbe sein (`os.SameFile`);
   ein Austausch zwischen Prüfung und Lesen ist `read_failed`.
5. Höchstens 1 MiB (1048576 Bytes), sonst `read_failed`.
6. Genau ein abschließendes `\n` und ein `\r` direkt davor werden entfernt (`echo wert > datei`
   hängt es an, kein Passwort endet damit). Leer danach ist `not_found` wie eine fehlende Datei:
   Ein leeres Passwort ist nie gültig.

Der Umgebungs-Treiber (`env:`) liest die Momentaufnahme `Options.Env`, die `cmd/bothy` einmal
liest; bei doppelten Namen gilt der erste Eintrag wie bei `os.Getenv`. Nicht gesetzt oder leer ist
`not_found`. `env:` ist erlaubt, aber nicht empfohlen (16 5.2).

## Cache und Rotation

- Je Verweis ein Eintrag mit Wert, Version und Lesezeit. `Get` liefert den Eintrag, solange
  `0 <= jetzt - Lesezeit < secrets.cache_ttl`; danach wird neu gelesen. Liegt die Lesezeit in der
  Zukunft (Uhr zurückgestellt, etwa durch NTP), gilt der Eintrag als abgelaufen. `0` liest bei
  jedem Aufruf und protokolliert dann jedes gescheiterte Lesen; darum ist der Schlüssel
  `secrets.cache_ttl` mindestens `1s`, und `0` gibt es nur in Tests.
- Gleichzeitige Fehlgriffe auf denselben Verweis lesen einmal (Mutex je Eintrag): Wer während
  eines Lesens kommt, wartet darauf und bekommt dessen Ergebnis; der Kontext wird beim Aufruf
  geprüft. Andere Verweise warten nicht.
- Scheitert das erste Lesen, geht der Fehler an den Aufrufer, und nichts wird gespeichert: Der
  Start bricht ab (02 "Ausfall einer Abhängigkeit").
- Scheitert ein erneutes Lesen, bleibt der alte Wert in Gebrauch. Dazu kommen eine Logzeile auf
  Stufe ERROR mit der festen Meldung `secret refresh failed, keeping previous value` und nur dem
  Attribut `error.type` (Fehlercode) sowie ein Schritt des Zählers
  `bothy.secrets.refresh.failures` (Einheit `{refresh}`, ohne Attribute, globaler
  OpenTelemetry-Meter, Prometheus `bothy_secrets_refresh_failures_total`). Der Versuch zählt als
  Lesen, so folgen der nächste Versuch und die nächste Zeile erst eine TTL später: eine Zeile je
  TTL und Verweis, nicht je Aufruf. Der Alarm `BothySecretRefreshFailing` hängt an diesem Zähler.
- Ein abgebrochener Kontext ist keine gescheiterte Rotation: `Get` gibt den Kontextfehler zurück,
  ohne Log, ohne Zähler und ohne den Eintrag zu ändern.

## Fehlercodes

`ParseRef`, die Treiber `file` und `env` und `Provider.Get` liefern einen
`*RefError{Ref, Err, Detail}` mit der Meldung `secrets: <verweis>: <fehler>: <detail>` und nie
mit Inhalt. `Err` ist ein Sentinel der Tabelle oder, bei abgebrochenem Aufruf, der Kontextfehler.
Den Fehler eines eigenen Treibers (`Options.Drivers`) gibt `Provider.Get` unverändert weiter;
`NewProvider` liefert einfache Konfigurationsfehler. `ErrorCode` bildet jeden Fehler auf einen
der acht Codes ab, für Logs, Metriken und `bothy config check --secrets`; die drei Schreib-Codes
kommen nur aus `Put` und `Delete` (M1-32):

| Code | Sentinel | Bedeutung |
|---|---|---|
| `invalid_ref` | `ErrInvalidRef` | Grammatik verletzt |
| `unsupported_scheme` | `ErrUnsupportedScheme` | Schema ohne Treiber in dieser Version |
| `not_found` | `ErrNotFound` | Datei oder Variable fehlt oder ist leer |
| `insecure_file` | `ErrInsecureFile` | keine reguläre Datei, falscher Modus oder Besitzer |
| `read_failed` | `ErrReadFailed` | Lesefehler, zu groß, ausgetauscht; auch jeder andere Fehler |
| `read_only` | `ErrReadOnly` | `Put` oder `Delete` außerhalb von `secrets.signing_key_dir`, `env:` oder ein Treiber ohne `Writer` |
| `exists` | `ErrExists` | `Put` auf einen vorhandenen Namen; nie überschrieben |
| `write_failed` | `ErrWriteFailed` | Schreib- oder Löschfehler |

## Schreiben: der Schlüsselspeicher (M1-32)

The server writes exactly one place: the key store `secrets.signing_key_dir` of the signing keys
([ADR 0066](../../../../../docs/adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md),
16 section 5.2). `Options.SigningKeyDir` names it; empty means no key store.

- `Provider.Put(ctx, ref, value)` and `Provider.Delete(ctx, ref)` accept only a `file:` reference
  whose file lies directly in that directory and whose driver implements `Writer`; every other
  reference, `env:` always, is `ErrReadOnly` before the driver is called. Both drop the cached
  value of the reference.
- The file driver's `Put` creates a temporary file (prefix `TempPrefix`, `.bothy-tmp-`) with mode
  `0600` in the same directory, writes and syncs it, links it to the final name (an existing name
  is `ErrExists`, never overwritten), removes the temporary name and syncs the directory. `Delete`
  removes the file and syncs the directory; a missing file is `ErrNotFound`.
- `CheckSigningKeyDir(dir)` is the start check of every role: an absolute clean path to a
  directory owned by the process user with mode `0700` in which a probe file can be created.
  The error names `secrets.signing_key_dir`, the expected mode and what was observed.

`Writer` is the driver interface for this; `azkv` and `awssm` implement it with V3-12.

## Version und Fingerabdruck

`Version` ist für `file:` und `env:` der Anfang (16 Hex-Zeichen) des SHA-256 des Werts: gleicher
Inhalt, gleiche Version. Der Fingerabdruck von `bothy secrets init` (Unterpaket `stack`) ist
ebenfalls ein gekürzter SHA-256. Beide sind keine Geheimnisse, gehören aber nicht in Logs: Bei
einem schwachen Wert erlauben sie, Vermutungen zu prüfen.

## `Config` und Bindung

`Config{Dir, CacheTTL, SigningKeyDir}` trägt die Schlüssel `secrets.dir`, `secrets.cache_ttl` und
`secrets.signing_key_dir` (Standardwerte in
der [Konfigurationsreferenz](../../../../../docs/operations/configuration.md)). `internal/wire`
füllt sie mit `config.Bind` und baut daraus mit der Umgebung aus `cmd/bothy` den einen Provider
des Prozesses (D10); nur `internal/wire` und `cmd/bothy` erzeugen Provider.

## Was fehlt

- Ein erneutes Lesen, das wartende Aufrufer nicht aufhält (etwa `singleflight` mit Kontext): Die
  Treiber in M1 lesen lokale Dateien und die Umgebung; die Netz-Treiber bringen es mit.
- Die Logzeile eines gescheiterten erneuten Lesens nennt den Verweis nicht (nur `error.type` aus
  der Liste in `docs/observability/signals.md` 5.3); `bothy config check --secrets` zeigt ihn.
- A key store shared by several nodes (stage 2 of 02 section 16) needs a shared volume or a
  writable vault driver; M7-17 decides it.
- Treiber `azkv` und `awssm` in einer späteren Etappe.

## Abhängigkeiten

Standardbibliothek, `internal/kernel/clock` und die API von OpenTelemetry
(`go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/metric`; Register: `platform.secrets` mit
den Vendor-Gruppen `age` und `otel`). Das SDK (`go.opentelemetry.io/otel/sdk/metric`) nutzen nur
die Tests.

## Kapitel

[16-betrieb.md](../../../../../docs/spec/16-betrieb.md),
[19-sicherheit.md](../../../../../docs/spec/19-sicherheit.md),
[02-architektur.md](../../../../../docs/spec/02-architektur.md),
[ADR 0011](../../../../../docs/adr/0011-12-factor-konfiguration-und-secret-provider.md),
[configuration.md](../../../../../docs/operations/configuration.md),
[signals.md](../../../../../docs/observability/signals.md)
