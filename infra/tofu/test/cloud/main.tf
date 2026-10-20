# Test root of the cloud modules hetzner, aws and azure (brief M7-16). It holds no resource: the
# test files beside it (`tofu test`, run by `make tofu-check`) call each module with mocked
# providers, so no plan or apply ever reaches a provider account and no credential is needed
# (docs/DECISIONS.md D-M7-16-1). The providers are required here so that `tofu init` installs
# them from the pinned offline mirror (tools/ci/tofu_providers.py) and checks them against the
# lock file of this root.

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
}
