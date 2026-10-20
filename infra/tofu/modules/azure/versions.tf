# Provider and OpenTofu versions of the module azure. The provider is pinned to one exact version
# (infra/tofu/README.md rule 4); Renovate raises it together with the hashes in the lock files of
# the roots (infra/tofu/test/*/.terraform.lock.hcl), and tools/ci/tests/test_tofu_cloud_modules.py
# holds the pin and the lock files equal. The module configures no provider: the root does, with
# the credentials of the operator, never with an input of this module.
terraform {
  required_version = ">= 1.13.1, < 2.0.0"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "= 5.10.0"
    }
  }
}
