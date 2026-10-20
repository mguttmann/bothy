# infra/tofu

OpenTofu modules per deployment target (ADR 0012). `generic-vm` is built (M7-15); the target
modules are still interface (`variables.tf`) and description, their resources follow in their
build tickets (M7-16).

## Modules

| Module | Target | Database | Files | Secrets | Status |
|---|---|---|---|---|---|
| [generic-vm](modules/generic-vm/README.md) | Linux host (Ubuntu or Debian) | container | volume or SeaweedFS in the stack | file | built (M7-15), tested on a local VM |
| [hetzner](modules/hetzner/README.md) | Hetzner Cloud | container | volume or Hetzner Object Storage | file | contract |
| [aws](modules/aws/README.md) | AWS | container or Amazon RDS | Amazon S3 | file, later AWS Secrets Manager | contract |
| [azure](modules/azure/README.md) | Azure | container or Azure Database for PostgreSQL | volume or SeaweedFS (no S3 API with Blob Storage) | file, later Azure Key Vault | contract |

## Structure

1. Every target module creates the network, host, storage and optionally managed services and
   then uses `generic-vm`, which sets up the host: Docker Engine with the Compose plugin, the
   verified installation bundle of the product release `bothy_version`, the setup wizard
   `install.sh --bundle` with options derived from the inputs (it writes `.env` and generates the
   secrets on the host), and a system service that starts the stack at boot. A target module
   passes the output `user_data` of `generic-vm` to its VM resource (cloud-init); an existing host
   gets the same setup over SSH (`provision_over_ssh = true`).
2. The shared inputs are the same in all modules (`variables.tf`, section "Contract of the
   module inputs"). Target-specific inputs follow below them.
3. Outputs (named the same in all modules):

| Output | Content |
|---|---|
| `public_url` | base URL of the instance |
| `ssh_endpoint` | maintenance access (VM targets) |
| `database_endpoint` | host of the managed service, otherwise empty |
| `object_storage_bucket` | bucket with `object_storage_mode = s3`, otherwise empty |
| `backup_offsite_location` | off-site target, if the module creates or receives it |
| `secret_store_ref` | prefix for `secret_ref` (for example a Key Vault URI), otherwise empty |

`generic-vm` also returns `user_data`, the rendered cloud-init user data without any secret.

## Rules

1. Secrets are never inputs and never outputs. They are created on the host (the setup wizard
   runs `secrets-init`) or in the secret service of the target. Reason: OpenTofu state stores
   values in plain text. A test scans the rendered user data and the plan of `generic-vm` for
   every secret name of the stack catalogue and for key material.
2. State is stored encrypted and separate from the target. OpenTofu encrypts state and plan files
   locally and in the backend (https://opentofu.org/docs/language/state/encryption/, retrieved
   2026-10-10) [verified]; the root module of every instance turns this on.
3. Ports as in the Compose stack: only the edge is public; database and storage only in the
   private network. `generic-vm` renders an nftables table that admits the sshd port and, with
   `edge_mode = reverse_proxy`, port 443 only; the stack publishes on `127.0.0.1`.
4. Provider versions are pinned (`required_providers`); Renovate keeps them current.
   `generic-vm` needs no provider (built-in features of OpenTofu only).
5. Every module has a test that runs the portability test
   ([16-betrieb.md](../../docs/spec/16-betrieb.md) 11.1) on a throwaway environment; for a cloud
   target this costs money at the provider and therefore runs only with the explicit approval of
   the owner. `generic-vm` is tested on a local VM without cost (`make tofu-vm-test`).

## Checked

- `make tofu-check` (part of `make check`): `tofu fmt -check`, then `tofu init -backend=false`
  with an empty local provider mirror (offline) and `tofu validate` of
  [test/validate](test/validate/main.tf), a root that calls all four modules, and of the test
  root [test/generic-vm](test/generic-vm/main.tf). `ops-check` validates each module on its own.
- `tools/ci/tests/test_tofu_generic_vm.py` (part of `make test`) plans the test root for every
  `edge_mode` and checks the rendered user data: no secret names or key material, the
  verification before the bundle is unpacked, the options passed to `install.sh`, only the edge
  port public, the refused inputs.
- `make tofu-vm-test` (build host only, outside `check`): the product trial, a pinned Ubuntu
  cloud image in QEMU with the rendered user data, the smoke probe against the stack in the VM,
  a second delivery over SSH that changes nothing, and a tampered bundle and a tampered
  checksum list that stop the setup ([test/generic-vm/README.md](test/generic-vm/README.md)).

Earlier check (2026-10-10, OpenTofu v1.13.1, before any resource): `tofu plan` with valid values
had no changes; `edge_mode = "funnel"` was rejected as expected.
