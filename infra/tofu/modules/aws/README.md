# Modul aws

Bothy auf AWS.

## Eingaben

Gemeinsame Eingaben siehe [variables.tf](variables.tf). Zusätzlich: `region`, `instance_type`,
`rds_instance_class`.

## Geplante Ressourcen

| Teil | Inhalt |
|---|---|
| Netz | VPC mit privaten Subnetzen für Host und Datenbank; öffentlich nur ein Load Balancer mit TLS (`edge_mode = reverse_proxy`) oder nichts (`private`) |
| Host | EC2 `instance_type`, eigener Datenträger für Docker-Daten, Instanzrolle ohne Schlüssel im Host |
| Datenbank | `database_mode = managed`: Amazon RDS for PostgreSQL mit PITR des Anbieters; Rollen aus [16-betrieb.md](../../../../docs/spec/16-betrieb.md) 7.1 per Bootstrap. Ob RDS PostgreSQL 18 anbietet, prüft das Bau-Ticket [vermutet] |
| Dateien | Amazon S3, Versionierung an; Zugriff über die Instanzrolle |
| Off-site | zweiter Bucket mit Object Lock, eigene Rolle nur mit Schreiben; Löschen nur ein getrenntes Konto |
| Geheimnisse | zunächst Treiber `file` auf dem Host; später AWS Secrets Manager (Treiber `awssm`) |
| Telemetrie | Overlay nach Wahl; kein AWS-spezifisches Backend im Produkt |
| Einrichtung | Aufruf von `generic-vm` |

## Hinweise

1. Mit RDS entfällt pgBackRest; `bothy-backupd` macht logische Sicherung, Objektsicherung und den
   Wiederherstellungstest ([backup-restore.md](../../../../docs/operations/backup-restore.md) 6).
2. AWS-SDKs berechnen seit 2025 standardmäßig Prüfsummen (`WHEN_SUPPORTED`)
   (https://docs.aws.amazon.com/sdkref/latest/guide/feature-dataintegrity.html, abgerufen
   2026-10-10) [belegt]. Für Amazon S3 passt das; für fremde S3-Dienste regelt Kapitel 12 die
   Einstellung.
