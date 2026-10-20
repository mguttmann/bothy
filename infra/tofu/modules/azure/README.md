# Module azure

Bothy on Azure. Brief M7-16, ADR 0012. The module creates a resource group (or uses one), a
virtual network, the network security group, a static public address and one Linux VM, and
passes the cloud-init user data of [generic-vm](../generic-vm/README.md) to the VM as custom
data, which then installs Docker, verifies the release and runs the setup wizard.

## Inputs

The shared inputs of the contract and the setup inputs of `generic-vm` are in
[variables.tf](variables.tf); their effect on the host is described in the README of
`generic-vm`. This module accepts them with two limits and one requirement:

| Input | Here |
|---|---|
| `database_mode` | `container` only: Azure Database for PostgreSQL offers PostgreSQL 18 (section "PostgreSQL 18"), but `install.sh` has no option for an external database (docs/DECISIONS.md D-M7-16-4) |
| `object_storage_mode` | `filesystem` (volume, driver `fs`) or `seaweedfs` (profile `s3`); `s3` is refused, because Azure Blob Storage has no S3 API ([16-betrieb.md](../../../../docs/spec/16-betrieb.md) 3 rule 4) |
| `admin_ssh_public_keys` | at least one key: Azure needs one for the administrator when password login is off; `ssh-rsa` or `ssh-ed25519` |
| `backup_offsite` | an S3 bucket elsewhere (section "Off-site") |

Inputs of this module:

| Input | Default | Meaning |
|---|---|---|
| `location` | - | Azure region of every resource, for example `westeurope` |
| `resource_group_name` | `""` | an existing resource group; empty creates `bothy-<instance_name>` |
| `vm_size` | - | for example `Standard_B2s` |
| `image_sku` | `server` | SKU of `Canonical:ubuntu-24_04-lts`; `server-arm64` for Arm sizes |
| `ssh_allowed_cidrs` | `[]` | networks from which the security group allows SSH; `/0` is refused |

The root configures the provider `azurerm` (`features {}`, subscription and credentials from the
environment); the module never receives them.

## Resources

| Resource | Content |
|---|---|
| `azurerm_resource_group.instance` | only without `resource_group_name` |
| `azurerm_virtual_network.instance`, `azurerm_subnet.host` | `10.42.0.0/16` with one subnet `10.42.1.0/24` |
| `azurerm_network_security_group.host` | allow TCP 443 from `Internet` with `edge_mode = reverse_proxy`, TCP 22 from each of `ssh_allowed_cidrs`, deny every other inbound packet at priority 4096 (also the virtual network and the load balancer that the default rules of Azure allow) |
| `azurerm_subnet_network_security_group_association.host`, `azurerm_network_interface_security_group_association.host` | the same group on subnet and interface |
| `azurerm_public_ip.host`, `azurerm_network_interface.host` | static Standard address that `public_hostname` points to |
| `azurerm_linux_virtual_machine.host` | `bothy-<instance_name>`, password login off, administrator `bothy` with the first key, `StandardSSD_LRS` system disk of 64 GiB, custom data of `generic-vm` |

No Application Gateway: the Compose stack publishes on `127.0.0.1` only, so the TLS proxy of the
edge runs on the host, as with `generic-vm`. `tailscale` and `private` publish no port. A change
of the custom data, of the first SSH key or of `image_sku` does not replace the VM (a new setup
goes over SSH with `generic-vm`). No managed identity: the host needs no Azure API (Key Vault
comes with the driver `azkv`).

## Off-site

`bothy-backupd` writes to S3 only (D-M7-02-4), and Blob Storage has no S3 API, so the module
creates no storage account. The off-site target is an S3 bucket with Object Lock at another
provider, set up by the owner as in [backup-restore.md](../../../../docs/operations/backup-restore.md) 2
and passed in `backup_offsite`. What the module enforces: nothing about the bucket.

## PostgreSQL 18

Azure Database for PostgreSQL flexible server supports the major version 18, current minor 18.6;
some extensions are not yet available for it
(https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-supported-versions,
retrieved 2026-10-20) [verified]. The module still offers only the container path: the setup
wizard and the Compose stack cannot use an external database yet (D-M7-16-4).

## Telemetry

The overlay `azure-monitor` needs credentials and is set up on the host
([backends.md](../../../../docs/observability/backends.md) 6); `generic-vm` refuses vendor
exporters in `telemetry_exporters`.

## Outputs

As in [infra/tofu/README.md](../../README.md): `ssh_endpoint` is `bothy@<static IPv4>`;
`database_endpoint`, `object_storage_bucket` and `secret_store_ref` are empty;
`backup_offsite_location` is `s3:<location>` of `backup_offsite` or empty.

## Tests

`make tofu-check` validates the module and runs [azure.tftest.hcl](../../test/cloud/azure.tftest.hcl)
with a mocked provider; `tools/ci/tests/test_tofu_cloud_modules.py` checks the interface. No test
reaches Azure (OQ-09, D-W-125).
