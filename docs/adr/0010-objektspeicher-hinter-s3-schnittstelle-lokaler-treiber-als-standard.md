---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team G Plattformdienste
---

# Objektspeicher hinter S3-Schnittstelle, lokaler Treiber als Standard

## Context and Problem Statement

Bothy speichert hochgeladene Dateien, Vorschaubilder und Sprachnachrichten. Das Erstziel ist ein
einzelner Mac mit Container-Laufzeit; spätere Ziele sind Linux-VMs und Clouds (Baseline B4, B12).
Wo liegen die Bytes, und wie bleibt der Server zwischen diesen Zielen portabel, ohne auf einem
kleinen Ziel einen zusätzlichen Speicherdienst zu erzwingen?

## Decision Drivers

- Portabilität zwischen Mac, Linux-VM und Cloud ohne Codeänderung.
- Kleine Installation ohne zusätzlichen Dienst.
- Jeder Abruf durch `authorize` (B7); Bytes nie ohne Prüfung.
- Kein Anbieterzwang; nur breit unterstützte Operationen.
- Einfache Sicherung und Wiederherstellung zusammen mit PostgreSQL.
- Lizenz- und Pflegelage der selbst gehosteten S3-Dienste (MinIO archiviert).

## Considered Options

- Port `ObjectStore` mit Dateisystem-Treiber (Standard) und S3-Treiber, der nur Kern-S3 nutzt
- Nur S3, lokal mit einem selbst gehosteten S3-Dienst
- Dateien in PostgreSQL (`bytea` oder Large Objects)
- Nur Dateisystem

## Decision Outcome

Gewählte Option: "Port `ObjectStore` mit Dateisystem-Treiber (Standard) und S3-Treiber, der nur
Kern-S3 nutzt", weil sie das kleine Ziel ohne weiteren Dienst bedient und in der Cloud jeden
S3-kompatiblen Speicher nutzt. Kern-S3 heißt: PUT, GET (mit Bereich), HEAD, DELETE einzelner
Objekte, Multipart-Upload und presigned URLs für Upload-Teile. Kein Auflisten, kein Kopieren,
kein Batch-Löschen, keine Versionierung, keine Bucket-Richtlinien. Details:
[12-dateien.md](../spec/12-dateien.md) Abschnitt 4.

### Consequences

- Gut, weil eine Installation auf einem Rechner nur Server, PostgreSQL und ClamAV braucht.
- Gut, weil der Umzug in eine Cloud nur Konfiguration und das Kopieren der Objekte ist; das
  Schlüsselschema ist in beiden Treibern gleich.
- Gut, weil der S3-Treiber mit wenigen Operationen auch mit Diensten außerhalb von AWS
  zusammenarbeitet (z. B. SeaweedFS).
- Gut, weil der Server jeden Schlüssel aus der Datenbank kennt und nie auflisten muss.
- Schlecht, weil zwei Treiber getestet werden müssen. Eine gemeinsame Konformitäts-Testreihe
  fängt das ab.
- Schlecht, weil der Dateisystem-Treiber nur mit einem `api`-Knoten oder gemeinsamem Volume
  skaliert. Mehrere Knoten brauchen den S3-Treiber.
- Schlecht, weil Downloads durch den Server laufen (Entscheidung R-FI-03 in 12-dateien); das
  kostet Bandbreite, ist aber für `authorize` zur Abrufzeit nötig.

### Confirmation

- Eine Konformitäts-Testreihe läuft gegen den Dateisystem-Treiber und gegen SeaweedFS in einem
  Testcontainer (Put, Get mit Bereich, Head, Delete fehlender Objekte, Multipart mit Abbruch).
  The suite is `filestest.RunObjectStoreConformance` in
  `services/server/internal/files/adapters/filestest`; the file system driver runs it in
  `TestFSConformance` (M3-01), the S3 driver follows with M3-02.
- Der S3-Treiber hängt nur von einer Go-Schnittstelle mit genau den Kern-S3-Operationen ab; eine
  Architektur-Regel erlaubt den Import des S3-SDK nur in `internal/files/adapters/s3`.
- Ein Eigenschaftstest prüft, dass kein Objektschlüssel Kanal-IDs, Bereichs-IDs oder Namen
  enthält.
  The property test is `TestObjectKeysCarryNoContext` in
  `services/server/internal/files/domain`.

## Pros and Cons of the Options

### Port mit Dateisystem-Treiber (Standard) und S3-Treiber mit Kern-S3

- Gut, weil kleinstes Betriebsminimum auf einem Rechner.
- Gut, weil Cloud-Speicher ohne Anbieterbindung nutzbar ist.
- Neutral, weil presigned Uploads nur mit S3 gehen; sie sind optional.
- Schlecht, weil zwei Implementierungen zu pflegen sind.

### Nur S3, lokal mit einem selbst gehosteten S3-Dienst

- Gut, weil nur ein Treiber existiert.
- Schlecht, weil jede Installation einen weiteren Dienst betreiben und sichern muss.
- Schlecht, weil die naheliegende Wahl MinIO archiviert ist und keine Binärdateien mehr liefert
  [Q1, belegt]; Alternativen wie Garage stehen unter AGPLv3 und haben eingeschränkte S3-Funktionen
  [Q2, belegt].

### Dateien in PostgreSQL

- Gut, weil Sicherung und Transaktion eine Einheit sind.
- Schlecht, weil große Binärdaten die Datenbank, WAL, Sicherungen und Replikation aufblähen.
- Schlecht, weil Bereichs-Abrufe für Audio und Video umständlich sind und Cloud-Objektspeicher
  ungenutzt bleiben.

### Nur Dateisystem

- Gut, weil am einfachsten.
- Schlecht, weil mehrere Knoten und verwaltete Cloud-Speicher ausgeschlossen sind und ein Umzug
  Codeänderung braucht.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 https://github.com/minio/minio [belegt]; Q2
  https://garagehq.deuxfleurs.fr/documentation/reference-manual/s3-compatibility/ und
  https://raw.githubusercontent.com/deuxfleurs-org/garage/main-v2/LICENSE [belegt];
  SeaweedFS https://github.com/seaweedfs/seaweedfs [belegt]; AWS-SDK-Prüfsummen
  https://docs.aws.amazon.com/sdkref/latest/guide/feature-dataintegrity.html [belegt].
- Der S3-Treiber setzt die SDK-Prüfsummen auf `when_required`, weil die Standard-Prüfsummen der
  SDKs mehrere S3-kompatible Dienste brechen; die eigene SHA-256 je Datei ersetzt sie.
- Baseline B4 und Ruling R-Objektspeicher; [12-dateien.md](../spec/12-dateien.md).
