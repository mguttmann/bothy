# Outputs of the module contract (infra/tofu/README.md "Structure" item 3). No output carries a
# secret.

output "public_url" {
  description = "Base URL of the instance (server.public_url)."
  value       = module.host.public_url
}

output "ssh_endpoint" {
  description = "Maintenance access user@<public IPv4 of the server>; reachable only from ssh_allowed_cidrs."
  value       = module.host.ssh_endpoint
}

output "database_endpoint" {
  description = "Host of a managed database; always empty, because PostgreSQL runs in the Compose stack."
  value       = module.host.database_endpoint
}

output "object_storage_bucket" {
  description = "Bucket of an external S3 service; always empty, because the files stay on the volume or in SeaweedFS of the stack."
  value       = module.host.object_storage_bucket
}

output "backup_offsite_location" {
  description = "Off-site target as install.sh --offsite receives it (s3:<location> of backup_offsite), otherwise empty."
  value       = module.host.backup_offsite_location
}

output "secret_store_ref" {
  description = "Prefix for secret_ref; empty, because the secrets stay in the file driver on the host."
  value       = module.host.secret_store_ref
}
