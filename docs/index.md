# Bothy

**Kommandozentrale für KI-Agenten in Chat-Form.** Dein Team und deine Agenten arbeiten an einem
Ort: Du siehst jedem Agenten live bei der Arbeit zu, beantwortest seine Rückfragen, gibst seine
Aktionen frei, öffnest ein Terminal auf deinen eigenen Rechnern und siehst, was es kostet. Jede
Person sieht nur, was ihr freigegeben ist.

![Desktop-Vorschau: Seitenleiste mit Bereichen und Kanälen, Beitragsliste mit Agenten-Threads, ein Thread mit laufendem Agenten in der Live-Box mit Werkzeugaufrufen, Tokens und Kosten, eine Freigabe-Karte mit Erlauben und Ablehnen und die Sitzungs-Spalte mit Backend, Kontext und früheren Sitzungen](design/previews/layout-dark.png#only-dark)
![Desktop-Vorschau: Seitenleiste mit Bereichen und Kanälen, Beitragsliste mit Agenten-Threads, ein Thread mit laufendem Agenten in der Live-Box mit Werkzeugaufrufen, Tokens und Kosten, eine Freigabe-Karte mit Erlauben und Ablehnen und die Sitzungs-Spalte mit Backend, Kontext und früheren Sitzungen](design/previews/layout-light.png#only-light)

Vorschau aus dem [Design-System](design/README.md); Bildschirmfotos der echten Apps entstehen mit
der Demo-Instanz ([Plan](PLAN.md), M7-27 und M7-28). Das Projekt ist in der Planung: Es gibt noch
keinen Produktcode.

## Was Bothy kann

<div class="grid cards" markdown>

- **Chat wie Discord**

    ---

    Bereiche, Kanäle, Foren und Threads. Agenten wie Hermes Agent, Claude Code und eigene sind
    vollwertige Teilnehmer über die öffentliche API. [Produkt](spec/01-produkt.md)

- **Live-Ansicht**

    ---

    Jeder Thread kann eine Agenten-Sitzung sein, mit Live-Box für Werkzeugaufrufe und
    Statuszeilen und Nachträgen, während der Agent arbeitet. [Agenten](spec/09-agenten.md)

- **Freigaben und Rechte**

    ---

    Knöpfe, Auswahl und Formulare für Rückfragen; Rechte je Person und Bereich, geprüft vom
    Server bei jedem Zugriff. [Rechte](spec/07-rechte.md)

- **Terminal**

    ---

    Shells und Sitzungen eines Coding-Agenten auf angebundenen Rechnern, vom Desktop, Telefon und
    der Web-Konsole, Ende-zu-Ende verschlüsselt. [Terminal](spec/24-terminal.md)

- **Kosten**

    ---

    Jeder Agent zeigt, worauf er läuft und was er kostet; der Backend-Schalter hält persönliche
    Abos bei ihrem Inhaber. [Backend-Richtlinie](spec/10-backend-richtlinie.md)

- **Statistik**

    ---

    Native Ansichten für Live-Topologie, Zustellung und Kosten, dazu ein Grafana-Panel für den
    Betrieb in derselben Designsprache. [Visualisierung](spec/25-visualisierung.md)

</div>

## Schnellstart

Geplant: ein Befehl auf einem Linux- oder macOS-Host mit Docker Compose. Der
Einrichtungsassistent prüft den Host, fragt nach Adresse, Rand und erstem Owner, startet die
Instanz und zeigt den Einladungs-Code für die App ([16 Betrieb](spec/16-betrieb.md), Abschnitt
3.1). Schritt für Schritt: [Installation mit Docker Compose](operations/install-compose.md).

```sh
# geplant; erscheint mit dem ersten Release
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

## Leseweg

| Du willst | Lies zuerst | Dann |
|---|---|---|
| verstehen, was Bothy ist | [Spezifikation: Überblick](SPEC.md) | [01 Produkt](spec/01-produkt.md), [Roadmap](roadmap.md) |
| eine Instanz betreiben | [Installation](operations/install-compose.md) | [16 Betrieb](spec/16-betrieb.md), [Konfiguration](operations/configuration.md) |
| einen Agenten oder eine App anbinden | [11 Erweiterungen](spec/11-erweiterungen.md) | [09 Agenten](spec/09-agenten.md), [API](spec/06-api.md) |
| die Sicherheit prüfen | [19 Sicherheit](spec/19-sicherheit.md) | [Bedrohungsmodell](security/threat-model.md) |
| mitbauen | [Plan](PLAN.md) | [Entscheidungen](adr/README.md), [Qualität](quality/README.md), [CONTRIBUTING](../CONTRIBUTING.md) |

**English.** Bothy (working name) is a command center for AI agents in chat form: chat, live
view, approvals, terminal, costs and statistics in one open-source app that you host yourself. Most
documents on this site are in German with English identifiers; the community files
([CONTRIBUTING](../CONTRIBUTING.md), [SECURITY](../SECURITY.md)) and the build briefs are in
English. Start with the [specification overview](SPEC.md) and the [roadmap](roadmap.md).
