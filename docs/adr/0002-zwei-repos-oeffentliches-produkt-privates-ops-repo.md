---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Zwei Repos: öffentliches Produkt, privates Ops-Repo

## Context and Problem Statement

Das Produkt soll quelloffen und für jede Organisation nutzbar sein. Die erste
Installation gehört aber einer konkreten Organisation mit eigenen Hostnamen, Konten,
Agenten-Zuordnungen, Migrationsdaten und Runbooks. Diese Angaben dürfen nie öffentlich
werden, auch nicht in der Git-Historie. Wo liegt was, und wie verhindern wir, dass
Instanzwissen ins Produkt gelangt?

## Decision Drivers

* White Label: kein Instanzbezug im Produkt, auch nicht in Beispielen, Tests, Commits.
* Die Instanz braucht trotzdem versionierte Konfiguration und Runbooks.
* Eine Grenze, die eine Maschine prüfen kann, nicht nur eine Absprache.
* Das Produkt-Repository soll später ohne Historienbereinigung öffentlich werden können.

## Considered Options

* Zwei Repositories: öffentliches Produkt, privates Ops-Repository
* Ein Repository mit privaten Ordnern, die vor dem Öffentlich-Schalten entfernt werden
* Ein privates Monorepo, aus dem ein öffentlicher Spiegel exportiert wird

## Decision Outcome

Gewählt: "Zwei Repositories", weil nur so die Grenze schon beim Schreiben besteht und
die Produkt-Historie nie Instanzwissen enthält.

Das Produkt-Repository enthält Server, Apps, SDKs, Erweiterungen, Doku, CI und Beispiele
mit neutralen Werten (example.com, Personen Alex, Sam, Robin). Das Ops-Repository enthält
Instanz-Konfiguration, Deployment-Runbooks, Zuordnungen zu Agenten-Profilen,
Migrationsdaten mit echten Kennungen und die private Deny-Liste, mit der das
Produkt-Repository geprüft wird (ADR 0042).

### Consequences

* Gut, weil das Produkt jederzeit öffentlich werden kann; die Prüfung dafür ist
  `make denylist-history`.
* Gut, weil Instanzwissen versioniert bleibt, nur eben privat.
* Schlecht, weil eine Änderung manchmal zwei Pull Requests braucht (Produkt-Funktion und
  Instanz-Konfiguration). Die Konfigurationsreferenz im Produkt beschreibt jeden Schlüssel,
  das Ops-Repository setzt nur Werte.

### Confirmation

Der Deny-List-Scanner (`make denylist`, Teil von `make check` und CI) prüft jeden Push
mit der Liste aus dem Ops-Repository; `make denylist-history` prüft vor dem
Öffentlich-Schalten die ganze Historie (docs/quality/public-switch.md).

## Pros and Cons of the Options

### Zwei Repositories

* Gut, weil Instanzwissen nie in die Produkt-Historie gelangt.
* Gut, weil Zugriffsrechte getrennt sind (das Ops-Repository bleibt privat).
* Schlecht, weil zwei Repositories gepflegt werden.

### Ein Repository mit privaten Ordnern

* Gut, weil alles an einem Ort liegt.
* Schlecht, weil die Historie die privaten Ordner behält; vor dem Öffentlich-Schalten
  wäre ein Umschreiben der ganzen Historie nötig, und jede Lücke ist dann öffentlich.

### Privates Monorepo mit öffentlichem Spiegel

* Gut, weil ein Arbeitsort.
* Schlecht, weil der Export-Filter die einzige Grenze ist und jeder Filterfehler leakt.
* Schlecht, weil Beiträge von außen auf dem Spiegel landen und zurückportiert werden müssen.

## More Information

Grundlage: Planungsgrundlage Abschnitt 0 und Anforderung 22 (White Label).
