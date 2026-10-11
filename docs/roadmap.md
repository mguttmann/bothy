# Roadmap

Bothy ist eine Kommandozentrale für KI-Agenten in Chat-Form: Chat, Live-Ansicht, Freigaben,
Terminal, Kosten und Statistik, selbst gehostet und quelloffen.

Stand: Planung (Etappe M0). Jede Etappe wird in Produktionsqualität abgeschlossen: alle
Gates grün, Red-Team-Gate mit Urteil JA, Dokumentation nachgezogen. Termine gibt es
bewusst nicht; die Reihenfolge nach M1 legt der [Plan](PLAN.md) fest. Was das Produkt nie
tun wird, steht in [Kapitel 01](spec/01-produkt.md).

| Etappe | Inhalt | Status |
|---|---|---|
| M0 Planung | Spezifikation, Kern-Vertrag, Architekturentscheidungen, Werkzeugkette, Bau-Tickets | in Arbeit |
| M1 Fundament | Werkzeugkette und lokale Gates; Server-Kern mit Identität, Geräten, Freigaben und `authorize`; Bereiche, Kanäle, Threads, Nachrichten; Ereignis-Log und Synchronisation; Audit; Grundlage der Observability; Server-Image mit Rauchtest | geplant |
| M2 Agenten-Plattform und Terminal | Adapter-Protokoll, Läufe mit Live-Box, Komponenten, Slash-Befehle, Backend-Richtlinie mit Kosten, Apps und Webhooks, SDKs, Referenz-Adapter für Hermes Agent und für einen Claude-Code-Runner; Terminal mit Host-Dienst für macOS, Linux und Windows, Ende-zu-Ende-verschlüsseltem Strom, eigenem Recht je Rechner und Invariante I6 ([Kapitel 24](spec/24-terminal.md)) | geplant |
| M3 Plattformdienste | Dateien mit Malware-Scan, Bilder, Sprachnachrichten, Suche, Benachrichtigungen und Push | geplant |
| M4 Clients Desktop | Design-System, Desktop-App für macOS und Windows mit Offline-Betrieb, Verwaltung und Terminal | geplant |
| M5 Clients Mobil | iOS und Android mit Push, Teilen aus anderen Apps, Sprachaufnahme und Terminal mit Sondertasten-Leiste | geplant |
| M6 Konsole, Statistik und Visualisierung | Web-Konsole für Verwaltung, Statistik und Terminal; Live-Topologie, Backend-Ansicht, Zustellung live, Executive-Seite, Timeline und TV-Modus aus eigener Messung ([Kapitel 25](spec/25-visualisierung.md)); die Pakete der Visualisierung entstehen als Spur schon nach M1 | geplant |
| M7 Betrieb, Release und Schaufenster | Sicherung und Wiederherstellung mit Test, Datenschutz-Abläufe, Statusseite, Signieren, Auto-Update, Portabilität, Grafana-Panel-Plugin "Bothy Live", Demo-Instanz, Bilder und Video | geplant |
| M8 Migration | Import aus Discord, Umschalten Kanal für Kanal | geplant |
| V1 Veröffentlichung | Repository öffentlich, Checkliste [public-switch](quality/public-switch.md) | geplant |
| V2 Mehrbenutzer-Entwicklungsumgebung | Abgeschottete Entwicklungsumgebungen je Person mit eigenem Terminal ([Kapitel 21](spec/21-dev-isolation.md)) | später |
| V3 Ausbau | Funktionen, die [Kapitel 01](spec/01-produkt.md) als "später" führt | später |
