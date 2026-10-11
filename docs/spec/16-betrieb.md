# 16 Betrieb

Status: Entwurf M0. Normativ für Deployment, Konfiguration, Geheimnisse, Sicherung,
Wiederherstellung, Upgrade, Umgebungen, Feature-Flags, Statusseite und Kapazität.
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Sync: [04-sync-protokoll.md](04-sync-protokoll.md). Aktionen: [05-aktionen.md](05-aktionen.md).
Telemetrie: [17-observability.md](17-observability.md). Releases: [18-release.md](18-release.md).
Anleitungen (Runbooks) stehen unter [docs/operations/](../operations/configuration.md).

Alle Grenzen, Fristen und Intervalle sind benannte Konfigurationswerte. Ihre Standardwerte stehen
in Abschnitt 14 und gesammelt in der [Konfigurationsreferenz](../operations/configuration.md).

## 1. Ziele

1. Ein Befehl richtet eine vollständige Instanz auf einem Host mit Docker-kompatibler Laufzeit
   ein und startet sie (Einrichtungsassistent, Abschnitt 3.1).
2. Mac, Linux-VM und Cloud nutzen dieselben Images. Unterschiede stehen nur in der Konfiguration.
3. Datenverlust bleibt unter `dr.rpo_target`, die Wiederherstellung unter `dr.rto_target`. Ein
   automatischer Wiederherstellungstest misst beides regelmäßig.
4. Ein Upgrade braucht keine Handarbeit an Daten. Der Rückweg um eine Version ist immer möglich.
5. Geheimnisse stehen nie in Images, Umgebungsvariablen, Logs, Telemetrie oder OpenTofu-State.

## 2. Bausteine einer Instanz

| Baustein | Image | Rolle |
|---|---|---|
| `api` | `bothy-server` | REST, Gateway, Fan-out (`bothy serve --role=api`, B1) |
| `worker` | `bothy-server` | Hintergrundjobs: Scan, Vorschaubilder, Push, Webhooks, Aufbewahrung, Statusprüfungen (`--role=worker`) |
| `migrate` | `bothy-server` | einmaliger Lauf vor `api` und `worker`: Rollen und Migrationen |
| `secrets-init` | `bothy-server` | einmaliger Lauf: Geheimnisse erzeugen und je Dienst ablegen (Abschnitt 5) |
| `postgres` | `bothy-postgres` | PostgreSQL 18 (B3) plus pgBackRest für die WAL-Archivierung |
| `backup` | `bothy-postgres` | `bothy-backupd`: pgBackRest-Repository, logische Sicherung, Objektsicherung, Wiederherstellungstest |
| `otel-collector` | OpenTelemetry Collector contrib | Telemetrie-Annahme, Datenschutz-Filter, Export (B11) |
| `clamav` | ClamAV | Virenscan der Uploads, eigener Prozess (ADR 0033, Team G) |
| `seaweedfs` | SeaweedFS | optionaler S3-Dienst für Einzelhosts (R-Objektspeicher) |
| `tailscale` | Tailscale | optionaler Rand mit `tailscale serve` (B9) |
| Observability-Stack | Prometheus, Alertmanager, Loki, Tempo, Grafana | optionaler lokaler Standard-Stack (ADR 0034) |

`bothy-postgres` ist das offizielle `postgres:18` plus pgBackRest und `bothy-backupd`. Ein
eigenes Image ist nötig, weil `archive_command` im Datenbank-Container läuft (Abschnitt 8.2).
Beide Bothy-Images entstehen in der Release-Kette aus [18-release.md](18-release.md).

Die Compose-Datei [infra/compose/compose.yaml](../../infra/compose/compose.yaml) ist die
Referenz dieser Bausteine. Profile schalten optionale Teile zu: `direct` oder `tailscale` (Rand),
`scan`, `s3`, `observability`.

![Compose-Stack](../operations/diagrams/deployment-compose.svg)

## 3. Deployment-Ziele

| Ziel | Laufzeit | Datenbank | Dateien | Rand | Geheimnisse |
|---|---|---|---|---|---|
| Mac | Docker Desktop oder Colima, Compose | Container `postgres` | Volume `files` | `direct` mit Reverse-Proxy oder `tailscale` | Treiber `file` |
| Linux-VM | Docker Engine, Compose | Container `postgres` | Volume oder S3-Dienst | Reverse-Proxy, `tailscale` oder privates Netz | `file` |
| Azure | VM mit Compose | Container oder Azure Database for PostgreSQL | Volume oder SeaweedFS | Application Gateway oder privates Netz | `file`, später `azkv` (Azure Key Vault) |
| AWS | EC2 mit Compose | Container oder Amazon RDS for PostgreSQL | Amazon S3 | Load Balancer oder privates Netz | `file`, später `awssm` (AWS Secrets Manager) |
| Hetzner | Cloud-Server mit Compose | Container | Volume oder Hetzner Object Storage | Reverse-Proxy oder `tailscale` | `file` |
| Kubernetes | später, Helm (`infra/helm`) | verwalteter Dienst | S3 | Ingress | Treiber des Clusters |

Regeln:

1. Der Server hat keine Mac-Abhängigkeit (B12). Der Mac ist nur ein Host für Container.
2. Auf Mac und Windows läuft Docker in einer VM. Ihr Speicher muss für alle Dienste reichen.
   ClamAV allein braucht laut Hersteller mindestens 3 GiB, besser 4 GiB
   (https://docs.clamav.net/manual/Installing/Docker.html, abgerufen 2026-10-10) [belegt].
3. Mit einem verwalteten PostgreSQL-Dienst übernimmt der Anbieter WAL-Archivierung und
   Point-in-Time-Recovery. `bothy-backupd` macht dann nur die logische Sicherung und die
   Objektsicherung (Abschnitt 8.1).
4. Azure Blob Storage bietet keine S3-API [vermutet, nicht geprüft]. Der S3-Treiber (B4) passt
   darum auf Azure nur mit SeaweedFS oder einem anderen S3-Dienst. Ein eigener Blob-Treiber ist
   eine Option für Kapitel 12.
5. Hetzner Object Storage ist S3-kompatibel und kennt Versionierung und Object Lock
   (https://docs.hetzner.com/storage/object-storage, abgerufen 2026-10-10) [belegt].
6. Ob die verwalteten Dienste von Azure und AWS PostgreSQL 18 anbieten, ist nicht geprüft
   [vermutet]. Das Modul-Ticket prüft es vor der Implementierung.
7. Mindestversionen für Compose: Docker Engine 26 (Volume-`subpath`) und Compose 2.26 (`configs`
   mit `content`, `depends_on.required`). Belege: Release Notes Docker Engine 26.0
   (https://docs.docker.com/engine/release-notes/26.0/) und Compose-Referenz
   (https://docs.docker.com/reference/compose-file/configs/), abgerufen 2026-10-10 [belegt].

OpenTofu-Module je Ziel beschreibt [infra/tofu/README.md](../../infra/tofu/README.md).

### 3.1 Installation in einem Befehl (Einrichtungsassistent)

Ziel 1 gilt für Fremde ohne Vorwissen. Werkzeug ist das Installationsskript `install.sh`
(POSIX-Shell, Quelle `infra/compose/install.sh`), ein Release-Artefakt der Komponente `bothy`
neben dem Installationsbündel ([18-release.md](18-release.md), Abschnitt 2). Es ist der
Einrichtungsassistent. Der Handweg aus [install-compose.md](../operations/install-compose.md)
bleibt gleichwertig; das Skript tut nichts, was der Handweg nicht auch tut. Der dokumentierte
Befehl nennt die Version des Bündels:

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

Ohne Terminal (CI, Automatisierung) stehen alle Antworten als Optionen hinter `sh -s --`:

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh \
  | sh -s -- --yes --public-url https://chat.example.com --edge direct --scan on --storage s3 \
      --owner-handle alex --owner-name Alex
```

`OWNER` ersetzt V1-03, die Version die Release-Kette. Kein Verweis auf `releases/latest`: Im
Monorepo ist das neueste Release oft eine andere Komponente.

Der Portabilitätstest (11.1) übergibt genau die Optionen nach `sh -s --` aus dem zweiten Block;
nur `--storage` wechselt je Lauf. Wo die Optionen stehen und wie der Test aufgerufen wird, legt
11.1 fest (Schnittstelle von `make portability`). Die Quelle des Skripts ist dort ein Parameter
(`PORTABILITY_INSTALL_SOURCE`), weil GitHub Releases nur Personen mit Leserecht auf das
Repository zeigt ("Anyone with read access to a repository can view and compare releases",
https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases, abgerufen
2026-10-11) [belegt]:

| Wert | Quelle | Aufruf |
|---|---|---|
| `url` (so läuft `portability.yml`) | Release-URL des Tags ohne Anmeldung | der zweite Block, mit `OWNER` und Version des Tags |
| `release-download` | `gh release download <tag>` mit einem Token mit Leserecht: `install.sh` und das Bündel `bothy-compose-<version>.tar.gz` | `sh install.sh --bundle <bündel>` und dieselben Optionen |

Ablauf:

1. **Vorprüfung.** Docker Engine und Compose in der Mindestversion (Abschnitt 3, Regel 7),
   Architektur, Speicher der Docker-VM (Hinweis für das Profil `scan`, Regel 2), Zielordner
   (Standard `./bothy`, Option `--dir`) leer oder eine frühere Installation dieses Skripts. Bei
   einem Fehler bricht es ab, bevor es etwas schreibt.
2. **Bündel holen.** Es lädt das Installationsbündel genau seiner eingebauten Version, prüft dessen
   SHA-256 gegen den eingebauten Wert und entpackt es. Mit `--bundle <datei>` nimmt es statt des
   Downloads diese Datei; die Prüfung der Prüfsumme bleibt gleich. Liegt `cosign` im Pfad, prüft es zusätzlich
   die Signatur (18, Abschnitt 6); sonst nennt es den Prüfbefehl.
3. **Fragen.** Nur mit Terminal (Eingabe über `/dev/tty`, damit der Befehl mit Pipe funktioniert):
   öffentliche Adresse (`BOTHY_PUBLIC_URL`), Rand `direct` oder `tailscale` (dann `TS_AUTHKEY`
   verdeckt), Virenscan (Standard an, wenn der Speicher reicht), Dateiablage `fs` (Standard,
   Volume `files`) oder `s3` (Profil `s3` mit dem Objektspeicher im Stack, setzt
   `BOTHY_STORAGE_DRIVER=s3`), lokaler Observability-Stack (Standard aus), Off-site-Ziel
   (optional), Handle und Anzeigename des ersten Owners. Jede Frage hat eine Option
   (`--public-url`, `--edge direct|tailscale`, `--scan on|off`, `--storage fs|s3`,
   `--observability on|off`, `--offsite`, `--owner-handle`, `--owner-name`) und `--yes` nimmt
   die Standards; fehlt ohne Terminal ein Pflichtwert (öffentliche Adresse, Handle und
   Anzeigename des Owners), bricht es ab, statt zu raten.
4. **Schreiben.** `.env` aus `.env.example` des Bündels mit den Antworten, Modus `0600`;
   `secrets/` mit Modus `0700`. `BOTHY_VERSION` und `BOTHY_REGISTRY` übernimmt es unverändert
   aus dem Bündel. `BOTHY_VERSION` ist die Server-Version, nicht die eigene Version: Produkt und
   Server haben getrennte Versionen ([18-release.md](18-release.md), Abschnitt 1). Beide Werte
   schreibt das Produkt-Release ins Bündel; ein Bündel mit Platzhalter entsteht dort nicht
   ([18-release.md](18-release.md), Abschnitt 2.1). Kein Geheimnis in `.env` außer dem einmaligen
   `TS_AUTHKEY` (R-OP-05). Alle erzeugbaren Geheimnisse erzeugt `secrets-init` beim ersten Start (5.3).
5. **Prüfen und starten.** `docker compose config --quiet`, dann `docker compose up --no-start`
   (legt Netze und Container an, startet nichts). Beim Rand `direct` liest es danach das Gateway
   des Compose-Netzes `default` (Netzname aus `docker compose config --format json`, Feld
   `networks.default.name`; `docker network inspect`, Feld `IPAM.Config[].Gateway`) und schreibt
   es als `<gateway>/32` (IPv6 `/128`) in `BOTHY_TRUSTED_PROXIES` von `.env`. Grund: Im Profil
   `direct` sieht der Server das Gateway als Gegenstelle, nicht den Reverse-Proxy; mit dem
   Standard (nur Loopback) weist er jede Anfrage über den veröffentlichten Port ab (4.3,
   SEC-TLS-1). Dann `docker compose up -d` und `bothy healthcheck` gegen `/readyz` im
   API-Container (wie [install-compose.md](../operations/install-compose.md), Abschnitt 4),
   höchstens `install.ready_timeout` lang. Zuletzt prüft es den Weg von außen:
   `GET /api/v1/instance/discovery` über `127.0.0.1:BOTHY_HTTP_PUBLISH_PORT` (Rand `direct`)
   antwortet mit `200`, sonst Abbruch mit Hinweis auf `BOTHY_TRUSTED_PROXIES`.
6. **Erster Owner.** `bothy setup bootstrap-owner` mit Handle und Anzeigename
   ([08-identitaet.md](08-identitaet.md), Abschnitt 4). Das Skript zeigt den Einladungs-Code einmal
   an und schreibt ihn in keine Datei und kein Protokoll.
7. **Abschluss.** Nächste Schritte mit Befehl: App öffnen, Server-Adresse und Einladungs-Code
   eingeben ([01-produkt.md](01-produkt.md), Abschnitt 5.6, E1 und E3); Geheimnisse hinterlegen
   (5.4); bei leerem Off-site-Ziel die Warnung "nur lokale Sicherung". Beim Rand `tailscale` leert
   es `TS_AUTHKEY` in `.env`, sobald die erste Anmeldung gelungen ist.

Regeln:

1. Ein zweiter Lauf im selben Ordner überschreibt weder `.env` noch `secrets/`. Einzige Ausnahme:
   Steht `BOTHY_TRUSTED_PROXIES` beim Rand `direct` noch auf dem Standard (nur Loopback), setzt
   er das Gateway wie in Schritt 5; einen anderen Wert lässt er stehen. Er startet den Stack und
   zeigt den Zustand; mit eingerichtetem Owner entfällt Schritt 6 (der Befehl bricht dann ohnehin
   ohne Änderung ab, 08 Abschnitt 4).
2. Das Skript ruft nie `sudo` auf und installiert keine Pakete, auch nicht Docker; die Vorprüfung
   nennt den Weg. Grund: Ein Skript aus dem Netz soll nichts außerhalb seines Ordners ändern.
3. Hosts: Linux und macOS. Unter Windows läuft es in WSL 2 mit Docker Desktop [abgeleitet: dort
   gibt es eine POSIX-Shell und dieselbe Docker-CLI; das Bau-Ticket belegt es].
4. Jeder Abbruch endet mit Exitcode ungleich `0` und nennt Schritt und Grund auf der
   Standardfehlerausgabe.
5. Belegt durch den Portabilitätstest (11.1): Er installiert auf dem frischen Runner ohne Terminal
   mit genau den dokumentierten Optionen (zweiter Block oben, `--yes` und Optionen; nur
   `--storage` wechselt je Lauf). Die Quelle des Skripts ist ein Parameter (Tabelle oben); mit
   `release-download` kommt nur `--bundle` hinzu. ShellCheck prüft das Skript im Gate.

## 4. Netz

### 4.1 Grundregeln

1. Jeder veröffentlichte Port bindet an `127.0.0.1` des Hosts. Im Compose-Stack liefert ein Rand
   (Reverse-Proxy, Load Balancer, `tailscale serve`) TLS und öffentliche Erreichbarkeit. Ohne Rand
   kann der Server TLS selbst beenden (`server.tls_cert_ref`, `server.tls_key_ref`;
   [19-sicherheit.md](19-sicherheit.md), SEC-TLS-2).
2. PostgreSQL hängt nur am internen Netz `internal` ohne Ausgang. Sein Port ist nie am Host
   veröffentlicht; so kollidiert er auch nie mit einem PostgreSQL des Hosts.
3. Ausgang brauchen nur `api` (OIDC-Anbieter), `worker` (Push-Dienste, Webhooks), `backup`
   (Off-site-Ziel), `otel-collector` (Telemetrie-Backend), `clamav` (Signaturen) und Alertmanager.
4. Der Server lädt keine fremden URLs (Baseline, SSRF). Ausgehende Webhooks gehen nur an Hosts
   aus `webhook.allowed_hosts` ([03-datenmodell.md](03-datenmodell.md), R-DM-17).

### 4.2 Zwei Listener

| Listener | Schlüssel | Inhalt |
|---|---|---|
| öffentlich | `server.listen_addr` | REST und Gateway |
| intern | `server.internal_listen_addr` | `/healthz`, `/readyz`, Eingangs-Webhooks von Diensten im Stack (Alertmanager) |

`/healthz` meldet nur, dass der Prozess lebt. `/readyz` meldet bereit, wenn der Start
abgeschlossen ist, die Datenbank antwortet, die Binärversion zum Schema passt (Abschnitt 10), je
Zweck ein aktiver Signierschlüssel besteht, kein Shutdown begonnen hat und (ab M3) der
Objektspeicher antwortet; die Namen der Prüfungen stehen in
[02-architektur.md](02-architektur.md), Abschnitt 15.2. Antworten: `GET` und `HEAD`,
`Content-Type: application/json`, `Cache-Control: no-store`; Erfolg `200` mit
`{"status":"ok"}`, sonst `503` mit `{"status":"unavailable","failed":[<Namen, sortiert>]}`.
Die Namen der fehlgeschlagenen Prüfungen sind das einzige Detail. Jede Prüfung, die eine
Abhängigkeit fragt, begrenzt sich selbst (die Datenbank über `database.acquire_timeout`).
`bothy healthcheck --url` fragt nur Loopback-Adressen über `http` ab (Exitcode `0` bei `2xx`, sonst
`1`) und nutzt den Loopback-Client von `platform/egress`
([19-sicherheit.md](19-sicherheit.md), SEC-NET-1).

### 4.3 Rand und Identität

| Rand | Anmeldung | Bedingung |
|---|---|---|
| Reverse-Proxy (Profil `direct`) | OIDC oder lokale Konten | `server.trusted_proxies` nennt die Gegenstelle, die der Server sieht: im Profil `direct` das Gateway des Compose-Netzes, nicht den Proxy selbst (Compose-Variable `BOTHY_TRUSTED_PROXIES`); sonst weist SEC-TLS-1 jede Anfrage ab, und die Client-IP für `rl.public_auth` stimmt nicht |
| `tailscale serve` (Profil `tailscale`) | Identitätsanbieter `trusted_header` | Der öffentliche Listener lauscht nur auf `127.0.0.1` im Netz-Namespace des Sidecars |
| privates Netz der Cloud | OIDC | Load Balancer oder Application Gateway terminiert TLS |

Der Server prüft beim Start: Ist ein Anbieter `trusted_header` aktiv und `server.listen_addr`
keine Loopback-Adresse, meldet `/readyz` den Fehler `trusted_header_requires_loopback` und der
Anbieter bleibt aus (fail closed). Grund: Jeder, der den Listener direkt erreicht, könnte die
Kopfzeilen selbst setzen. Tailscale nennt das ausdrücklich
(https://tailscale.com/kb/1312/serve, abgerufen 2026-10-10) [belegt]. Details:
[tailscale.md](../operations/tailscale.md).

Der Zugang zum Host ist die Vertrauenswurzel der Instanz: Wer Server-Befehle ausführen kann, legt
den ersten Owner an (`bothy setup bootstrap-owner`) und stellt Owner-Codes zur Wiederherstellung
aus (`bothy setup issue-enrollment-code`, Systemaktion `owner.recover`;
[08-identitaet.md](08-identitaet.md), Abschnitt 4). Kein Weg über die API ersetzt das, und keiner
umgeht es. Darum gehören Shell-Zugang, Container-Laufzeit und Secret-Provider des Hosts zum
geschützten Kern; der Betreiber gibt sie nur Personen, die auch Owner sein dürften.

### 4.4 Terminal: Weiterleitung und Host-Dienst

Normativ zusammen mit [24-terminal.md](24-terminal.md).

Rand und Proxy:

1. Der Rand leitet WebSocket-Upgrades für `/api/v1/terminal/sessions/{session_id}/stream`,
   `/api/v1/hosts/connect` und `/api/v1/hosts/sessions/{session_id}/stream` weiter, mit einer
   Leerlauf-Frist über `terminal.host_heartbeat_interval` (sonst schneidet der Proxy ruhige
   Terminals ab).
2. Mit `cluster.mode = multi` leitet der Rand beide Strom-Pfade derselben Sitzung per Hash über die
   Sitzungs-ID auf denselben api-Knoten (konsistentes Hashing über das Pfadsegment
   `{session_id}`). Beide Pfade haben die Sitzungs-ID an derselben Stelle nach dem festen Präfix, damit
   eine Regel beide trifft. Ohne diese Regel scheitert jede zweite Sitzung sichtbar mit `4102`
   (Ruling T6, TM-T15). Mit einem Knoten ist nichts zu tun.
3. Der Rand puffert diese Verbindungen nicht und schreibt keine Inhalte in Logs (Zugriffslog nur mit
   Routen-Muster, [19-sicherheit.md](19-sicherheit.md), SEC-LOG-*).

Installation des Host-Dienstes auf einem Rechner:

1. Benutzer ohne Administratorrechte: der Dienstbenutzer `bothy-host` mit dem Zustandsordner
   `host_service.state_dir` (Modus `0700`) und je Person, die auf dem Rechner ein Terminal bekommen
   soll, ein eigener Sitzungsbenutzer aus `host_service.session_users`, ohne Leserecht darauf (24,
   Abschnitt 2.2; Ruling T12). `bothy-host install --session-user <name>` legt den ersten an,
   `bothy-host session-user add <name>` (wieder mit Administratorrechten) jeden weiteren, beide
   mit einem Home nur für diesen Benutzer. `session-user add` startet den Host-Dienst danach neu,
   weil er seine Konfiguration nur beim Start liest; mit dem nächsten `report` meldet der Rechner
   den neuen Namen, und der Owner kann ihn ohne weiteren Schritt zuordnen (24, Abschnitt 2.2
   Punkt 5). `bothy-host session-user remove <name>` (ebenso mit Administratorrechten) nimmt einen
   heraus: Der Befehl streicht den Namen aus `host_service.session_users` und seinen Eintrag aus
   `host_service.claude_workdirs`, entfernt den Dienst seines PTY-Helfers, schließt sein Home für
   alle anderen Benutzer (Modus `0700`, unter Windows eine ACL nur für ihn und die
   vertrauenswürdigen Besitzer) und startet den Host-Dienst neu. Das Home zu schließen ist nötig,
   weil HD9 es danach als Home eines Benutzers prüft, der kein Sitzungsbenutzer ist; ein offenes
   Home ließe sonst den Start scheitern (24, Abschnitt 2.2 Punkt 4). Die Bindung des Namens an
   seine Person bleibt im Zustandsordner, und `session-user add` lehnt einen dort gebundenen
   Namen ab (24, Abschnitt 2.2 Punkt 3). Andere Änderungen der Konfiguration des Host-Dienstes
   (Arbeitsordner in `host_service.claude_workdirs`, Profile, `host_service.claude_bin`) trägt der
   Betreiber in die Konfiguration ein und ruft danach `bothy-host restart` (mit
   Administratorrechten) auf, das den Host-Dienst neu startet. Jeder Neustart beendet alle
   laufenden Sitzungen des Rechners (24, Abschnitt 6.2); der Betreiber wählt dafür eine ruhige
   Zeit. Einen Sitzungsbenutzer
   einer anderen Person oder einen früher vergebenen Namen nie wiederverwenden, auch nicht nach dem
   Zurückziehen und neuen Koppeln des Rechners oder nach dem Verlust des Zustandsordners (dann
   prüfen weder Server noch Host-Dienst das; 24, Abschnitt 2.2). Der Owner ordnet die Namen den
   Personen zu (24, Abschnitt 3.2). Keiner dieser Benutzer in `docker`, `lxd`, `libvirt`, `disk`
   oder einer Admin-Gruppe (HD2). Kein Sitzungsbenutzer kann einen Home-Ordner eines anderen
   Benutzers auflisten: das Home eines Benutzers, der kein Sitzungsbenutzer ist, prüft HD9 (sonst
   verweigert der Rechner den Start), das Home eines anderen Sitzungsbenutzers prüft HD11 als
   dessen Ort (sonst bekommt nur dieser keine Sitzungen); unter macOS ist darum keiner in `staff`. Kein Ort mit Code, Einstellungen oder Anmeldungen ist für einen
   Sitzungsbenutzer schreibbar und für einen anderen lesbar (Ruling T13, HD11): Arbeitsordner des
   Profils `claude` stehen in `host_service.claude_workdirs` beim Namen genau eines
   Sitzungsbenutzers (einen Ordner, den dessen PTY-Helfer nicht auflösen kann, etwa einen im Home
   eines anderen, streicht HD7 mit einer Warnung aus seiner Liste; 24, Abschnitt 2.2 Punkt 6),
   gehören ihm mit Modus `0700`, kein Ordner darüber ist für einen anderen
   Sitzungsbenutzer schreibbar, und in diesen Ordnern liegt keine Kettendatei (`CLAUDE.md`,
   `CLAUDE.local.md`, `AGENTS.md`, `.mcp.json`, `.claude`, `.git`), die einem anderen gehört oder
   die er schreiben kann; Programme der Profile gehören samt jedem Ordner auf dem Weg dorthin einem
   vertrauenswürdigen Besitzer (Linux und macOS `root`, Windows `SYSTEM`, `Administrators` oder
   `NT SERVICE\TrustedInstaller`; 24, Abschnitt 2.2 Punkt 6), nie einem anderen Konto, auch nicht
   dem Konto einer Person mit Administratorrechten. Dazu gehört bei `organization`
   `host_service.claude_bin`, das Programm des Profils `claude`, unter Windows als `.exe`; ein
   `claude` in einem Ordner eines Paketverwalters, der einem solchen Konto gehört, besteht HD10
   nicht. Gemeinsamen Code tauschen die Personen über git
   aus, nie über einen gemeinsamen Ordner; auch außerhalb der Konfiguration richtet der Betreiber
   keinen Ordner ein, den zwei Sitzungsbenutzer schreiben können (etwa über eine gemeinsame Gruppe),
   weil HD11 nur die konfigurierten Pfade prüft.
   Unter Windows fragt `bothy-host install` zusätzlich, ob er an der Wurzel des Systemlaufwerks
   und jedes Laufwerks, auf dem ein Ort liegt, den Standard-Eintrag
   `Authenticated Users:(AD)` entfernen darf (Ordner anlegen in der Wurzel, 24, Abschnitt 2.2
   Punkt 6). Folge der Bestätigung: Standardbenutzer legen in der Wurzel dieser Laufwerke keine
   Ordner mehr an; Rechte darunter ändern sich nicht, auch nicht der vererbte Eintrag
   `Authenticated Users:(OI)(CI)(IO)(M)`, weil der Befehl nur die DACL der Wurzel schreibt, ohne
   Weitergabe an die Ordner darunter (24, Abschnitt 2.2 Punkt 6). Der Betreiber entfernt den
   Eintrag darum nur mit diesem Befehl, nicht mit einem Werkzeug, das die Vererbung auf das ganze
   Laufwerk neu anwenden kann. Ohne Bestätigung ändert der Befehl nichts, und
   `bothy-host doctor` verweigert den Start mit dem Code `drive_root_writable` (HD11). Kommt später
   ein Ort auf einem weiteren Laufwerk dazu, ruft der Betreiber `bothy-host install --drive-root
   <Laufwerk>` (mit Administratorrechten) auf; der Befehl macht nur diesen Schritt, mit derselben
   Frage. Einträge mit den Namen der Kettendateien in der Wurzel (etwa `C:\.git`, `C:\.claude`),
   die ein Sitzungsbenutzer vorher angelegt hat, entfernt der Betreiber nach Punkt 6. Arbeitsordner
   legt er ins Home des Sitzungsbenutzers oder setzt die ACL eines neuen Ordners in der Wurzel
   ohne Vererbung neu, weil ein solcher Ordner sonst den Eintrag `(OI)(CI)(IO)(M)` erbt und für
   jeden Sitzungsbenutzer schreibbar ist.
2. Dienste des Betriebssystems: launchd (macOS), systemd (Linux), Windows-Dienst; einer für den
   Host-Dienst, je Sitzungsbenutzer einer für seinen PTY-Helfer; Neustart bei Absturz, beim
   Host-Dienst aber nicht nach einem verweigerten Start, den er mit eigenem Exit-Code beendet
   (24, Abschnitt 2.2). Beide sind Dienste des Systems
   (LaunchDaemon, systemd-Systemdienst, Windows-Dienst), keine Dienste eines Benutzers; Binärdatei,
   Dienst-Definitionen und der Ordner des Kanal-Endpunkts gehören einem vertrauenswürdigen Besitzer
   (Punkt 1).
   Den Endpunkt des Kanals legt unter Linux und macOS der Dienstverwalter an (Socket-Aktivierung).
   Grund: Kein Sitzungsbenutzer darf seinen Helfer ersetzen können (24, Abschnitt 2.2, HD10). Der Installationsbefehl
   `bothy-host install` legt Benutzer, Ordner, lokale Kanäle und Dienste an und braucht dafür
   einmal Administratorrechte; die Dienste laufen danach ohne sie.
3. `bothy-host pair --server <url> --code <code>` koppelt (24, Abschnitt 3.2); `bothy-host doctor`
   prüft HD1 bis HD11 vor jedem Start und auf Befehl, auf dem Rechner mit den Namen der
   betroffenen Sitzungsbenutzer und einem Exit-Code ungleich 0, sobald eine Prüfung nicht besteht
   (24, Abschnitt 2.2).
4. Nicht auf dem Server-Host installieren. Wo es doch nötig ist, erklärt der Owner den Rechner als
   Server-Host; dann bekommen nur Owner ein Terminal dort (07, SR10).
5. Für das Profil `claude` legt der Betreiber bei `organization` den API-Schlüssel als Datei mit
   Modus `0600` in den Zustandsordner. Bei `personal` meldet sich der Inhaber selbst im Konto
   seines Sitzungsbenutzers an (`/login` oder ein eigenes Token aus `claude setup-token` in einer
   Datei dieses Sitzungsbenutzers); in den Zustandsordner kommt keine Abo-Anmeldung
   ([24-terminal.md](24-terminal.md), Abschnitt 7.4). Nie als Umgebungsvariable des Dienstes.
6. Meldet `doctor` HD6 nicht bestanden (Abo-Anmeldung bei `none` oder `organization`), ist der
   ganze Rechner gesperrt, bis HD6 in jedem Sitzungsbenutzer besteht (24, Abschnitt 7.4). Die
   Konsole nennt jeden betroffenen Sitzungsbenutzer, auch bei einem Treffer beim Start, etwa nach
   einem Neustart des Rechners: Dann startet der Host-Dienst nicht, meldet das Ergebnis aber
   vorher, und der Rechner bleibt getrennt. Kommt diese Meldung im Server nicht an, ruft der
   Betreiber `bothy-host doctor` auf dem Rechner auf; es nennt dieselben Namen (24, Abschnitt 2.2).
   Der Betreiber entfernt die Anmeldung in genau diesen Konten (`claude auth logout` als dieser
   Benutzer bzw. die Anmeldedatei, das Token in seinen Startdateien oder den Eintrag im
   Schlüsselbund). Wiederholt eine Person das oder lässt sich die Anmeldung nicht entfernen,
   beendet der Owner ihre Zuordnung (`host.manage`), und der Betreiber nimmt ihren
   Sitzungsbenutzer nach Punkt 1 mit `bothy-host session-user remove` heraus und sperrt oder
   löscht ihn danach auf dem Rechner. Sperren oder das Ende der Zuordnung allein beenden den
   Stillstand nicht, weil HD6 im PTY-Helfer jedes Namens aus `host_service.session_users` läuft.
   Soll die Person später wieder ein Terminal bekommen, legt er für sie nach Punkt 1 einen neuen
   Sitzungsbenutzer an (24, Abschnitt 2.2; Bedrohungsmodell T-TM-14). Nach einem verweigerten
   Start, bei HD6 wie bei jeder anderen Prüfung, startet der Dienstverwalter den Host-Dienst nicht
   von selbst neu (Punkt 2): Ist der Verstoß behoben, startet ihn der Betreiber mit
   `bothy-host restart`; `bothy-host session-user remove` startet ihn selbst neu (Punkt 1).
   Meldet `doctor` HD11 nicht bestanden, bekommt nur
   der genannte Sitzungsbenutzer keine Sitzungen: Der Betreiber stellt Besitz und Modus des Ortes
   wieder her, prüft Dateien darin und die Kettendateien in den Ordnern darüber, die ein anderer
   Sitzungsbenutzer geschrieben haben kann (vor allem `.claude/`, `CLAUDE.md` und Startdateien),
   und ruft `bothy-host doctor` erneut auf; erst dieser Aufruf findet Einträge, die ein anderer
   Sitzungsbenutzer im Ort angelegt hat, und erst ein solcher voller Lauf hebt einen Fund der
   Prüfung der Einträge wieder auf (24, Abschnitt 2.2 Punkt 6; Bedrohungsmodell T-TM-13). Meldet
   HD11 für einen Sitzungsbenutzer `entry_scan_timeout`, wurde die Prüfung seines Home in
   `host_service.entry_scan_timeout` nicht fertig: Der Betreiber erhöht den Wert und ruft
   `bothy-host restart` auf, oder die Person verkleinert ihr Home, und er ruft `bothy-host doctor`
   auf. `entry_scan_running` heißt nur, dass diese Prüfung nach dem Start noch läuft. Nennt die
   Meldung von HD11 keinen Sitzungsbenutzer, liegt der Verstoß an einem Ort für alle (einem
   Programm oder einem Ordner auf dem Weg dorthin): Der Rechner nimmt keine Sitzungen an, bis der
   Betreiber Besitz und Rechte wiederhergestellt hat und ein Lauf von HD11 besteht.
   Nennt die Meldung von HD11 mehrere Sitzungsbenutzer, deren Orte unter demselben Ordner liegen,
   liegt der Verstoß meist in diesem gemeinsamen Ordner auf dem Weg, etwa eine Kettendatei einer
   Person in der Wurzel eines Laufwerks: Sie legt alle anderen still (24, Abschnitt 2.2, "Folge,
   ehrlich"). Der Betreiber entfernt diesen Eintrag, und wenn der Ordner noch für einen
   Sitzungsbenutzer schreibbar ist, schließt er ihn (unter Windows an der Wurzel mit
   `bothy-host install --drive-root`; Bedrohungsmodell T-TM-14). Meldet HD11 `drive_root_writable`,
   fehlt der Schritt aus Punkt 1 für dieses Laufwerk; die lokale Ausgabe von `bothy-host doctor`
   nennt das Laufwerk.

## 5. Konfiguration und Geheimnisse

### 5.1 Konfiguration (ADR 0011)

1. Prozess-Konfiguration kommt aus Umgebungsvariablen und einer optionalen YAML-Datei
   (`BOTHY_CONFIG_FILE` oder `--config`). Vorrang: Kommandozeile (nur `--role`, `--config`) vor
   Umgebung vor Datei vor Standardwert.
2. Namensregel: Schlüssel `a.b_c.d` heißt in der Umgebung `BOTHY_A__B_C__D` (Punkt wird `__`).
   Die Regel ist umkehrbar, weil Schlüssel nie zwei Unterstriche hintereinander enthalten.
3. Unbekannte Variablen mit Präfix `BOTHY_` und unbekannte Schlüssel in der Datei brechen den
   Start ab. Grund: Ein Tippfehler darf nie still eine Sicherheitsgrenze auf den Standard setzen.
   Ausgenommen sind nur `BOTHY_CONFIG_FILE` und die Variablen, die ein wirksamer Verweis `env:`
   nennt (5.2). Werkzeuge des Repositorys mit demselben Präfix (`BOTHY_DENYLIST_*`,
   `BOTHY_CODERABBIT`) gehören nicht in die Umgebung eines Servers: Maintainer legen den Pfad der
   Deny-Liste in `.denylist-path` ab oder setzen die Variable nur für den einen Befehl, und
   Test-Werkzeuge starten den Server mit einer ausdrücklichen Umgebung. Grund: Eine Ausnahme für
   Werkzeugnamen würde die Tippfehler-Prüfung für genau diese Namen abschalten.
4. Ungültige Werte (Typ, Bereich) brechen den Start ab. `bothy config check` prüft ohne Start,
   `bothy config print --redacted` zeigt die wirksame Konfiguration ohne Geheimnisse.
5. Prozess-Konfiguration wird nur beim Start gelesen. Ausnahme: Geheimnisse (5.2).
6. Instanz-Einstellungen (`instance_settings`, z. B. `branding.*`) und Feature-Flags
   (`feature_flags`) sind keine Prozess-Konfiguration. Sie ändern sich zur Laufzeit über die
   Admin-Konsole mit Step-up und Audit ([05-aktionen.md](05-aktionen.md) 4.2).
7. Standard-Variablen von OpenTelemetry (`OTEL_*`) gelten unverändert (Kapitel 17).

Die vollständige Liste aller Schlüssel mit Umgebungsname, Typ, Standard und Bedeutung steht in
[docs/operations/configuration.md](../operations/configuration.md).

### 5.2 Secret-Provider (ADR 0011)

Jedes Geheimnis ist ein Verweis `secret_ref` (Spalten `secret_ref`, `private_key_ref` in
[03-datenmodell.md](03-datenmodell.md), Schlüssel `*_ref` in der Konfiguration). Form:
`<schema>:<pfad>`.

| Schema | Beispiel | Treiber | Schreiben | Status |
|---|---|---|---|---|
| `file` | `file:/run/bothy/secrets/db_app_password` | Datei lesen | nur unter `secrets.signing_key_dir` ([ADR 0066](../adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md)) | M1 |
| `env` | `env:BOTHY_SECRET_DB_APP_PASSWORD` | Umgebungsvariable lesen | nein | M1, nur wo die Plattform nichts anderes kann |
| `azkv` | `azkv://vault-name/secret-name` | Azure Key Vault | ja | später |
| `awssm` | `awssm://eu-central-1/bothy/db_app_password` | AWS Secrets Manager | ja | später |

Schnittstelle des Treibers: `Get(ref) -> (Wert, Version)`, optional `Put(ref, Wert)` bei
schreibbaren Treibern. Der Server hält gelesene Werte höchstens `secrets.cache_ttl` und liest
danach neu; so wirkt eine Rotation ohne Neustart. Werte erscheinen nie in Logs, Telemetrie,
Fehlermeldungen oder Ereignissen. `env:` ist erlaubt, aber nicht empfohlen: Umgebungsvariablen
erben Kindprozesse und Absturzberichte.

Fehlt ein Verweis bei einem Schlüssel `*_ref`, gilt `file:<secrets.dir>/<name>` mit dem Namen aus
dem Katalog in der Konfigurationsreferenz (Abschnitt "Geheimnisse"). Die Kapiteltabellen schreiben
diesen Standard aus. "leer" steht nur bei Geheimnissen ohne Katalogeintrag, die zu einer
abschaltbaren Funktion gehören (z. B. `server.tls_cert_ref`, `server.tls_key_ref`: leer heißt, der
Server beendet kein TLS); eine eingeschaltete Funktion ohne Verweis bricht den Start ab.

Signierschlüssel des Servers sind die einzige Ausnahme vom Lesen: Der Server erzeugt sie selbst
in `secrets.signing_key_dir` (Verzeichnis im Besitz des Dienstbenutzers, Modus `0700`, Dateien
`0600`) über `Put` des Datei-Treibers; jeder andere Verweis bleibt nur lesbar ([ADR 0066](../adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md),
[19-sicherheit.md](19-sicherheit.md) Abschnitt 10). Grund: Planmäßige und Notfall-Rotation
brauchen neue Schlüssel ohne Handgriff auf dem Host.

### 5.3 Geheimnisse im Compose-Stack

1. Der Owner legt gelieferte Geheimnisse (Push-Schlüssel, OIDC-Client-Secret, Zugang zum
   Off-site-Ziel, Telemetrie-Schlüssel, Webhook-URLs für Alarme) als Dateien in
   `infra/compose/secrets/` ab. Das Verzeichnis ist nie im Repository.
2. `secrets-init` erzeugt fehlende Geheimnisse (Datenbank-Passwörter, Token-Pepper,
   Pseudonym-Schlüssel der Telemetrie, Schlüssel der Sicherung) und legt im Volume `secrets` je
   Dienst ein Unterverzeichnis an. Jeder Dienst bindet nur sein Unterverzeichnis ein (`subpath`),
   nur lesend. Signierschlüssel erzeugt nicht `secrets-init`, sondern der Server (5.2).
3. Die eine kanonische Kopie jedes Geheimnisses liegt in `_store/` des Volumes (Verzeichnis
   `0700`, Dateien `0400`); die Dienstverzeichnisse sind Kopien daraus. Abgleich mit den vom Owner
   gelieferten Dateien: Fehlt ein Eintrag, wird er erzeugt bzw. übernommen; ein gelieferter
   Eintrag mit anderem Inhalt ersetzt den gespeicherten (die Quelle des Owners ist maßgeblich);
   ein erzeugter Eintrag mit anderem Inhalt in der Quelle bricht ab, außer mit `--adopt-source`
   (Wiederherstellung aus dem Bündel, [disaster-recovery.md](../operations/disaster-recovery.md)
   Fall B). Eine Datei der Quelle ohne Katalogeintrag bricht ab. Ein zweiter Lauf ohne Änderung
   schreibt nichts.
4. Eigentum und Modus der Kopien: Dienste mit `--owner <dienst>=<uid>:<gid>` bekommen Verzeichnis
   `0500` und Dateien `0400` mit diesem Besitzer; `server` und `migrate` brauchen ihn, weil der
   Datei-Treiber nur eigene Dateien mit `0400` oder `0600` liest (SEC-SEC-1). Das Image
   `bothy-server` läuft als uid und gid `65532` ([18-release.md](18-release.md), Abschnitt 7).
   Fremddienste ohne `--owner` (Datenbank, Sicherung, Collector) bekommen Kopien im Besitz von
   root mit Verzeichnis `0755` und Dateien `0444`, weil ihre Images eigene Benutzer haben; jeder
   sieht trotzdem nur sein Unterverzeichnis.
5. `secrets-init` gibt am Ende die Fingerabdrücke aus, nie die Werte, und erinnert an die
   Hinterlegung (5.4). Katalogeinträge, deren Erzeuger eine spätere Etappe baut, meldet es als
   `pending` (Konfigurationsreferenz, Abschnitt "Geheimnisse").

### 5.4 Hinterlegung (Escrow)

Ohne einige Geheimnisse ist eine Sicherung wertlos. Der Owner exportiert sie nach jeder Änderung
verschlüsselt (`bothy secrets export --age-recipient <öffentlicher Schlüssel>`) und bewahrt das
Bündel getrennt vom Server und getrennt vom Off-site-Ziel auf. Das Bündel enthält `_store/` und den
Schlüsselspeicher `secrets.signing_key_dir`.

| Geheimnis | Folge bei Verlust |
|---|---|
| Token-Pepper | Alle API-Tokens, Refresh-Tokens und Eingangs-Webhook-Tokens werden ungültig (Hashes nicht mehr prüfbar) |
| Signierschlüssel | Zugriffstokens und Aufruf-Marken ungültig; alle Geräte melden sich neu an |
| Schlüssel der Sicherung (pgBackRest, restic) | Sicherungen nicht mehr lesbar |
| Push-Schlüssel (APNs, FCM) | Kein Push, bis der Owner neue Schlüssel beim Anbieter erzeugt |
| Pseudonym-Schlüssel der Telemetrie | Alte und neue Pseudonyme sind nicht mehr verknüpfbar (gewollte Folge einer Rotation) |

## 6. Umgebungen und Feature-Flags

### 6.1 Umgebungen

| Umgebung | Zweck | Daten | Push | Schutz |
|---|---|---|---|---|
| `dev` | Entwicklung, lokale Tests | nur synthetisch | APNs-Sandbox | Lockerungen nur hier erlaubt, z. B. HTTP ohne TLS auf Loopback |
| `staging` | Abnahme eines Release-Kandidaten, Portabilitätstest | synthetisch oder Import aus Testquellen | eigene Schlüssel | wie `prod`; Banner "Staging" in den Apps |
| `prod` | Betrieb | echte Daten | Produktion | alle Prüfungen scharf |

Regeln:

1. `instance.environment` ist der Standard `prod`. Lockerungen muss `dev` ausdrücklich setzen.
2. Zwischen Umgebungen wandert dasselbe Image per Digest, nie ein neu gebautes.
3. Produktionsdaten werden nie nach `dev` oder `staging` kopiert. Es gibt kein
   Anonymisierungswerkzeug; ein Datenabzug wäre ein Datenschutzvorfall (Kapitel 20).
4. Jede Umgebung hat eigene Geheimnisse, eigene Push-Schlüssel und eine eigene `instance_id`.

### 6.2 Feature-Flags

Entscheidung: [ADR 0064](../adr/0064-feature-flag-registrierung-mit-aufraeum-gate.md).

| Thema | Regel |
|---|---|
| Speicher | Tabelle `feature_flags` ([03-datenmodell.md](03-datenmodell.md) 4.1): globaler Wert plus `rollout` mit Personen und Gruppen, für die der Wert abweichend gilt |
| Registrierung | Jedes Flag ist im Code registriert: Schlüssel, Art (`release` oder `option`), Beschreibung, Standard, bei `release` die Version `remove_by` |
| Auswertung | `flags.Enabled(principal, key)` im Server; Wert = global, umgekehrt, wenn der Prinzipal oder eine seiner gültigen Gruppen in `rollout` steht. Unbekannte Schlüssel sind aus. Clients bekommen die für sie ausgewerteten Werte per REST; `instance.updated` nennt nur geänderte Schlüssel |
| Admin-Konsole | Liste mit Art, Beschreibung, globalem Wert, Rollout-Listen und letzter Änderung; Ändern mit `feature_flag.update` (Owner, Step-up, Audit) |
| Aufräumregel | Ein `release`-Flag verschwindet spätestens in Version `remove_by`. Ein Check im Gate schlägt fehl, wenn die Version des Servers `remove_by` erreicht und das Flag noch registriert ist. Die Migration, die den Code entfernt, löscht auch die Zeile |
| Grenze | Flags schalten Funktionen, nie Sicherheitsgrenzen, Grenzwerte oder Geheimnisse; dafür gibt es Konfiguration |

`option`-Flags sind dauerhafte Produktschalter, z. B. `dm.enabled`, `voice.enabled`,
`tokens.personal_enabled` ([05-aktionen.md](05-aktionen.md)). Anleitung:
[environments-and-flags.md](../operations/environments-and-flags.md).

## 7. Datenbank

### 7.1 Rollen

| Rolle | Rechte | Nutzer |
|---|---|---|
| `postgres` | Superuser (`migrate.bootstrap_user`) | nur `migrate --bootstrap` und `bothy-backupd` |
| `bothy_owner` | Eigentümer der Datenbank, des Schemas `public` und von `jobs.schema`; DDL | `migrate` |
| `bothy_app` | DML; auf `audit_events` nur `INSERT` und `SELECT` | `api`, `worker`, Server-Befehle `bothy setup ...` (sie schreiben nur Zeilen) |
| `bothy_maint` | Kürzen des Audit-Logs am Fristende mit Anker-Eintrag | Aufbewahrungsjob ([03-datenmodell.md](03-datenmodell.md) 4.12) |
| `bothy_monitor` | `pg_monitor` | `otel-collector` |

`migrate --bootstrap` legt fehlende Rollen idempotent an und setzt ihre Passwörter aus den
Geheimnissen (`migrate.*_password_ref`). Dafür verbindet es sich als `migrate.bootstrap_user` mit
`migrate.bootstrap_password_ref`; die Migrationen danach laufen als `database.user` (`bothy_owner`)
mit `database.password_ref`, wie ohne `--bootstrap`. Regeln:

1. Je Rolle `CREATE ROLE` bzw. `ALTER ROLE` mit `LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
   NOREPLICATION NOBYPASSRLS`; das Passwort geht nur als SCRAM-SHA-256-Verifier an die Datenbank,
   nie im Klartext.
2. `bothy_owner` besitzt die Datenbank und das Schema `public`; `PUBLIC` verliert alle Rechte an
   beiden; `bothy_app`, `bothy_maint` und `bothy_monitor` bekommen `CONNECT`, die ersten beiden
   `USAGE` auf `public`; `bothy_monitor` bekommt `pg_monitor`.
3. Keine Standardrechte im Schema `public`: Jede Migration, die eine Tabelle anlegt, erteilt
   `bothy_app` genau die Rechte, die 03 und dieses Kapitel vorsehen (meist `SELECT, INSERT,
   UPDATE, DELETE`, weniger etwa bei `audit_events`); ein Test prüft je Tabelle die Rechte gegen
   eine Erwartungsliste. Grund: Ein vergessenes `GRANT` scheitert laut im Test, ein vergessenes
   `REVOKE` wäre still. Ausnahme: Im Schema `jobs.schema` legt River Tabellen selbst an; dort
   gelten Standardrechte für `bothy_app`.
4. `migrate` läuft als `bothy_owner` (`database.user`) und prüft das vor der ersten Migration.

Mit einem verwalteten Dienst legt das OpenTofu-Modul die Rollen mit denselben Namen an und führt
dieselben Anweisungen aus; `--bootstrap` entfällt dort.

DDL im Betrieb (heute nur Partitionen von `event_log`) läuft nie mit Rechten von `bothy_app`.
Muster: eine Funktion `SECURITY DEFINER` im Besitz von `bothy_owner`, `search_path` fest auf das
Domänenschema und `pg_temp`, `EXECUTE` von `PUBLIC` entzogen und nur `bothy_app` erteilt; Namen
nur über `format('%I')`. Die Funktion nimmt nur Zahlen und Zeitpunkte als Parameter, nie Namen.
Grund: Die Anwendungsrolle bleibt bei DML, und die einzige DDL-Stelle ist eng und prüfbar.
Spätere DDL-Jobs nutzen dasselbe Muster.

### 7.2 Einstellungen

| Einstellung | Wert | Grund |
|---|---|---|
| `wal_level` | `replica` | WAL-Archivierung |
| `archive_mode` | `on` | PITR |
| `archive_timeout` | `backup.pg.archive_timeout` | begrenzt den Datenverlust bei wenig Schreiblast |
| Isolationsstufe | `READ COMMITTED` | Zähler-Muster ([04-sync-protokoll.md](04-sync-protokoll.md) 3.2) |
| `statement_timeout` je Anwendungsverbindung | `database.statement_timeout` | keine hängenden Abfragen |

## 8. Sicherung und Wiederherstellung

Entscheidung: [ADR 0058](../adr/0058-sicherung-mit-pgbackrest-objektsicherung-und-restore-test.md) (R-OP-06 bis R-OP-10).

### 8.1 Was gesichert wird

| Daten | Werkzeug | Häufigkeit | Aufbewahrung | Ziel |
|---|---|---|---|---|
| PostgreSQL physisch, WAL fortlaufend | pgBackRest | WAL je Segment, spätestens nach `backup.pg.archive_timeout`; Vollsicherung `backup.pg.full_schedule`, Differenz `backup.pg.diff_schedule` | `backup.pg.retention_full` Vollsicherungen | repo1 lokal, repo2 off-site |
| PostgreSQL logisch | `pg_dump -Fc` | `backup.logical.schedule` | `backup.logical.retention` | restic off-site |
| Objekte, Treiber `fs` | restic | `backup.objects.schedule` | `backup.objects.retention` | restic off-site |
| Objekte, Treiber `s3` | Versionierung und Replikation des Anbieters, sonst Spiegel ohne Löschen | wie oben | wie oben | zweiter Bucket |
| Geheimnisse | `bothy secrets export` (age) | nach jeder Änderung | bis zur Ablösung | offline beim Owner (5.4) |
| Konfiguration (`.env`, Overlays) | eigenes Repository des Betreibers | jede Änderung | unbegrenzt | Repository |

Begründungen:

1. pgBackRest (MIT) bietet Voll-, Differenz- und Inkrementalsicherung, Verschlüsselung des
   Repositorys, Prüfsummen je Datei mit Prüfung bei Restore und `verify`, mehrere Repositorys
   und Aufbewahrungsregeln (https://pgbackrest.org/, abgerufen 2026-10-10) [belegt]. Ein eigenes
   Skript mit `pg_basebackup` müsste all das nachbauen.
2. Der logische Abzug ist unabhängig von Hauptversion und Anbieter. Er ist das Werkzeug für den
   Umzug (Abschnitt 11) und die letzte Rückfallebene.
3. Dateien ändern sich nie (`object_key` eindeutig, keine Deduplizierung, R-DM-13). Eine
   inkrementelle, verschlüsselte Sicherung mit Snapshots passt; restic prüft Stichproben mit
   `check --read-data-subset` (https://restic.readthedocs.io/en/stable/045_working_with_repos.html,
   abgerufen 2026-10-10) [belegt].
4. Am 2026-10-04 hat pgBackRest 2.59.3 einen Fehler behoben, durch den schwache Schlüssel und
   Salts entstehen konnten (https://pgbackrest.org/release.html, abgerufen 2026-10-10) [belegt].
   `bothy-postgres` nutzt nur Versionen ab 2.59.3. Ob vorher erzeugte Repositorys neu
   verschlüsselt werden müssen, ist offen; für neue Instanzen stellt sich die Frage nicht.

### 8.2 Ablauf

1. `postgres` schiebt jedes WAL-Segment mit `archive-push` über das TLS-Protokoll von pgBackRest
   an den Repository-Host `backup`. Der Datenbank-Container braucht dafür keinen Ausgang und
   keine Zugangsdaten des Off-site-Ziels.
2. `bothy-backupd` startet Voll- und Differenzsicherungen, `pg_dump`, die Objektsicherung und
   `expire` nach Plan. Jeder Lauf schreibt ein Manifest: Zeilenzahl je Tabelle, `head_seq` des
   Ereignis-Logs, letzter `seq` und `hash` des Audit-Logs, Schema-Version, Server-Version, Zahl
   und Summe der Dateigrößen.
3. Off-site-Repositorys sind verschlüsselt und nur anhängend: Object Lock oder ein
   Append-only-Zugang. Löschen nach Ablauf der Frist darf nur ein getrenntes Konto. Grund: Wer den
   Server übernimmt, darf die Sicherungen nicht löschen können (Ransomware).
4. Jeder Lauf schreibt eine Zeile in `backup_runs` (03, Modul `status`) und meldet
   Kennzahlen (Kapitel 17). Die Admin-Konsole und die Statusseite lesen die Tabelle.

![Sicherung](../operations/diagrams/backup-flow.svg)

### 8.3 Automatischer Wiederherstellungstest

Nach Plan `backup.restore_test.schedule` stellt `bothy-backupd` die letzte Sicherung in
`backup-scratch` wieder her, startet dort eine Datenbank nur auf einem Unix-Socket und prüft:

1. `pgbackrest verify` und die Prüfsummen beim Restore.
2. Schema-Version und Zeilenzahlen gegen das Manifest.
3. Ereignis-Log lückenlos: je Partition `count(*) = max(seq) - min(seq) + 1`, und
   `sequence_counters` ist mindestens so groß wie jede vergebene Nummer.
4. Audit-Kette: dieselbe Prüfung wie `audit.verify` vom Anker bis zum Ende.
5. Stichprobe `backup.restore_test.sample_threads` Threads: `thread_seq` lückenlos.
6. Stichprobe `backup.restore_test.sample_files` Dateien: Objekt aus der Objektsicherung holen,
   SHA-256 gegen `files.sha256`.
7. Dauer des ganzen Tests gegen `dr.rto_target`.

Das Ergebnis geht in `backup_runs` (Art `restore_test`) und in die Kennzahlen. Fehlt ein
erfolgreicher Test länger als `backup.restore_test.max_age`, alarmiert das System. Anleitung:
[backup-restore.md](../operations/backup-restore.md).

### 8.4 Wiederherstellung und Sync

Eine Wiederherstellung setzt die globale Sequenz zurück. R-SY-04 erkennt das nur, solange
`cursor > head_seq` gilt. Nach neuen Schreibvorgängen überholt die neue Zeitlinie alte Cursor,
und ein Client würde fremde Ereignisse mit alten Nummern still überspringen.

Lösung (03 Abschnitt 6.7, 04 Abschnitt 7.5, [ADR 0059](../adr/0059-sync-epoche-nach-wiederherstellung.md)): `instance.sync_epoch` (UUIDv7) benennt
die Zeitlinie. `bothy restore finalize` setzt sie nach jeder Wiederherstellung neu. `hello`,
`ready` und `GET /api/v1/events` nennen die Epoche; der Client speichert sie mit dem Cursor. Passt
sie nicht, antwortet der Server `resync_required` mit Grund `epoch_changed`. Derselbe Befehl setzt
alle `agent_consumer_cursors.acked_seq`, `webhook_subscriptions.last_delivered_seq` und
`event_consumer_cursors.last_seq` auf den neuen `head_seq` und schreibt einen Audit-Eintrag. Ein
Umzug ohne Datenverlust (Abschnitt 11) ändert die Epoche nicht.

## 9. Disaster Recovery

Ziele: `dr.rpo_target` (Datenverlust) und `dr.rto_target` (Dauer bis zum Betrieb). Das Runbook
[disaster-recovery.md](../operations/disaster-recovery.md) behandelt diese Fälle:

| Fall | Weg |
|---|---|
| Daten versehentlich gelöscht oder beschädigt | PITR auf einen Zeitpunkt davor, neue Epoche |
| Host verloren | neuer Host, Secrets-Bündel, Restore aus repo2 und restic, neue Epoche |
| Off-site-Ziel verloren | neues Ziel, sofort Vollsicherung, Alarm bis zum ersten Test |
| Signierschlüssel kompromittiert | Rotation ([05-aktionen.md](05-aktionen.md) `signing_key.rotate`), alle Sitzungen widerrufen |
| Token-Pepper kompromittiert | neuer Pepper, alle API-Tokens neu ausgeben, alle Sitzungen widerrufen |
| Ransomware auf dem Host | Host als verloren behandeln; Off-site-Daten sind unveränderlich |

## 10. Upgrade

1. Jede Version bringt ihre Migrationen mit; `migrate` läuft vor dem Start von `api` und
   `worker`. Gibt es offene Migrationen auf einem bestehenden Schema und keine erfolgreiche
   Sicherung jünger als `migrate.recent_backup_max_age`, bricht `migrate` ab, solange
   `migrate.require_recent_backup` gilt. Eine leere Datenbank (Erstinstallation) ist ausgenommen.
   Diese Prüfung gilt ab M7-02, weil sie `backup_runs` liest; bis dahin prüft `migrate` keine
   Sicherung, und die Konfigurationsreferenz führt beide Schlüssel als geplant.
2. Migrationen folgen Expand/Contract: Version N darf nur entfernen, was Version N-1 nicht mehr
   nutzt. Damit läuft der Code von N-1 gegen das Schema von N, und der Rückweg ist ein
   Image-Wechsel ohne Datenbank-Rückbau.
3. Down-Migrationen gibt es in Produktion nicht. Fehler korrigiert die nächste Version.
4. Der Server prüft beim Start das Schema: Ist es älter als nötig, bricht der Start mit Hinweis
   auf `bothy migrate up` ab. Verlangt es eine neuere Server-Version
   (`instance.min_server_version`, gesetzt von `migrate`; 03), läuft der Prozess weiter, wird aber
   nie bereit (`/readyz` meldet `schema_requires_newer_server`). Grund für den Unterschied:
   [02-architektur.md](02-architektur.md), Abschnitt 15.1. Ein neueres, aber verträgliches Schema
   ist erlaubt; das ist der Rückweg aus Punkt 2.
5. Ein Major-Upgrade von PostgreSQL ist ein eigener Schritt mit Wartungsfenster.

Regeln im Detail und Ablauf: [upgrade.md](../operations/upgrade.md).

## 11. Umzug und Cloud-Portabilität

Ein Umzug ist eine Wiederherstellung auf einem anderen Ziel ohne Datenverlust:

1. Ziel bereitstellen (OpenTofu), gleiche Version, Secrets-Bündel einspielen.
2. Probe: letzte Sicherung zurückspielen, Prüfungen aus 8.3 grün.
3. Wartungsmodus `server.maintenance_mode = read_only` auf der alten Instanz
   ([ADR 0065](../adr/0065-wartungsmodus-read-only.md)). Aktionen mit `effect: write`
   (05, Abschnitt 2.7) bekommen `503` mit Fehlercode `maintenance` und `Retry-After` (Kapitel 06
   legt Code und Format fest). Clients halten ihr Ausgangsfach. Ausnahmen nennt ADR 0065, Punkt 4:
   Abschluss laufender Läufe, Token-Erneuerung ohne Rotation, Notfall-Widerrufe und Zustellungen.
4. Abschluss-Sicherung, Restore auf dem Ziel, Manifest vergleichen.
5. DNS und Identität umstellen: Redirect-URIs bei OIDC, RP-ID der Passkeys (gleiche Domain behält
   die Passkeys), Tailscale-Gerät. Push-Schlüssel und App-Identität bleiben gleich, darum bleiben
   die Push-Registrierungen gültig.
6. Clients verbinden mit demselben Cursor neu; weil kein Ereignis verloren ging, bleibt die
   Epoche gleich und es gibt kein Neuladen.

Ändert sich der Hostname, brauchen die Apps die neue Adresse. Geplant: Die alte Instanz meldet im
Wartungsmodus über `instance.discovery.read` ein `moved_to`, signiert mit dem Instanzschlüssel;
der Client übernimmt es nur bei gleicher `instance_id` (Frage an Team B, siehe Bericht).
Anleitung: [cloud-migration.md](../operations/cloud-migration.md).

### 11.1 Portabilitätstest in CI

Ein Test belegt die Portabilität nach jedem erfolgreichen Release des Produkts (Komponente
`bothy`, Tag `vX.Y.Z`, auch Beta) und auf Anforderung (`workflow_dispatch` mit einem
Produkt-Tag), nicht nächtlich: Er prüft ein veröffentlichtes Release, auf frischen
GitHub-gehosteten Runnern
([ADR 0039](../adr/0039-nur-github-gehostete-runner-oeffentlich-ab-dem-ersten-push.md)). `release.yml` ruft `portability.yml` erst nach dem
Job `publish` auf, weil `install.sh` und das Bündel erst dann unter dem Tag liegen. Server-Tags
(`server/v*`) starten ihn nicht: Unter ihnen liegt kein `install.sh`. Eine Server-Version prüft
der Test des Produkt-Releases, das sie installiert (`BOTHY_VERSION` im Bündel,
[18-release.md](18-release.md), Abschnitt 2.1). Der Job checkt in
beiden Startarten den geprüften Produkt-Tag aus, nicht den Stand, von dem der Lauf startet: So
passen `make portability` und `infra/compose/smoke.sh` zum Release. Ablauf:

1. Frischer Linux-Runner (amd64 und arm64), nur Docker; Images per Digest aus der Registry. An
   der Registry meldet sich der Job nicht an; er zieht wie ein Betreiber ohne Anmeldung, und ein
   noch privates Paket lässt ihn scheitern
   ([public-switch.md](../quality/public-switch.md) Schritt 21).
2. Installation mit dem Einrichtungsassistenten (3.1) ohne Terminal mit den dokumentierten
   Optionen, Quelle des Skripts `url` (3.1), einmal mit `--storage fs` (Profile `direct,scan`) und einmal mit `--storage s3` (Profile
   `direct,scan,s3`); danach ist `/readyz` grün und
   `GET /api/v1/instance/discovery` über den veröffentlichten Port antwortet mit `200`, ohne
   weiteren Handgriff.
3. Smoke-Tests über die öffentliche API: Owner aus dem Einrichtungsassistenten, Bereich, Kanal, Thread,
   Nachricht, WebSocket-Empfang, Datei-Upload sauber und mit EICAR-Testdatei, Lauf eines
   Test-Adapters, Suche, Statusseite.
4. Sicherung auslösen, Wiederherstellungstest auslösen, alle Prüfungen grün.
5. Telemetrie über das Overlay `local` empfangen; Datenschutz-Kanarie (Kapitel 17) nicht gefunden.
6. Ergebnis als Status-Check am Release.

**Schnittstelle von `make portability`.** `portability.yml` übergibt nur diese drei
Umgebungsvariablen und keine Option von `install.sh`. M7-14 baut die Schnittstelle, M7-29 legt
die Optionen fest.

| Variable | Inhalt | Ohne Wert |
|---|---|---|
| `PORTABILITY_TAG` | geprüfter Produkt-Tag (`vX.Y.Z` oder `vX.Y.Z-beta.N`) | `make portability` bleibt der Rauchtest aus M1-73 (`infra/compose/smoke.sh` mit dem lokal gebauten Image, ohne `install.sh`) |
| `PORTABILITY_INSTALL_SOURCE` | Quelle von `install.sh`: `url` oder `release-download` (Tabelle in 3.1) | Abbruch vor der Installation, wenn `PORTABILITY_TAG` gesetzt ist |
| `PORTABILITY_RELEASE_DIR` | Ordner mit `install.sh` und `bothy-compose-<version>.tar.gz` aus `gh release download` | bei `release-download` Abbruch vor der Installation; bei `url` nicht gelesen |

Regeln:

1. Mit `PORTABILITY_TAG` führt `make portability` den Test aus `tests/portability` aus: die
   Schritte 2 bis 5 des Ablaufs oben. Ein ungültiger Wert einer der drei Variablen bricht vor der
   Installation ab und nennt die Variable.
2. Die Optionen nach `sh -s --` stehen genau einmal, in `tests/portability/install-options`
   (M7-29 legt die Datei an, M7-14 liest sie). Der Check aus M7-29 vergleicht sie mit dem
   zweiten Block aus 3.1; nur `--storage` (setzt der Test je Lauf) und `--bundle` dürfen
   abweichen.
3. Quelle `url`: Der Test baut die URL des zweiten Blocks aus 3.1 selbst, mit
   `GITHUB_REPOSITORY` an der Stelle von `OWNER/bothy` und `PORTABILITY_TAG` an der Stelle von
   `v0.0.0`. GitHub setzt `GITHUB_REPOSITORY` in jedem Schritt auf "The owner and repository
   name" (https://docs.github.com/en/actions/reference/workflows-and-actions/variables,
   abgerufen 2026-10-11) [belegt]; ein Lauf außerhalb von GitHub Actions setzt sie selbst. Die URL
   liest er nicht aus der Doku des Tags. So ist für die URL von `install.sh` gleich, ob der Tag
   vor oder nach dem Ersetzen von `OWNER` (V1-03) entstand;
   [public-switch.md](../quality/public-switch.md) Schritt 21 nimmt den letzten Produkt-Tag. Die
   Registry der Images steht nicht in dieser URL, sondern im Bündel des Tags; dort setzt sie das
   Release aus dem Owner des Repositorys ([18-release.md](18-release.md), Abschnitt 2.1).
4. Quelle `release-download`: `sh install.sh --bundle <bündel>` aus `PORTABILITY_RELEASE_DIR`
   mit denselben Optionen; die Prüfsumme prüft `install.sh` wie beim Download (3.1, Schritt 2).

Der Test gehört zur Gate-Kette von Team H (ADR 0038, 0039); dieses Kapitel legt den Inhalt fest.

## 12. Statusseite

| Teil | Regel |
|---|---|
| Komponenten | `status_components`; bei der Einrichtung angelegt: `api` (Nachrichten und Sync), `push`, `files` (Dateien und Scan), `search`; je Agent eine Komponente mit `agent_id` |
| Automatischer Zustand | Der `worker` prüft alle `status.check_interval` die Quellen aus `health_source` und setzt den Zustand. Eine Prüfung scheitert, wenn sie keine Antwort bekommt oder ein Fehleranteil über `status.error_ratio_degraded` liegt; nach `status.degrade_after` mit scheiternden Prüfungen gilt `degraded`, über `status.error_ratio_outage` oder ohne jede Antwort `major_outage` |
| Störungen | Dauert `major_outage` länger als `status.incident_after`, legt der `worker` eine Störung an (`created_by` NULL). Nach Erholung und `status.resolve_after` setzt er sie auf `resolved`. Der Owner kann jederzeit Texte ergänzen (`status.manage`) |
| Agenten | Zustand aus `agents.reported_runtime_state` (Heartbeat). Menschen sehen nur Agenten mit `agent.read` |
| Sichtbarkeit | `status.read` für angemeldete Prinzipale; ohne Anmeldung nur mit `status.public_enabled` |
| Grenze | Die Statusseite läuft im Server. Fällt der ganze Server aus, zeigt die App "Server nicht erreichbar"; den Owner erreicht dann nur der externe Alarmweg (Kapitel 17, Watchdog) |

| `health_source` | Prüfung |
|---|---|
| `api.http` | Anteil 5xx seit der letzten Prüfung, aus allen `api`-Prozessen |
| `sync.fanout` | Fan-out-Verzug aller `api`-Prozesse unter `status.fanout_lag_degraded` |
| `push.apns`, `push.fcm` | Fehleranteil der Sendungen seit der letzten Prüfung, ohne ungültige Tokens |
| `files.storage` | Schreiben, Lesen, Löschen eines Prüfobjekts |
| `files.scan` | `PING` an clamd |
| `search.query` | Testabfrage gegen den Index |
| `agent.presence` | Zustand `online` des Agenten der Komponente |

Sicherung und Wiederherstellungstest sind keine Komponenten der Statusseite, weil diese alle
Menschen sehen. Ihr Zustand steht in der Admin-Konsole (`backup_runs`) und in den Alarmen.

## 13. Kapazität und Ressourcen

Feste Größen gibt es erst nach dem Lasttest: M1-72 bestätigt oder ändert `database.max_conns` mit
Messwerten, M7-18 füllt die Kapazitätstabelle. Bis dahin gelten Startwerte aus der Konfiguration
und dieses Modell:

| Engpass | Kennzahl | Hebel |
|---|---|---|
| Serieller Abschnitt am Zähler `event_seq` ([04-sync-protokoll.md](04-sync-protokoll.md) 3.2) | `bothy.db.event_seq.lock_wait` | schnellerer Datenträger für WAL, kürzere Transaktionen |
| Verbindungen zur Datenbank | `postgresql.backends`, Pool-Kennzahlen | `database.max_conns`, mehrere `api`-Prozesse nur mit Pooler |
| WebSocket-Verbindungen | `bothy.gateway.connections.active` | weitere `api`-Prozesse; Fan-out liest jeder Prozess selbst |
| Fan-out-Verzug | `bothy.event.fanout.lag` | weniger Empfänger je Prozess |
| Speicher ClamAV | Container-Speicher | eigener Host für den Scan |

Plattenbedarf schätzt der Betreiber mit:

```
Datenbank   ≈ Nachrichten pro Tag × mittlere Größe × Tage
            + Ereignisse pro Tag × mittlere Nutzlast × sync.event_retention in Tagen
            + Indexe (ihren Anteil misst der Lasttest)
WAL-Archiv  ≈ WAL pro Tag × Tage bis zur ältesten behaltenen Vollsicherung
Dateien     ≈ Uploads pro Tag × mittlere Größe × Tage (plus Vorschaubilder)
Sicherungen ≈ Vollsicherungen × Datenbank + WAL-Archiv + restic-Snapshots (dedupliziert)
```

Die Kennzahl `bothy.storage.volume.free` warnt rechtzeitig (Alarm `BothyVolumeFull`).

## 14. Konfigurationswerte dieses Kapitels

Spalte "Ort": Server = Konfigurationsreferenz des Servers; Installationsskript = Option von
`install.sh` (3.1), keine Variable des Servers.

| Name | Ort | Standard | Bedeutung |
|---|---|---|---|
| `instance.environment` | Server | `prod` | `dev`, `staging` oder `prod` (6.1) |
| `server.role` | Server | `all` | `api`, `worker` oder `all` |
| `server.listen_addr` | Server | `127.0.0.1:8080` | öffentlicher Listener (REST, Gateway) |
| `server.internal_listen_addr` | Server | `127.0.0.1:8081` | Health, Readiness, Eingangs-Webhooks aus dem Stack |
| `server.public_url` | Server | keiner, Pflicht | Origin, unter dem Apps den Server erreichen (Schema, Host, optional Port; ohne Pfad, Query und Fragment, sonst scheitert `bothy config check`); Grundlage von `aud`, DPoP-`htu` und WebAuthn ([08-identitaet.md](08-identitaet.md), Abschnitte 7.2 und 7.3) |
| `server.trusted_proxies` | Server | `127.0.0.1/32, ::1/128` | Gegenstellen, deren Weiterleitungs- und Identitäts-Kopfzeilen gelten und die Klartext senden dürfen ([19-sicherheit.md](19-sicherheit.md), SEC-TLS-1) |
| `server.maintenance_mode` | Server | `off` | `off` oder `read_only` (Umzug, Upgrade; [ADR 0065](../adr/0065-wartungsmodus-read-only.md)) |
| `secrets.dir` | Server | `/run/bothy/secrets` | Basis der Standardverweise `file:` |
| `secrets.cache_ttl` | Server | `5m` | so lange gilt ein gelesenes Geheimnis |
| `secrets.signing_key_dir` | Server | `/var/lib/bothy/signing-keys` | beschreibbares Verzeichnis der privaten Signierschlüssel; im Besitz des Dienstbenutzers, Modus `0700` (5.2, [ADR 0066](../adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md)) |
| `server.tls_cert_ref` | Server | leer | Verweis auf die Zertifikatskette (PEM, Blatt zuerst); leer: der Server beendet kein TLS (SEC-TLS-2) |
| `server.tls_key_ref` | Server | leer | Verweis auf den privaten Schlüssel (PEM); nur zusammen mit `server.tls_cert_ref` |
| `database.host` | Server | `localhost` | Host der Datenbank |
| `database.port` | Server | `5432` | Port der Datenbank im Netz des Servers |
| `database.name` | Server | `bothy` | Datenbankname |
| `database.user` | Server | `bothy_app` | Rolle des Servers |
| `database.password_ref` | Server | `file:<secrets.dir>/db_app_password` | Verweis auf das Passwort |
| `database.sslmode` | Server | `verify-full` | TLS zur Datenbank; im internen Compose-Netz `disable` (SEC-TLS-5) |
| `database.sslrootcert` | Server | leer | CA-Bündel für `verify-full`; leer nutzt die Wurzeln des Systems |
| `database.max_conns` | Server | `20` | Größe des Verbindungspools je Prozess (Startwert bis zum Lasttest) |
| `migrate.require_recent_backup` | Server | `true` | `migrate` verlangt eine frische Sicherung |
| `migrate.recent_backup_max_age` | Server | `24h` | so alt darf diese Sicherung sein |
| `migrate.lock_timeout` | Server | `5s` | `lock_timeout` je Migrationsschritt |
| `migrate.bootstrap_user` | Server | `postgres` | Superuser-Anmeldung nur für `migrate up --bootstrap` |
| `migrate.bootstrap_password_ref` | Server | `file:<secrets.dir>/db_superuser_password` | dessen Passwort |
| `migrate.owner_password_ref` | Server | `file:<secrets.dir>/db_owner_password` | Passwort, das `--bootstrap` für `bothy_owner` setzt |
| `migrate.app_password_ref` | Server | `file:<secrets.dir>/db_app_password` | Passwort für `bothy_app` |
| `migrate.maint_password_ref` | Server | `file:<secrets.dir>/db_maint_password` | Passwort für `bothy_maint` |
| `migrate.monitor_password_ref` | Server | `file:<secrets.dir>/db_monitor_password` | Passwort für `bothy_monitor` |
| `backup.pg.full_schedule` | Server | `0 2 * * 0` | Plan der Vollsicherung (cron) |
| `backup.pg.diff_schedule` | Server | `0 2 * * 1-6` | Plan der Differenzsicherung (cron) |
| `backup.pg.archive_timeout` | Server | `60s` | spätestens dann wird ein WAL-Segment archiviert |
| `backup.pg.retention_full` | Server | `4` | behaltene Vollsicherungen in repo2 |
| `backup.pg.local_retention_full` | Server | `1` | behaltene Vollsicherungen in repo1 |
| `backup.logical.schedule` | Server | `0 3 * * *` | Plan von `pg_dump` (cron) |
| `backup.logical.retention` | Server | `30d` | Aufbewahrung der logischen Sicherung |
| `backup.objects.schedule` | Server | `30 3 * * *` | Plan der Objektsicherung (cron) |
| `backup.objects.retention` | Server | `90d` | Aufbewahrung der Objekt-Snapshots |
| `backup.offsite.target` | Server | leer | Off-site-Ziel; leer heißt nur lokal (Warnung in Konsole und Status) |
| `backup.max_age` | Server | `26h` | ohne erfolgreiche Sicherung länger als das: Alarm |
| `backup.restore_test.schedule` | Server | `0 5 * * 6` | Plan des Wiederherstellungstests (cron) |
| `backup.restore_test.sample_files` | Server | `200` | geprüfte Dateien je Test |
| `backup.restore_test.sample_threads` | Server | `200` | geprüfte Threads je Test |
| `backup.restore_test.max_age` | Server | `8d` | ohne erfolgreichen Test länger als das: Alarm |
| `dr.rpo_target` | Server | `5m` | Ziel für den größten Datenverlust |
| `dr.rto_target` | Server | `4h` | Ziel für die Dauer der Wiederherstellung |
| `status.check_interval` | Server | `30s` | Abstand der automatischen Prüfungen |
| `status.degrade_after` | Server | `2m` | so lange Fehler bis `degraded` |
| `status.incident_after` | Server | `5m` | so lange `major_outage` bis zur automatischen Störung |
| `status.resolve_after` | Server | `10m` | so lange gesund bis zur automatischen Auflösung |
| `status.error_ratio_degraded` | Server | `0.05` | Fehleranteil, ab dem eine Prüfung scheitert |
| `status.error_ratio_outage` | Server | `0.5` | Fehleranteil, ab dem `major_outage` gilt |
| `status.fanout_lag_degraded` | Server | `5s` | Fan-out-Verzug, ab dem `sync.fanout` scheitert |
| `install.ready_timeout` | Installationsskript | `5m` | so lange wartet `install.sh` nach `docker compose up -d` auf `/readyz` (Option `--ready-timeout`) |

## 15. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-OP-01 | Zwei Listener: öffentlich und intern. | Health und interne Webhooks brauchen kein Header-Vertrauen und keinen öffentlichen Weg. |
| R-OP-02 | Header-Identität nur mit öffentlichem Listener auf Loopback; sonst bleibt der Anbieter aus. | Erzwingt die Bedingung aus B9 statt sie anzunehmen. |
| R-OP-03 | Tailscale als Sidecar mit geteiltem Netz-Namespace und Kernel-Netz (kein Userspace-Modus), statt `tailscale serve` auf dem Host; dazu Tailnet-Richtlinie nur Port 443. | Nur so lauscht der Server im Container nachweisbar nur auf Loopback; ob der Userspace-Modus an Loopback weiterreicht, ist nicht dokumentiert. |
| R-OP-04 | Unbekannte Konfigurationsschlüssel brechen den Start ab. | Ein Tippfehler darf nie still einen Standard setzen. |
| R-OP-05 | Geheimnisse als Dateien je Dienst (`subpath`), nie als Umgebungsvariablen. | Kindprozesse, Absturzberichte und `docker inspect` zeigen keine Werte; jeder Dienst sieht nur seine Geheimnisse. |
| R-OP-06 | pgBackRest für physische Sicherung und PITR, `pg_dump` für Portabilität, restic für Objekte ([ADR 0058](../adr/0058-sicherung-mit-pgbackrest-objektsicherung-und-restore-test.md)). | Bewährte Werkzeuge mit Prüfsummen, Verschlüsselung und Aufbewahrung; kein Eigenbau. |
| R-OP-07 | Eigenes Image `bothy-postgres` (postgres:18 plus pgBackRest und `bothy-backupd`). | `archive_command` läuft im Datenbank-Container; ein Image für beide Rollen hält Versionen gleich. |
| R-OP-08 | Repository-Host `backup` per pgBackRest-TLS; der Datenbank-Container bleibt ohne Ausgang. | Off-site-Zugangsdaten liegen nur dort, wo sie gebraucht werden. |
| R-OP-09 | Off-site-Sicherungen nur anhängend; Löschen nur mit getrenntem Konto. | Ein übernommener Server darf seine Sicherungen nicht zerstören. |
| R-OP-10 | Automatischer Wiederherstellungstest mit Manifest-Vergleich, Sequenz- und Hash-Prüfung, Stichproben ([ADR 0058](../adr/0058-sicherung-mit-pgbackrest-objektsicherung-und-restore-test.md)). | Eine Sicherung ist erst belegt, wenn sie zurückgespielt wurde. |
| R-OP-11 | Wiederherstellung setzt eine neue Sync-Epoche (03, 04, [ADR 0059](../adr/0059-sync-epoche-nach-wiederherstellung.md)). | Ohne Epoche überholt die neue Zeitlinie alte Cursor still. |
| R-OP-12 | Expand/Contract mit Rückweg um genau eine Version; keine Down-Migrationen in Produktion. | Rückweg ohne Datenbank-Rückbau; zwei Versionen zurück braucht eine Wiederherstellung. |
| R-OP-13 | `migrate` verlangt eine frische Sicherung. | Erzwingt den wichtigsten Schritt vor jeder Schemaänderung. |
| R-OP-14 | Feature-Flags mit Art `release` und `remove_by`, Gate-Check; Flags nie für Sicherheitsgrenzen ([ADR 0064](../adr/0064-feature-flag-registrierung-mit-aufraeum-gate.md)). | Verhindert tote Flags und versteckte Sicherheitsschalter. |
| R-OP-15 | Statusseite im Server, automatische Zustände aus internen Prüfungen, nicht aus Prometheus. | Funktioniert ohne optionalen Observability-Stack. |
| R-OP-16 | Produktionsdaten nie in `dev` oder `staging`. | Kein Anonymisierungswerkzeug; ein Abzug wäre ein Datenschutzvorfall. |
| R-OP-17 | DDL im Betrieb nur über `SECURITY DEFINER`-Funktionen von `bothy_owner`; Tabellenrechte je Migration ausdrücklich, keine Standardrechte in `public`. | Die Anwendungsrolle bleibt bei DML; ein fehlendes Recht fällt im Test auf. |
| R-OP-18 | Signierschlüssel erzeugt der Server in einem eigenen beschreibbaren Verzeichnis ([ADR 0066](../adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md)). | Rotation ohne Handgriff auf dem Host; alle anderen Geheimnisse bleiben nur lesbar. |
| R-OP-19 | Hash-Routing über die Sitzungs-ID für beide Strom-Pfade des Terminals als Betriebsanforderung mit mehreren Knoten. | Kein Broker im Server; ein Knoten hält beide Enden einer Sitzung ([24-terminal.md](24-terminal.md), Abschnitt 9). |
| R-OP-20 | Einrichtungsassistent als POSIX-Shell-Skript `install.sh` auf dem Host, Release-Artefakt mit eingebauter Version und Prüfsumme des Bündels; kein Assistent im Container. | Compose muss auf dem Host laufen; ein Assistent im Container bräuchte den Docker-Socket, also Root-Rechte auf dem Host. Die Shell gibt es überall, wo Docker läuft. |
