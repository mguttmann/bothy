# Paket `platform/secrets/stack`

Status: gebaut (M1-09). Normativ sind
[16-betrieb.md](../../../../../../docs/spec/16-betrieb.md) 5.3 "Geheimnisse im Compose-Stack" und
5.4 "Hinterlegung (Escrow)" und die
[Konfigurationsreferenz](../../../../../../docs/operations/configuration.md) Abschnitt 5
"Geheimnisse"; bei Abweichung gilt das Kapitel.

## Zweck

Vorbereitung des Volumes `secrets` für den Compose-Stack: `bothy secrets init` erzeugt fehlende
Geheimnisse, übernimmt die vom Owner gelieferten Dateien und legt je Dienst ein Verzeichnis mit
genau den Kopien an, die der Dienst über `subpath` einbindet. `bothy secrets export` schreibt die
kanonischen Kopien und den Schlüsselspeicher der Signierschlüssel als mit age verschlüsseltes
Bündel für die Hinterlegung. Beide Befehle lesen nur
ihre Flags und die genannten Verzeichnisse: keine Konfiguration, keine Umgebung, keine Datenbank
(der Container `secrets-init` hat nichts davon).

## Code

| Datei | Inhalt |
|---|---|
| `catalog.go` | `Entry`, `Origin`, `Generator`, `Catalog`, `Services`, Zuordnung eines Pfads zu seinem Eintrag |
| `init.go` | `Init`, `InitOptions`, `Owner`, `ParseOwner`, `Report`, `Line`, `Status`, `ErrUsage`, `ErrConflict` |
| `export.go` | `Export`, `ExportOptions` |
| `fsutil.go` | Durchsuchen ohne Folgen von Links, Lesen mit Prüfung der geöffneten Datei, atomares Schreiben, Escape-Folgen in Fehlermeldungen |
| `fsutil_unix.go`, `fsutil_other.go` | Besitzer lesen, Verzeichnis-`fsync`, laufender Benutzer je Plattform |
| `lock_flock.go`, `lock_other.go` | Sperre auf `<target>` mit `flock`, ohne `flock` Abbruch |
| `doc.go` | Paketbeschreibung |

## Schnittstelle

- `Catalog()` liefert die Einträge in der Reihenfolge der Tabelle (Kopie), `Services()` die
  Dienste sortiert und eindeutig.
- `Init(ctx, InitOptions{Target, Source, Owners, AdoptSource, Rand})` liefert einen `Report`;
  `Report.WriteTo` schreibt eine Zeile je Eintrag und als letzte Zeile `Report.Reminder`. Bei
  einem Fehler ist der Bericht leer.
- `ParseOwner("<dienst>=<uid>:<gid>")` prüft nur die Form (Dienst aus Kleinbuchstaben, Ziffern
  und `-`, Dezimalzahlen von 0 bis 2147483647) und wiederholt die Eingabe nie; ob der Dienst
  existiert, prüft `Init`.
- `Export(ctx, ExportOptions{From, Recipients, Output, Armor, Stdout, IsTerminal})`.
- Fehler, die `ErrUsage` umhüllen, sind Bedienfehler (Exitcode 2); alle anderen, auch
  `ErrConflict`, sind Exitcode 1. `cmd/bothy` bildet sie auf die Exitcodes ab.

## Katalog

Jede Zeile der Tabelle in configuration.md Abschnitt 5 ist ein Eintrag mit Name, Herkunft und
Diensten; `TestCatalogMatchesConfigurationDoc` vergleicht beide Richtungen und die Reihenfolge,
`TestCatalogServicesMatchComposeSubpaths` die Dienste mit den `subpath`-Einbindungen in
`infra/compose/compose.yaml`.

- Dateieintrag: genau eine Datei (`db_app_password`).
- Verzeichniseintrag, Name endet auf `/`: jede reguläre Datei darunter, rekursiv (`offsite/`,
  `pgbackrest_tls/`).
- Mustereintrag: jede Datei, deren relativer Pfad dem verankerten Muster entspricht
  (`oidc/<provider>_client_secret` mit `^oidc/[a-z0-9_-]+_client_secret$`,
  `oidc/<provider>_client_key.pem` mit `^oidc/[a-z0-9_-]+_client_key\.pem$`); im Bericht eine
  Zeile je Datei, ohne Datei eine Zeile mit dem Platzhalter.
- Erzeuger: `token` sind 32 Zufallsbytes als base64url ohne Auffüllung (43 Zeichen), `key` 32
  Zufallsbytes als Hex in Kleinbuchstaben (64 Zeichen); Quelle ist `crypto/rand`.
  `pgbackrest-tls` (M7-01, `pgbackrest_tls.go`) creates the directory `pgbackrest_tls/`: a
  self-signed CA limited to leaf certificates (`ca.crt`; its key is never written) and per host
  `postgres` and `backup` an ECDSA P-256 key in PKCS#8 (`<host>.key`) with a certificate for the
  host name with server and client use (`<host>.crt`), all valid for `PGBackRestTLSValidity`.
- `pending`: Der Erzeuger kommt mit einem späteren Ticket (`Entry.PendingOn`):
  `s3_access_key` und `s3_secret_key` (M3, bis dahin geliefert).
  Fehlen sie, meldet `init` sie als `pending` und endet mit Exitcode 0; geliefert werden sie
  übernommen wie jeder andere Eintrag.

## Ablage

- `<target>` hat Modus `0700` und den laufenden Benutzer als Besitzer (beides repariert `init`);
  ist es nicht vorhanden, legt `init` es an. Ein vorhandenes `<target>` muss leer sein (bis auf
  `lost+found`) oder `_store` enthalten; sonst bricht `init` mit Exitcode 2 ab, bevor es etwas
  ändert, damit ein vertipptes `--target` kein fremdes Verzeichnis übernimmt.
- `<target>/_store/` hält die eine kanonische Kopie jedes Eintrags: Verzeichnisse `0700`, Dateien
  `0400`, Besitzer ist der laufende Benutzer. Dateien ohne Katalogeintrag bleiben unberührt und
  werden nicht kopiert; eine Datei, die weder Verzeichnis noch reguläre Datei ist, bricht ab.
- `<target>/<dienst>/` für jeden Dienst aus `Services()`, auch ohne Kopie, weil eine
  `subpath`-Einbindung ein vorhandenes Verzeichnis braucht. Erwartet sind alle Einträge in
  `_store`, deren Zeile den Dienst nennt; alles andere im Dienstverzeichnis wird entfernt, auch
  Links (ohne ihnen zu folgen).
- Relative Namen sind überall die Katalognamen; so hat ein entschlüsseltes Bündel das Layout einer
  Quelle (plus `signing-keys/`, which `init` skips) ([disaster-recovery.md](../../../../../../docs/operations/disaster-recovery.md) Fall B).

## Abgleich

Je Eintrag zwischen `_store` und `--source`:

| `_store` | Quelle | erzeugter Eintrag | gelieferter Eintrag |
|---|---|---|---|
| fehlt | fehlt | erzeugen (`generated`) | `missing`; mit ausstehendem Erzeuger (`Entry.PendingOn`: `s3_*`) `pending` |
| fehlt | vorhanden | übernehmen (`adopted`) | übernehmen (`adopted`) |
| vorhanden | fehlt | behalten (`kept`) | behalten (`kept`) |
| vorhanden | gleich | behalten (`kept`) | behalten (`kept`) |
| vorhanden | anders | Konflikt, Exitcode 1; mit `--adopt-source` `updated` | ersetzen (`updated`) |

Ein Verzeichniseintrag gilt als gleich, wenn dieselben Dateien mit demselben Inhalt vorliegen.
Beim Ersetzen entfernt `init` zuerst Dateien, die in der Quelle fehlen, und Verzeichnisse, an
deren Stelle jetzt eine Datei steht, und schreibt danach; so folgt `_store` auch einem Pfad, der
in der Quelle von einer Datei zu einem Verzeichnis wird oder umgekehrt.

Vor dem ersten Schreiben prüft `init` alles: Optionen, Quelle, Besitzer, Konflikte (alle auf
einmal genannt) und Zufall. Schlägt eine Prüfung fehl, wird nichts geschrieben, auch `<target>`
nicht angelegt.

- Die Quelle darf nur Verzeichnisse und reguläre Dateien mit Katalogeintrag enthalten. Ein Link,
  ein Gerät oder eine Pipe bricht mit Exitcode 2 ab (`init` läuft als root über ein Verzeichnis
  des Owners und folgt keinem Link), ebenso jede unbekannte Datei: Auch `.DS_Store` und ähnliche
  Dateien, die ein Dateimanager anlegt, brechen mit Exitcode 2 ab. Eine Datei eines Datei- oder
  Mustereintrags ohne Wert (leer oder nur ein Zeilenende) bricht ebenso ab; unter einem
  Verzeichniseintrag (`offsite/`) darf eine Datei leer sein. Ein leerer Wert in `_store` bricht
  mit Exitcode 1 ab.
- Ein Lauf hält von vor dem Lesen von `_store` bis zum letzten Schreiben eine exklusive Sperre
  (`flock`) auf `<target>`. Findet ein zweiter Lauf, etwa `docker compose run secrets-init`
  während `up`, sie belegt, bricht er mit Exitcode 1 ab (`another secrets init is running on
  <target>`), bevor er etwas liest oder schreibt: Er würde sonst die temporären Dateien des ersten
  Laufs als Reste entfernen und könnte ein eben erzeugtes Geheimnis ersetzen. Auf Systemen ohne
  `flock` bricht `init` ab.
- Geschrieben wird atomar: temporäre Datei `.bothy-tmp-<zufall>` im selben Verzeichnis, schreiben,
  `fsync`, `chmod`, `chown`, umbenennen, `fsync` des Verzeichnisses. Solche Dateien im Ziel sind
  Reste eines abgebrochenen Laufs und werden entfernt; in der Quelle sind sie unbekannte Dateien.
- Eine Kopie wird nur geschrieben, wenn sie fehlt oder Inhalt, Modus oder Besitzer abweichen; ein
  abweichender Modus oder Besitzer in `_store` wird repariert. Ein zweiter Lauf ohne Änderung
  schreibt, benennt und ändert nichts, auch keine Änderungszeit.
- Gelesen wird jede Datei ohne Blockieren und nur, wenn die geöffnete Datei dieselbe ist wie beim
  Durchsuchen; ein in der Zwischenzeit getauschter Link oder eine Pipe wird abgewiesen.

## Besitzer und Modi

| Dienst | Verzeichnisse | Dateien | Besitzer |
|---|---|---|---|
| mit `--owner <dienst>=<uid>:<gid>` | `0500` | `0400` | `uid:gid` |
| ohne `--owner` | `0755` | `0444` | root, ohne root der laufende Benutzer |

`server` und `migrate` brauchen `--owner` (Exitcode 2), weil der Datei-Treiber nur eigene Dateien
mit `0400` oder `0600` liest (SEC-SEC-1); der Compose-Stack übergibt `65532:65532`. Ein
unbekannter Dienst oder `_store` als Dienst ist Exitcode 2. Ohne root muss jeder Besitzer dem
laufenden Benutzer gleichen (sonst Exitcode 1, nichts geschrieben); so laufen die Tests ohne root.
Ein Verzeichnis, das sich ändert, wird dafür auf `0700` geöffnet und danach auf seinen Modus
gesetzt. Auf Systemen ohne Unix-Besitzer bricht `init` ab, statt offene Kopien zu schreiben.

## Ausgabe und Fingerabdruck

Eine Zeile je Eintrag in Katalogreihenfolge: `<status> <name> <fingerabdruck>`, für `missing` und
`pending` ohne Fingerabdruck; die letzte Zeile (`Report.Reminder`) ist der Export-Befehl mit dem
`_store` des Laufs, etwa
`back up the secrets: bothy secrets export --from /run/bothy/secrets/_store --age-recipient <age public key>`
(ein Pfad, der kein einfaches Shell-Wort ist, in einfachen Anführungszeichen). Der Fingerabdruck
ist `sha256:` und die ersten 12 Hex-Zeichen des SHA-256 des Inhalts in `_store`; für einen
Verzeichniseintrag des Textes aus den Zeilen `<pfad im eintrag> <hex-sha256 der datei>\n`, nach
Pfad sortiert. Der Fingerabdruck reicht zum Vergleich zweier Hosts; wie jeder ungesalzene Hash
bestätigt er aber einen erratenen Wert mit wenig Entropie (ein schwaches geliefertes Passwort, eine
Webhook-URL), deshalb gehört der Bericht, den auch das Log des Containers `secrets-init` behält,
nicht an öffentliche Orte. Werte stehen nie in der Ausgabe und nie in Fehlern. Fehler nennen
Pfade; jedes nicht druckbare Zeichen einer Fehlermeldung von `Init` und `Export` erscheint als
Go-Escape-Folge (`\n`), so kann ein Dateiname des Owners unter `offsite/` keine Zeile im Terminal
oder Log vortäuschen.

## Exitcodes

| Code | Fall |
|---|---|
| 0 | erfolgreich, auch mit `missing` und `pending` |
| 1 | Laufzeitfehler, Konflikt eines erzeugten Eintrags, fremder Besitzer ohne root, ein anderer `init` auf demselben Ziel, vorhandene Ausgabedatei beim Export |
| 2 | Bedienfehler (`ErrUsage`): kein Ziel, Quelle kein Verzeichnis, unbekannte oder nicht reguläre Datei in der Quelle, ungültiger oder fehlender Besitzer, beim Export kein oder ungültiger Empfänger, Binärausgabe auf ein Terminal, `--from` kein Verzeichnis |

## Export

- `--from` ist ein `_store`-Verzeichnis. Das Bündel ist ein POSIX-tar (PAX) mit relativen Namen in
  sortierter Reihenfolge, Verzeichnisse mit `/` am Ende und eingeschlossen, Modusbits wie in
  `_store`, uid und gid 0 ohne Namen, Änderungszeit 0. Reste `.bothy-tmp-*` fehlen im Bündel; ein
  Link, Gerät oder eine Pipe bricht ab. Eine Datei ohne Katalogeintrag (etwa beim Export des
  Volume-Stamms statt `_store`) ist ein Bedienfehler, der alle nennt, bevor die Ausgabe entsteht:
  `init --source` würde das Bündel sonst bei der Wiederherstellung ablehnen.
- Verschlüsselt mit `filippo.io/age` an jeden `--age-recipient` (X25519, `age1...`); jeder
  Empfänger kann es allein entschlüsseln. Mit `--armor` in der ASCII-Hülle von age.
- Empfänger werden vor allem anderen geprüft. Ein ungültiger Empfänger wird nur mit seiner
  Position genannt, nie mit seinem Text: Ein versehentlich eingefügter privater Schlüssel
  (`AGE-SECRET-KEY-...`) darf nicht im Terminal oder in einem Log landen.
- `--output -` (Standard) schreibt auf die Standardausgabe und verweigert Binärausgabe auf ein
  Terminal ohne `--armor` (Terminal heißt: Zeichengerät). `--output <datei>` legt die Datei
  exklusiv mit Modus `0600` an, überschreibt nie eine vorhandene (Exitcode 1, Datei unverändert)
  und entfernt sie wieder, wenn der Export scheitert.
- Klartext entsteht nur im Speicher; es gibt keine temporären Dateien.
- The key store of the signing keys (`--signing-key-dir`, default
  `/var/lib/bothy/signing-keys`, `secrets.signing_key_dir`, ADR 0066 rule 5) follows under the
  prefix `signing-keys/` (`KeyStorePrefix`): the directory and every regular file
  `<purpose>-<kid>.pem` with its mode. Leftovers `.bothy-tmp-*` of an interrupted write are
  skipped; a subdirectory, a link or another name stops the export (usage error, exit code 2),
  because a restore must not install a file the server never wrote. A key store named with
  `--signing-key-dir` that does not exist is a usage error; the missing default is skipped with
  one line on stderr (`signing key store <dir> not found; the bundle holds _store only`).

Entschlüsseln und Auspacken zur Wiederherstellung:
`age --decrypt -i <identität> bundle.age | tar -x -C <quelle>`, danach
`secrets init --adopt-source` (Fall B). `init` skips the subtree `signing-keys/` of the source;
its files belong to the volume of `secrets.signing_key_dir` (disaster-recovery.md Fall B).

## Was fehlt

- Die erzeugte Form von
  `s3_access_key` und `s3_secret_key` (M3).

## Abhängigkeiten

Standardbibliothek, `filippo.io/age` und `filippo.io/age/armor` (Register: `platform.secrets` mit
Vendor-Gruppe `age`). Keine anderen Projekt-Pakete, auch nicht `internal/platform/secrets`; die
Fingerabdrücke rechnet das Paket selbst.

## Kapitel

[16-betrieb.md](../../../../../../docs/spec/16-betrieb.md),
[19-sicherheit.md](../../../../../../docs/spec/19-sicherheit.md),
[configuration.md](../../../../../../docs/operations/configuration.md),
[disaster-recovery.md](../../../../../../docs/operations/disaster-recovery.md)
