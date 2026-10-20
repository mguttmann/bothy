# Test root of the module generic-vm (brief M7-15). tools/ci/tests/test_tofu_generic_vm.py plans it
# for every edge_mode, and `make tofu-vm-test` (tools/ci/tofu_vm_test.py) applies it twice against a
# local VM: first with cloud-init, then with the SSH delivery. Only documentation addresses and
# example.com appear as defaults. No provider, no backend: the state of a run is local and
# temporary, and it holds no secret (the module has none to hold).

terraform {
  required_version = ">= 1.13.1, < 2.0.0"
}

variable "bothy_version" {
  description = "Product version of the bundle."
  type        = string
}

variable "image_registry" {
  description = "Registry the bundle pins."
  type        = string
}

variable "public_hostname" {
  description = "Public DNS name of the instance."
  type        = string
}

variable "release_repository" {
  description = "Repository of the releases (owner/name)."
  type        = string
}

variable "release_base_url" {
  description = "Download address of the release files; null for the GitHub release."
  type        = string
  default     = null
}

variable "release_public_key" {
  description = "Public key of the private signing phase; null for keyless."
  type        = string
  default     = null
}

variable "owner_handle" {
  description = "Handle of the first owner."
  type        = string
}

variable "owner_name" {
  description = "Display name of the first owner."
  type        = string
}

variable "edge_mode" {
  description = "Edge of the instance."
  type        = string
  default     = "reverse_proxy"
}

variable "object_storage_mode" {
  description = "File storage of the instance."
  type        = string
  default     = "filesystem"
}

variable "database_mode" {
  description = "Database of the instance."
  type        = string
  default     = "container"
}

variable "telemetry_exporters" {
  description = "Telemetry overlays."
  type        = list(string)
  default     = ["none"]
}

variable "backup_offsite" {
  description = "Off-site target of the backup."
  type = object({
    type     = string
    location = string
  })
  default = null
}

variable "virus_scan" {
  description = "Virus scan: auto, on or off."
  type        = string
  default     = "auto"
}

variable "admin_ssh_public_keys" {
  description = "Public SSH keys of the maintenance user."
  type        = list(string)
  default     = []
}

variable "ssh_host" {
  description = "Address of the host."
  type        = string
}

variable "ssh_port" {
  description = "SSH port of the host as seen from here."
  type        = number
  default     = 22
}

variable "provision_over_ssh" {
  description = "Deliver the setup over SSH instead of cloud-init only."
  type        = bool
  default     = false
}

module "vm" {
  source = "../../modules/generic-vm"

  instance_name         = "vmtest"
  environment           = "dev"
  bothy_version         = var.bothy_version
  image_registry        = var.image_registry
  public_hostname       = var.public_hostname
  edge_mode             = var.edge_mode
  database_mode         = var.database_mode
  object_storage_mode   = var.object_storage_mode
  backup_offsite        = var.backup_offsite
  telemetry_exporters   = var.telemetry_exporters
  admin_ssh_public_keys = var.admin_ssh_public_keys
  ssh_host              = var.ssh_host
  ssh_port              = var.ssh_port
  provision_over_ssh    = var.provision_over_ssh
  release_repository    = var.release_repository
  release_base_url      = var.release_base_url
  release_public_key    = var.release_public_key
  owner_handle          = var.owner_handle
  owner_name            = var.owner_name
  virus_scan            = var.virus_scan
}

output "user_data" {
  description = "Rendered cloud-init user data."
  value       = module.vm.user_data
}

output "public_url" {
  description = "Contract output public_url."
  value       = module.vm.public_url
}

output "ssh_endpoint" {
  description = "Contract output ssh_endpoint."
  value       = module.vm.ssh_endpoint
}

output "database_endpoint" {
  description = "Contract output database_endpoint."
  value       = module.vm.database_endpoint
}

output "object_storage_bucket" {
  description = "Contract output object_storage_bucket."
  value       = module.vm.object_storage_bucket
}

output "backup_offsite_location" {
  description = "Contract output backup_offsite_location."
  value       = module.vm.backup_offsite_location
}

output "secret_store_ref" {
  description = "Contract output secret_store_ref."
  value       = module.vm.secret_store_ref
}
