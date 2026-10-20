# Module hetzner

Bothy on Hetzner Cloud. Brief M7-16, ADR 0012. The module creates the firewall, the SSH keys
and one cloud server, and passes the cloud-init user data of [generic-vm](../generic-vm/README.md)
to the server, which then installs Docker, verifies the release and runs the setup wizard.

## Inputs

The shared inputs of the contract and the setup inputs of `generic-vm` are in
[variables.tf](variables.tf); their effect on the host is described in the README of
`generic-vm`. This module accepts them with two limits and one requirement:

| Input | Here |
|---|---|
| `database_mode` | `container` only: Hetzner Cloud has no managed PostgreSQL service (the provider `hetznercloud/hcloud` 1.70.0 has no resource for one) |
| `object_storage_mode` | `filesystem` or `seaweedfs`; `s3` is refused, because `install.sh` has no option for an external S3 service such as Hetzner Object Storage |
| `admin_ssh_public_keys` | at least one key: without one Hetzner Cloud sets a root password and sends it by e-mail |
| `backup_offsite` | an S3 bucket the owner created (section "Off-site"), passed to `install.sh --offsite` |

Inputs of this module:

| Input | Default | Meaning |
|---|---|---|
| `location` | - | Hetzner Cloud location, for example `nbg1` |
| `server_type` | - | server type, for example `cx22` (4 GiB for the virus scan, [16-betrieb.md](../../../../docs/spec/16-betrieb.md) 3 rule 2) |
| `image` | `ubuntu-24.04` | system image (Ubuntu or Debian with systemd) |
| `ssh_allowed_cidrs` | `[]` | networks from which the firewall admits SSH; `/0` is refused |

The root configures the provider `hcloud` with the API token from the environment
(`HCLOUD_TOKEN`); the module never receives it.

## Resources

| Resource | Content |
|---|---|
| `hcloud_firewall.host` | inbound TCP 443 from everywhere with `edge_mode = reverse_proxy`, TCP 22 from `ssh_allowed_cidrs`, nothing else; outbound open |
| `hcloud_ssh_key.admin` | one per entry of `admin_ssh_public_keys` |
| `hcloud_server.host` | `bothy-<instance_name>`, public IPv4 and IPv6, behind the firewall, user data of `generic-vm` |

`tailscale` and `private` publish no port; the nftables table of `generic-vm` on the host is the
second layer. The TLS proxy of the edge runs on the host, as with `generic-vm`. Data lives on the
disk of the server; changes of user data, keys and image do not replace the server (a new setup
goes over SSH with `generic-vm`).

## Off-site

Hetzner Object Storage is S3-compatible and has versioning and Object Lock
(https://docs.hetzner.com/storage/object-storage, retrieved 2026-10-10) [verified], but the
provider `hcloud` 1.70.0 has no resource for buckets or their keys, so the module creates none.
The owner, by hand in the Console:

1. A bucket at another location than the server, with Object Lock enabled at creation, a default
   retention in mode Compliance of at least `backup.objects.retention` and the lifecycle rule of
   [backup-restore.md](../../../../docs/operations/backup-restore.md) 2.
2. A key for `bothy-backupd` (list, read, write) into `secrets/offsite/` on the host, and a
   separate key that may delete, kept outside the stack (section 9 there).
3. `backup_offsite = { type = "s3", location = "https://<location>.your-objectstorage.com/<bucket>" }`.

What the module enforces: nothing about the bucket. Object Lock in compliance mode is what keeps
the backups from a compromised host (R-OP-09).

## PostgreSQL 18

Hetzner Cloud offers no managed PostgreSQL service, so only the container path exists
([16-betrieb.md](../../../../docs/spec/16-betrieb.md) 3 rule 6).

## Outputs

As in [infra/tofu/README.md](../../README.md): `ssh_endpoint` is `bothy@<IPv4 of the server>`;
`database_endpoint`, `object_storage_bucket` and `secret_store_ref` are empty;
`backup_offsite_location` is `s3:<location>` of `backup_offsite` or empty.

## Tests

`make tofu-check` validates the module and runs [hetzner.tftest.hcl](../../test/cloud/hetzner.tftest.hcl)
with a mocked provider; `tools/ci/tests/test_tofu_cloud_modules.py` checks the interface. No test
reaches Hetzner Cloud (OQ-09, D-W-125).
