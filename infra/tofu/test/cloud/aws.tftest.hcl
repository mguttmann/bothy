# Tests of the module aws with a mocked aws provider (brief M7-16 requirements 13 to 16).
# `command = apply` against the mock creates nothing: every resource exists only in the memory
# of `tofu test`. The data sources get fixed values, so the host image follows the architecture
# of the instance type.

mock_provider "aws" {
  mock_data "aws_ec2_instance_type" {
    defaults = {
      supported_architectures = ["arm64"]
    }
  }
  mock_data "aws_ssm_parameter" {
    defaults = {
      value = "ami-0123456789abcdef0"
    }
  }
}

variables {
  instance_name      = "demo"
  environment        = "prod"
  bothy_version      = "1.2.3"
  image_registry     = "ghcr.io/alex"
  public_hostname    = "chat.example.com"
  release_repository = "alex/bothy"
  owner_handle       = "alex"
  owner_name         = "Alex Example"
  region             = "eu-central-1"
  instance_type      = "t4g.medium"
}

run "edge_port_only_public" {
  command = apply

  module {
    source = "../../modules/aws"
  }

  assert {
    condition = alltrue([
      for r in values(aws_vpc_security_group_ingress_rule.host) : r.from_port == 443 && r.to_port == 443 && r.ip_protocol == "tcp"
      if r.cidr_ipv4 == "0.0.0.0/0" || r.cidr_ipv6 == "::/0"
    ])
    error_message = "Only the edge port 443 may be open to the internet."
  }
  assert {
    condition     = length(aws_vpc_security_group_ingress_rule.host) == 1
    error_message = "reverse_proxy without ssh_allowed_cidrs opens exactly the edge port."
  }
  assert {
    condition     = aws_instance.host.vpc_security_group_ids == toset([aws_security_group.host.id])
    error_message = "The host sits behind the security group of the module only."
  }
  assert {
    condition     = aws_instance.host.user_data_base64 == base64gzip(module.host.user_data)
    error_message = "The host must get the user data of generic-vm, compressed below the 16 KiB of EC2."
  }
  assert {
    condition     = aws_instance.host.metadata_options[0].http_tokens == "required"
    error_message = "The host allows only IMDSv2."
  }
  assert {
    condition     = aws_instance.host.root_block_device[0].encrypted
    error_message = "The system disk is encrypted."
  }
  assert {
    condition     = aws_instance.host.ami == "ami-0123456789abcdef0" && strcontains(data.aws_ssm_parameter.ubuntu.name, "/arm64/")
    error_message = "The host image follows the architecture of the instance type."
  }
  assert {
    condition     = output.public_url == "https://chat.example.com"
    error_message = "public_url is https://<public_hostname>."
  }
  assert {
    condition     = output.database_endpoint == "" && output.object_storage_bucket == "" && output.secret_store_ref == ""
    error_message = "aws runs PostgreSQL and the files in the stack and keeps the secrets on the host."
  }
  assert {
    condition     = output.ssh_endpoint == "bothy@${aws_eip.host.public_ip}"
    error_message = "ssh_endpoint names the elastic address of the host."
  }
  assert {
    condition     = output.backup_offsite_location == "" && length(aws_s3_bucket.offsite) == 0
    error_message = "Without offsite_region and backup_offsite there is no off-site target."
  }
}

run "no_public_port_without_reverse_proxy" {
  command = apply

  module {
    source = "../../modules/aws"
  }

  variables {
    edge_mode         = "private"
    ssh_allowed_cidrs = ["198.51.100.0/24"]
  }

  assert {
    condition = alltrue([
      for r in values(aws_vpc_security_group_ingress_rule.host) : r.cidr_ipv4 != "0.0.0.0/0" && r.cidr_ipv6 != "::/0"
    ])
    error_message = "private opens nothing to the internet."
  }
  assert {
    condition = alltrue([
      for r in values(aws_vpc_security_group_ingress_rule.host) : r.from_port == 22 && r.to_port == 22 && r.cidr_ipv4 == "198.51.100.0/24"
    ])
    error_message = "SSH is open only to ssh_allowed_cidrs."
  }
}

run "offsite_bucket_is_locked" {
  command = apply

  module {
    source = "../../modules/aws"
  }

  variables {
    offsite_region         = "eu-north-1"
    offsite_retention_days = 45
  }

  assert {
    condition     = aws_s3_bucket.offsite[0].object_lock_enabled && aws_s3_bucket.offsite[0].region == "eu-north-1"
    error_message = "The off-site bucket has Object Lock and lives in offsite_region."
  }
  assert {
    condition = (
      aws_s3_bucket_object_lock_configuration.offsite[0].rule[0].default_retention[0].mode == "COMPLIANCE" &&
      aws_s3_bucket_object_lock_configuration.offsite[0].rule[0].default_retention[0].days == 45
    )
    error_message = "Every object version is locked in compliance mode for offsite_retention_days."
  }
  assert {
    condition     = aws_s3_bucket_versioning.offsite[0].versioning_configuration[0].status == "Enabled"
    error_message = "Object Lock needs versioning."
  }
  assert {
    condition = (
      aws_s3_bucket_public_access_block.offsite[0].block_public_acls &&
      aws_s3_bucket_public_access_block.offsite[0].block_public_policy &&
      aws_s3_bucket_public_access_block.offsite[0].ignore_public_acls &&
      aws_s3_bucket_public_access_block.offsite[0].restrict_public_buckets
    )
    error_message = "The off-site bucket is never public."
  }
  assert {
    condition = alltrue([
      for a in jsondecode(aws_iam_user_policy.offsite_writer[0].policy).Statement[0].Action :
      !contains(["s3:DeleteObjectVersion", "s3:PutObjectRetention", "s3:PutObjectLegalHold", "s3:BypassGovernanceRetention", "s3:PutBucketObjectLockConfiguration", "s3:PutLifecycleConfiguration", "s3:DeleteBucket", "s3:PutBucketPolicy", "s3:PutBucketVersioning"], a)
    ])
    error_message = "The writer of the instance cannot delete versions, change the lock or the bucket."
  }
  assert {
    condition     = startswith(output.backup_offsite_location, "s3:https://s3.eu-north-1.amazonaws.com/${aws_s3_bucket.offsite[0].bucket}")
    error_message = "The created bucket reaches install.sh --offsite."
  }
}

run "offsite_region_and_backup_offsite_exclude_each_other" {
  command = plan

  module {
    source = "../../modules/aws"
  }

  variables {
    offsite_region = "eu-north-1"
    backup_offsite = { type = "s3", location = "https://s3.example.com/bothy-demo" }
  }

  expect_failures = [var.offsite_region]
}

run "ssh_from_anywhere_is_refused" {
  command = plan

  module {
    source = "../../modules/aws"
  }

  variables {
    ssh_allowed_cidrs = ["0.0.0.0/0"]
  }

  expect_failures = [var.ssh_allowed_cidrs]
}

run "managed_database_is_refused" {
  command = plan

  module {
    source = "../../modules/aws"
  }

  variables {
    database_mode = "managed"
  }

  expect_failures = [var.database_mode]
}

run "external_object_storage_is_refused" {
  command = plan

  module {
    source = "../../modules/aws"
  }

  variables {
    object_storage_mode = "s3"
  }

  expect_failures = [var.object_storage_mode]
}
