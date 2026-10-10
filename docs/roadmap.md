# Roadmap

Stand: Planung (Etappe M0). Jede Etappe wird in Produktionsqualität abgeschlossen: alle
Gates grün, Red-Team-Gate mit Urteil JA, Dokumentation nachgezogen. Termine gibt es
bewusst nicht; die Reihenfolge nach M1 legt der [Plan](PLAN.md) fest. Was das Produkt nie
tun wird, steht in [Kapitel 01](spec/01-produkt.md).

| Etappe | Inhalt | Status |
|---|---|---|
| M0 Planung | Spezifikation, Kern-Vertrag, Architekturentscheidungen, Werkzeugkette, Bau-Tickets | in Arbeit |
| M1 Fundament | Werkzeugkette und lokale Gates; Server-Kern mit Identität, Geräten, Freigaben und `authorize`; Bereiche, Kanäle, Threads, Nachrichten; Ereignis-Log und Synchronisation; Audit; Grundlage der Observability; Compose-Installation | geplant |
| Agenten-Plattform | Adapter-Protokoll, Läufe mit Live-Box, Komponenten, Slash-Befehle, Backend-Richtlinie, SDKs, Referenz-Adapter für Hermes Agent und für einen Claude-Code-Runner | geplant |
| Clients | Design-System, Desktop (macOS, Windows), iOS, Android, Offline-Betrieb, Push | geplant |
| Plattformdienste | Dateien, Sprachnachrichten, Suche, Benachrichtigungen, Teilen aus anderen Apps | geplant |
| Admin-Konsole und Statistik | Web-Konsole für Verwaltung und Statistik | geplant |
| Betrieb und Release | Signieren, Auto-Update, Sicherung und Wiederherstellung, Portabilität | geplant |
| Migration | Import aus Discord | geplant |
| Veröffentlichung | Repository öffentlich, Checkliste [public-switch](quality/public-switch.md) | geplant |
| Mehrbenutzer-Entwicklungsumgebung | Abgeschottete Entwicklungsumgebungen je Person ([Kapitel 21](spec/21-dev-isolation.md)) | später |
