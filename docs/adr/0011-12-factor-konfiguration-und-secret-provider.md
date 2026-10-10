---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team F (Betrieb)
consulted: Team A (Architektur), Team C (Sicherheit)
informed: alle Teams
---

# 12-Factor-Konfiguration und Secret-Provider-Schnittstelle

## Context and Problem Statement

Bothy läuft auf einem Mac mit Compose, auf Linux-VMs und in Clouds (B12). Jedes Ziel liefert
Konfiguration und Geheimnisse anders: Dateien, Umgebungsvariablen, Azure Key Vault, AWS Secrets
Manager. Das Datenmodell speichert fremde Zugangsdaten nur als Verweis `secret_ref`
([03-datenmodell.md](../spec/03-datenmodell.md) 1.9). Wie kommt der Server zu Konfiguration und
Geheimnissen, ohne dass Geheimnisse in Images, Logs oder Telemetrie landen und ohne dass ein Ziel
den Code ändert?

## Decision Drivers

* gleiche Images auf allen Zielen, Unterschiede nur in Konfiguration (B12)
* Geheimnisse nie in Umgebungsvariablen, Logs, Fehlermeldungen oder OpenTofu-State
* Rotation von Geheimnissen ohne Neuinstallation
* Tippfehler dürfen nie still einen unsicheren Standard setzen
* Betrieb ohne zusätzlichen Pflichtdienst auf einem Einzelhost

## Considered Options

* Umgebung plus optionale Datei; Geheimnisse nur als Verweis über eine Secret-Provider-Schnittstelle
* Nur Umgebungsvariablen, Geheimnisse direkt als Werte
* Konfiguration in der Datenbank, verwaltet nur über die Admin-Konsole
* Externer Secret-Store als Pflicht (z. B. ein Vault-Dienst)

## Decision Outcome

Chosen option: "Umgebung plus optionale Datei; Geheimnisse nur als Verweis über eine
Secret-Provider-Schnittstelle", weil sie auf jedem Ziel ohne weiteren Dienst funktioniert und
Geheimnisse trotzdem aus Umgebung und Logs heraushält.

Regeln (Einzelheiten [16-betrieb.md](../spec/16-betrieb.md) 5):

1. Prozess-Konfiguration aus `BOTHY_*`-Umgebungsvariablen und optional einer YAML-Datei; Vorrang
   Umgebung vor Datei vor Standard. Schlüssel `a.b_c` heißt `BOTHY_A__B_C`.
2. Unbekannte Schlüssel und ungültige Werte brechen den Start ab.
3. Geheimnisse sind Verweise `<schema>:<pfad>`. Treiber in M1: `file`, `env`; später `azkv`,
   `awssm`. Schnittstelle `Get` und optional `Put`; Lesecache `secrets.cache_ttl`.
4. Instanz-Einstellungen und Feature-Flags bleiben in der Datenbank (Admin-Konsole, Step-up,
   Audit); sie sind keine Prozess-Konfiguration.
5. Im Compose-Stack erzeugt `secrets-init` fehlende Geheimnisse und legt je Dienst ein
   Unterverzeichnis an; jeder Dienst sieht nur seines.

### Consequences

* Gut, weil ein Ziel nur Konfiguration liefert und der Code gleich bleibt.
* Gut, weil Geheimnisse als Dateien je Dienst weder in `docker inspect` noch in Kindprozessen
  sichtbar sind.
* Gut, weil Rotation über den Cache ohne Neustart wirkt.
* Schlecht, weil `secrets-init` ein eigener Baustein mit Rechten zum Schreiben ist.
* Schlecht, weil die Konfigurationsreferenz bis M1 von Hand mit den Kapiteln gleich gehalten wird.
* Neutral, weil Plattformen ohne Dateien (manche PaaS) `env:` nutzen können, mit den bekannten
  Nachteilen.

### Confirmation

* Unit-Test: unbekannter `BOTHY_*`-Schlüssel und Typfehler brechen den Start ab.
* Kanarien-Test ([privacy.md](../observability/privacy.md) 6) enthält Geheimnisse als Geheimwort;
  sie dürfen in keinem Log und keiner Telemetrie stehen.
* Generator der Konfigurationsreferenz aus dem Schema; ein Check vergleicht Schema, Referenz und
  Kapiteltabellen.
* `docker compose config` im Gate; Prüfung, dass kein Dienst Geheimnisse als Umgebungswert setzt.

## Pros and Cons of the Options

### Umgebung plus Datei, Geheimnisse als Verweis

* Gut, weil es 12-Factor folgt und Geheimnisse trotzdem als Dateien oder in Stores liegen.
* Gut, weil der Verweis im Datenmodell (`secret_ref`) und in der Konfiguration gleich aussieht.
* Schlecht, weil zwei Wege (Wert und Verweis) zu erklären sind.

### Nur Umgebungsvariablen mit Werten

* Gut, weil am einfachsten und überall verfügbar.
* Schlecht, weil Werte in Kindprozesse, Absturzberichte und `docker inspect` gelangen.
* Schlecht, weil Rotation einen Neustart braucht.

### Konfiguration in der Datenbank

* Gut, weil die Admin-Konsole alles ändern könnte, mit Audit.
* Schlecht, weil der Server die Datenbank-Zugangsdaten vor der Datenbank braucht (Henne und Ei).
* Schlecht, weil ein Konsolen-Fehler den Betrieb lahmlegen kann.

### Externer Secret-Store als Pflicht

* Gut, weil Rotation, Audit und Zugriffsrechte im Store liegen.
* Schlecht, weil ein Einzelhost dann einen weiteren hochverfügbaren Dienst betreiben müsste.
* Neutral, weil Stores als spätere Treiber trotzdem möglich bleiben.

## More Information

* Konfigurationsreferenz: [docs/operations/configuration.md](../operations/configuration.md).
* Hinterlegung der Geheimnisse und Folgen bei Verlust: [16-betrieb.md](../spec/16-betrieb.md) 5.4.
* Folge-ADR vorgeschlagen: Treiber für Azure Key Vault und AWS Secrets Manager, sobald ein
  Cloud-Ziel ihn braucht.
