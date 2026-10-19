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

1. Ein Befehl startet eine vollständige Instanz auf einem Host mit Docker-kompatibler Laufzeit.
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

As built (M7-01, [image README](../../services/server/images/bothy-postgres/README.md)): the image
runs both roles as the user `postgres` of the base image (uid and gid `999`); its entry point
writes the pgBackRest configuration of the role. Besides PostgreSQL, the container `postgres` runs
a pgBackRest TLS server through which the repository host `backup` runs its backups; `backup`
runs `bothy-backupd`, which supervises the TLS server of the repository and creates the stanza
`bothy` on its first start.

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

### 3.1 Installation in one command (setup wizard)

Goal 1 applies to strangers without prior knowledge. The tool is the installation script `install.sh`
(POSIX shell, source `infra/compose/install.sh`), a release artifact of the component `bothy`
next to the installation bundle ([18-release.md](18-release.md), section 2). It is the
setup wizard. The manual path from [install-compose.md](../operations/install-compose.md)
stays equivalent; the script does nothing that the manual path does not also do. The documented
command names the version of the bundle:

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

Without a terminal (CI, automation), all answers are available as options after `sh -s --`:

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh \
  | sh -s -- --yes --public-url https://chat.example.com --edge direct --scan on --storage s3 \
      --owner-handle alex --owner-name Alex
```

V1-03 replaces `OWNER`, the release chain replaces the version. No reference to `releases/latest`: in
the monorepo, the newest release is often a different component.

The portability test (11.1) passes exactly the options after `sh -s --` from the second block;
only `--storage` changes per run. Where the options are and how the test is called is defined in
11.1 (interface of `make portability`). The source of the script is a parameter there
(`PORTABILITY_INSTALL_SOURCE`), because GitHub Releases are shown only to people with read
permission on the repository ("Anyone with read access to a repository can view and compare releases",
https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases, retrieved
2026-10-11) [verified]:

| Value | Source | Invocation |
|---|---|---|
| `url` (this is how `portability.yml` runs) | Release URL of the tag without login | the second block, with `OWNER` and version of the tag |
| `release-download` | `gh release download <tag>` with a token with read permission: `install.sh` and the bundle `bothy-compose-<version>.tar.gz` | `sh install.sh --bundle <bundle>` and the same options |

Steps:

1. **Preflight check.** Docker Engine and Compose in the minimum version (section 3, rule 7),
   architecture, memory of the Docker VM (note for the profile `scan`, rule 2), target folder
   (default `./bothy`, option `--dir`) empty or an earlier installation of this script. On
   an error it aborts before it writes anything.
2. **Fetch the bundle.** It downloads the installation bundle of exactly its built-in version from its
   built-in download address (`https://github.com/<repository>/releases/download/v<version>/`, set
   at release time from the name of the repository, 18 section 2.1 rule 3, not from the source with
   `OWNER`), checks its SHA-256 against the built-in value and unpacks it. With `--bundle <file>` it
   takes this file instead of the download; the checksum check stays the same. If `cosign` is in the
   path, it additionally checks the signature from the same address (18, section 6); otherwise it
   names the verification command.
3. **Prompts.** Only with a terminal (input via `/dev/tty`, so that the command works with a pipe):
   public address (`BOTHY_PUBLIC_URL`), edge `direct` or `tailscale` (then `TS_AUTHKEY`
   hidden), virus scan (default on if the memory suffices), file storage `fs` (default,
   volume `files`) or `s3` (profile `s3` with the object storage in the stack, sets
   `BOTHY_STORAGE_DRIVER=s3`), local observability stack (default off), off-site target
   (optional), handle and display name of the first owner. Every prompt has an option
   (`--public-url`, `--edge direct|tailscale`, `--scan on|off`, `--storage fs|s3`,
   `--observability on|off`, `--offsite`, `--owner-handle`, `--owner-name`) and `--yes` takes
   the defaults; if a required value (public address, handle and display name of the owner) is
   missing without a terminal, it aborts instead of guessing.
4. **Write.** `.env` from `.env.example` of the bundle with the answers, mode `0600`;
   `secrets/` with mode `0700`. It takes `BOTHY_VERSION` and `BOTHY_REGISTRY` unchanged
   from the bundle. `BOTHY_VERSION` is the server version, not the script's own version: product and
   server have separate versions ([18-release.md](18-release.md), section 1). The product release
   writes both values into the bundle; no bundle with a placeholder is produced there
   ([18-release.md](18-release.md), section 2.1). No secret in `.env` except the one-time
   `TS_AUTHKEY` (R-OP-05). `secrets-init` generates all generatable secrets on the first start (5.3).
5. **Check and start.** `docker compose config --quiet`, then `docker compose up --no-start`
   (creates networks and containers, starts nothing). With the edge `direct`, it then reads the gateway
   of the Compose network `default` (network name from `docker compose config --format json`, field
   `networks.default.name`; `docker network inspect`, field `IPAM.Config[].Gateway`) and writes
   it as `<gateway>/32` (IPv6 `/128`) into `BOTHY_TRUSTED_PROXIES` of `.env`. Reason: in the profile
   `direct`, the server sees the gateway as the peer, not the reverse proxy; with the
   default (loopback only) it rejects every request via the published port (4.3,
   SEC-TLS-1). Then `docker compose up -d` and `bothy healthcheck` against `/readyz` in the
   API container (like [install-compose.md](../operations/install-compose.md), section 4),
   for at most `install.ready_timeout`. Finally it checks the path from outside:
   `GET /api/v1/instance/discovery` via `127.0.0.1:BOTHY_HTTP_PUBLISH_PORT` (edge `direct`)
   answers with `200`, otherwise it aborts with a hint to `BOTHY_TRUSTED_PROXIES`.
6. **First owner.** `bothy setup bootstrap-owner` with handle and display name
   ([08-identitaet.md](08-identitaet.md), section 4). The script shows the enrollment code once
   and writes it to no file and no log.
7. **Finish.** Next steps with command: open the app, enter the server address and enrollment code
   ([01-produkt.md](01-produkt.md), section 5.6, E1 and E3); escrow secrets
   (5.4); with an empty off-site target, the warning "local backup only". With the edge `tailscale`, it
   clears `TS_AUTHKEY` in `.env` as soon as the first login has succeeded.

Rules:

1. A second run in the same folder overwrites neither `.env` nor `secrets/`. Only exception:
   if `BOTHY_TRUSTED_PROXIES` is still at the default (loopback only) with the edge `direct`, it sets
   the gateway as in step 5; it leaves any other value as it is. It starts the stack and
   shows the state; with an owner already set up, step 6 is skipped (the command then aborts anyway
   without a change, 08 section 4).
2. The script never calls `sudo` and installs no packages, not even Docker; the preflight check
   names the way. Reason: a script from the network should change nothing outside its folder.
3. Hosts: Linux and macOS. On Windows it runs in WSL 2 with Docker Desktop [derived: there
   is a POSIX shell and the same Docker CLI there; the build ticket verifies it].
4. Every abort ends with an exit code other than `0` and names the step and the reason on
   standard error.
5. Verified by the portability test (11.1): it installs on the fresh runner without a terminal
   with exactly the documented options (second block above, `--yes` and options; only
   `--storage` changes per run). The source of the script is a parameter (table above); with
   `release-download` only `--bundle` is added. ShellCheck checks the script in the gate.

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

Installation of the host service on a host (M2-42; [24-terminal.md](24-terminal.md) 2.2; the
rows `D-M2-42-<n>` of [DECISIONS.md](../DECISIONS.md) give the reasons):

1. Users without administrator rights. `bothy-host install` (once, as root or an elevated member of
   `Administrators`) creates the service user `bothy-host` with the state directory
   `host_service.state_dir` (mode `0700`, on Windows a protected DACL for the virtual account
   `NT SERVICE\bothy-host`, `SYSTEM` and `Administrators`), one session user per person from
   `host_service.session_users` with a home only for that user (on macOS hidden, with an own
   primary group and not in `staff`), one PTY helper per session user as a system service under
   that user, and the host service. Program folder: Linux `/opt/bothy-host/bin/`, macOS
   `/Library/Application Support/bothy-host/bin/` (the binary is signed ad hoc with the hardened
   runtime unless it already carries a signature with it), Windows `%ProgramFiles%\bothy-host\`.
   Configuration file: Linux `/etc/bothy-host/config.yaml`, macOS
   `/Library/Application Support/bothy-host/config.yaml`, Windows
   `%ProgramFiles%\bothy-host\config.yaml`; `install` writes it only when it is missing.
   `bothy-host session-user add <name>` adds a person later and refuses a name the state directory
   already binds to a person; `bothy-host session-user remove <name>` takes a name out, removes its
   helper and closes its home, keeps the binding and ends no process of the user. Right after
   `remove` the operator deletes the user with its long runners (crontab entries, scheduled tasks,
   user services), because a long runner can reopen the home and stop the host through HD9, and
   never gives the name to another person.
2. Services of the operating system: systemd (Linux: `bothy-host.service`,
   `bothy-host-helper-<name>.socket` and `.service`), launchd (macOS: `bothy-host` and
   `bothy-host.helper.<name>` in `/Library/LaunchDaemons`), Windows services (`bothy-host` and
   `bothy-host-helper-<name>`). A crash is restarted; a refused start (exit code `78`) is not.
3. `bothy-host pair --server <url> --code <code> [--ca-file <pem>]` pairs the host (24, 3.2) with
   the identity of the service user: as an administrator it goes through the running service
   (Windows: the service must run) and restarts it afterwards; it prints the fingerprint in groups
   of four. `bothy-host doctor` runs HD1 to HD11 before every start and on command; as an
   administrator it runs the full run inside the running service. Exit codes: `0` passed (warnings
   allowed), `1` a check failed, `2` usage or environment error (missing administrator rights,
   unreadable configuration).
4. Not on the server host. Where it is needed anyway, the owner declares the host as server host;
   then only owners get a terminal there (07, SR10).
5. For the profile `claude` at `organization` the operator puts the API key as the file
   `claude_api_key` with mode `0600` into the state directory
   ([24-terminal.md](24-terminal.md), section 7.4). Never as an environment variable of the
   service.
6. After a refused start the host service stays stopped. The operator fixes the finding that
   `bothy-host doctor` names and runs `bothy-host restart`; the same command makes a changed
   configuration take effect.

Windows drive root: `install` asks before it removes the entry `Authenticated Users:(AD)` from the
root of the system drive and of every drive with a place; `--drive-root-confirm yes|no` answers
for unattended installs, and without a terminal and without the flag the answer is no.
`bothy-host install --drive-root <drive>` does only this step for a drive that gets a place later.

Peer check: on a platform whose platform run has not proven the peer check of the local channel,
HD10 fails with `peer_check_unavailable` and the host gives no terminal (RK-23); the operator
waits for a release with the recorded platform run.

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

Regeln des Providers (`internal/platform/secrets`, Entscheidungen M1-09 in
[DECISIONS.md](../DECISIONS.md)):

1. Grammatik: `file:` mit absolutem Pfad in sauberer Form (kein `.` oder `..`, kein doppelter oder
   abschließender `/`, kein NUL-Zeichen), `env:` mit einem Namen aus `[A-Z_][A-Z0-9_]*`; `azkv`
   und `awssm` meldet M1 als nicht unterstützt. `<secrets.dir>` steht im Verweis für den
   wirksamen Wert von `secrets.dir`, im Standardwert wie in einem gesetzten Wert.
2. Datei-Treiber: Er folgt Symlinks und liest nur eine reguläre Datei mit Modus `0400` oder
   `0600`, deren Besitzer der Prozessbenutzer ist (SEC-SEC-1); geprüft wird an der geöffneten
   Datei, damit sie zwischen Prüfung und Lesen nicht wechseln kann. Genau ein Zeilenende am
   Schluss (`\n` oder `\r\n`) gehört nicht zum Wert, weil `echo wert > datei` der übliche Weg ist,
   eine Datei anzulegen, und ein Zeilenende nie Teil eines Passworts ist. Eine danach leere Datei
   gilt als nicht gefunden.
3. Env-Treiber: Er liest die Umgebung, die der Prozess beim Start bekommen hat; eine fehlende oder
   leere Variable gilt als nicht gefunden.
4. Cache: Je Verweis gilt ein gelesener Wert höchstens `secrets.cache_ttl` lang; danach liest der
   Provider neu, und eine neue Version wirkt ab diesem Zugriff. Gleichzeitige Zugriffe auf einen
   abgelaufenen Verweis lesen einmal. Eine Uhr, die vor dem Lesezeitpunkt steht, gilt als
   abgelaufen.
5. Scheitert das erste Lesen, bricht der Start ab ([02-architektur.md](02-architektur.md) 14.4).
   Scheitert ein erneutes Lesen, bleibt der alte Wert; der Provider schreibt eine Logzeile mit
   `error.type` (nur der Fehlercode), erhöht `bothy.secrets.refresh.failures` (Alarm
   `BothySecretRefreshFailing`) und versucht es nach einer weiteren `secrets.cache_ttl` erneut.
6. Fehler nennen den Verweis und die verletzte Regel (bei Dateien Modus und Besitzer), nie den
   Inhalt; den Text eines ungültigen Verweises nennen sie nicht, weil dort ein Geheimnis stehen
   kann. Fehlercodes: `invalid_ref`, `unsupported_scheme`, `not_found`, `insecure_file`,
   `read_failed`.

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
   `postgres` and `backup` get the owner `999:999`, the user `postgres` of `bothy-postgres`,
   because pgBackRest refuses TLS keys that another user can read (M7-01). Third-party services
   without `--owner` (collector and the others) get copies owned by root with directory `0755`
   and files `0444`, because their images have their own users; each one still sees only its
   own subdirectory.
5. `secrets-init` gibt am Ende die Fingerabdrücke aus, nie die Werte, und erinnert an die
   Hinterlegung (5.4). Katalogeinträge, deren Erzeuger eine spätere Etappe baut, meldet es als
   `pending` (Konfigurationsreferenz, Abschnitt "Geheimnisse"). Je Eintrag steht eine Zeile
   `<status> <name> <fingerabdruck>` auf der Standardausgabe mit dem Status `generated`,
   `adopted`, `kept`, `updated`, `missing` (gelieferter Eintrag fehlt) oder `pending`; der
   Fingerabdruck ist `sha256:` mit den ersten zwölf Hex-Zeichen des SHA-256 des Inhalts (bei einem
   Verzeichnis-Eintrag des SHA-256 der sortierten Zeilen `<relativer pfad> <sha256 der datei>`),
   bei `missing` und `pending` fehlt er. Exitcodes: `0` Erfolg, `1` Laufzeitfehler, Konflikt oder
   ein zweiter gleichzeitiger Lauf auf demselben Volume, `2` Aufruffehler (unbekannte oder nicht
   reguläre Datei der Quelle, gelieferte Datei ohne Wert, vorhandenes Ziel mit Dateien, aber ohne
   `_store`, ungültiger oder unbekannter `--owner`, fehlender `--owner` für `server` oder
   `migrate`, `--adopt-source` ohne `--source`). Scheitert
   eine Prüfung, schreibt `secrets-init` nichts. Die letzte Zeile nennt den Export-Befehl mit dem
   `_store` des Laufs (5.4).

### 5.4 Hinterlegung (Escrow)

Ohne einige Geheimnisse ist eine Sicherung wertlos. Der Owner exportiert sie nach jeder Änderung
verschlüsselt (`bothy secrets export --from <_store> --age-recipient <öffentlicher Schlüssel>`,
Aufruf im Compose-Stack unten) und bewahrt das
Bündel getrennt vom Server und getrennt vom Off-site-Ziel auf. Das Bündel enthält `_store/` und den
Schlüsselspeicher `secrets.signing_key_dir` unter `signing-keys/` (`--signing-key-dir`, Standard
`/var/lib/bothy/signing-keys`, das `secrets-init` nur lesend einbindet; M1-32).

Aufruf im Compose-Stack, ohne Konfigurationsdatei und ohne Datenbank:

```sh
docker compose run --rm -T secrets-init secrets export \
  --from /run/bothy/secrets/_store --age-recipient age1... > bundle.age
```

`--age-recipient` darf mehrfach stehen (jeder Empfänger kann entschlüsseln); `--armor` schreibt
ASCII statt Binärdaten, ohne `--armor` verweigert der Export die Ausgabe auf ein Terminal.
`--output <datei>` legt eine neue Datei mit Modus `0600` an und überschreibt nie. Das Bündel ist
ein tar-Archiv mit den relativen Namen des Katalogs, entschlüsselt also genau die Ablage von
`infra/compose/secrets/`, die `secrets-init` übernimmt; Dateien ohne Katalogeintrag in `--from`
lehnt der Export darum mit Exitcode 2 ab, statt ein Bündel zu schreiben, das die Wiederherstellung
später verweigert. `secrets init` skips the part `signing-keys/` of a decrypted bundle; its files
go back into the volume of `secrets.signing_key_dir` ([disaster-recovery.md](../operations/disaster-recovery.md)
Fall B). Die letzte Zeile von `secrets init` nennt diesen Aufruf mit dem `_store` des
Laufs.

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
| Character classification of the database (`LC_CTYPE`) | UTF-8 capable, for example `en_US.utf8` (the official image's default); not `C` | upper and lower case of non-ASCII letters in text search: under `C` a search for "ärger" never finds "Ärger"; the start check `pg.CheckTextSearch` refuses such a database ([02-architektur.md](02-architektur.md) 15.1 step 4, M3-13) |

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
   keine Zugangsdaten des Off-site-Ziels. Both sides authenticate with certificates of the
   stack's own CA (`pgbackrest_tls/`) and accept only the peer's name; the repository is
   encrypted with `pgbackrest_cipher_pass`, which only `backup` holds.
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

### 11.1 Portability test in CI

A test proves portability after every successful release of the product (component
`bothy`, tag `vX.Y.Z`, also beta) and on request (`workflow_dispatch` with a
product tag), not nightly: it checks a published release, on fresh
GitHub-hosted runners
([ADR 0039](../adr/0039-runner-strategie-privat-oeffentlich-und-release-ausnahmen.md)). `release.yml` calls `portability.yml` only after the
job `publish`, because only then are `install.sh` and the bundle available under the tag. Server
tags (`server/v*`) do not start it: there is no `install.sh` under them. A server version is checked
by the test of the product release that installs it (`BOTHY_VERSION` in the bundle,
[18-release.md](18-release.md), section 2.1). In both start modes, the job checks out
the product tag under test, not the state from which the run starts: this way
`make portability` and `infra/compose/smoke.sh` match the release. Flow:

1. Fresh Linux runner (amd64 and arm64), Docker only. The server images come, as with every
   installation, by their tag from `BOTHY_REGISTRY` and `BOTHY_VERSION` in the bundle of the tag
   ([18-release.md](18-release.md), section 2.1), not by digest. `install.sh` pulls and starts in
   one run (3.1, step 5); that is why the test pulls and checks them itself beforehand. It reads
   both values from the `.env.example` of the bundle (source `release-download`: from
   `PORTABILITY_RELEASE_DIR`; source `url`: downloaded from the address at which `install.sh`
   looks for the bundle, interface rule 3) and pulls every image that the `compose.yaml` of the
   bundle names with `${BOTHY_VERSION}` (the same list as in [18-release.md](18-release.md),
   section 2.1, rule 4). Then it checks the signature of each of these digests (`docker image
   inspect`, field `RepoDigests`) like an instance operator
   ([release.md](../operations/release.md), section 5; ADR 0036): a digest passes if it can be
   verified with the public key of the private phase from the repository or keyless with the
   identity of the release workflow, because a tag from the private phase carries only the key
   signature even after switching to public
   ([ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md), item 5). cosign runs
   for this as a container with a digest pin (ADR 0041), because the runner brings only Docker.
   The cosign project names the image: "Signed release images are available at
   `ghcr.io/sigstore/cosign/cosign`"
   (https://docs.sigstore.dev/cosign/system_config/installation/, retrieved 2026-10-11)
   [verified]. Only then does the test call `install.sh` (step 2). Compose pulls an image only if
   it is missing locally: "Compose pulls the image only if it's not available in the platform
   cache", the default of `pull_policy` without `build`
   (https://docs.docker.com/reference/compose-file/services/, retrieved 2026-10-11) [verified],
   and `compose.yaml` sets neither of the two. So the installation starts the checked images.
   After the installation the test compares the images with which the containers of this
   installation run with the checked ones and fails on a difference; this also catches another
   bundle or a second pull. The result (step 6) names the digests. The
   job does not log in to the registry; it pulls without login like an instance operator, and a
   package that is still private makes it fail
   ([public-switch.md](../quality/public-switch.md) step 21).
2. Installation with the setup wizard (3.1) without a terminal with the documented
   options, source of the script `url` (3.1), once with `--storage fs` (profiles `direct,scan`) and once with `--storage s3` (profiles
   `direct,scan,s3`); afterwards `/readyz` is green and
   `GET /api/v1/instance/discovery` via the published port answers with `200`, without any
   further manual step.
3. Smoke tests via the public API: owner from the setup wizard, space, channel, thread,
   message, WebSocket reception, file upload clean and with the EICAR test file, run of a
   test adapter, search, status page.
4. Trigger a backup, trigger a restore test, all checks green.
5. Receive telemetry via the overlay `local`; privacy canary (chapter 17) not found.
6. Result as a status check on the release, with the digests of the checked server images.

**Interface of `make portability`.** `portability.yml` passes only these three
environment variables and no option of `install.sh`. M7-14 builds the interface, M7-29 defines
the options.

| Variable | Content | Without a value |
|---|---|---|
| `PORTABILITY_TAG` | product tag under test (`vX.Y.Z` or `vX.Y.Z-beta.N`) | `make portability` stays the smoke test from M1-73 (`infra/compose/smoke.sh` with the locally built image, without `install.sh`) |
| `PORTABILITY_INSTALL_SOURCE` | Source of `install.sh` and of the bundle: `url` or `release-download` (table in 3.1) | abort before the test downloads, pulls or installs anything, if `PORTABILITY_TAG` is set |
| `PORTABILITY_RELEASE_DIR` | Folder with `install.sh` and `bothy-compose-<version>.tar.gz` from `gh release download` | with `release-download` abort before the test pulls or installs anything; with `url` not read |

Rules:

1. With `PORTABILITY_TAG`, `make portability` runs the test from `tests/portability`: from
   step 1 reading the bundle, pulling and checking the signatures of the images and the comparison
   after the installation, then the steps 2 to 5 and from step 6 the output of the digests. The
   runner and the status check are provided by `portability.yml`. An invalid value of one of the
   three variables aborts before the test downloads, pulls or installs anything, and names the
   variable.
2. The options after `sh -s --` are stated exactly once, in `tests/portability/install-options`
   (M7-29 creates the file, M7-14 reads it). The check from M7-29 compares them with the
   second block from 3.1; only `--storage` (the test sets it per run) and `--bundle` may
   differ.
3. Source `url`: the test builds the URL of the second block from 3.1 itself, with
   `GITHUB_REPOSITORY` in place of `OWNER/bothy` and `PORTABILITY_TAG` in place of
   `v0.0.0`. GitHub sets `GITHUB_REPOSITORY` in every step to "The owner and repository
   name" (https://docs.github.com/en/actions/reference/workflows-and-actions/variables,
   retrieved 2026-10-11) [verified]; a run outside GitHub Actions sets it itself. The test does
   not read the URL from the documentation of the tag. So for the URL of `install.sh` it does not
   matter whether the tag was created before or after `OWNER` was replaced (V1-03);
   [public-switch.md](../quality/public-switch.md) step 21 takes the last product tag. The same
   holds for everything `install.sh` downloads afterwards: the release set the download address
   of the bundle into `install.sh` at build time and the registry of the images into the bundle,
   both from the repository in which the release ran ([18-release.md](18-release.md), section
   2.1, rules 2 and 3). The test downloads the bundle for step 1 from the same address, built from
   `GITHUB_REPOSITORY` and `PORTABILITY_TAG` like the URL of `install.sh`.
4. Source `release-download`: `sh install.sh --bundle <bundle>` from `PORTABILITY_RELEASE_DIR`
   with the same options; `install.sh` checks the checksum as with the download (3.1, step 2).
5. Logs on failure: if a run fails, the test shows, before it cleans up,
   `docker compose ps --all` and the end of the logs of all services of exactly this
   installation, in the folder into which `install.sh` installed and with the project name under
   which it runs there. Only the test knows folder and project; a later step in `portability.yml`
   would see the checkout instead of the installation and, after the cleanup, no stack any more.
   The smoke test without `PORTABILITY_TAG` does the same (`infra/compose/smoke.sh`, M1-73).

The test belongs to the gate chain of team H (ADR 0038, 0039); this chapter defines the content.

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

The measurement is `make perf` (M1-72, ADR 0063): it records the pool occupancy under
`perf.sustained_messages_per_s` and the growth of the database and its indexes per thousand
messages in `.cache/perf/results.json`; `services/server/perf/README.md` describes how to read
them. `database.max_conns` keeps its start value until a run on `perf.reference_profile` (mode
`match` or `emulated`) confirms or changes it; a run in mode `below` is no evidence for it.

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

| Name | Standard | Bedeutung |
|---|---|---|
| `instance.environment` | `prod` | `dev`, `staging` oder `prod` (6.1) |
| `server.role` | `all` | `api`, `worker` oder `all` |
| `server.listen_addr` | `127.0.0.1:8080` | öffentlicher Listener (REST, Gateway) |
| `server.internal_listen_addr` | `127.0.0.1:8081` | Health, Readiness, Eingangs-Webhooks aus dem Stack |
| `server.public_url` | keiner, Pflicht | Origin, unter dem Apps den Server erreichen (Schema, Host, optional Port; ohne Pfad, Query und Fragment, sonst scheitert `bothy config check`); Grundlage von `aud`, DPoP-`htu` und WebAuthn ([08-identitaet.md](08-identitaet.md), Abschnitte 7.2 und 7.3) |
| `server.trusted_proxies` | `127.0.0.1/32, ::1/128` | Gegenstellen, deren Weiterleitungs- und Identitäts-Kopfzeilen gelten und die Klartext senden dürfen ([19-sicherheit.md](19-sicherheit.md), SEC-TLS-1) |
| `server.maintenance_mode` | `off` | `off` oder `read_only` (Umzug, Upgrade; [ADR 0065](../adr/0065-wartungsmodus-read-only.md)) |
| `secrets.dir` | `/run/bothy/secrets` | Basis der Standardverweise `file:`; absoluter Pfad in sauberer Form (kein `.` oder `..`, kein doppelter oder abschließender `/`), nicht `/`, ohne NUL-Zeichen und ohne `<`, sonst scheitert `bothy config check` |
| `secrets.cache_ttl` | `5m` | so lange gilt ein gelesenes Geheimnis; mindestens `1s` |
| `secrets.signing_key_dir` | `/var/lib/bothy/signing-keys` | beschreibbares Verzeichnis der privaten Signierschlüssel; im Besitz des Dienstbenutzers, Modus `0700` (5.2, [ADR 0066](../adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md)) |
| `console.assets_dir` | leer | directory of the built web console (`flutter build web` of `apps/console`) that the api process serves under `/console/` ([02-architektur.md](02-architektur.md), 3.1); empty: no console, `404` under `/console/`; the image sets `/usr/share/bothy/console`; a set directory without `index.html` ends the start |
| `server.tls_cert_ref` | leer | Verweis auf die Zertifikatskette (PEM, Blatt zuerst); leer: der Server beendet kein TLS (SEC-TLS-2) |
| `server.tls_key_ref` | leer | Verweis auf den privaten Schlüssel (PEM); nur zusammen mit `server.tls_cert_ref` |
| `database.host` | `localhost` | Host der Datenbank |
| `database.port` | `5432` | Port der Datenbank im Netz des Servers |
| `database.name` | `bothy` | Datenbankname |
| `database.user` | `bothy_app` | Rolle des Servers |
| `database.password_ref` | `file:<secrets.dir>/db_app_password` | Verweis auf das Passwort |
| `database.sslmode` | `verify-full` | TLS zur Datenbank; im internen Compose-Netz `disable` (SEC-TLS-5) |
| `database.sslrootcert` | leer | CA-Bündel für `verify-full`; leer nutzt die Wurzeln des Systems |
| `database.max_conns` | `20` | Größe des Verbindungspools je Prozess (Startwert bis zum Lasttest) |
| `migrate.require_recent_backup` | `true` | `migrate` verlangt eine frische Sicherung |
| `migrate.recent_backup_max_age` | `24h` | so alt darf diese Sicherung sein |
| `migrate.lock_timeout` | `5s` | `lock_timeout` je Migrationsschritt |
| `migrate.bootstrap_user` | `postgres` | Superuser-Anmeldung nur für `migrate up --bootstrap` |
| `migrate.bootstrap_password_ref` | `file:<secrets.dir>/db_superuser_password` | dessen Passwort |
| `migrate.owner_password_ref` | `file:<secrets.dir>/db_owner_password` | Passwort, das `--bootstrap` für `bothy_owner` setzt |
| `migrate.app_password_ref` | `file:<secrets.dir>/db_app_password` | Passwort für `bothy_app` |
| `migrate.maint_password_ref` | `file:<secrets.dir>/db_maint_password` | Passwort für `bothy_maint` |
| `migrate.monitor_password_ref` | `file:<secrets.dir>/db_monitor_password` | Passwort für `bothy_monitor` |
| `backup.pg.full_schedule` | `0 2 * * 0` | Plan der Vollsicherung (cron) |
| `backup.pg.diff_schedule` | `0 2 * * 1-6` | Plan der Differenzsicherung (cron) |
| `backup.pg.archive_timeout` | `60s` | spätestens dann wird ein WAL-Segment archiviert |
| `backup.pg.retention_full` | `4` | behaltene Vollsicherungen in repo2 |
| `backup.pg.local_retention_full` | `1` | behaltene Vollsicherungen in repo1 |
| `backup.logical.schedule` | `0 3 * * *` | Plan von `pg_dump` (cron) |
| `backup.logical.retention` | `30d` | Aufbewahrung der logischen Sicherung |
| `backup.objects.schedule` | `30 3 * * *` | Plan der Objektsicherung (cron) |
| `backup.objects.retention` | `90d` | Aufbewahrung der Objekt-Snapshots |
| `backup.offsite.target` | leer | Off-site-Ziel; leer heißt nur lokal (Warnung in Konsole und Status) |
| `backup.max_age` | `26h` | ohne erfolgreiche Sicherung länger als das: Alarm |
| `backup.restore_test.schedule` | `0 5 * * 6` | Plan des Wiederherstellungstests (cron) |
| `backup.restore_test.sample_files` | `200` | geprüfte Dateien je Test |
| `backup.restore_test.sample_threads` | `200` | geprüfte Threads je Test |
| `backup.restore_test.max_age` | `8d` | ohne erfolgreichen Test länger als das: Alarm |
| `dr.rpo_target` | `5m` | Ziel für den größten Datenverlust |
| `dr.rto_target` | `4h` | Ziel für die Dauer der Wiederherstellung |
| `status.check_interval` | `30s` | Abstand der automatischen Prüfungen |
| `status.degrade_after` | `2m` | so lange Fehler bis `degraded` |
| `status.incident_after` | `5m` | so lange `major_outage` bis zur automatischen Störung |
| `status.resolve_after` | `10m` | so lange gesund bis zur automatischen Auflösung |
| `status.error_ratio_degraded` | `0.05` | Fehleranteil, ab dem eine Prüfung scheitert |
| `status.error_ratio_outage` | `0.5` | Fehleranteil, ab dem `major_outage` gilt |
| `status.fanout_lag_degraded` | `5s` | Fan-out-Verzug, ab dem `sync.fanout` scheitert |
| `install.ready_timeout` | `5m` | option `--ready-timeout` of the installation script `install.sh` (3.1), not a key of the server: how long `install.sh` waits for `/readyz` after `docker compose up -d` |

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
| R-OP-20 | Setup wizard as the POSIX shell script `install.sh` on the host, a release artifact with built-in version, checksum and download address of the bundle (18 2.1); no wizard in the container. | Compose must run on the host; a wizard in the container would need the Docker socket, that is root permissions on the host. The shell exists everywhere Docker runs. |
