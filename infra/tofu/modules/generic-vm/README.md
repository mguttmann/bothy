# Module generic-vm

Sets up an existing Linux host for Bothy. Base of all VM targets; the modules `azure`, `aws` and
`hetzner` call it after they have created their host.

## Inputs

For the shared inputs see [variables.tf](variables.tf). In addition: `ssh_host`, `ssh_user`.

## Planned resources

| Part | Content |
|---|---|
| cloud-init or provisioning by SSH | Docker Engine and Compose plugin from the Docker package repositories, user `bothy`, directory for the stack |
| Compose bundle | `infra/compose` and `infra/otel`, `infra/observability` of version `bothy_version`, signature checked ([18-release.md](../../../../docs/spec/18-release.md) 6) |
| `.env` | generated from the inputs; contains no secrets |
| System service | starts `docker compose up -d` at boot |
| Edge | `edge_mode = reverse_proxy`: Caddy or another proxy on the host with TLS; `tailscale`: profile `tailscale`; `private`: no public port |

## Outputs

See [infra/tofu/README.md](../../README.md). `database_endpoint` and `object_storage_bucket` stay
empty unless the inputs point to external services.

## Open

Which tool sets up the host by SSH (cloud-init for new hosts, a provider for existing hosts) is
decided by the build ticket.
