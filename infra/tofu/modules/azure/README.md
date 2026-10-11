# Module azure

Bothy on Azure.

## Inputs

For the shared inputs see [variables.tf](variables.tf). In addition: `location`,
`resource_group_name`, `vm_size`, `postgres_sku`.

## Planned resources

| Part | Content |
|---|---|
| Network | VNet with private subnets; public only Application Gateway with TLS (`edge_mode = reverse_proxy`) or nothing (`private`) |
| Host | Linux VM `vm_size`, managed disk for Docker data, managed identity |
| Database | `database_mode = managed`: Azure Database for PostgreSQL Flexible Server `postgres_sku` with PITR of the provider, private access. Whether PostgreSQL 18 is available is checked by the build ticket [assumed] |
| Files | `filesystem` (managed disk) or `seaweedfs`. Azure Blob Storage offers no S3 API [assumed, not checked]; a Blob driver would be an extension of chapter 12 |
| Off-site | storage account with immutable retention, access for `bothy-backupd` write only. restic and pgBackRest speak Azure Blob directly (pgBackRest: repositories in S3-, Azure- and GCS-compatible storage, https://pgbackrest.org/, retrieved 2026-10-10) [verified for pgBackRest; restic: assumed] |
| Secrets | at first driver `file`; later Azure Key Vault (driver `azkv`) with managed identity |
| Telemetry | overlay `azure-monitor` with managed identity instead of a service principal ([backends.md](../../../../docs/observability/backends.md) 6) |
| Setup | call of `generic-vm` |
