# Module aws

Bothy on AWS.

## Inputs

For the shared inputs see [variables.tf](variables.tf). In addition: `region`, `instance_type`,
`rds_instance_class`.

## Planned resources

| Part | Content |
|---|---|
| Network | VPC with private subnets for host and database; public only a load balancer with TLS (`edge_mode = reverse_proxy`) or nothing (`private`) |
| Host | EC2 `instance_type`, own disk for Docker data, instance role without keys on the host |
| Database | `database_mode = managed`: Amazon RDS for PostgreSQL with PITR of the provider; roles from [16-operations.md](../../../../docs/spec/16-operations.md) 7.1 by bootstrap. Whether RDS offers PostgreSQL 18 is checked by the build ticket [assumed] |
| Files | Amazon S3, versioning on; access through the instance role |
| Off-site | second bucket with Object Lock, own role with write only; deletion only by a separate account |
| Secrets | at first driver `file` on the host; later AWS Secrets Manager (driver `awssm`) |
| Telemetry | overlay of choice; no AWS-specific backend in the product |
| Setup | call of `generic-vm` |

## Notes

1. With RDS, pgBackRest is not needed; `bothy-backupd` does the logical backup, the object backup
   and the restore test ([backup-restore.md](../../../../docs/operations/backup-restore.md) 6).
2. Since 2025, AWS SDKs compute checksums by default (`WHEN_SUPPORTED`)
   (https://docs.aws.amazon.com/sdkref/latest/guide/feature-dataintegrity.html, retrieved
   2026-10-10) [verified]. For Amazon S3 this fits; for third-party S3 services chapter 12 sets the
   option.
