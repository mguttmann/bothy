# Module aws

Bothy on AWS. Brief M7-16, ADR 0012. The module creates a VPC, the security group, one EC2 host
with an elastic address and, on request, an off-site bucket with Object Lock, and passes the
cloud-init user data of [generic-vm](../generic-vm/README.md) to the host, which then installs
Docker, verifies the release and runs the setup wizard.

## Inputs

The shared inputs of the contract and the setup inputs of `generic-vm` are in
[variables.tf](variables.tf); their effect on the host is described in the README of
`generic-vm`. This module accepts them with two limits:

| Input | Here |
|---|---|
| `database_mode` | `container` only: Amazon RDS offers PostgreSQL 18 (section "PostgreSQL 18"), but `install.sh` has no option for an external database (docs/DECISIONS.md D-M7-16-4) |
| `object_storage_mode` | `filesystem` or `seaweedfs`; `s3` is refused, because `install.sh` has no option for an external S3 service such as Amazon S3 |
| `backup_offsite` | an existing S3 bucket; excludes `offsite_region` |

Inputs of this module:

| Input | Default | Meaning |
|---|---|---|
| `region` | - | region of every resource of the host; the module sets it on each resource (provider 6 argument `region`), so the region of the provider configuration does not matter |
| `instance_type` | - | for example `t4g.medium`; the Ubuntu 24.04 image of Canonical follows its architecture (public SSM parameter) |
| `root_volume_size` | `40` | encrypted gp3 system disk in GiB, at least 20 |
| `offsite_region` | `null` | region of an off-site bucket the module creates; `null` creates none |
| `offsite_retention_days` | `90` | Object Lock period of that bucket, at least `backup.objects.retention` |
| `ssh_allowed_cidrs` | `[]` | networks from which the security group admits SSH; `/0` is refused |

The root configures the provider `aws` with the credentials of the operator (environment or
profile); the module never receives them.

## Resources

| Resource | Content |
|---|---|
| `aws_vpc.instance`, `aws_subnet.host`, `aws_internet_gateway.instance`, `aws_route_table.host` | `10.42.0.0/16` with one subnet `10.42.1.0/24` and a default route to the internet gateway |
| `aws_security_group.host` | no inline rules |
| `aws_vpc_security_group_ingress_rule.host` | every inbound rule: TCP 443 from `0.0.0.0/0` with `edge_mode = reverse_proxy`, TCP 22 from each of `ssh_allowed_cidrs` |
| `aws_vpc_security_group_egress_rule.all` | egress for Docker, the release, the images and the services of the server |
| `aws_instance.host` | IMDSv2 only with one hop (the containers cannot reach the metadata service), no instance role, encrypted system disk, the user data of `generic-vm` gzip-compressed (in plain form it exceeds the 16 KiB of EC2) |
| `aws_eip.host` | the fixed address that `public_hostname` points to |

The host has a public address in a public subnet: it needs egress, and a NAT gateway would cost
more than the host. A load balancer is not created: the Compose stack publishes on `127.0.0.1`
only, so the TLS proxy of the edge runs on the host, as with `generic-vm`. Changes of image and
user data do not replace the host (a new setup goes over SSH with `generic-vm`).

## Off-site

With `offsite_region` the module creates, in that region:

| Resource | Content |
|---|---|
| `aws_s3_bucket.offsite` | `bothy-<instance_name>-<suffix>`, Object Lock on |
| `aws_s3_bucket_versioning.offsite`, `aws_s3_bucket_object_lock_configuration.offsite` | versioning and a default retention in mode Compliance of `offsite_retention_days` |
| `aws_s3_bucket_lifecycle_configuration.offsite` | noncurrent versions expire `offsite_retention_days` after they became noncurrent, expired delete markers go ([backup-restore.md](../../../../docs/operations/backup-restore.md) 2) |
| `aws_s3_bucket_public_access_block.offsite`, `aws_s3_bucket_ownership_controls.offsite`, `aws_s3_bucket_server_side_encryption_configuration.offsite`, `aws_s3_bucket_policy.offsite` | never public, no ACLs, SSE-S3, TLS only |
| `aws_iam_user.offsite_writer`, `aws_iam_user_policy.offsite_writer` | the identity of `bothy-backupd`: `ListBucket`, `ListBucketVersions`, `GetBucketLocation`, `GetObject`, `PutObject`, `DeleteObject` (a delete marker only), no `DeleteObjectVersion`, no retention, no bucket setting |

What the module enforces: compliance mode, so no account, not even the root user, deletes a
version before its period ends (R-OP-09). What the owner does by hand, because a key created by
the module would be in the state: the access key of `offsite_writer`
(`aws iam create-access-key --user-name bothy-<instance_name>-offsite-writer`) into
`secrets/offsite/` on the host; a separate key that may delete old versions, kept outside the
stack ([backup-restore.md](../../../../docs/operations/backup-restore.md) 9); and the region of the
bucket for the server (`backup.offsite.region`, default `us-east-1`, set for the service `backup`
in a Compose override file), because the target `s3:https://s3.<offsite_region>.amazonaws.com/<bucket>`
names the endpoint, not the signing region. A bucket in another account is the stronger choice;
pass it in `backup_offsite` instead.

## PostgreSQL 18

Amazon RDS for PostgreSQL offers the major version 18, minor versions 18.1 to 18.6
(https://docs.aws.amazon.com/AmazonRDS/latest/PostgreSQLReleaseNotes/postgresql-versions.html,
retrieved 2026-10-20) [verified]. The module still offers only the container path: the setup
wizard and the Compose stack cannot use an external database yet (D-M7-16-4), so an RDS instance
would stay unused. With RDS, pgBackRest would be left out and `bothy-backupd` would do the
logical and object backup only ([backup-restore.md](../../../../docs/operations/backup-restore.md) 6).

## Notes

AWS SDKs calculate checksums by default since 2025 (`WHEN_SUPPORTED`)
(https://docs.aws.amazon.com/sdkref/latest/guide/feature-dataintegrity.html, retrieved
2026-10-10) [verified]. That fits Amazon S3; chapter 12 sets it for other S3 services.

## Outputs

As in [infra/tofu/README.md](../../README.md): `ssh_endpoint` is `bothy@<elastic IPv4>`;
`database_endpoint`, `object_storage_bucket` and `secret_store_ref` are empty;
`backup_offsite_location` is `s3:<location>` of the created bucket or of `backup_offsite`, or empty.

## Tests

`make tofu-check` validates the module and runs [aws.tftest.hcl](../../test/cloud/aws.tftest.hcl)
with a mocked provider; `tools/ci/tests/test_tofu_cloud_modules.py` checks the interface. No test
reaches AWS (OQ-09, D-W-125).
