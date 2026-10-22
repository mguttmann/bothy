# The chat server on Hetzner Cloud (brief M7-16, ADR 0012, infra/tofu/README.md "Structure" item 1): a
# firewall that admits only the edge port from the internet, the SSH keys, and one cloud server
# whose cloud-init user data comes from generic-vm, which installs Docker, verifies the release
# and runs the setup wizard on the host. PostgreSQL and the files stay in the Compose stack:
# Hetzner Cloud has no managed PostgreSQL service, and install.sh has no option for an external
# S3 service (docs/DECISIONS.md D-M7-16-4). The off-site bucket with Object Lock is created by
# the owner in the Console and passed in backup_offsite (D-M7-16-5).

locals {
  name = "bothy-${var.instance_name}"
  labels = merge(var.tags, {
    "bothy-instance"    = var.instance_name
    "bothy-environment" = var.environment
    "managed-by"        = "opentofu"
  })

  # The inbound rules: the edge port for everyone with reverse_proxy, SSH only from the
  # networks of the operator. tailscale and private publish nothing (as the nftables table of
  # generic-vm, which stays the second layer on the host).
  edge_rules = var.edge_mode == "reverse_proxy" ? [{
    port        = "443"
    source_ips  = ["0.0.0.0/0", "::/0"]
    description = "edge (HTTPS)"
  }] : []
  ssh_rules = length(var.ssh_allowed_cidrs) == 0 ? [] : [{
    port        = "22"
    source_ips  = var.ssh_allowed_cidrs
    description = "SSH of the operator"
  }]
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
  backup_offsite        = var.backup_offsite
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
  ssh_host = hcloud_server.host.ipv4_address
}

resource "hcloud_firewall" "host" {
  name   = local.name
  labels = local.labels

  dynamic "rule" {
    for_each = concat(local.edge_rules, local.ssh_rules)
    content {
      direction   = "in"
      protocol    = "tcp"
      port        = rule.value.port
      source_ips  = rule.value.source_ips
      description = rule.value.description
    }
  }
}

# One key per entry of admin_ssh_public_keys, unless existing_ssh_keys names keys already in the
# project: Hetzner Cloud refuses a public key that the project holds once (uniqueness_error), so a
# second instance or a key uploaded in the Console uses the existing keys. With at least one key
# Hetzner Cloud sets no root password (and sends none by e-mail); cloud-init of generic-vm
# creates its maintenance user with the keys of admin_ssh_public_keys in both cases.
resource "hcloud_ssh_key" "admin" {
  count = length(var.existing_ssh_keys) == 0 ? length(var.admin_ssh_public_keys) : 0

  name       = "${local.name}-${count.index}"
  public_key = var.admin_ssh_public_keys[count.index]
  labels     = local.labels
}

resource "hcloud_server" "host" {
  name         = local.name
  server_type  = var.server_type
  image        = var.image
  location     = var.location
  ssh_keys     = length(var.existing_ssh_keys) == 0 ? hcloud_ssh_key.admin[*].id : var.existing_ssh_keys
  firewall_ids = [hcloud_firewall.host.id]
  user_data    = module.host.user_data
  labels       = local.labels

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }

  lifecycle {
    # The setup runs once at the first boot; a later change of the user data must not replace
    # the server with its data. A new setup goes over SSH (generic-vm provision_over_ssh).
    ignore_changes = [user_data, ssh_keys, image]
  }
}
