# Outputs of the module contract (infra/tofu/README.md, "Structure" item 3) and the rendered
# user data of the cloud-init delivery. No output carries a secret.

output "user_data" {
  description = "cloud-init user data (#cloud-config) that sets up the host: pass it to the VM resource of the target. Contains no secret."
  value       = "#cloud-config\n${yamlencode(local.cloud_config)}"

  precondition {
    condition     = local.supported.database
    error_message = "database_mode = \"managed\" is not supported by generic-vm: install.sh has no option for an external database."
  }
  precondition {
    condition     = local.supported.storage
    error_message = "object_storage_mode = \"s3\" is not supported by generic-vm: install.sh has no option for an external S3 service; use filesystem or seaweedfs."
  }
  precondition {
    condition     = local.supported.telemetry
    error_message = "telemetry_exporters: generic-vm supports none and local; a vendor exporter needs credentials and is set up on the host."
  }
  precondition {
    condition     = local.supported.offsite
    error_message = "backup_offsite.type = \"sftp\" is not supported: bothy-backupd writes to an S3 bucket only."
  }
}

output "public_url" {
  description = "Base URL of the instance (server.public_url)."
  value       = local.public_url
}

output "ssh_endpoint" {
  description = "Maintenance access: user@host, or ssh://user@host:port for another port than 22."
  value       = var.ssh_port == 22 ? "${var.ssh_user}@${var.ssh_host}" : "ssh://${var.ssh_user}@${var.ssh_host}:${var.ssh_port}"
}

output "database_endpoint" {
  description = "Host of a managed database; always empty, because generic-vm runs PostgreSQL in the Compose stack."
  value       = ""
}

output "object_storage_bucket" {
  description = "Bucket of an external S3 service; always empty, because generic-vm uses the volume or SeaweedFS in the stack."
  value       = ""
}

output "backup_offsite_location" {
  description = "Off-site target of the backup as install.sh --offsite receives it (s3:<location>), otherwise empty."
  value       = local.offsite
}

output "secret_store_ref" {
  description = "Prefix for secret_ref; empty, because the secrets stay in the file driver on the host."
  value       = ""
}
