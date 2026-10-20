# Tests of the module hetzner with a mocked hcloud provider (brief M7-16 requirements 13 to 16).
# `command = apply` against the mock creates nothing: every resource exists only in the memory
# of `tofu test`.

mock_provider "hcloud" {
  # The provider keeps the IDs of firewalls as numbers in the server; the mock needs one.
  mock_resource "hcloud_firewall" {
    defaults = {
      id = "1001"
    }
  }
  mock_resource "hcloud_server" {
    defaults = {
      ipv4_address = "192.0.2.30"
    }
  }
}

variables {
  instance_name         = "demo"
  environment           = "prod"
  bothy_version         = "1.2.3"
  image_registry        = "ghcr.io/alex"
  public_hostname       = "chat.example.com"
  release_repository    = "alex/bothy"
  owner_handle          = "alex"
  owner_name            = "Alex Example"
  admin_ssh_public_keys = ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGli5i6DCRTCblR1XFviXAN6md6ke0167D63NKgFFxHK alex.com"]
  location              = "nbg1"
  server_type           = "cx22"
}

run "edge_port_only_public" {
  command = apply

  module {
    source = "../../modules/hetzner"
  }

  assert {
    condition = alltrue([
      for r in hcloud_firewall.host.rule : r.port == "443" && r.protocol == "tcp"
      if r.direction == "in" && (contains(r.source_ips, "0.0.0.0/0") || contains(r.source_ips, "::/0"))
    ])
    error_message = "Only the edge port 443 may be open to the internet."
  }
  assert {
    condition     = length([for r in hcloud_firewall.host.rule : r if r.direction == "in"]) == 1
    error_message = "reverse_proxy without ssh_allowed_cidrs opens exactly the edge port."
  }
  assert {
    condition     = contains(hcloud_server.host.firewall_ids, tonumber(hcloud_firewall.host.id))
    error_message = "The server must sit behind the firewall."
  }
  assert {
    condition     = hcloud_server.host.user_data == module.host.user_data
    error_message = "The server must get the user data of generic-vm."
  }
  assert {
    condition     = length(module.host.user_data) <= 32768
    error_message = "Hetzner Cloud accepts at most 32 KiB of user data."
  }
  assert {
    condition     = output.public_url == "https://chat.example.com"
    error_message = "public_url is https://<public_hostname>."
  }
  assert {
    condition     = output.database_endpoint == "" && output.object_storage_bucket == "" && output.secret_store_ref == ""
    error_message = "hetzner runs PostgreSQL and the files in the stack and keeps the secrets on the host."
  }
  assert {
    condition     = output.ssh_endpoint == "bothy@192.0.2.30"
    error_message = "ssh_endpoint names the server's public address."
  }
}

run "no_public_port_without_reverse_proxy" {
  command = apply

  module {
    source = "../../modules/hetzner"
  }

  variables {
    edge_mode         = "tailscale"
    ssh_allowed_cidrs = ["198.51.100.0/24"]
  }

  assert {
    condition = alltrue([
      for r in hcloud_firewall.host.rule : !contains(r.source_ips, "0.0.0.0/0") && !contains(r.source_ips, "::/0")
    ])
    error_message = "tailscale opens nothing to the internet."
  }
  assert {
    condition = alltrue([
      for r in hcloud_firewall.host.rule : r.port == "22" && r.source_ips == toset(["198.51.100.0/24"])
      if r.direction == "in"
    ])
    error_message = "SSH is open only to ssh_allowed_cidrs."
  }
}

run "offsite_is_passed_to_the_host" {
  command = apply

  module {
    source = "../../modules/hetzner"
  }

  variables {
    backup_offsite = { type = "s3", location = "https://fsn1.your-objectstorage.com/bothy-demo-offsite" }
  }

  assert {
    condition     = output.backup_offsite_location == "s3:https://fsn1.your-objectstorage.com/bothy-demo-offsite"
    error_message = "The off-site bucket the owner created reaches install.sh --offsite."
  }
}

run "ssh_from_anywhere_is_refused" {
  command = plan

  module {
    source = "../../modules/hetzner"
  }

  variables {
    ssh_allowed_cidrs = ["0.0.0.0/0"]
  }

  expect_failures = [var.ssh_allowed_cidrs]
}

run "managed_database_is_refused" {
  command = plan

  module {
    source = "../../modules/hetzner"
  }

  variables {
    database_mode = "managed"
  }

  expect_failures = [var.database_mode]
}

run "external_object_storage_is_refused" {
  command = plan

  module {
    source = "../../modules/hetzner"
  }

  variables {
    object_storage_mode = "s3"
  }

  expect_failures = [var.object_storage_mode]
}

run "a_server_without_ssh_key_is_refused" {
  command = plan

  module {
    source = "../../modules/hetzner"
  }

  variables {
    admin_ssh_public_keys = []
  }

  expect_failures = [var.admin_ssh_public_keys]
}
