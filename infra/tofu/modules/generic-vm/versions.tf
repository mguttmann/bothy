# The module uses only the built-in features of OpenTofu (templatefile, yamlencode and the
# resource terraform_data with its file and remote-exec provisioners), so it needs no provider
# and `tofu init` needs no network (docs/DECISIONS.md D-M7-15-3). The OpenTofu version itself is
# pinned in tools/bin/tools.lock.json; this constraint names the oldest release it was checked
# with.
terraform {
  required_version = ">= 1.13.1, < 2.0.0"
}
