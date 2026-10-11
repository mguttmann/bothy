# infra/tofu

OpenTofu modules per deployment target (ADR 0012). State M0: interface (`variables.tf`) and
description; resources follow in build tickets per module.

## Modules

| Module | Target | Database | Files | Secrets | Status |
|---|---|---|---|---|---|
| [generic-vm](modules/generic-vm/README.md) | existing Linux host with Docker | container | volume or S3 service | file | contract |
| [hetzner](modules/hetzner/README.md) | Hetzner Cloud | container | volume or Hetzner Object Storage | file | contract |
| [aws](modules/aws/README.md) | AWS | container or Amazon RDS | Amazon S3 | file, later AWS Secrets Manager | contract |
| [azure](modules/azure/README.md) | Azure | container or Azure Database for PostgreSQL | volume or SeaweedFS (no S3 API with Blob Storage) | file, later Azure Key Vault | contract |

## Structure

1. Every target module creates the network, host, storage and optionally managed services and
   then calls `generic-vm`, which sets up the host: Docker, the Compose bundle of the chosen
   `bothy_version`, `.env` from the inputs, a system service for `docker compose up -d`.
2. The shared inputs are the same in all modules (`variables.tf`, section "Contract of the
   module inputs"). Target-specific inputs follow below them.
3. Outputs (planned, named the same in all modules):

| Output | Content |
|---|---|
| `public_url` | base URL of the instance |
| `ssh_endpoint` | maintenance access (VM targets) |
| `database_endpoint` | host of the managed service, otherwise empty |
| `object_storage_bucket` | bucket with `object_storage_mode = s3`, otherwise empty |
| `backup_offsite_location` | off-site target, if the module creates it |
| `secret_store_ref` | prefix for `secret_ref` (for example a Key Vault URI), otherwise empty |

## Rules

1. Secrets are never inputs and never outputs. They are created on the host (`bothy secrets
   init`) or in the secret service of the target. Reason: OpenTofu state stores values in plain
   text.
2. State is stored encrypted and separate from the target. OpenTofu encrypts state and plan files
   locally and in the backend (https://opentofu.org/docs/language/state/encryption/, retrieved
   2026-10-10) [verified]; the root module of every instance turns this on.
3. Ports as in the Compose stack: only the edge is public; database and storage only in the
   private network.
4. Provider versions are pinned (`required_providers`); Renovate keeps them current.
5. Every module has a test that runs the portability test
   ([16-operations.md](../../docs/spec/16-operations.md) 11.1) on a throwaway environment; this costs
   money at the provider and therefore runs only with the explicit approval of the owner.

## Checked

`tofu init -backend=false` and `tofu validate` (OpenTofu v1.13.1) on a root that calls all four
modules, on 2026-10-10: valid; `tofu plan` with valid values: no changes; with
`edge_mode = "funnel"`: rejected as expected.
