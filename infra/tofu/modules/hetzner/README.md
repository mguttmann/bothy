# Module hetzner

Bothy on Hetzner Cloud.

## Inputs

For the shared inputs see [variables.tf](variables.tf). In addition: `location`, `server_type`.

## Planned resources

| Part | Content |
|---|---|
| Network | private network, firewall: public only the edge (HTTPS at the reverse proxy); SSH only with keys from `admin_ssh_public_keys` and only from networks that the instance operator allows |
| Host | cloud server `server_type` in `location`, volume for Docker data separate from the system disk |
| Files | `object_storage_mode = s3`: bucket in Hetzner Object Storage (S3-compatible, versioning and Object Lock available; https://docs.hetzner.com/storage/object-storage, retrieved 2026-10-10) [verified]. Access keys are created by the Cloud Console, not by the API [verified, same source]; therefore the owner supplies them as a secret |
| Off-site | second bucket with Object Lock at another location |
| Database | container on the host (no managed PostgreSQL service assumed) |
| Setup | call of `generic-vm` |

## Notes

1. According to Hetzner, Object Storage exists only at European locations [verified, same source].
2. According to a community guide, Hetzner Object Storage does not support the newest signature
   version of S3 that some libraries use by default [assumed, only via a search result]. The S3
   driver (chapter 12) tests against Hetzner before the module offers `s3`.
