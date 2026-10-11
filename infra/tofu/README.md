# infra/tofu

OpenTofu-Module je Deployment-Ziel (ADR 0012). Stand M0: Schnittstelle (`variables.tf`) und
Beschreibung; Ressourcen folgen in Bau-Tickets je Modul.

## Module

| Modul | Ziel | Datenbank | Dateien | Geheimnisse | Status |
|---|---|---|---|---|---|
| [generic-vm](modules/generic-vm/README.md) | vorhandener Linux-Host mit Docker | Container | Volume oder S3-Dienst | Datei | Vertrag |
| [hetzner](modules/hetzner/README.md) | Hetzner Cloud | Container | Volume oder Hetzner Object Storage | Datei | Vertrag |
| [aws](modules/aws/README.md) | AWS | Container oder Amazon RDS | Amazon S3 | Datei, später AWS Secrets Manager | Vertrag |
| [azure](modules/azure/README.md) | Azure | Container oder Azure Database for PostgreSQL | Volume oder SeaweedFS (keine S3-API bei Blob Storage) | Datei, später Azure Key Vault | Vertrag |

## Aufbau

1. Jedes Zielmodul erzeugt Netz, Host, Speicher und optional verwaltete Dienste und ruft dann
   `generic-vm` auf, das den Host einrichtet: Docker, das Compose-Bündel der gewählten
   `bothy_version`, `.env` aus den Eingaben, ein Systemdienst für `docker compose up -d`.
2. Die gemeinsamen Eingaben sind in allen Modulen gleich (`variables.tf`, Abschnitt "Vertrag der
   Modul-Eingaben"). Zielspezifische Eingaben stehen darunter.
3. Ausgaben (geplant, in allen Modulen gleich benannt):

| Ausgabe | Inhalt |
|---|---|
| `public_url` | Basis-URL der Instanz |
| `ssh_endpoint` | Wartungszugang (VM-Ziele) |
| `database_endpoint` | Host des verwalteten Dienstes, sonst leer |
| `object_storage_bucket` | Bucket bei `object_storage_mode = s3`, sonst leer |
| `backup_offsite_location` | Off-site-Ziel, wenn das Modul es anlegt |
| `secret_store_ref` | Präfix für `secret_ref` (z. B. Key-Vault-URI), sonst leer |

## Regeln

1. Geheimnisse sind nie Eingaben und nie Ausgaben. Sie entstehen auf dem Host (`bothy secrets
   init`) oder im Secret-Dienst des Ziels. Grund: OpenTofu-State speichert Werte im Klartext.
2. State liegt verschlüsselt und getrennt vom Ziel. OpenTofu verschlüsselt State- und Plan-Dateien
   lokal und im Backend (https://opentofu.org/docs/language/state/encryption/, abgerufen
   2026-10-10) [belegt]; das Wurzelmodul jeder Instanz schaltet das ein.
3. Ports wie im Compose-Stack: nur der Rand ist öffentlich; Datenbank und Speicher nur im privaten
   Netz.
4. Versionen der Provider sind gepinnt (`required_providers`), Renovate hält sie aktuell.
5. Jedes Modul hat einen Test, der auf einer Wegwerf-Umgebung den Portabilitätstest
   ([16-betrieb.md](../../docs/spec/16-betrieb.md) 11.1) ausführt; das kostet Geld beim Anbieter
   und läuft darum nur auf ausdrückliche Freigabe des Owners.

## Geprüft

`tofu init -backend=false` und `tofu validate` (OpenTofu v1.13.1) auf einer Wurzel, die alle vier
Module aufruft, am 2026-10-10: gültig; `tofu plan` mit gültigen Werten ohne Änderungen, mit
`edge_mode = "funnel"` wie erwartet abgelehnt.
