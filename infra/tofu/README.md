# infra/tofu

OpenTofu modules per deployment target (ADR 0012). `generic-vm` is built (M7-15), the target
modules `hetzner`, `aws` and `azure` are built on top of it (M7-16). No module has been applied
against a provider account: a live test costs money and waits for the owner's approval (OQ-09,
D-W-125).

## Modules

| Module | Target | Database | Files | Secrets | Status |
|---|---|---|---|---|---|
| [generic-vm](modules/generic-vm/README.md) | Linux host (Ubuntu or Debian) | container | volume or SeaweedFS in the stack | file | built (M7-15), tested on a local VM |
| [hetzner](modules/hetzner/README.md) | Hetzner Cloud | container (no managed service) | volume or SeaweedFS in the stack | file | built (M7-16), checked with mocked providers |
| [aws](modules/aws/README.md) | AWS | container (Amazon RDS offers PostgreSQL 18, not usable yet) | volume or SeaweedFS in the stack | file, later AWS Secrets Manager | built (M7-16), checked with mocked providers |
| [azure](modules/azure/README.md) | Azure | container (Azure Database for PostgreSQL offers 18, not usable yet) | volume or SeaweedFS (no S3 API with Blob Storage) | file, later Azure Key Vault | built (M7-16), checked with mocked providers |

The managed database and the S3 service of a target need an option of the setup wizard
`install.sh` for an external database and an external S3 service; until it exists the target
modules refuse `database_mode = managed` and `object_storage_mode = s3` with the reason
(docs/DECISIONS.md D-M7-16-4).

## Structure

1. Every target module creates the network, host, storage and optionally managed services and
   then uses `generic-vm`, which sets up the host: Docker Engine with the Compose plugin, the
   verified installation bundle of the product release `bothy_version`, the setup wizard
   `install.sh --bundle` with options derived from the inputs (it writes `.env` and generates the
   secrets on the host), and a system service that starts the stack at boot. A target module
   passes the output `user_data` of `generic-vm` to its VM resource (cloud-init); an existing host
   gets the same setup over SSH (`provision_over_ssh = true`).
2. The shared inputs are the same in all modules (`variables.tf`, section "Contract of the
   module inputs"), with the types of `generic-vm`. The target modules add the setup inputs of
   `generic-vm` (`release_repository`, `release_base_url`, `release_public_key`, `owner_handle`,
   `owner_name`, `virus_scan`), which they pass on unchanged, `ssh_allowed_cidrs` (the networks
   from which their security rules admit SSH), and then their target-specific inputs.
   `tools/ci/tests/test_tofu_cloud_modules.py` holds the names and types equal.
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
   2026-10-10) [verified]; the root module of every instance turns this on with
   `enforced = true` for state and plan, as the example root
   [test/validate](test/validate/main.tf) does (passphrase as an ephemeral input, or a key
   provider of the target). The credentials of a provider belong to the root, never to an input
   of a module.
3. Ports as in the Compose stack: only the edge is public; database and storage only in the
   private network. `generic-vm` renders an nftables table that admits the sshd port and, with
   `edge_mode = reverse_proxy`, port 443 only; the stack publishes on `127.0.0.1`. The target
   modules add the security rules of the target: from the internet only port 443 with
   `reverse_proxy`, SSH only from `ssh_allowed_cidrs` (never `/0`), nothing with `tailscale` or
   `private`.
4. Provider versions are pinned to one exact version (`required_providers`), and the lock files
   of the test roots ([test/validate](test/validate/.terraform.lock.hcl),
   [test/cloud](test/cloud/.terraform.lock.hcl)) hold their hashes for every platform of the
   gates; Renovate keeps both current (group "OpenTofu providers"). `generic-vm` needs no provider
   (built-in features of OpenTofu only).
5. Every module has a test that runs the portability test
   ([16-operations.md](../../docs/spec/16-operations.md) 11.1) on a throwaway environment; for a cloud
   target this costs money at the provider and therefore runs only with the explicit approval of
   the owner. `generic-vm` is tested on a local VM without cost (`make tofu-vm-test`).

## Checked

- `make tofu-check` (part of `make check`): `tofu fmt -check`; `tools/bin/tofu_providers.py`
  fills a local mirror with the provider archives the lock files pin (each checked against its
  `zh:` hash); then `tofu init -backend=false -lockfile=readonly` with that mirror as the only
  provider source (offline) and `tofu validate` of [test/validate](test/validate/main.tf), a root
  that calls all four modules, of the test root [test/generic-vm](test/generic-vm/main.tf) and of
  [test/cloud](test/cloud/main.tf); then `tofu test` in `test/cloud`, which applies each cloud
  module against mocked providers ([hetzner](test/cloud/hetzner.tftest.hcl),
  [aws](test/cloud/aws.tftest.hcl), [azure](test/cloud/azure.tftest.hcl)): only port 443 public
  and only with `reverse_proxy`, SSH only from `ssh_allowed_cidrs`, the user data of `generic-vm`
  within the limit of the target, the outputs, the locked off-site bucket of `aws` and the
  refused inputs. `ops-check` validates each module on its own with the same mirror.
- `tools/ci/tests/test_tofu_cloud_modules.py` (part of `make test`): the inputs and outputs of
  every module against the contract, a description on each, no secret among them, one resource
  per module that admits traffic (the one the tests assert over), the enforced encryption of the
  example root, and the provider pins against the lock files.
- `tools/ci/tests/test_tofu_generic_vm.py` (part of `make test`) plans the test root for every
  `edge_mode` and checks the rendered user data: no secret names or key material, the
  verification before the bundle is unpacked, the options passed to `install.sh`, only the edge
  port public, the refused inputs.
- `make tofu-vm-test` (build host only, outside `check`): the product trial, a pinned Ubuntu
  cloud image in QEMU with the rendered user data, the smoke probe against the stack in the VM,
  a second delivery over SSH that changes nothing, and a tampered bundle and a tampered
  checksum list that stop the setup ([test/generic-vm/README.md](test/generic-vm/README.md)).

Last check of the cloud modules: 2026-10-20 with OpenTofu 1.13.1, `hetznercloud/hcloud` 1.70.0,
`hashicorp/aws` 6.69.1 and `hashicorp/azurerm` 5.10.0 (`make tofu-check`, `make trivy`), mocked
providers only.

Earlier check (2026-10-10, OpenTofu v1.13.1, before any resource): `tofu plan` with valid values
had no changes; `edge_mode = "funnel"` was rejected as expected.
