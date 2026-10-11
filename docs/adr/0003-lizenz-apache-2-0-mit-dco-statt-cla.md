---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Lizenz Apache-2.0 mit DCO statt CLA

## Context and Problem Statement

Das Produkt wird quelloffen. Betreiber sollen es frei einsetzen, anpassen und in eigene
Infrastruktur einbetten können, auch kommerziell. Beiträge von außen brauchen einen
klaren Herkunftsnachweis, ohne Beitragende mit einem Vertrag abzuschrecken.

## Decision Drivers

* Freie Nutzung durch Organisationen jeder Größe, auch mit eigenen Erweiterungen.
* Ausdrückliche Patentlizenz der Beitragenden.
* Herkunftsnachweis je Beitrag, maschinell prüfbar.
* Geringe Hürde für Beitragende, kein Vertrag mit einer Firma.

## Considered Options

* Apache-2.0 mit Developer Certificate of Origin (DCO)
* Apache-2.0 mit Contributor License Agreement (CLA)
* MIT mit DCO
* AGPL-3.0
* MPL-2.0

## Decision Outcome

Gewählt: "Apache-2.0 mit DCO", weil Apache-2.0 eine ausdrückliche Patentlizenz enthält
und erlaubend ist, und weil das DCO je Commit einen prüfbaren Herkunftsnachweis gibt,
ohne dass Beitragende einen Vertrag unterschreiben. Beiträge stehen unter derselben
Lizenz wie das Projekt (inbound = outbound, Apache-2.0 Abschnitt 5).

### Consequences

* Gut, weil Betreiber und Integratoren keine Copyleft-Pflichten für eigene Erweiterungen
  haben; Erweiterungen laufen ohnehin außerhalb des Server-Prozesses (Planungsgrundlage B6).
* Gut, weil jeder Commit `Signed-off-by` trägt und das maschinell geprüft wird.
* Schlecht, weil ein späterer Lizenzwechsel die Zustimmung aller Beitragenden bräuchte;
  das ist gewollt.
* Abhängigkeiten müssen mit Apache-2.0 vereinbar sein; das Lizenz-Gate prüft das
  (docs/quality/licenses.md).

### Confirmation

* `tools/ci/license_check.py` (Gate `licenses`): `LICENSE` ist der unveränderte
  Apache-2.0-Text (sha256), jede handgeschriebene Quelldatei trägt
  `SPDX-License-Identifier: Apache-2.0`.
* `tools/hooks/commit_msg.py --require-signoff`: commit-msg-Hook und Gate `commitlint`
  in CI für jeden Commit eines Pull Requests.

## Pros and Cons of the Options

### Apache-2.0 mit DCO

* Gut, weil Patentlizenz und Patent-Rückfall-Klausel enthalten sind.
* Gut, weil das DCO keine Unterschrift und keine Firma als Vertragspartner braucht.
* Neutral, weil `NOTICE`-Pflichten bei Weitergabe gelten.

### Apache-2.0 mit CLA

* Gut, weil der Inhaber später umlizenzieren könnte.
* Schlecht, weil ein CLA Beitragende abschreckt und einen Prozess zum Unterschreiben braucht.

### MIT mit DCO

* Gut, weil sehr kurz und bekannt.
* Schlecht, weil keine ausdrückliche Patentlizenz.

### AGPL-3.0

* Gut, weil Änderungen an gehosteten Instanzen offengelegt werden müssten.
* Schlecht, weil viele Organisationen AGPL-Software nicht einsetzen dürfen; das
  widerspricht dem Ziel freier Nutzung.

### MPL-2.0

* Gut, weil Datei-Copyleft mit Patentlizenz.
* Schlecht, weil Pflichten je Datei Betreiber mit eigenen Anpassungen belasten.

## More Information

Apache License 2.0: https://www.apache.org/licenses/LICENSE-2.0 (abgerufen 2026-10-10)
[belegt]. Developer Certificate of Origin 1.1: https://developercertificate.org/
(abgerufen 2026-10-10) [belegt]. Ablauf für Beitragende: `CONTRIBUTING.md`.
