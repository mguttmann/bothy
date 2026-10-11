# Disaster Recovery

Status: Entwurf M0. Runbook zu [16-betrieb.md](../spec/16-betrieb.md) 9. Ziele:
`dr.rpo_target` (größter Datenverlust) und `dr.rto_target` (Dauer bis zum Betrieb). Der
automatische Wiederherstellungstest misst die Dauer bei jedem Lauf.

## 1. Vorbereitet sein

| Punkt | Wo | Prüfung |
|---|---|---|
| Secrets-Bündel (age), aktuell | offline beim Owner, getrennt vom Off-site-Ziel | Probe-Entschlüsselung nach jeder Änderung |
| Privater age-Schlüssel des Owners | zweiter Ort (z. B. Passwortmanager mit Notfallzugang) | jährlich |
| Off-site-Zugang zum Lesen | im Secrets-Bündel | Wiederherstellungstest liest von dort |
| Konfiguration (`.env`, Overlays, OpenTofu-Variablen) | Repository des Betreibers | bei jeder Änderung |
| Externer Alarmweg | Alertmanager `external-oncall`, Watchdog | Watchdog-Herzschlag kommt an |
| Dieses Runbook offline | Kopie neben dem Secrets-Bündel | jährlich |

## 2. Entscheidungsbaum

1. Ist der Host erreichbar und das Volume `pgdata` lesbar? Nein: Fall B.
2. Sind Daten falsch (gelöscht, beschädigt) und ist die Zeit des Fehlers bekannt? Ja: Fall A.
3. Sind Schlüssel abgeflossen? Ja: Fall D oder E, danach prüfen, ob A oder B nötig ist.
4. Fehlt das Off-site-Ziel? Fall C.

## 3. Fälle

### Fall A: Daten falsch, Host gesund

1. Störung auf der Statusseite anlegen.
2. Zeitpunkt bestimmen: Audit-Log (`audit.read`), Logs, Trace der fehlerhaften Anfrage.
3. PITR nach [backup-restore.md](backup-restore.md) 5.2.
4. Nach dem Start: Statusseite aktualisieren, betroffene Menschen informieren (verlorener Zeitraum).

### Fall B: Host verloren

1. Neuen Host bereitstellen (gleiches OpenTofu-Modul oder neue VM), gleiche `BOTHY_VERSION`.
2. Secrets-Bündel entschlüsseln und nach `infra/compose/secrets/` legen; `secrets-init` übernimmt
   die Dateien statt neue zu erzeugen. Gleicher Token-Pepper und gleiche Signierschlüssel heißen:
   Tokens und Geräte bleiben gültig.
3. `docker compose up -d secrets-init postgres backup`, dann Restore aus repo2 nach
   [backup-restore.md](backup-restore.md) 5.2 ab Schritt 4 (Ziel: letzter Stand).
4. Objekte aus restic zurückspielen.
5. `bothy restore finalize`, Wiederherstellungstest, dann alle Dienste starten.
6. DNS bzw. Tailscale-Gerät auf den neuen Host. Bei `tailscale` ist das Gerät neu; `TS_HOSTNAME`
   gleich halten, das alte Gerät in der Tailnet-Verwaltung entfernen.
7. Dauer festhalten und mit `dr.rto_target` vergleichen.

### Fall C: Off-site-Ziel verloren oder kompromittiert

1. Neues Ziel anlegen (andere Zugangsdaten), `BOTHY_BACKUP_OFFSITE_TARGET` umstellen.
2. Sofort Vollsicherung und logischen Abzug, dann Wiederherstellungstest.
3. Bei Kompromittierung: Schlüssel der Sicherung rotieren (neue Repositorys, alte nach Frist
   löschen), denn der Angreifer kann alte Sicherungen mit dem alten Schlüssel lesen.

### Fall D: Signierschlüssel kompromittiert

1. `signing_key.rotate` mit `mode = emergency` für den betroffenen Zweck (Owner, Step-up): alle
   Schlüssel des Zwecks werden sofort `retired`, ein neuer wird `active`. War der Schlüsselspeicher
   (`secrets.signing_key_dir`) selbst betroffen, alle Zwecke rotieren.
2. Alle Sitzungen widerrufen (`session.revoke` für alle) bei `access_token`; bei `invocation`
   reicht die Rotation, weil Marken nur `agent.invocation_token_ttl` gelten; bei `webhook_out`
   die Empfänger informieren.
3. Audit-Log nach Nutzung des alten Schlüssels durchsuchen.

### Fall E: Token-Pepper kompromittiert

1. Neuen Pepper als Datei bereitstellen, Server neu starten.
2. Folge: alle API-Tokens, Refresh-Tokens und Eingangs-Webhook-Tokens sind ungültig. Owner gibt
   Tokens für Agenten und Apps neu aus; Menschen melden sich neu an.
3. Ein Datenbank-Abzug ohne Pepper verrät keine nutzbaren Tokens (R-DM-06); mit Pepper schon.
   Darum: prüfen, ob auch die Datenbank abgeflossen ist.

### Fall F: Ransomware oder Übernahme des Hosts

1. Host vom Netz, nicht reparieren. Als Fall B behandeln, mit neuem Host.
2. Alle Geheimnisse rotieren (Fälle D und E, Datenbank-Passwörter, Push-Schlüssel beim Anbieter
   neu erzeugen, OIDC-Client-Secret neu).
3. Wiederherstellung aus dem Off-site-Ziel; dieses ist nur anhängend und darum unversehrt.
4. Zeitpunkt vor der Übernahme wählen (Fall A), wenn Daten manipuliert sein könnten.

## 4. Nach jedem Fall

1. Störung abschließen mit Ursache, Dauer, verlorenem Zeitraum.
2. Nachbesprechung: Was hätte früher gewarnt? Neue Alarme oder Prüfungen als Ticket.
3. Wiederherstellungstest und Watchdog wieder grün.
