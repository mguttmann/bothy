# Umgebungen und Feature-Flags

Status: Entwurf M0. Anleitung zu [16-betrieb.md](../spec/16-betrieb.md) 6.

## 1. Umgebungen

| Umgebung | Wer | Wo | Daten | Images |
|---|---|---|---|---|
| `dev` | Entwicklung | Arbeitsrechner, Compose | synthetisch (Seed-Daten der Tests) | lokal gebaut oder beliebige Version |
| `staging` | Abnahme vor stable | eigener Host oder Cloud, Compose | synthetisch oder Import aus einer Test-Quelle | Release-Kandidat per Digest |
| `prod` | Betrieb | Zielsystem | echt | Release per Digest |

### 1.1 Was sich unterscheiden darf

Nur Konfiguration: `instance.environment`, `server.public_url`, Geheimnisse, Push-Umgebung
(APNs-Sandbox in `dev`), Telemetrie-Overlay, Pläne der Sicherung. Code, Images und Migrationen
sind gleich.

### 1.2 Was `instance.environment` bewirkt

| Wirkung | `dev` | `staging` | `prod` |
|---|---|---|---|
| HTTP ohne TLS auf Loopback für Clients | erlaubt | nein | nein |
| Banner in den Apps | "Dev" | "Staging" | keins |
| `deployment.environment.name` in Telemetrie | `dev` | `staging` | `prod` |
| Warnung ohne Off-site-Ziel | Hinweis | Warnung in der Admin-Konsole | Warnung in der Admin-Konsole bei jeder Anmeldung des Owners |
| Seed-Befehle (`bothy dev seed`) | erlaubt | nein | nein |

### 1.3 Beförderung

1. Beta (`server/vX.Y.Z-beta.N`) nach `staging`; Portabilitätstest und Abnahme.
2. Bewährt: stable (`X.Y.Z`) aus demselben Commit nach `staging`, kurze Prüfung, dann derselbe
   Digest nach `prod` ([18-release.md](../spec/18-release.md) 5).
3. Jede Umgebung hat eigene Geheimnisse, eigene `instance_id`, eigene Push-Schlüssel.
4. Produktionsdaten verlassen `prod` nie. Fehler, die nur mit echten Daten auftreten, untersucht
   der Owner in `prod` mit `access.view_as` und Traces, nicht mit einer Kopie.

## 2. Feature-Flags

Registrierung und Aufräum-Gate: [ADR 0064](../adr/0064-feature-flag-registrierung-mit-aufraeum-gate.md).

### 2.1 Arten

| Art | Zweck | Lebensdauer | Beispiel |
|---|---|---|---|
| `release` | neue Funktion schrittweise freischalten | bis `remove_by` (eine Version im Code) | neue Live-Box-Darstellung |
| `option` | dauerhafter Produktschalter des Owners | dauerhaft, dokumentiert in [configuration.md](configuration.md) 7 | `dm.enabled` |

Flags sind keine Konfiguration: keine Zahlen, keine Grenzen, keine Sicherheitsgrenzen, keine
Geheimnisse. Ein Flag darf eine Funktion nur ein- oder ausschalten.

### 2.2 Registrierung im Code

Jedes Flag ist an einer Stelle im Server registriert mit: Schlüssel, Art, Beschreibung (Englisch,
für die Konsole), Standard, bei `release` die Version `remove_by` und das zuständige Modul.
Unbekannte Schlüssel in der Tabelle `feature_flags` wertet der Server als aus und zeigt sie in
der Konsole als "verwaist".

### 2.3 Auswertung

```
wert = flag.enabled
wenn principal oder eine seiner gültigen Gruppen in flag.rollout steht:
    wert = nicht flag.enabled
```

Der Server wertet aus, nie der Client. Clients bekommen die für sie gültigen Werte per REST
(`instance.read`) und laden neu, wenn `instance.updated` einen geänderten Schlüssel nennt.
Auf Agenten und Apps wirken Flags wie auf Menschen; `rollout` kennt aber nur Menschen und Gruppen.

### 2.4 Admin-Konsole

| Ansicht | Inhalt |
|---|---|
| Liste | Schlüssel, Art, Beschreibung, globaler Wert, Zahl der Ausnahmen, letzte Änderung |
| Detail | Ausnahmen (Personen, Gruppen), Verlauf aus dem Audit-Log |
| Ändern | `feature_flag.update`: Owner, Step-up, Grund, Audit |

### 2.5 Aufräumregel

1. Ein `release`-Flag nennt `remove_by`. Ein Gate-Check schlägt fehl, wenn die Version in
   `VERSION` des Servers `remove_by` erreicht und das Flag noch registriert ist.
2. Wer das Flag entfernt, entfernt Code und Registrierung und löscht die Zeile in derselben
   Änderung per Migration.
3. Ein `release`-Flag wird nie zu `option` umgedeutet; wenn ein Schalter dauerhaft bleiben soll,
   entsteht ein neues `option`-Flag mit eigenem Schlüssel.
