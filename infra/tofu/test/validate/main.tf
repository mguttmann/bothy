# Validation root of `make tofu-check` (brief M7-15): calls all four modules, so `tofu validate`
# checks every module and the shared contract of their inputs together. It is never planned or
# applied; the values are documentation examples (example.com, 192.0.2.0/24).

terraform {
  required_version = ">= 1.13.1, < 2.0.0"
}

locals {
  shared = {
    environment     = "prod"
    bothy_version   = "1.2.3"
    image_registry  = "ghcr.io/alex"
    public_hostname = "chat.example.com"
  }
}

module "generic_vm" {
  source = "../../modules/generic-vm"

  instance_name      = "generic"
  environment        = local.shared.environment
  bothy_version      = local.shared.bothy_version
  image_registry     = local.shared.image_registry
  public_hostname    = local.shared.public_hostname
  ssh_host           = "192.0.2.10"
  release_repository = "alex/bothy"
  owner_handle       = "alex"
  owner_name         = "Alex"
}

module "hetzner" {
  source = "../../modules/hetzner"

  instance_name   = "hetzner"
  environment     = local.shared.environment
  bothy_version   = local.shared.bothy_version
  image_registry  = local.shared.image_registry
  public_hostname = local.shared.public_hostname
  location        = "nbg1"
  server_type     = "cx22"
}

module "aws" {
  source = "../../modules/aws"

  instance_name   = "aws"
  environment     = local.shared.environment
  bothy_version   = local.shared.bothy_version
  image_registry  = local.shared.image_registry
  public_hostname = local.shared.public_hostname
  region          = "eu-central-1"
  instance_type   = "t4g.medium"
}

module "azure" {
  source = "../../modules/azure"

  instance_name   = "azure"
  environment     = local.shared.environment
  bothy_version   = local.shared.bothy_version
  image_registry  = local.shared.image_registry
  public_hostname = local.shared.public_hostname
  location        = "westeurope"
  vm_size         = "Standard_B2s"
}
