# The chat server on AWS (brief M7-16, ADR 0012, infra/tofu/README.md "Structure" item 1): a
# VPC with one public subnet, a security group that admits only the edge port from the internet, one EC2
# host with an elastic address whose user data comes from generic-vm (Docker, the verified
# release, the setup wizard), and optionally an off-site bucket with Object Lock in compliance
# mode in another region. PostgreSQL and the files stay in the Compose stack: install.sh has no
# option for an external database or S3 service yet (docs/DECISIONS.md D-M7-16-4). Every
# resource names its region (provider 6 resource argument `region`), so the module honours the
# input region whatever the provider configuration of the root says.
#
# The host sits in a public subnet with its own address, because it needs egress for Docker,
# the release and the images, and a NAT gateway would cost more than the host; the security
# group is the boundary, and the nftables table of generic-vm the second layer on the host.

locals {
  name = "bothy-${var.instance_name}"
  tags = merge(var.tags, {
    "bothy-instance"    = var.instance_name
    "bothy-environment" = var.environment
    "managed-by"        = "opentofu"
  })

  vpc_cidr    = "10.42.0.0/16"
  subnet_cidr = "10.42.1.0/24"

  # Ubuntu 24.04 of Canonical for the architecture of the instance type (Canonical publishes
  # the image IDs as public SSM parameters).
  arch = contains(data.aws_ec2_instance_type.host.supported_architectures, "arm64") ? "arm64" : "amd64"

  # Every inbound rule of the security group, keyed for for_each: the edge port for everyone
  # with reverse_proxy, SSH only from the networks of the operator.
  ingress = merge(
    var.edge_mode == "reverse_proxy" ? {
      "edge-ipv4" = { port = 443, cidr_ipv4 = "0.0.0.0/0", cidr_ipv6 = null, description = "edge (HTTPS)" }
    } : {},
    { for c in var.ssh_allowed_cidrs : "ssh-${c}" => {
      port        = 22
      cidr_ipv4   = strcontains(c, ":") ? null : c
      cidr_ipv6   = strcontains(c, ":") ? c : null
      description = "SSH of the operator"
    } },
  )

  offsite = var.offsite_region == null ? null : {
    type     = "s3"
    location = "https://s3.${var.offsite_region}.amazonaws.com/${aws_s3_bucket.offsite[0].bucket}"
  }
}

module "host" {
  source = "../generic-vm"

  instance_name         = var.instance_name
  environment           = var.environment
  bothy_version         = var.bothy_version
  image_registry        = var.image_registry
  public_hostname       = var.public_hostname
  edge_mode             = var.edge_mode
  database_mode         = var.database_mode
  object_storage_mode   = var.object_storage_mode
  backup_offsite        = var.offsite_region == null ? var.backup_offsite : local.offsite
  telemetry_exporters   = var.telemetry_exporters
  admin_ssh_public_keys = var.admin_ssh_public_keys
  tags                  = var.tags

  release_repository = var.release_repository
  release_base_url   = var.release_base_url
  release_public_key = var.release_public_key
  owner_handle       = var.owner_handle
  owner_name         = var.owner_name
  virus_scan         = var.virus_scan

  # cloud-init delivery: the address is only the target of the output ssh_endpoint.
  ssh_host = aws_eip.host.public_ip
}

data "aws_ec2_instance_type" "host" {
  region        = var.region
  instance_type = var.instance_type
}

data "aws_ssm_parameter" "ubuntu" {
  region = var.region
  name   = "/aws/service/canonical/ubuntu/server/24.04/stable/current/${local.arch}/hvm/ebs-gp3/ami-id"
}

# ---- Network ---------------------------------------------------------------------------------

resource "aws_vpc" "instance" {
  region               = var.region
  cidr_block           = local.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = merge(local.tags, { Name = local.name })
}

resource "aws_internet_gateway" "instance" {
  region = var.region
  vpc_id = aws_vpc.instance.id
  tags   = merge(local.tags, { Name = local.name })
}

resource "aws_subnet" "host" {
  region                  = var.region
  vpc_id                  = aws_vpc.instance.id
  cidr_block              = local.subnet_cidr
  map_public_ip_on_launch = false
  tags                    = merge(local.tags, { Name = "${local.name}-host" })
}

resource "aws_route_table" "host" {
  region = var.region
  vpc_id = aws_vpc.instance.id
  tags   = merge(local.tags, { Name = "${local.name}-host" })
}

resource "aws_route" "egress" {
  region                 = var.region
  route_table_id         = aws_route_table.host.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.instance.id
}

resource "aws_route_table_association" "host" {
  region         = var.region
  subnet_id      = aws_subnet.host.id
  route_table_id = aws_route_table.host.id
}

# The security group has no inline rules: every inbound rule is one instance of
# aws_vpc_security_group_ingress_rule.host, the resource the network test asserts over.
resource "aws_security_group" "host" {
  region      = var.region
  name_prefix = "${local.name}-"
  description = "Chat server host: only the edge port from the internet, SSH from the operator"
  vpc_id      = aws_vpc.instance.id
  tags        = merge(local.tags, { Name = local.name })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_vpc_security_group_ingress_rule" "host" {
  for_each = local.ingress

  region            = var.region
  security_group_id = aws_security_group.host.id
  ip_protocol       = "tcp"
  from_port         = each.value.port
  to_port           = each.value.port
  cidr_ipv4         = each.value.cidr_ipv4
  cidr_ipv6         = each.value.cidr_ipv6
  description       = each.value.description
  tags              = local.tags
}

# Egress to everywhere: the host fetches Docker, the release and the images, the server talks
# to the OIDC provider, the push services and webhook targets (docs/spec/16-operations.md 4.1 rule 3);
# the server's own egress filter (SEC-NET) decides per request.
#trivy:ignore:AVD-AWS-0104 The host needs egress to package mirrors, registries and the push services.
resource "aws_vpc_security_group_egress_rule" "all" {
  region            = var.region
  security_group_id = aws_security_group.host.id
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
  description       = "egress of the host"
  tags              = local.tags
}

# ---- Host ------------------------------------------------------------------------------------

resource "aws_instance" "host" {
  region                 = var.region
  ami                    = data.aws_ssm_parameter.ubuntu.value
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.host.id
  vpc_security_group_ids = [aws_security_group.host.id]
  # gzip, because the user data of generic-vm exceeds the 16 KiB limit of EC2 in plain form;
  # cloud-init unpacks it.
  user_data_base64 = base64gzip(module.host.user_data)
  tags             = merge(local.tags, { Name = local.name })
  volume_tags      = local.tags

  # IMDSv2 only and one hop: the containers on the Docker bridge cannot reach the metadata
  # service (the host has no instance role anyway).
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  root_block_device {
    volume_type = "gp3"
    volume_size = var.root_volume_size
    encrypted   = true
  }

  lifecycle {
    # The setup runs once at the first boot; a new image or user data must not replace the
    # host with its data. A new setup goes over SSH (generic-vm provision_over_ssh).
    ignore_changes = [ami, user_data_base64]
  }
}

# A fixed address for the DNS name public_hostname, which the operator points at it.
resource "aws_eip" "host" {
  region   = var.region
  domain   = "vpc"
  instance = aws_instance.host.id
  tags     = merge(local.tags, { Name = local.name })

  depends_on = [aws_internet_gateway.instance]
}

# ---- Off-site bucket (offsite_region) ----------------------------------------------------------
#
# Append-only for the instance (docs/spec/16-operations.md 8.2 item 3, R-OP-09,
# docs/operations/backup-restore.md 2): versioning, Object Lock in compliance mode with a default
# retention, a lifecycle rule that removes noncurrent versions after the retention, and a writer
# user whose policy cannot delete versions or change the lock. The module creates no access key:
# its secret would be in the state. The owner creates the key of the writer and the separate
# key that may delete by hand (README "Off-site").

resource "aws_s3_bucket" "offsite" {
  count = var.offsite_region == null ? 0 : 1

  region              = var.offsite_region
  bucket_prefix       = "bothy-${substr(var.instance_name, 0, 24)}-"
  object_lock_enabled = true
  tags                = merge(local.tags, { Name = "${local.name}-offsite" })
}

resource "aws_s3_bucket_versioning" "offsite" {
  count = length(aws_s3_bucket.offsite)

  region = var.offsite_region
  bucket = aws_s3_bucket.offsite[0].id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_object_lock_configuration" "offsite" {
  count = length(aws_s3_bucket.offsite)

  region = var.offsite_region
  bucket = aws_s3_bucket.offsite[0].id
  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = var.offsite_retention_days
    }
  }

  depends_on = [aws_s3_bucket_versioning.offsite]
}

resource "aws_s3_bucket_lifecycle_configuration" "offsite" {
  count = length(aws_s3_bucket.offsite)

  region = var.offsite_region
  bucket = aws_s3_bucket.offsite[0].id
  rule {
    id     = "expire-noncurrent-versions"
    status = "Enabled"
    filter {}
    noncurrent_version_expiration {
      noncurrent_days = var.offsite_retention_days
    }
    expiration {
      expired_object_delete_marker = true
    }
  }

  depends_on = [aws_s3_bucket_versioning.offsite]
}

resource "aws_s3_bucket_public_access_block" "offsite" {
  count = length(aws_s3_bucket.offsite)

  region                  = var.offsite_region
  bucket                  = aws_s3_bucket.offsite[0].id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "offsite" {
  count = length(aws_s3_bucket.offsite)

  region = var.offsite_region
  bucket = aws_s3_bucket.offsite[0].id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

#trivy:ignore:AVD-AWS-0132 SSE-S3 suffices: restic and pgBackRest encrypt every backup before the upload.
resource "aws_s3_bucket_server_side_encryption_configuration" "offsite" {
  count = length(aws_s3_bucket.offsite)

  region = var.offsite_region
  bucket = aws_s3_bucket.offsite[0].id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_policy" "offsite" {
  count = length(aws_s3_bucket.offsite)

  region = var.offsite_region
  bucket = aws_s3_bucket.offsite[0].id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "DenyInsecureTransport"
      Effect    = "Deny"
      Principal = "*"
      Action    = "s3:*"
      Resource  = [aws_s3_bucket.offsite[0].arn, "${aws_s3_bucket.offsite[0].arn}/*"]
      Condition = { Bool = { "aws:SecureTransport" = "false" } }
    }]
  })

  depends_on = [aws_s3_bucket_public_access_block.offsite]
}

# The identity of bothy-backupd. Its access key goes to secrets/offsite/ on the host; the
# owner creates it (aws iam create-access-key), because a key created here would be in the
# state.
resource "aws_iam_user" "offsite_writer" {
  count = length(aws_s3_bucket.offsite)

  name = "${local.name}-offsite-writer"
  tags = local.tags
}

# List, read and write, and DeleteObject for restic's own lock files, which on the versioned
# bucket only adds a delete marker; no DeleteObjectVersion, no retention or legal hold, no
# bucket setting (docs/operations/backup-restore.md 2).
resource "aws_iam_user_policy" "offsite_writer" {
  count = length(aws_s3_bucket.offsite)

  name = "${local.name}-offsite-writer"
  user = aws_iam_user.offsite_writer[0].name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid      = "AppendOnlyBackup"
      Effect   = "Allow"
      Action   = ["s3:ListBucket", "s3:ListBucketVersions", "s3:GetBucketLocation", "s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
      Resource = [aws_s3_bucket.offsite[0].arn, "${aws_s3_bucket.offsite[0].arn}/*"]
    }]
  })
}
