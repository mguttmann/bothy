# Module generic-vm

Sets up a Linux host (Ubuntu or Debian with systemd, amd64 or arm64) for Bothy. Base of all VM
targets: the modules `azure`, `aws` and `hetzner` create their host and pass the output
`user_data` to it; an existing host gets the same setup over SSH. Brief M7-15, ADR 0012.

## Inputs

The shared inputs are in [variables.tf](variables.tf), section "Contract of the module inputs".
How they reach the host:

| Input | Effect on the host |
|---|---|
| `bothy_version` | the product release whose bundle `bothy-compose-<version>.tar.gz` is installed; the bundle pins the server version and the registry ([18-release.md](../../../../docs/spec/18-release.md) 2.1) |
| `image_registry` | checked against `BOTHY_REGISTRY` of the bundle; the setup stops on a difference |
| `public_hostname` | `install.sh --public-url https://<public_hostname>` |
| `edge_mode` | `reverse_proxy`: `--edge direct`, port 443 public for the proxy of the host or of the target; `tailscale`: `--edge tailscale`, no public port; `private`: `--edge direct`, no public port |
| `database_mode` | `container` only; `managed` stops the plan (install.sh has no option for an external database) |
| `object_storage_mode` | `filesystem`: `--storage fs`; `seaweedfs`: `--storage s3` (SeaweedFS in the stack); `s3` stops the plan |
| `backup_offsite` | type `s3`: `--offsite s3:<location>`; `sftp` stops the plan; credentials go to `secrets/offsite/` on the host |
| `telemetry_exporters` | `local`: `--observability on`; `none`: off; a vendor exporter stops the plan (it needs credentials, set up on the host) |
| `admin_ssh_public_keys` | cloud-init creates `ssh_user` with these keys and password-less sudo |
| `environment`, `instance_name` | named in the header of `setup.env`; `install.sh` has no option for the environment, so `BOTHY_ENVIRONMENT` keeps the bundle's default (`prod`) |
| `tags` | not used (no tagged resource) |

Inputs of this module:

| Input | Default | Meaning |
|---|---|---|
| `ssh_host`, `ssh_port`, `ssh_user` | -, `22`, `bothy` | the host for the SSH delivery and the output `ssh_endpoint` |
| `sshd_port` | `ssh_port` | the port of sshd on the host, which the firewall admits; set it only when `ssh_port` is a forwarded port (NAT) |
| `provision_over_ssh` | `false` | `true` runs the setup on `ssh_host` over SSH; the key comes from the SSH agent |
| `release_repository` | - | `<owner>/<name>` of the product releases |
| `release_base_url` | GitHub release of `bothy_version` | download address of the release files |
| `release_public_key` | `null` | public key of the private signing phase (ADR 0062); `null` verifies keyless against the release workflow of `release_repository` |
| `owner_handle`, `owner_name` | - | the first owner (`--owner-handle`, `--owner-name`) |
| `virus_scan` | `auto` | `auto` lets `install.sh` decide by the memory; `on` or `off` is `--scan` |
| `stack_dir` | `/srv/bothy` | `install.sh --dir`, owned by the user `bothy` |

No input is a secret. With `edge_mode = tailscale` the one-time auth key is placed on the host by
the operator in `/etc/bothy/tailscale-auth-key` (mode `0600`) before the setup runs; without it
the setup stops and names the file, and `/usr/local/lib/bothy/bothy-setup` runs it again.

## Resources and files

The module creates no cloud resource. It renders five files, which both delivery paths write
before they run the same script ([files/bothy-setup.sh](files/bothy-setup.sh)):

| File on the host | Content |
|---|---|
| `/usr/local/lib/bothy/bothy-setup` | the setup script (POSIX `sh`) |
| `/etc/bothy/setup.env` | version, download address, signing mode and identity, registry, edge, the pinned cosign; no secret |
| `/etc/bothy/install-options` | the options of `install.sh`, one per line |
| `/etc/bothy/firewall.nft` | the nftables table `inet bothy_edge`: input policy drop, the sshd port and the edge port only |
| `/etc/bothy/release.pub` | only with `release_public_key` |

The script, in this order:

1. Packages: `curl`, `gnupg`, `nftables`, `ca-certificates` when missing.
2. Docker Engine and the Compose plugin from `download.docker.com` (key checked by its
   fingerprint `9DC8 5822 9FC7 DD38 854A E2D8 8D81 803C 0EBF CD88`), unless Engine 26 and
   Compose 2.26 or newer are present ([16-operations.md](../../../../docs/spec/16-operations.md) 3
   rule 7).
3. The system user `bothy` in the group `docker`.
4. cosign 3.1.3 from its GitHub release, checked against the SHA-256 of
   `tools/bin/tools.lock.json`.
5. `SHA256SUMS`, the bundle and `install.sh`, each with its `.sigstore.json`.
6. Verification before anything is unpacked: the signature of `SHA256SUMS`, the checksums of
   bundle and `install.sh` from it, their own signatures, then the registry of the bundle. A
   wrong checksum or signature stops the setup (exit code 1, step `verify`).
7. The firewall table and the unit `bothy-firewall.service`, which loads it at boot before the
   network (`DefaultDependencies=no`, `Before=network-pre.target`, like `nftables.service`).
8. `install.sh --bundle <bundle> --dir <stack_dir>` with the options, as the user `bothy`, in a
   new session without a controlling terminal (`setsid`, standard input closed; the SSH delivery
   runs under a pseudo terminal, and with a terminal `install.sh` would print the code to
   `/dev/tty`). The enrollment code of the first owner is replaced in the log by the command that
   issues a new one (`bothy setup issue-enrollment-code --user <owner_handle>` in the API service of the edge,
   `api` or `api-tailscale`), because cloud-init
   and the SSH delivery keep the output in a log.
9. The unit `bothy.service` (`docker compose up -d` at boot), enabled and started.

A second run changes nothing: every file is compared before it is written, Docker and cosign
are kept when they match, and `install.sh` keeps `.env` and `secrets/` (16 3.1 rule 1).

## Outputs

See [infra/tofu/README.md](../../README.md). `public_url` is `https://<public_hostname>`;
`ssh_endpoint` is `<ssh_user>@<ssh_host>` (or `ssh://...:<port>`); `database_endpoint`,
`object_storage_bucket` and `secret_store_ref` are empty; `backup_offsite_location` is the
`--offsite` value or empty; `user_data` is the cloud-init user data.

## Tool choice

New hosts get cloud-init user data (`#cloud-config` with `users`, `write_files` and `runcmd`);
existing hosts get the same files and script over SSH from `terraform_data` with a
`remote-exec` provisioner (`ssh_host`, `ssh_port`, `ssh_user`, the key from the SSH agent). One
script, two delivery paths; no provider (docs/DECISIONS.md D-M7-15-1).

## Tests

`make tofu-check`, `tools/ci/tests/test_tofu_generic_vm.py` and `make tofu-vm-test`
([infra/tofu/README.md](../../README.md), "Checked").
