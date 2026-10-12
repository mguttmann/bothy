# services/host

Host-Dienst `bothy-host`: eigenes Go-Binary auf angebundenen Rechnern für das Terminal.
Normativ: [24 Terminal](../../docs/spec/24-terminal.md),
[ADR 0068](../../docs/adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md).

Er läuft als Benutzer ohne Administratorrechte, verbindet sich nur ausgehend mit dem Server, ist
Noise-Gegenstelle des Geräts und startet je Terminal-Sitzung eine PTY (macOS, Linux, Windows über
ConPTY).

## Stand

Mit M2-38 gibt es das Go-Modul `github.com/OWNER/bothy/services/host` mit dem Noise-Kern. Das
Binary hat noch keine Befehle; M2-42 trägt `install`, `pair`, `doctor`, `run`, `restart`,
`session-user` und `pty-helper` ein, die Schlüsselablage und den lokalen Kanal. M2-43 baut die
Steuer- und Datenverbindungen, M2-45 die Prüfung der Sitzungs-Marke und die PTY.

## Aufbau

| Pfad | Inhalt |
|---|---|
| `cmd/bothy-host/` | Befehlsverteiler ohne Befehle (Nutzung, Exitcodes 0, 1 und 2) |
| `internal/noise/` | Handshake `Noise_NK_25519_ChaChaPoly_SHA256` über `github.com/flynn/noise`, Prolog, Transport mit den Grenzen der Noise-Spezifikation, Hash des ephemeren Schlüssels für `cek` (24 5.2, 5.3) |
| `internal/frame/` | Rahmen `start` bis `replay_end` mit Byte-Reihenfolge, Inhaltsregeln und Richtungen (24 5.4), gefuzzter Parser |
| `internal/fingerprint/` | Fingerabdruck des Rechners in Crockford-Base32 mit Anzeigeform (24 4.3) |
| `testdata/noise/` | öffentliche Testvektoren, Aufzeichnungen von Go und Dart, Fingerabdruck-Vektoren ([README](testdata/noise/README.md)) |
| `.go-arch-lint.yml` | Architekturregeln, von Hand gepflegt, eine Komponente je Paket |

Was die Pakete zusichern: kein Netz, kein Prozessstart, keine Logs; Fehlertexte nennen Codes,
Längen und Typ-Bytes, nie Nutzlast, Klartext, Schlüssel oder Marken. Nach einem gescheiterten
Handshake-Schritt oder einer gescheiterten Entschlüsselung verweigert das Objekt jeden weiteren
Aufruf.

## Befehle

```sh
cd services/host
GOTOOLCHAIN=local go test -race ./...                         # Tests, auch TM-T01 und TM-T02
make recordings                                               # recordings/go.json und fingerprint.json neu schreiben
make release-build VERSION=$(cat VERSION) OUT=<ordner>        # unsignierte Binaries für fünf Plattformen
GOTOOLCHAIN=local go test -run='^$' -fuzz=FuzzDecode -fuzztime=60s ./internal/frame
```

`release-build` baut ohne cgo `bothy-host_<version>_<os>_<arch>` (Windows mit `.exe`) für
darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 und windows/amd64
([18 Release](../../docs/spec/18-release.md), Zeile `host`). Signatur, SBOM, Prüfsummen und
Installationspakete ergänzt M7-23; Releases tragen Tags `host/v<version>`.
