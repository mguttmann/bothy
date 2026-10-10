# 21 Mehrbenutzer-Dev-Umgebung und Abschottung

Status: Entwurf M0. Spätere Etappe (Baseline Abschnitt 6: "Mehrbenutzer-Dev-Umgebung (Abschottung)
als spätere Etappe"). Bis diese Etappe abgeschlossen ist, gilt: Dev-Projekte lösen nur Owner aus
(07, SR2 und SR3). Dieses Kapitel legt fest, was vor einer Freischaltung für weitere Personen gelten
muss, und ist schon jetzt normativ für die Schutzregel und den Datensatz `dev_environments`.
Entscheidung: ADR 0046. Rechte: [07-rechte.md](07-rechte.md). Agenten und Runner:
[09-agenten.md](09-agenten.md). Backend-Richtlinie: [10-backend-richtlinie.md](10-backend-richtlinie.md).
Bedrohung: T-DEV-01 in [../security/threat-model.md](../security/threat-model.md).

## 1. Ausgangslage

Ein Dev-Projekt ist ein Forum mit Projektbindung; Bediener ist ein Runner-Agent (Baseline Abschnitt
3). Der Referenz-Runner startet einen Coding-Agenten (z. B. Claude Code im Modus `-p` mit
stream-json) je Thread mit eigener Sitzung (Baseline R-Claude-Code-Runner). Typischer Betrieb heute,
allgemein beschrieben:

- Der Runner läuft als **ein** Benutzer des Hosts, derselbe Benutzer, dem die Geheimnisse des
  Betreibers gehören.
- Der Coding-Agent läuft **ohne Werkzeug-Rückfragen** (bei Claude Code der Schalter, der alle
  Berechtigungsabfragen überspringt), damit Läufe ohne Menschen am Terminal durchlaufen.
- Es gibt **keine Sandbox**: Shell, Dateisystem und Netz stehen dem Agenten mit den Rechten dieses
  Benutzers offen.
- Die Umgebung des Prozesses ist zwar eine Erlaubnisliste, enthält aber z. B. den SSH-Agenten des
  Benutzers und das Home-Verzeichnis, in dem die Anmeldung des Coding-Agenten liegt.
- Die einzige Grenze ist: Nur der Owner löst aus.

## 2. Warum das für weitere Personen unsicher ist

1. **Auslösen heißt Shell.** Wer einen Lauf auslösen kann, kann den Agenten bitten, beliebige
   Befehle als Host-Benutzer auszuführen. Das ist Fernzugriff auf den Host, gewollt gebaut.
2. **Prompt-Injection reicht.** Auch ohne böse Absicht der Person: Ein Repository, eine Abhängigkeit,
   eine Issue-Beschreibung oder eine Webseite kann Anweisungen enthalten, die der Agent ausführt
   (T-AI-03).
3. **Geheimnisse des Betreibers liegen offen.** Konfigurationsdateien, SSH-Schlüssel, Cloud-Zugänge,
   Anmeldungen anderer Werkzeuge und die Anmeldung des Coding-Agenten selbst.
4. **Keine Trennung zwischen Personen.** Alle Projekte liegen im selben Dateisystem; eine Person
   kann die Arbeit einer anderen lesen und ändern.
5. **Abo-Bedingungen.** Läuft der Agent auf dem persönlichen Abo des Owners, wäre ein Lauf einer
   anderen Person Nutzung des Abos durch Dritte (Baseline I1, R-Anthropic).
6. **Netz ohne Grenze.** Interne Dienste, das private Netz und Cloud-Metadaten sind erreichbar.

## 3. Zielbild

| Nr | Anforderung |
|---|---|
| Z1 | Je Person eine eigene Umgebung mit **Hypervisor-Grenze** (VM). Ein eigener Benutzer im selben Betriebssystem reicht nicht (Abschnitt 4). |
| Z2 | Eigener LLM-Zugang je Person: entweder die eigene Anmeldung der Person im unveränderten Coding-Agenten (Anthropic erlaubt das ausdrücklich, auch wenn eine Plattform den Agenten betreibt; https://code.claude.com/docs/en/legal-and-compliance, abgerufen 2026-10-10 [belegt]) oder ein Organisations-API-Schlüssel aus einem eigenen Workspace mit Monatsgrenze (Spend Limits je Workspace: https://platform.claude.com/docs/en/manage-claude/workspaces, abgerufen 2026-10-10 [belegt]). |
| Z3 | Eigener Git-Zugang je Person mit minimalen Rechten: nur freigegebene Repositories, keine Verwaltung, kein Schreiben auf geschützte Branches, keine Workflow-Rechte. |
| Z4 | Keine Geheimnisse des Owners in der Umgebung: kein geteiltes Home, kein weitergereichter SSH-Agent, keine Cloud-Zugänge, keine Anmeldung des Owners. |
| Z5 | Netz-Egress nur zu einer Erlaubnisliste: LLM-Anbieter, Git-Host, Paketquellen. Kein Zugriff auf Host, privates Netz, Overlay-Netz, Cloud-Metadaten, andere Umgebungen, den Server direkt. |
| Z6 | Ein Arbeitsordner (Clone) je Person und Projekt in ihrer Umgebung; Austausch nur über den Git-Host. |
| Z7 | Grenzen für CPU, Speicher, Platte und Prozesse je Umgebung. |
| Z8 | Zurücksetzen auf ein sauberes Abbild jederzeit; Abbilder versioniert. |
| Z9 | Protokoll der Egress-Entscheidungen (nur Metadaten) und der Läufe je Umgebung. |
| Z10 | Ein Lauf einer Person läuft nur in ihrer Umgebung; ein Thread bleibt an die Umgebung gebunden, in der er begann (07, SR3). |

## 4. Optionen

| Kriterium | O1 Gemeinsamer Runner mit Werkzeug-Rückfragen | O2 Eigener OS-Benutzer auf dem Host | O3 macOS-VM je Person | O4 Linux-VM je Person (auch Container mit eigener VM) | O5 Cloud-VM je Person |
|---|---|---|---|---|---|
| Isolationsgrenze | keine | Dateirechte, gemeinsamer Kernel | Hypervisor | Hypervisor | Hypervisor und getrennte Hardware |
| Geheimnisse des Owners erreichbar | ja | bei jeder Fehlkonfiguration, über lokale Dienste und Kernel-Lücken | nein, wenn keine Ordner geteilt | nein, wenn keine Ordner geteilt | nein |
| Egress steuerbar je Person | nein | schwer (Regeln je Benutzer auf dem Host) | ja, an der virtuellen Netzschnittstelle | ja, an der virtuellen Netzschnittstelle | ja, Sicherheitsgruppen des Anbieters |
| Apple-Werkzeuge (Xcode) | ja | ja | ja | nein | nur mit Mac-Instanzen des Anbieters |
| Grenzen auf einem Mac | | | Apple begrenzt laufende macOS-VMs je Mac; Virtualization.framework verweigert die dritte macOS-VM (https://eclecticlight.co/2022/08/04/virtualisation-on-apple-silicon-macs-8-how-apple-limits-vms/, abgerufen 2026-10-10 [belegt]; aktueller Lizenztext nicht geprüft [vermutet]) | keine bekannte feste Grenze | Kosten je Stunde |
| Werkzeuge | | | UTM (Apache-2.0, nutzt GPL-Bestandteile; startet macOS-Gäste über Virtualization.framework; https://github.com/utmapp/UTM [belegt]); Tart (Repository mit Thema "fair-source", https://github.com/openai/tart [belegt]; genaue Lizenzbedingungen vor Einsatz prüfen [vermutet]) | auf macOS 26 mit Apple Silicon: Apple Containerization startet jeden Linux-Container in einer eigenen leichten VM, Apache-2.0 (https://github.com/apple/containerization, abgerufen 2026-10-10 [belegt]); auf Linux-Hosts KVM-basierte VMs | Cloud-Anbieter |
| Zurücksetzen | nein | aufwendig | Abbild klonen | Abbild klonen | Abbild neu starten |
| Aufwand | gering | mittel | mittel bis hoch | mittel | mittel, plus Kosten und Datenstandort |
| Bewertung | abgelehnt: Werkzeug-Rückfragen werden in langen Läufen abgeschaltet oder blind bestätigt; keine Grenze gegen Injektion | abgelehnt als alleinige Grenze | zulässig, wenn Apple-Werkzeuge nötig sind | **Standard** | zulässig, gleichwertig zu O4 |

## 5. Entscheidung

1. Weitere Personen bekommen Dev-Läufe nur in einer Umgebung mit Hypervisor-Grenze (O3, O4, O5).
   O1 und O2 sind ausgeschlossen (ADR 0046).
2. Standard ist O4: Linux-VM je Person, auf einem Host, der nicht der Server-Host ist, oder als O5
   in der Cloud. Grund: Die meisten Projekte brauchen keine Apple-Werkzeuge; Linux-VMs sind ohne
   Mengengrenze und mit Standardwerkzeugen für Netzregeln betreibbar.
3. O3 nur für Projekte, die Apple-Werkzeuge brauchen, und nur so viele Umgebungen, wie die
   Plattform zulässt.
4. Wo die Umgebungen laufen (eigener Rechner, Firmen-Hardware, Cloud), entscheidet der Owner
   (Kosten, Datenstandort). Die Entscheidung steht im Ops-Bereich des Betreibers, nicht im Produkt.
5. LLM-Zugang in Umgebungen anderer Personen in dieser Etappe nur `organization` (Organisations-
   Schlüssel aus einem Workspace mit Monatsgrenze je Person). Grund: I1 ist je Agent definiert. Ein
   Runner bedient ein Forum, auf dem jeder Owner `trigger` hat; ein Runner im Modus `personal` mit
   einer anderen Person als `credential_holder` verletzt I1 immer. Eine Backend-Richtlinie je
   Dev-Umgebung (die Person nutzt ihr eigenes Abo, nur sie löst in ihrer Umgebung aus, SR3 sichert
   das) ist als Ergänzung beantragt (Abschnitt 11) und braucht eine Entscheidung zu I1.

## 6. Datensatz `dev_environments` und Bereitschaftsprüfung (Kern-Vertrag: Ergänzung beantragt)

### 6.1 Tabelle

| Feld | Typ | Null | Standard | Regeln / Hinweise |
|---|---|---|---|---|
| `id` | `uuid` | nein | | UUIDv7 |
| `user_id` | `uuid` | nein | | Person, der die Umgebung gehört |
| `runner_agent_id` | `uuid` | nein | | Runner, der die Umgebung betreibt |
| `isolation_kind` | `text` | nein | | `CHECK (isolation_kind IN ('macos_vm','linux_vm','cloud_vm'))` |
| `environment_ref` | `text` | nein | | undurchsichtiger Verweis, den der Runner auflöst; kein Pfad und kein Hostname des Servers |
| `image_ref` | `text` | nein | | Version des sauberen Abbilds |
| `billing_mode` | `text` | nein | `'organization'` | `CHECK (billing_mode IN ('organization'))` in dieser Etappe; Erweiterung beantragt |
| `state` | `text` | nein | `'provisioning'` | `CHECK (state IN ('provisioning','verifying','ready','suspended','retired'))` |
| `checks` | `jsonb` | nein | `'{}'` | Schema `dev_environment_checks.v1`: Ergebnis und Zeit je Prüfung D1 bis D10 |
| `verified_at` | `timestamptz` | ja | | letzte vollständig bestandene Prüfung |
| `red_team_passed_at` | `timestamptz` | ja | | Red Team für Art und Abbild bestanden |
| `red_team_ref` | `text` | ja | | Verweis auf den Bericht |
| `approved_by`, `approved_at` | `uuid`, `timestamptz` | ja | | Owner mit Step-up |
| `suspended_reason` | `text` | ja | | z. B. `check_failed`, `user_disabled`, `manual` |
| `created_at` | `timestamptz` | nein | `now()` | |
| `retired_at` | `timestamptz` | ja | | |
| `version` | `bigint` | nein | `1` | |

PK: `id`. FK: `user_id` -> `users(principal_id)`, `runner_agent_id` -> `agents(principal_id)`,
`approved_by` -> `principals(id)`, alle ON DELETE RESTRICT. Unique: Teilindex `(user_id,
runner_agent_id) WHERE state <> 'retired'`. Aufbewahrung: `security`. Modul: `agents`.

Zusätzlich beantragt: `agent_sessions.dev_environment_id` und `runs.dev_environment_id` (FK auf diese
Tabelle, NULL bei Owner-Läufen in der Owner-Umgebung), Claim `dev_environment_id` in der
Aufruf-Marke, Aktionen `dev_environment.read` (Owner oder die Person selbst),
`dev_environment.manage` (Owner, Step-up, Audit, Rechte `ja`: anlegen, freigeben, sperren,
zurückziehen) und `dev_environment.report` (Runner, `agent-self`: Prüfergebnisse melden).

### 6.2 Bereit

Eine Umgebung ist bereit genau dann, wenn alles gilt:

- `state = 'ready'`,
- `verified_at` jünger als `dev.reverify_interval`,
- `red_team_passed_at` gesetzt und das Red Team galt für dieselbe `isolation_kind` und denselben
  `image_ref`,
- `approved_at` gesetzt,
- die Person ist aktiv und hat `trigger` auf dem Kanal (07).

SR3 prüft das bei Annahme und vor jeder Zustellung eines Aufrufs (07, Abschnitt 5).

### 6.3 Prüfungen

Der Runner führt sie beim Anlegen, nach jedem Zurücksetzen, nach jedem Abbild-Wechsel und
spätestens nach `dev.reverify_interval` aus und meldet sie mit `dev_environment.report`.

| Nr | Prüfung | Bestanden, wenn |
|---|---|---|
| D1 | Isolationsart | Runner bestätigt aus seiner Bereitstellung eine VM der gemeldeten Art (nicht aus der VM heraus gemeldet) |
| D2 | Kein Host-Zugang | im Gast kein Mount des Host-Home, kein geteilter Ordner außer dem Arbeitsordner, Gast-Benutzer ist nicht der Host-Benutzer |
| D3 | Netz gesperrt | aus dem Gast sind Host, eine Probe aus jedem privaten Bereich, Shared Address Space, Cloud-Metadaten, der Server direkt und eine andere Umgebung nicht erreichbar |
| D4 | Netz erlaubt | LLM-Anbieter, Git-Host und Paketquellen der Erlaubnisliste erreichbar |
| D5 | Keine fremden Geheimnisse | keine bekannten Variablen für LLM-, Cloud- oder Agenten-Zugänge außer dem Soll; kein SSH-Agent; Secret-Scan des Gast-Home ohne Fund |
| D6 | LLM-Modus gleich Soll | Ist-Modus des Coding-Agenten aus der Laufzeit (bei Claude Code `apiKeySource` im Init-Ereignis, Baseline R-Claude-Code-Runner) entspricht `billing_mode`; im Modus `organization` sind OAuth-Variablen entfernt |
| D7 | Git-Zugang begrenzt | Klonen eines nicht freigegebenen Test-Repositorys scheitert; Push auf einen geschützten Branch scheitert |
| D8 | Grenzen gesetzt | CPU, Speicher, Platte, Prozesse nach `dev.limits` |
| D9 | Zurücksetzen | sauberes Abbild vorhanden und Zurücksetzen zuletzt erfolgreich |
| D10 | Protokoll | Egress-Protokoll aktiv |

Scheitert eine Prüfung, setzt der Server `state = 'suspended'` mit `suspended_reason =
'check_failed'`; laufende Läufe in dieser Umgebung bekommen `run.cancel_requested`. Zurück zu `ready`
nur über eine neue, vollständig bestandene Prüfung.

### 6.4 Zustände

```mermaid
stateDiagram-v2
    [*] --> provisioning
    provisioning --> verifying: Umgebung angelegt
    verifying --> ready: D1 bis D10 bestanden, Red Team, Owner-Freigabe
    verifying --> suspended: Prüfung gescheitert
    ready --> verifying: Zurücksetzen, Abbild-Wechsel, Fristablauf
    ready --> suspended: Prüfung gescheitert, Person gesperrt, Owner
    suspended --> verifying: Owner veranlasst neue Prüfung
    ready --> retired: Owner zieht zurück
    suspended --> retired: Owner zieht zurück
    retired --> [*]
```

## 7. Ablauf eines Laufs

```mermaid
flowchart LR
    P["Person mit trigger"] -->|"Nachricht im Projekt-Forum"| S["Server: agent.invoke<br/>SR2, SR3, I2"]
    S -->|"Aufruf-Marke mit<br/>dev_environment_id, tool_class"| R["Runner auf eigenem Host"]
    R -->|"prüft Marke, wählt Umgebung"| V["VM der Person"]
    V -->|"Egress-Erlaubnisliste"| L["LLM-Anbieter, Git-Host, Paketquellen"]
    V -.->|"gesperrt"| X["Host, privates Netz, Server, andere VMs"]
    R -->|"Live-Box-Patches, Ergebnis"| S
```

1. Der Server prüft `agent.invoke` mit SR2 und SR3 und schreibt `dev_environment_id` in Lauf und
   Marke.
2. Der Runner prüft die Marke (I3) und startet nur in genau dieser Umgebung. Fehlt sie oder ist sie
   nicht bereit, meldet er `failed` mit Grund `dev_environment_not_ready`.
3. Der Coding-Agent läuft im Arbeitsordner der Person in der VM. Werkzeug-Rückfragen dürfen dort
   abgeschaltet sein; die Grenze ist die VM.
4. Dateien, die der Agent in den Chat lädt, kommen nur aus dem Arbeitsordner und dem Anhang-Ordner
   der Umgebung; nie Dateien mit Geheimnis-Mustern.
5. Stopp beendet die Prozessgruppe in der VM. Der Runner hält die Steuerverbindung zur VM, nicht der
   Server.

## 8. Tests und Red Team vor Freischaltung

### 8.1 Automatische Tests

| ID | Test | Erwartet |
|---|---|---|
| DEV-T01 | Nicht-Owner löst ohne Umgebung aus | `dev_environment_not_ready` (07, AZ-M15) |
| DEV-T02 | Nicht-Owner löst in Thread einer anderen Person aus | `dev_environment_foreign_thread` (AZ-M17) |
| DEV-T03 | Umgebung `ready`, aber `verified_at` älter als `dev.reverify_interval` | abgelehnt |
| DEV-T04 | Prüfung D3 scheitert bei laufendem Lauf | Umgebung `suspended`, Lauf `cancel_requested` |
| DEV-T05 | Marke mit fremder `dev_environment_id` an den Runner | kein Start |
| DEV-T06 | `user.disable` der Person | Umgebung `suspended` |
| DEV-T07 | Runner meldet `isolation_kind` außerhalb der erlaubten Werte | abgelehnt |

### 8.2 Red Team

Vor der ersten Freischaltung jeder Kombination aus `isolation_kind` und `image_ref` prüft ein Red
Team (Stufe P3 in [19-sicherheit.md](19-sicherheit.md), Abschnitt 13) mit diesen Aufgaben, jeweils
als Auftrag an den Coding-Agenten in der Umgebung und von Hand:

1. Dateien des Host-Benutzers lesen oder schreiben.
2. Dienste des Hosts, des privaten Netzes, des Overlay-Netzes und Cloud-Metadaten erreichen.
3. Den Server direkt erreichen, am Gateway vorbei.
4. Die Umgebung oder den Arbeitsordner einer anderen Person lesen.
5. Das API-Token des Runners oder andere Geheimnisse finden.
6. Daten über DNS oder erlaubte Ziele abfließen lassen (Messung, was die Erlaubnisliste zulässt).
7. Ressourcen erschöpfen (Fork-Bombe, Platte, Speicher) und den Host beeinträchtigen.
8. Etwas über ein Zurücksetzen hinaus erhalten.
9. Mit dem Git-Zugang auf nicht freigegebene Repositories oder geschützte Branches schreiben.
10. Über eine präparierte Datei im Repository den Agenten zu 1 bis 9 bringen (Injektion).

Bestanden heißt: 1 bis 5, 8 und 9 unmöglich; 6 und 7 begrenzt und protokolliert, mit dokumentiertem
Restrisiko. Ergebnis und Bericht setzen `red_team_passed_at` und `red_team_ref`; der Owner gibt
danach mit Step-up frei. Jeder Abbild-Wechsel, der Netz, Mounts oder Benutzer ändert, verlangt ein
neues Red Team.

## 9. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-DV-01 | Hypervisor-Grenze Pflicht; OS-Benutzer allein reicht nicht. | Gemeinsamer Kernel, lokale Dienste und Fehlkonfigurationen öffnen den Host. |
| R-DV-02 | Linux-VM je Person als Standard, macOS-VM nur bei Bedarf, Cloud-VM gleichwertig. | Kosten, Mengengrenzen und Werkzeuge. |
| R-DV-03 | Bereitschaft ist ein Datensatz mit Prüfungen, Red Team und Owner-Freigabe; SR3 prüft ihn bei jedem Aufruf. | Bereitschaft ändert sich; nur eine Prüfung beim Aufruf ist immer aktuell. |
| R-DV-04 | Threads in Dev-Projekten sind an die Umgebung ihres ersten Laufs gebunden. | Eine Sitzung arbeitet auf den Dateien ihrer Umgebung. |
| R-DV-05 | Fremde Umgebungen in dieser Etappe nur mit Organisations-Schlüssel. | I1 ist je Agent definiert; ein persönlicher Modus je Umgebung braucht eine Änderung an I1. |

## 10. Offene Punkte

- Ort der Umgebungen (eigener Rechner, Firmen-Hardware, Cloud): Owner.
- Persönliches Abo in der eigenen Umgebung: braucht die beantragte Erweiterung von I1.
- Gemeinsame Sitzungen zweier Menschen in einem Thread: bewusst ausgeschlossen (Studie C).

## 11. Ergänzungen am Kern-Vertrag (beantragt)

| Nr | Ergänzung |
|---|---|
| K-DV-1 | Tabelle `dev_environments` (Abschnitt 6.1), Spalten `agent_sessions.dev_environment_id`, `runs.dev_environment_id`, Claim `dev_environment_id`, Aktionen `dev_environment.read`, `dev_environment.manage`, `dev_environment.report` (gleich K-RE-4 in 07). |
| K-DV-2 | Backend-Richtlinie je Dev-Umgebung: `dev_environments.billing_mode IN ('organization','personal')`, bei `personal` ist der `credential_holder` die Person der Umgebung; I1 für Runner-Agenten je Umgebung statt je Agent, gesichert durch SR3. Entscheidung durch die Leitung bzw. den Owner nötig. |

## 12. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `dev.reverify_interval` | `24h` | spätestens dann wird eine Umgebung neu geprüft |
| `dev.limits` | `{"cpus": 4, "memory_mib": 8192, "disk_gib": 50, "pids": 2048}` | Grenzen je Umgebung |
| `dev.egress_allowlist` | leer | erlaubte Ziele (Hosts) für Umgebungen; ohne Eintrag kein Egress |
