---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team F (Betrieb)
consulted: Team H (Qualität und Repo), Team D (Agenten-Plattform), Team E (Apps)
informed: alle Teams
---

# Releases: SemVer je Komponente, signierte Artefakte, SBOM, Provenance

## Context and Problem Statement

Das Monorepo enthält Server, Apps, SDKs (Python, TypeScript) und Adapter. Sie ändern sich in
verschiedenem Takt und haben verschiedene Nutzer: Betreiber, App-Herausgeber, Entwickler von
Integrationen. Betreiber müssen Herkunft und Inhalt jedes Artefakts prüfen können. Wie wird
versioniert, getaggt, signiert und dokumentiert?

## Decision Drivers

* unabhängige Releases ohne künstliche Versionssprünge
* Tags müssen je Komponente eindeutig und für Werkzeuge auflösbar sein
* prüfbare Herkunft ohne langlebige Signierschlüssel
* SBOM für Lizenz- und Schwachstellenprüfung beim Betreiber
* Release Notes, die sagen, ob ein Upgrade Handarbeit braucht

## Considered Options

* SemVer je Komponente mit `VERSION` und `CHANGELOG.md` je Komponente, Tags `<komponente>/vX.Y.Z`;
  cosign keyless, SBOM (Syft), Build-Provenance (`actions/attest`)
* Eine gemeinsame Version für das ganze Monorepo (Gleichschritt)
* Signatur mit langlebigem Schlüssel (GPG oder cosign-Schlüsselpaar)
* Keine Signatur, nur Prüfsummen

## Decision Outcome

Chosen option: "SemVer je Komponente; cosign keyless, SBOM, Provenance", weil Komponenten
unabhängig veröffentlicht werden und die Herkunft ohne Schlüsselverwaltung prüfbar ist.

1. Tags `<komponente>/vX.Y.Z` (`server/vX.Y.Z`, `client/vX.Y.Z`, `sdk-python/vX.Y.Z` usw.), das
   Installationsbündel `vX.Y.Z`; Namen und Präfixe aus `tools/ci/components.py`
   ([18-release.md](../spec/18-release.md) 2). Die Tag-Regel von Go für Module in
   Unterverzeichnissen (https://go.dev/ref/mod, abgerufen 2026-10-10) gilt für das Server-Modul
   nicht, weil es als Hauptmodul nie geladen wird (PLAN P-05).
2. Kanäle stable und beta; Apps befördern denselben Build, Server und Pakete eine Vorabversion
   aus demselben Commit ([18-release.md](../spec/18-release.md) 5).
3. Jedes Artefakt: SBOM (SPDX und CycloneDX), cosign-Signatur (keyless; solange das Repository
   privat ist, ein Schlüsselpaar ohne Transparenz-Log nach ADR 0062), Prüfsummen; Attestation über
   `actions/attest`, sobald das Repository öffentlich ist (in Free, Pro und Team nur für
   öffentliche Repositorys verfügbar; https://github.com/actions/attest-build-provenance,
   abgerufen 2026-10-10).
4. Kein Tag `latest`; Installationen pinnen Versionen.

### Consequences

* Gut, weil ein SDK-Fehler kein Server-Release erzwingt und umgekehrt.
* Gut, weil Betreiber mit `cosign verify` und `gh attestation verify` die Herkunft prüfen.
* Schlecht, weil mehrere Changelogs und Tags zu pflegen sind (Werkzeug erzeugt Entwürfe).
* Schlecht, weil bis zur Veröffentlichung des Repositorys keine Attestation entsteht.
* Neutral, weil der Go-Modulpfad des Servers bei einem Major-Sprung gleich bleibt, solange das
  Modul nie als Abhängigkeit geladen wird (PLAN P-05).

### Confirmation

* Gate-Check: `VERSION` gleich Tag; Changelog-Abschnitt vorhanden.
* Release-Kette bricht ab, wenn Signatur, SBOM oder Prüfsummen fehlen.
* Portabilitätstest zieht Images per Digest und prüft vorher die Signatur.

## Pros and Cons of the Options

### SemVer je Komponente

* Gut, weil Versionen etwas über die jeweilige Komponente aussagen.
* Schlecht, weil die Kompatibilität zwischen Komponenten eigene Regeln braucht (Protokollversion,
  Mindest-App-Version).

### Gemeinsame Version

* Gut, weil eine Zahl für alles.
* Schlecht, weil jede Änderung an einer Komponente alle anderen neu veröffentlicht.
* Schlecht, weil SemVer-Aussagen (bricht ja oder nein) für die meisten Komponenten falsch würden.

### Langlebiger Signierschlüssel

* Gut, weil unabhängig von CI und öffentlicher Infrastruktur.
* Schlecht, weil der Schlüssel geschützt, rotiert und bei Verlust widerrufen werden muss.

### Nur Prüfsummen

* Gut, weil einfach.
* Schlecht, weil eine Prüfsumme nur Übertragungsfehler zeigt, keine Herkunft.

## More Information

* Ablauf: [docs/operations/release.md](../operations/release.md).
* Werkzeug für Changelog-Entwürfe: git-cliff hinter `make release-prep`
  ([ADR 0060](0060-release-automatisierung-im-monorepo.md)).
* Signaturen in der privaten Phase: [ADR 0062](0062-release-signaturen-solange-das-repo-privat-ist.md).
