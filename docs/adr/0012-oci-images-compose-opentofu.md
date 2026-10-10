---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team F (Betrieb)
consulted: Team A (Architektur), Team H (Qualität)
informed: alle Teams
---

# OCI-Images multi-arch, Compose als Installation, OpenTofu je Ziel

## Context and Problem Statement

Erstes Ziel ist ein Mac; danach Linux-VMs und Clouds (Azure, AWS, Hetzner und andere). Der Server
ist ein Go-Binary mit Rollen (B1), dazu PostgreSQL 18, Objektspeicher, Virenscan, Collector und
optional ein Observability-Stack. Wie wird eine Instanz installiert, sodass derselbe Weg auf allen
Zielen funktioniert und ein Umzug nur Konfiguration ist?

## Decision Drivers

* ein Befehl für eine vollständige Instanz
* keine Mac-Abhängigkeit im Server (B12)
* gleiche Artefakte für amd64 und arm64 (Mac mit Apple Silicon, Cloud)
* Umzug zwischen Zielen ohne Änderung am Code (Portabilitätstest)
* Infrastruktur als Code für Cloud-Ziele

## Considered Options

* OCI-Images für linux/amd64 und linux/arm64, Docker Compose als Installation, OpenTofu-Module je
  Ziel, Helm später
* Kubernetes und Helm als erster Weg
* Native Pakete und Systemdienste ohne Container (Homebrew, deb, LaunchAgent, systemd)

## Decision Outcome

Chosen option: "OCI-Images, Compose, OpenTofu je Ziel, Helm später", weil Compose auf einem Mac,
einer VM und in jeder Cloud gleich läuft und OpenTofu die Unterschiede der Ziele außerhalb des
Codes hält.

Kern der Umsetzung:

1. Images `bothy-server` (statisches Go-Binary, ohne Shell, ohne Root) und `bothy-postgres`
   (`postgres:18` plus pgBackRest und `bothy-backupd`), je als Multi-Arch-Manifest
   ([18-release.md](../spec/18-release.md) 7).
2. [infra/compose/compose.yaml](../../infra/compose/compose.yaml) mit Profilen für Rand, Scan, S3
   und Observability; Ports nur auf `127.0.0.1`; PostgreSQL nur im internen Netz.
3. OpenTofu-Module `generic-vm`, `hetzner`, `aws`, `azure` mit gleicher Schnittstelle; die
   Zielmodule rufen `generic-vm` auf. Verwaltete Datenbank und Objektspeicher sind Optionen.
4. OpenTofu statt Terraform: OpenTofu steht unter MPL-2.0 (GitHub-Lizenzangabe des Repositorys
   opentofu/opentofu, abgerufen 2026-10-10) [belegt]; das Repository von Terraform meldet keine
   OSI-Lizenz-Kennung ("NOASSERTION", gleiche Abfrage) [belegt]. Dazu kommt die
   State-Verschlüsselung im Werkzeug.

### Consequences

* Gut, weil ein Betreiber mit Docker-Grundwissen eine Instanz betreiben kann.
* Gut, weil derselbe Stack in CI auf einem frischen Runner als Portabilitätstest läuft.
* Gut, weil Hilfsdienste (ClamAV, SeaweedFS, Collector) eigene Prozesse bleiben (Lizenz- und
  Fehlergrenzen).
* Schlecht, weil Compose keine Hochverfügbarkeit bietet; mehrere `api`-Prozesse über Hosts
  brauchen später Helm oder eine Cloud-Plattform.
* Schlecht, weil auf dem Mac eine VM nötig ist (Docker Desktop oder Colima) und ihr Speicher für
  ClamAV reichen muss.
* Neutral, weil `bothy-postgres` ein eigenes Image ist, das mit dem Server versioniert wird.

### Confirmation

* `docker compose config --quiet` im Gate (alle Profile).
* Portabilitätstest in CI ([16-betrieb.md](../spec/16-betrieb.md) 11.1) auf amd64 und arm64.
* `tofu validate` der Module im Gate.
* Image-Prüfung im Release: Multi-Arch-Manifest, Benutzer ohne Root, keine Shell in
  `bothy-server`.

## Pros and Cons of the Options

### OCI-Images, Compose, OpenTofu

* Gut, weil ein Weg für alle Ziele, auch den Mac.
* Gut, weil Compose-Dateien lesbar und prüfbar sind.
* Schlecht, weil Compose-Funktionen (`subpath`, `configs.content`) eine aktuelle Docker-Version
  verlangen.

### Kubernetes und Helm zuerst

* Gut, weil Hochverfügbarkeit und Skalierung eingebaut sind.
* Schlecht, weil ein Mac oder eine kleine VM dafür unverhältnismäßig viel Betrieb braucht.
* Schlecht, weil das Erstziel (ein Mac) damit am schlechtesten bedient ist.

### Native Pakete ohne Container

* Gut, weil kein Container-Laufzeit nötig ist und Leistung ohne VM.
* Schlecht, weil jedes Ziel eigene Pakete, Dienste und Pfade braucht; der Mac würde zum
  Sonderfall im Code (Widerspruch zu B12).
* Schlecht, weil ClamAV, PostgreSQL und Collector dann vom Betriebssystem abhängen.

## More Information

* Betrieb: [16-betrieb.md](../spec/16-betrieb.md); Module: [infra/tofu/README.md](../../infra/tofu/README.md);
  Helm: [infra/helm/README.md](../../infra/helm/README.md).
* OpenTofu verschlüsselt State- und Plan-Dateien
  (https://opentofu.org/docs/language/state/encryption/, abgerufen 2026-10-10).
