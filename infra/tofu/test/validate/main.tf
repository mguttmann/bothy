# Validation root of `make tofu-check` (briefs M7-15 and M7-16): calls all four modules, so
# `tofu validate` checks every module and the shared contract of their inputs together. It is
# never planned or applied; the values are documentation examples (example.com, 192.0.2.0/24,
# 198.51.100.0/24).
#
# It is also the example of an instance root (infra/tofu/README.md rule 2): state and plan files
# are encrypted, and `enforced = true` makes OpenTofu refuse to write or read them unencrypted.
# The passphrase is an ephemeral input of the root (TF_VAR_state_passphrase), never stored in
# the state; an instance may use a key provider of its target instead (AWS KMS, for example).

terraform {
  required_version = ">= 1.13.1, < 2.0.0"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "= 1.70.0"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "= 6.69.1"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "= 5.10.0"
    }
  }

  encryption {
    key_provider "pbkdf2" "state" {
      passphrase = var.state_passphrase
    }
    method "aes_gcm" "state" {
      keys = key_provider.pbkdf2.state
    }
    state {
      method   = method.aes_gcm.state
      enforced = true
    }
    plan {
      method   = method.aes_gcm.state
      enforced = true
    }
  }
}

variable "state_passphrase" {
  description = "Passphrase of the state and plan encryption (at least 16 characters); set it in the environment as TF_VAR_state_passphrase."
  type        = string
  sensitive   = true
  ephemeral   = true
}

# The root configures the providers with the operator's credentials from the environment; the
# modules never receive them.
provider "hcloud" {}

provider "aws" {
  region = "eu-central-1"
}

provider "azurerm" {
  features {}
}

locals {
  shared = {
    environment        = "prod"
    bothy_version      = "1.2.3"
    image_registry     = "ghcr.io/alex"
    public_hostname    = "chat.example.com"
    release_repository = "alex/bothy"
    owner_handle       = "alex"
    owner_name         = "Alex"
    admin_keys         = ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGli5i6DCRTCblR1XFviXAN6md6ke0167D63NKgFFxHK alex.com"]
    ssh_allowed_cidrs  = ["198.51.100.0/24"]
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
  release_repository = local.shared.release_repository
  owner_handle       = local.shared.owner_handle
  owner_name         = local.shared.owner_name
}

module "hetzner" {
  source = "../../modules/hetzner"

  instance_name         = "hetzner"
  environment           = local.shared.environment
  bothy_version         = local.shared.bothy_version
  image_registry        = local.shared.image_registry
  public_hostname       = local.shared.public_hostname
  release_repository    = local.shared.release_repository
  owner_handle          = local.shared.owner_handle
  owner_name            = local.shared.owner_name
  admin_ssh_public_keys = local.shared.admin_keys
  ssh_allowed_cidrs     = local.shared.ssh_allowed_cidrs
  location              = "nbg1"
  server_type           = "cx22"
}

module "aws" {
  source = "../../modules/aws"

  instance_name      = "aws"
  environment        = local.shared.environment
  bothy_version      = local.shared.bothy_version
  image_registry     = local.shared.image_registry
  public_hostname    = local.shared.public_hostname
  release_repository = local.shared.release_repository
  owner_handle       = local.shared.owner_handle
  owner_name         = local.shared.owner_name
  ssh_allowed_cidrs  = local.shared.ssh_allowed_cidrs
  region             = "eu-central-1"
  instance_type      = "t4g.medium"
  offsite_region     = "eu-north-1"
}

module "azure" {
  source = "../../modules/azure"

  instance_name         = "azure"
  environment           = local.shared.environment
  bothy_version         = local.shared.bothy_version
  image_registry        = local.shared.image_registry
  public_hostname       = local.shared.public_hostname
  release_repository    = local.shared.release_repository
  owner_handle          = local.shared.owner_handle
  owner_name            = local.shared.owner_name
  admin_ssh_public_keys = local.shared.admin_keys
  ssh_allowed_cidrs     = local.shared.ssh_allowed_cidrs
  location              = "westeurope"
  vm_size               = "Standard_B2s"
}
