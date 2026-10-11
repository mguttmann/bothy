# services/host

Host-Dienst `bothy-host`: eigenes Go-Binary auf angebundenen Rechnern für das Terminal. Heute nur
Planung, kein Code.

Er läuft als Benutzer ohne Administratorrechte, verbindet sich nur ausgehend mit dem Server, ist
Noise-Gegenstelle des Geräts und startet je Terminal-Sitzung eine PTY (macOS, Linux, Windows über
ConPTY). Normativ: [24 Terminal](../../docs/spec/24-terminal.md),
[ADR 0068](../../docs/adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md).

Releases tragen Tags `host/v<version>` und brauchen `host/VERSION`, `host/CHANGELOG.md` und das
Ziel `release-build` in `host/Makefile` ([18 Release](../../docs/spec/18-release.md)).
