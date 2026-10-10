# apps/client

Status: geplant (M0). Noch kein Code.

Flutter-App für iOS, Android, macOS und Windows. Eine Codebasis für Oberfläche und Logik; nativer
Code nur, wo das System es verlangt.

## Zweck

- Bereiche, Kanäle, Foren und Threads; Agenten-Sitzungen mit Live-Box, Nachträgen, Komponenten und
  Slash-Befehlen ([01-produkt.md](../../docs/spec/01-produkt.md)).
- Offline lesen und schreiben mit Ausgangsfach; Nachholen nach [04-sync-protokoll.md](../../docs/spec/04-sync-protokoll.md).
- Push ohne Inhalt, Text holt die App selbst; Teilen aus anderen Apps; Sprachnachrichten.
- Verwaltung auf Desktop (aus `packages/admin_ui`), Notfall-Aktionen auf dem Telefon.

## Aufbau (geplant)

| Pfad | Inhalt |
|---|---|
| `lib/` | App-Hülle, Router (`go_router`), Layouts je Formfaktor (Desktop vier Spalten, Mobil Stapel), Chat-Funktionen |
| `ios/` | Runner, Notification Service Extension, Share Extension, `ClientKit` (Swift Package, geteilt mit macOS) |
| `macos/` | Runner, Notification Service Extension, Share Extension, Sparkle 2 |
| `android/` | Activity, eigener FCM-Dienst mit WorkManager, Teilen-Activity |
| `windows/` | Runner, Infobereich, MSIX-Manifest mit Share Target, Toast-Aktivierung |
| `integration_test/` | Tests gegen den Fake-Server; Geräte-E2E mit `patrol` |

## Plattformen

| Ziel | Mindestversion | Verteilung |
|---|---|---|
| iOS | 15 | TestFlight, App Store |
| Android | API 26 | Play Internal Testing oder Firebase App Distribution |
| macOS | 12 | Developer ID, Notarisierung, Sparkle 2 (stable, beta) |
| Windows | 10 Version 2004 | MSIX mit `.appinstaller` |

Gründe und Belege: [15-client-architektur.md](../../docs/spec/15-client-architektur.md), Abschnitt 2.
Release und Signieren: [18-release.md](../../docs/spec/18-release.md).

## Bauen (geplant)

Ein Make-Ziel je Plattform aus dem Repo-Wurzelverzeichnis (Qualitäts-Team). Die
Build-Konfiguration der Instanz (`branding.yaml`, ADR 0044) liefert App-Name, IDs, Push-Themen,
Update-Feeds und Signier-Identitäten; ohne sie entsteht eine neutrale Fassung mit Platzhaltern unter
`example.com`.
