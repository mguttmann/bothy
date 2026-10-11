# Produktname

Status: Entwurf M0. **Keine Rechtsberatung.** Diese Seite fasst eine Websuche zusammen. Sie ersetzt
keine Markenrecherche in den Registern (DPMA, EUIPO, TMview, USPTO) für die Nizza-Klassen 9
(Software) und 42 (Software als Dienst) durch eine Fachperson. Die endgültige Wahl trifft der
Owner (OPEN-QUESTIONS).

## Ergebnis

- **Arbeitsname: Bothy.** Kein Chat- oder KI-Produkt dieses Namens gefunden; einzige Treffer in
  Markenregistern liegen außerhalb von Software.
- **Namensgleiche Repositories:** Auf GitHub gibt es kleine Projekte namens `bothy`, darunter ein
  Terminal-Arbeitsplatz und eine Konsole für einen einzelnen Rechner, beide mit wenigen Sternen
  (https://github.com/search?q=bothy&type=repositories, GitHub-Suche abgerufen 2026-10-11)
  [belegt]. Keines ist ein Team-Chat; der Terminal-Arbeitsplatz liegt aber nahe am Terminal des
  Produkts. Folge: Das Repository und der Go-Modulpfad brauchen einen eindeutigen Besitzer-Pfad,
  und die Registerrecherche (unten) bezieht diese Treffer ein [abgeleitet].
- **Alternativen: Corvo, Gilde.**
- Der Name ist Konfiguration (`branding.product_name`, ADR 0044). Im Code steht er nur an wenigen,
  gelisteten Stellen (Go-Modulpfad, Paketnamen, Binary `bothy`), damit ein Wechsel nach der
  Markenprüfung billig bleibt.

## Kriterien

1. Keine Kollision mit einem Produkt im Feld Chat, Zusammenarbeit oder KI-Agenten.
2. Keine bekannte Marke, kein Firmenname, kein Personenname, kein belasteter Begriff.
3. In Deutsch und Englisch sprechbar und schreibbar.
4. Ein eigenes Bild, das zum Produkt passt.
5. Kurz genug für App-Icon-Beschriftungen.

## Kandidaten

Alle Belege per Websuche, abgerufen 2026-10-10. "Risiko" ist eine Einschätzung [abgeleitet].

| Name | Befund | Risiko | Ergebnis |
|---|---|---|---|
| **Bothy** | Keine Chat-App "Bothy" in Web und App Store gefunden [belegt, Websuche]. Kleine namensgleiche GitHub-Projekte außerhalb von Chat (siehe Ergebnis) [belegt]. Markentreffer nur außerhalb von Software: "BOTHY BLENDS" für Spirituosen, UK, Klasse 33 (https://www.ipo.gov.uk/t-tmj/tm-journals/2026-007/UK00004321045.html) und "BOTHOY" für Hardware, US, Klasse 9 (https://trademark.justia.com/885/15/bothoy-88515994.html) [belegt]. Bedeutung: offene Schutzhütte für alle (https://en.wikipedia.org/wiki/Bothy) [belegt]. | niedrig | Arbeitsname |
| **Corvo** | Keine Chat- oder KI-App gefunden [belegt, Websuche]. Name der Hauptfigur einer Spielreihe (https://en.wikipedia.org/wiki/Corvo_Attano) [belegt]; klangähnlich "Korvo", ein lokaler KI-Arbeitsplatz (https://www.everydev.ai/tools/korvo) [belegt]. | niedrig bis mittel | Alternative 1 |
| **Gilde** | Softwareanbieter "Gilde Software" in den Niederlanden (https://www.g2.com/sellers/gilde) [belegt]; die englische Entsprechung nutzt eine Plattform für KI-Agenten (https://docs.guild.ai/) [belegt]. Sofort deutsch verständlich. | mittel | Alternative 2 |
| Plenum | bekanntes Open-Source-Repository gleichen Namens (https://github.com/hyperledger-indy/indy-plenum) und eine CMS-Oberfläche "Plenum UI" (https://agilitycms.com/resources/posts/what-is-plenum-ui) [belegt]; ein IT-Beratungsunternehmen gleichen Namens laut Suchzusammenfassung [vermutet] | mittel | nachrangig |
| Heinzel | gefördertes Forschungsprojekt "Heinzel AI" im Feld KI-Helfer (https://www.uni-bonn.de/en/news/188-2025) [belegt] | mittel bis hoch | verworfen |
| Kobold | verbreitete Open-Source-KI-Chat-Werkzeuge gleichen Namens (https://github.com/LostRuins/koboldcpp) und eine seit Jahrzehnten geführte Produktmarke eines Haushaltsgeräte-Herstellers (https://www.vorwerk.com/de/de/c/home/allgemein/presse/pressemitteilungen-kobold/90-jahre-schmutz-bekaempfer) [belegt] | hoch | verworfen |
| Tandem | Team-Büro-App gleichen Namens (https://apps.apple.com/app/id1521413406) und große Sprach-App gleichen Namens (https://apps.apple.com/app/tandem-learn-languages/id959001619) [belegt] | hoch | verworfen |
| Rudel | im deutschen Sprachraum mit einer historisch belasteten Person verbunden [belegt, Enzyklopädie-Eintrag; Verweis bewusst nicht aufgenommen] | hoch (Ruf) | verworfen |

Domains: Alle geprüften kurzen `.dev`- und `.app`-Domains der acht Wörter waren im DNS registriert
(DNS-Abfrage `dig NS`, 2026-10-10) [belegt]; ein freies NXDOMAIN heißt nicht sicher frei. Plan: mit
Zusatz arbeiten (zum Beispiel ein Wort plus "chat" oder "app"), Kaufbarkeit erst beim Registrar
prüfen [abgeleitet].

Im Feld "Team-Chat mit KI-Agenten als Mitglieder" gibt es bereits Anbieter, die mit "agentic team
chat" werben (https://glue.ai/blog/how-agentic-team-chat-is-changing-the-way-we-work) [belegt]; der
Name sollte sich davon klar abheben [abgeleitet].

## Warum Bothy

- **Bild:** eine einfache Hütte, offen für alle, in der man unterkommt und arbeitet. Das Produkt ist
  der Ort, an dem Menschen und Agenten zusammen arbeiten, selbst gehostet wie eine eigene Hütte.
- **"bot" steckt im Wort**, ohne dass der Name nach Technik klingt.
- **Design:** Die Palette (Fichte, Ginster, Moos, Vogelbeere, Granit) und das Zeichen (ein Giebel)
  folgen dem Bild; siehe [README.md](README.md).
- **Nachteil:** Das "th" ist für deutsche Sprecher ungewohnt; viele werden "Botti" sagen
  [vermutet, kein Aussprachetest gemacht].

## Nächste Schritte (Owner)

1. Registerrecherche Klasse 9 und 42 für Bothy, Corvo, Gilde durch eine Fachperson.
2. Domain mit Zusatz wählen und beim Registrar prüfen.
3. Aussprachetest mit einigen deutschsprachigen Personen.
4. Entscheidung in DECISIONS.md eintragen; bei Wechsel nur `branding.*`, die gelisteten
   Code-Stellen und die Build-Konfiguration der Apps ändern (ADR 0044).
