# Paket `platform/config`

Status: gebaut (M1-07), Verweise erweitert (M1-09). Normativ sind
[02-architektur.md](../../../../../docs/spec/02-architektur.md) "Konfigurationsladen",
[16-betrieb.md](../../../../../docs/spec/16-betrieb.md) "Konfiguration (ADR 0011)" und die
[Konfigurationsreferenz](../../../../../docs/operations/configuration.md) Abschnitte 1 bis 3; bei
Abweichung gilt das Kapitel.

## Zweck

Einziger Weg, auf dem Prozess-Konfiguration in den Server kommt: Schema aller Schlüssel, Parser
je Art, Laden mit Vorrang, Prüfung beim Start und Bindung an die `Config`-Strukturen der Nutzer.
Einstellungen des Owners (`instance_settings`, Feature-Flags) sind keine Konfiguration und gehören
nicht hierher.

## Code

| Datei | Inhalt |
|---|---|
| `kind.go` | `Kind` und die Parser je Art mit kanonischem Text |
| `envname.go` | Namensregel `a.b_c` zu `BOTHY_A__B_C` und zurück |
| `schema.go` | `Key`, `Schema`, `NewSchema` mit allen Prüfregeln, `AddCheck` |
| `load.go` | `Sources`, `Load`: Datei, Umgebung, Flag, unbekannte Namen, Querprüfungen |
| `values.go` | `Values` mit Zugriffen je Art, `Source` |
| `bind.go` | `Bind`: Werte in eine Struktur mit Tags `config:"<schlüssel>"` |
| `errors.go` | `LoadError` und `Problem` |
| `doc.go` | Paketbeschreibung |

## Schnittstelle

- `NewSchema(keys)` prüft die Schlüssel; `(*Schema).AddCheck(name, keys, fn)` registriert eine
  Querprüfung; `(*Schema).Keys()` liefert die Schlüssel nach Namen sortiert.
- `Load(schema, Sources{Env, File, Flags})` liefert `*Values` oder `*LoadError` mit allen
  Problemen.
- `Values`: `String`, `Int`, `Float`, `Bool`, `Duration`, `Strings`, `Prefixes`, `Text`,
  `Source`, `File`. Ein unbekannter Schlüssel oder eine falsche Art ist ein Programmierfehler und
  führt zu einem Panic, den Tests finden.
- `Bind(values, &cfg)` füllt eine Struktur des nutzenden Pakets; `internal/wire` ruft es auf.
- `EnvName` und `KeyFromEnv` sind für gültige Namen exakt umkehrbar.

## Regeln

1. Vorrang von niedrig nach hoch: Standardwert, Datei (`--config` oder `BOTHY_CONFIG_FILE`),
   Umgebung (`BOTHY_*`), Flag (nur `--role`). `Values.Source` nennt je Schlüssel den Gewinner.
2. Jeder Wert aus Datei, Umgebung oder Flag wird geprüft, auch wenn eine höhere Quelle ihn
   überschreibt; ein Fehler in der Datei bleibt so nicht verborgen, bis die Variable wegfällt.
3. Unbekannte `BOTHY_`-Variablen und unbekannte Datei-Schlüssel brechen ab. Ausgenommen sind
   `BOTHY_CONFIG_FILE` und Variablen, die ein gültiger Verweis `env:` im wirksamen Wert eines
   `*_ref`-Schlüssels nennt. Variablen ohne Präfix `BOTHY_` bleiben unbeachtet.
4. Die Datei ist ein YAML-Dokument, verschachtelt wie die Namen. Doppelte und gepunktete
   Schlüssel, Anker, Aliase, Merge-Schlüssel und ausdrückliche Tags sind Probleme. Skalare gehen
   als geschriebener Text an denselben Parser wie die Umgebung, so ändert die implizite Typisierung
   von YAML keinen Wert (`yes` bleibt `yes`, `1.10` bleibt `1.10`).
5. Pflicht-Schlüssel brauchen einen nicht leeren Wert. Grenzen und Aufzählungen gelten für jede
   Quelle. Querprüfungen laufen nur, wenn alle ihre Schlüssel ohne Problem geladen sind (auch
   keiner in einer verworfenen Datei oder unter einem verworfenen Abschnitt), und sehen nur diese
   Schlüssel.
6. Probleme werden gesammelt, nach Schlüssel sortiert und zusammen gemeldet:
   `config: <schlüssel>: <meldung> (source: <quelle>)`. Keine Meldung enthält einen Wert, auch
   keinen ungültigen: Ein Geheimnis kann im falschen Schlüssel stehen. Darum nennt eine Meldung
   einen Datei-Schlüssel ohne Schlüsselform nur mit seiner Zeile, und Pfade oder Namen mit
   Steuerzeichen stehen in Anführungszeichen.
7. Geheimnisse stehen nur als Verweis in Schlüsseln mit Endung `_ref` (Art `secret_ref`). Dieses
   Paket prüft nur die Form; aufgelöst wird nichts. Ein Verweis `file:` nennt einen absoluten,
   bereinigten Pfad (kein Element `.` oder `..`, kein doppelter und kein abschließender
   Schrägstrich, kein NUL), dieselbe Regel wie im Secret-Provider (`internal/platform/secrets`),
   damit `bothy config check` und der Start dieselben Verweise annehmen. Ein Verweis `env:` nennt
   weder `BOTHY_CONFIG_FILE` noch eine Variable in der Form einer Schlüssel-Variable. In
   Standardwerten von `secret_ref`-Schlüsseln steht `<schlüssel>` für den wirksamen Wert eines
   String-Schlüssels; in gesetzten Werten aus Datei und Umgebung wird nur `<secrets.dir>` ersetzt,
   mit dem wirksamen Wert von `secrets.dir` (M1-09). Eingesetzter Text wird nicht erneut
   durchsucht. Ein Problem des ersetzten Texts beginnt mit `after expanding <secrets.dir>:`; hat
   `secrets.dir` selbst ein Problem, meldet der Verweis kein zweites. `secrets.dir` prüft der
   Lader selbst (Regel, die keine Art ausdrückt, M1-09/15): absolut, sauber, nicht `/`, ohne NUL
   und ohne `<`. A `secret_ref` key with an empty default (chapter cell "leer", for example
   `server.tls_cert_ref`) is optional: the empty text means "no reference" and switches its
   feature off; a key with a reference as default still refuses the empty text
   (docs/DECISIONS.md M1-21/1).
8. Das Paket liest nie die Prozess-Umgebung. `cmd/bothy` liest sie einmal und übergibt sie; eine
   forbidigo-Regel in `.golangci.yml` verbietet die Lesefunktionen (auch `exec.Cmd.Environ` und
   `http.ProxyFromEnvironment`) außerhalb von `cmd/bothy` und der Tests unter `tools/`.

Grammatiken der Arten: Konfigurationsreferenz, Abschnitt "Namensregel und Formate".

## Neuer Schlüssel

Ein Schlüssel kommt mit dem Ticket ins Schema, das seinen ersten Nutzer baut (02, 13 Punkt 5):
Zeile in der Tabelle "Konfigurationswerte dieses Kapitels" seines Kapitels, Eintrag in der Datei
seines Bereichs `internal/wire/configkeys_<bereich>.go` mit demselben Standard und dem Kapitel als
Konstante aus `internal/wire/configchapters.go`, Feld in der `Config`-Struktur des nutzenden
Pakets. Eine neue Querprüfung gehört in die Liste `configChecks` in
`internal/wire/configchecks.go`. Parallele Tickets teilen sich damit vier Stellen: die Liste der
Schlüsselgruppen in `internal/wire/config.go`, die Liste `configChecks`, die Kapitel-Konstanten
und die erwarteten Schlüssel in `TestConfigSchemaBuilds`.

## Abhängigkeiten

Standardbibliothek und `go.yaml.in/yaml/v3` (Register: `platform.config` mit Vendor-Gruppe
`yaml`). Keine anderen Projekt-Pakete; `wire` und `cmd` nutzen dieses Paket.

## Kapitel

[02-architektur.md](../../../../../docs/spec/02-architektur.md),
[16-betrieb.md](../../../../../docs/spec/16-betrieb.md),
[ADR 0011](../../../../../docs/adr/0011-12-factor-konfiguration-und-secret-provider.md),
[configuration.md](../../../../../docs/operations/configuration.md)
