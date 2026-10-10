# Modul azure

Bothy auf Azure.

## Eingaben

Gemeinsame Eingaben siehe [variables.tf](variables.tf). Zusätzlich: `location`,
`resource_group_name`, `vm_size`, `postgres_sku`.

## Geplante Ressourcen

| Teil | Inhalt |
|---|---|
| Netz | VNet mit privaten Subnetzen; öffentlich nur Application Gateway mit TLS (`edge_mode = reverse_proxy`) oder nichts (`private`) |
| Host | Linux-VM `vm_size`, Managed Disk für Docker-Daten, Managed Identity |
| Datenbank | `database_mode = managed`: Azure Database for PostgreSQL Flexible Server `postgres_sku` mit PITR des Anbieters, privater Zugang. Ob PostgreSQL 18 verfügbar ist, prüft das Bau-Ticket [vermutet] |
| Dateien | `filesystem` (Managed Disk) oder `seaweedfs`. Azure Blob Storage bietet keine S3-API [vermutet, nicht geprüft]; ein Blob-Treiber wäre eine Erweiterung von Kapitel 12 |
| Off-site | Storage Account mit unveränderlicher Aufbewahrung, Zugriff für `bothy-backupd` nur Schreiben. restic und pgBackRest sprechen Azure Blob direkt (pgBackRest: Repositorys in S3-, Azure- und GCS-kompatiblem Speicher, https://pgbackrest.org/, abgerufen 2026-10-10) [belegt für pgBackRest; restic: vermutet] |
| Geheimnisse | zunächst Treiber `file`; später Azure Key Vault (Treiber `azkv`) mit Managed Identity |
| Telemetrie | Overlay `azure-monitor` mit Managed Identity statt Service Principal ([backends.md](../../../../docs/observability/backends.md) 6) |
| Einrichtung | Aufruf von `generic-vm` |
